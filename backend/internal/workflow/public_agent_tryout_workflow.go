package workflow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

const executePublicAgentTryoutActivityName = "workflow.execute_public_agent_tryout"

type ExecutePublicAgentTryoutInput struct {
	TryoutID uuid.UUID `json:"tryout_id"`
}

func PublicAgentTryoutExecutionWorkflow(ctx sdkworkflow.Context, input PublicAgentTryoutExecutionWorkflowInput) error {
	ctx = sdkworkflow.WithActivityOptions(ctx, agentHarnessExecutionActivityOptions(defaultAgentHarnessTimeoutSeconds))
	return sdkworkflow.ExecuteActivity(ctx, executePublicAgentTryoutActivityName, ExecutePublicAgentTryoutInput{
		TryoutID: input.TryoutID,
	}).Get(ctx, nil)
}

func (a *Activities) ExecutePublicAgentTryout(ctx context.Context, input ExecutePublicAgentTryoutInput) error {
	if a.publicTryoutRepo == nil {
		return ErrAgentHarnessRepositoryMissing
	}

	started := time.Now().UTC()
	tryout, err := a.publicTryoutRepo.GetAgentTryoutByID(ctx, input.TryoutID)
	if err != nil {
		return wrapActivityError(err)
	}
	if tryout.WorkspaceID != nil {
		return wrapActivityError(fmt.Errorf("public tryout %s is workspace-owned", tryout.ID))
	}

	if _, err := a.publicTryoutRepo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:      tryout.ID,
		Status:  repository.AgentTryoutStatusRunning,
		Summary: publicTryoutSummary("running", "Hosted public tryout is running."),
	}); err != nil {
		return wrapActivityError(err)
	}
	if err := a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeSystemRunStarted, map[string]any{
		"status": "running",
	}); err != nil {
		return wrapActivityError(err)
	}

	outputs, err := a.executePublicTryoutSandbox(ctx, tryout)
	if err != nil {
		redaction := repository.AgentTryoutRedactionNotRequired
		_, _ = a.publicTryoutRepo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
			ID:              tryout.ID,
			Status:          repository.AgentTryoutStatusFailed,
			Summary:         publicTryoutSummary("failed", "Hosted public tryout failed. Please try another task or sign in to keep working."),
			LatencyMS:       int64Ptr(time.Since(started).Milliseconds()),
			RedactionStatus: &redaction,
		})
		_ = a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeSystemRunFailed, map[string]any{
			"status": "failed",
		})
		return wrapActivityError(err)
	}

	latencyMS := time.Since(started).Milliseconds()
	scorecard := publicTryoutScorecard(tryout.EvaluationSpecSnapshot, outputs, latencyMS)
	if err := a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeScoringCompleted, map[string]any{
		"passed": scorecard["passed_validators"],
		"total":  scorecard["total_validators"],
		"score":  scorecard["score"],
	}); err != nil {
		return wrapActivityError(err)
	}

	redaction := repository.AgentTryoutRedactionPassed
	if _, err := a.publicTryoutRepo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:              tryout.ID,
		Status:          repository.AgentTryoutStatusCompleted,
		Summary:         publicTryoutCompletedSummary(outputs, scorecard),
		LatencyMS:       int64Ptr(latencyMS),
		RedactionStatus: &redaction,
	}); err != nil {
		return wrapActivityError(err)
	}
	if err := a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeSystemRunCompleted, map[string]any{
		"status": "completed",
	}); err != nil {
		return wrapActivityError(err)
	}
	return nil
}

// Interactive session tuning. The sandbox is kept alive while the user chats;
// the loop ends on an explicit end turn, on idle, or at the hard session cap.
const (
	publicTryoutTurnPollInterval = 2 * time.Second
	publicTryoutIdleTimeout      = 4 * time.Minute
	publicTryoutPerTurnTimeout   = 5 * time.Minute
	// publicTryoutSessionCap bounds the whole interactive chat. It must stay
	// below the activity StartToCloseTimeout (defaultAgentHarnessTimeoutSeconds).
	publicTryoutSessionCap = 15 * time.Minute
)

// executePublicTryoutSandbox runs an interactive, multi-turn agent session: it
// keeps one sandbox alive, runs the opening turn, then drains user turns from
// agent_tryout_turns (resuming the same agent session each time) until the user
// ends the chat, it goes idle, or the hard cap is hit. The latest output
// artifacts are returned for the scorecard + summary.
func (a *Activities) executePublicTryoutSandbox(ctx context.Context, tryout repository.AgentTryout) ([]map[string]any, error) {
	config := NormalizePublicAgentTryoutConfig(a.publicTryoutConfig)
	harnessKind := publicTryoutHarnessKind(config, tryout.SelectedHarnessKind)
	credentialRef := publicTryoutCredentialRef(config, harnessKind)
	credential, err := provider.EnvCredentialResolver{}.Resolve(ctx, credentialRef)
	if err != nil {
		return nil, err
	}

	harness := publicTryoutHarnessSnapshot(config, tryout, harnessKind, credentialRef)
	env := publicTryoutRunnerEnv(harnessKind, harness, credential)

	sessionCap := publicTryoutSessionCap

	runAgentID := uuid.New()
	session, err := a.sandboxProvider.Create(ctx, sandbox.CreateRequest{
		RunID:      tryout.ID,
		RunAgentID: runAgentID,
		Timeout:    sessionCap,
		ToolPolicy: sandbox.ToolPolicy{
			AllowShell:   true,
			AllowNetwork: true,
		},
		Filesystem: sandbox.FilesystemSpec{
			WorkingDirectory: agentHarnessWorkspaceDir,
			ReadableRoots:    []string{agentHarnessWorkspaceDir},
			WritableRoots:    []string{agentHarnessWorkspaceDir},
		},
		Labels: map[string]string{
			"agentclash_kind":   "public_agent_tryout",
			"agent_tryout":      tryout.ID.String(),
			"agentclash_public": "true",
		},
		TemplateID: config.E2BTemplateID,
		EnvVars:    env,
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Destroy(context.Background()) }()

	deadline := time.Now().Add(sessionCap)
	var outputs []map[string]any

	// Opening turn: the task prompt assembled from the template + user input.
	if err := a.runPublicTryoutTurn(ctx, tryout, session, harnessKind, env, harness.TaskPrompt, true); err != nil {
		return nil, err
	}
	outputs = a.publicTryoutOutputPreviews(ctx, tryout.ID, session, harness.ExecutionConfig)

	// Conversational turns: drain user messages until end / idle / cap.
	lastActivity := time.Now()
	for time.Now().Before(deadline) {
		turn, ok, claimErr := a.publicTryoutRepo.ClaimNextPendingAgentTryoutTurn(ctx, tryout.ID)
		if claimErr != nil {
			return nil, claimErr
		}
		if !ok {
			if time.Since(lastActivity) > publicTryoutIdleTimeout {
				break
			}
			if !sleepWithContext(ctx, publicTryoutTurnPollInterval) {
				break
			}
			continue
		}
		if strings.TrimSpace(turn.Role) == "system_end" {
			_ = a.publicTryoutRepo.MarkAgentTryoutTurnProcessed(ctx, turn.ID)
			break
		}

		turnEnv := maputilClone(env)
		turnEnv["AGENTCLASH_TURN_MESSAGE"] = turn.Message
		if err := a.runPublicTryoutTurn(ctx, tryout, session, harnessKind, turnEnv, turn.Message, false); err != nil {
			_ = a.publicTryoutRepo.MarkAgentTryoutTurnProcessed(ctx, turn.ID)
			return nil, err
		}
		_ = a.publicTryoutRepo.MarkAgentTryoutTurnProcessed(ctx, turn.ID)
		outputs = a.publicTryoutOutputPreviews(ctx, tryout.ID, session, harness.ExecutionConfig)
		lastActivity = time.Now()
	}

	if err := a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeSystemOutputFinalized, map[string]any{
		"status": "finalized",
	}); err != nil {
		return nil, err
	}
	return outputs, nil
}

// runPublicTryoutTurn executes one agent turn in the live sandbox, streaming the
// agent's reasoning + tool calls into the tryout event log as it runs.
func (a *Activities) runPublicTryoutTurn(ctx context.Context, tryout repository.AgentTryout, session sandbox.Session, harnessKind string, env map[string]string, message string, firstTurn bool) error {
	command, err := publicTurnCommand(harnessKind, agentHarnessWorkspaceDir, message, firstTurn)
	if err != nil {
		return err
	}
	started := time.Now()
	_ = a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeSandboxCommandStarted, map[string]any{
		"harness_kind": harnessKind,
		"turn":         turnLabel(firstTurn),
	})

	remainder := ""
	lineCount := 0
	const maxStreamEvents = 200
	onStdout := func(chunk []byte) error {
		remainder += string(chunk)
		lines := strings.Split(remainder, "\n")
		remainder = lines[len(lines)-1]
		for _, line := range lines[:len(lines)-1] {
			if lineCount >= maxStreamEvents {
				return nil
			}
			if a.recordPublicAgentStreamLine(ctx, tryout.ID, harnessKind, line) {
				lineCount++
			}
		}
		return nil
	}

	result, execErr := session.Exec(ctx, sandbox.ExecRequest{
		Command:          command,
		WorkingDirectory: agentHarnessWorkspaceDir,
		Timeout:          publicTryoutPerTurnTimeout,
		Environment:      env,
		OnStdout:         onStdout,
	})
	if remainder != "" && lineCount < maxStreamEvents {
		a.recordPublicAgentStreamLine(ctx, tryout.ID, harnessKind, remainder)
	}

	durationMS := time.Since(started).Milliseconds()
	eventType := runevents.EventTypeSandboxCommandCompleted
	if execErr != nil || result.ExitCode != 0 {
		eventType = runevents.EventTypeSandboxCommandFailed
	}
	_ = a.recordPublicTryoutEvent(ctx, tryout.ID, eventType, map[string]any{
		"harness_kind": harnessKind,
		"exit_code":    result.ExitCode,
		"duration_ms":  durationMS,
	})
	if execErr != nil {
		return execErr
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("public tryout agent turn exited with code %d", result.ExitCode)
	}
	return nil
}

func turnLabel(firstTurn bool) string {
	if firstTurn {
		return "opening"
	}
	return "followup"
}

// publicTurnCommand returns the per-CLI command for a turn. The first turn opens
// the session; later turns resume it so conversation state carries across turns
// (verified live: codex `exec resume --last`, claude `--continue`, openclaw
// reuses --session-id).
func publicTurnCommand(harnessKind, workdir, message string, firstTurn bool) ([]string, error) {
	switch domain.NormalizeAgentHarnessKind(harnessKind) {
	case domain.AgentHarnessKindClaudeE2B:
		cmd := []string{"claude", "-p", "--output-format", "stream-json", "--verbose", "--permission-mode", "bypassPermissions"}
		if !firstTurn {
			cmd = append(cmd, "--continue")
		}
		return append(cmd, message), nil
	case domain.AgentHarnessKindOpenClawE2B:
		if firstTurn {
			script := strings.Join([]string{
				"set -euo pipefail",
				`if [ -n "${OPENAI_API_KEY:-}" ]; then AUTH_CHOICE=openai-api-key`,
				`elif [ -n "${ANTHROPIC_API_KEY:-}" ]; then AUTH_CHOICE=apiKey`,
				`elif [ -n "${OPENROUTER_API_KEY:-}" ]; then AUTH_CHOICE=openrouter-api-key`,
				`else echo "missing OpenClaw provider API key" >&2; exit 1; fi`,
				`openclaw setup --workspace "$PWD" --mode local --non-interactive --accept-risk`,
				`openclaw onboard --non-interactive --mode local --auth-choice "$AUTH_CHOICE" --secret-input-mode ref --accept-risk --skip-bootstrap --skip-health`,
				`exec openclaw agent --local --session-id agentclash-tryout --json --timeout "${AGENTCLASH_HARNESS_TIMEOUT_SECONDS:-1800}" -m "$AGENTCLASH_HARNESS_TASK"`,
			}, "\n")
			return []string{"bash", "-lc", script}, nil
		}
		script := `set -euo pipefail
exec openclaw agent --local --session-id agentclash-tryout --json --timeout "${AGENTCLASH_HARNESS_TIMEOUT_SECONDS:-1800}" -m "$AGENTCLASH_TURN_MESSAGE"`
		return []string{"bash", "-lc", script}, nil
	default: // codex_e2b
		// E2B already isolates the sandbox; Codex's own OS sandbox fails nested
		// (it can't set up landlock around /workspace), so bypass it. Verified
		// live: --full-auto cannot write files inside the E2B container.
		if !firstTurn {
			// `codex exec resume` restores the session's original cwd and does
			// NOT accept -C (passing it exits 2). Verified live.
			return []string{
				"codex", "exec", "resume", "--last",
				"--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check",
				"--json", message,
			}, nil
		}
		return []string{
			"codex", "exec",
			"--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check",
			"--json", "-C", workdir, message,
		}, nil
	}
}

// recordPublicAgentStreamLine condenses one line of the agent's JSON stream into
// a timeline event so the user sees the agent "thinking" live. Returns true when
// an event was recorded. Public payloads are redacted downstream on read.
func (a *Activities) recordPublicAgentStreamLine(ctx context.Context, tryoutID uuid.UUID, harnessKind, line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(line), &decoded); err != nil {
		// Plain (non-JSON) chatter is noisy; skip it.
		return false
	}
	// Determine the step kind. Codex nests the meaningful payload under "item"
	// (e.g. {"type":"item.completed","item":{"type":"agent_message","text":...}}
	// or item.type "file_change" / "command_execution"); claude stream-json and
	// openclaw use top-level "type". Tool-ish steps map to tool_call, the rest to
	// planning, so the chat shows "thinking" vs "acting".
	kind, _ := decoded["type"].(string)
	itemType := ""
	if item, ok := decoded["item"].(map[string]any); ok {
		itemType, _ = item["type"].(string)
	}
	summary, ok := streamLineSummary(decoded)
	if !ok {
		return false
	}
	lowered := strings.ToLower(kind + " " + itemType)
	eventType := runevents.EventTypeModelCallStarted
	if strings.Contains(lowered, "tool") ||
		strings.Contains(lowered, "file_change") ||
		strings.Contains(lowered, "command") {
		eventType = runevents.EventTypeToolCallStarted
	}
	streamType := itemType
	if streamType == "" {
		streamType = kind
	}
	_ = a.recordPublicTryoutEvent(ctx, tryoutID, eventType, map[string]any{
		"harness_kind": harnessKind,
		"stream_type":  streamType,
		"summary":      summary,
	})
	return true
}

// streamLineSummary extracts a short human-readable summary from one streamed
// JSON object, looking inside a nested "item" first (Codex), then top level.
// Returns ok=false for housekeeping frames with nothing worth showing.
func streamLineSummary(decoded map[string]any) (string, bool) {
	clip := func(value string) string {
		value = strings.TrimSpace(value)
		if len(value) > 240 {
			value = value[:240]
		}
		return value
	}
	if item, ok := decoded["item"].(map[string]any); ok {
		for _, key := range []string{"text", "message", "summary"} {
			if value, ok := item[key].(string); ok && strings.TrimSpace(value) != "" {
				return clip(value), true
			}
		}
		if changes, ok := item["changes"].([]any); ok && len(changes) > 0 {
			if first, ok := changes[0].(map[string]any); ok {
				if p, ok := first["path"].(string); ok && p != "" {
					return clip("Edited " + p), true
				}
			}
		}
		if itemType, ok := item["type"].(string); ok && strings.TrimSpace(itemType) != "" {
			return clip(strings.ReplaceAll(itemType, "_", " ")), true
		}
	}
	for _, key := range []string{"text", "message", "content", "summary", "name", "tool"} {
		if value, ok := decoded[key].(string); ok && strings.TrimSpace(value) != "" {
			return clip(value), true
		}
	}
	// Bare lifecycle frames (thread.started, turn.started) carry no content.
	return "", false
}

// sleepWithContext sleeps for d unless ctx is cancelled first; returns false if
// the context was cancelled.
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func maputilClone(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (a *Activities) publicTryoutOutputPreviews(ctx context.Context, tryoutID uuid.UUID, session sandbox.Session, executionConfig json.RawMessage) []map[string]any {
	specs := expectedArtifactsFromExecutionConfig(executionConfig)
	outputs := make([]map[string]any, 0, len(specs))
	for _, spec := range specs {
		rel := strings.TrimPrefix(path.Clean("/"+spec.Path), "/")
		if rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		data, err := session.DownloadFile(ctx, path.Join(agentHarnessWorkspaceDir, rel))
		if err != nil || len(data) == 0 {
			continue
		}
		contentType := capturedArtifactContentType(rel)
		entry := map[string]any{
			"key":           spec.Key,
			"type":          spec.Type,
			"relative_path": rel,
			"content_type":  contentType,
			"size_bytes":    len(data),
		}
		if publicTryoutOutputIsTextPreviewable(spec.Type, rel, contentType) {
			preview := data
			truncated := false
			if len(preview) > 32*1024 {
				preview = preview[:32*1024]
				truncated = true
			}
			entry["encoding"] = "utf-8"
			entry["preview"] = string(preview)
			entry["truncated"] = truncated
		} else {
			entry["encoding"] = "base64"
			entry["preview"] = base64.StdEncoding.EncodeToString(data)
			entry["truncated"] = false
		}
		outputs = append(outputs, entry)
		_ = a.recordPublicTryoutEvent(ctx, tryoutID, runevents.EventTypeSandboxFileWritten, map[string]any{
			"relative_path": rel,
			"file_path":     rel,
		})
	}
	return outputs
}

func publicTryoutOutputIsTextPreviewable(templateType, rel, contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(templateType)) {
	case "markdown", "json", "csv", "patch", "text":
		return true
	case "pdf", "pptx", "png", "jpg", "jpeg", "gif", "webp", "binary":
		return false
	}
	switch strings.ToLower(path.Ext(rel)) {
	case ".md", ".json", ".csv", ".patch", ".diff", ".txt", ".yaml", ".yml":
		return true
	}
	return strings.HasPrefix(contentType, "text/")
}

func publicTryoutHarnessSnapshot(config PublicAgentTryoutConfig, tryout repository.AgentTryout, harnessKind string, credentialRef string) agentHarnessSnapshot {
	prompt := publicTryoutTaskPrompt(tryout)
	secretName := publicTryoutSecretName(credentialRef)
	return agentHarnessSnapshot{
		ID:                     uuid.New(),
		WorkspaceID:            uuid.Nil,
		OrganizationID:         uuid.Nil,
		HarnessKind:            harnessKind,
		TaskPrompt:             prompt,
		CodexTemplate:          config.E2BTemplateID,
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &secretName,
		ExecutionConfig:        publicTryoutExecutionConfig(tryout),
		EvaluationConfig:       tryout.EvaluationSpecSnapshot,
	}
}

func publicTryoutTaskPrompt(tryout repository.AgentTryout) string {
	var template struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Runtime     json.RawMessage `json:"runtime"`
	}
	_ = json.Unmarshal(tryout.TemplateSnapshot, &template)
	name := strings.TrimSpace(template.Name)
	if name == "" {
		name = tryout.TemplateSlug
	}
	description := strings.TrimSpace(template.Description)
	if description == "" {
		description = "Complete the requested office-work task."
	}
	return strings.Join([]string{
		"You are running a public AgentClash tryout.",
		"Task: " + name + " (" + tryout.TemplateSlug + ")",
		"Goal: " + description,
		"Use the sandbox to produce concise, inspectable office-work outputs for the user.",
		"Do not reveal secrets or environment variables.",
		"User input JSON:",
		"```json",
		string(tryout.InputSnapshot),
		"```",
		"Runtime instructions JSON:",
		"```json",
		string(template.Runtime),
		"```",
	}, "\n")
}

func publicTryoutExecutionConfig(tryout repository.AgentTryout) json.RawMessage {
	var template struct {
		Runtime json.RawMessage `json:"runtime"`
	}
	_ = json.Unmarshal(tryout.TemplateSnapshot, &template)
	payload, _ := json.Marshal(map[string]any{
		"timeout_seconds": tryout.MaxDurationSeconds,
		"agent_tryout": map[string]any{
			"template_slug": tryout.TemplateSlug,
			"runtime":       json.RawMessage(template.Runtime),
			"tool_policy":   json.RawMessage(tryout.ToolPolicySnapshot),
		},
	})
	return payload
}

func publicTryoutRunnerEnv(harnessKind string, harness agentHarnessSnapshot, credential string) map[string]string {
	env := map[string]string{}
	secretName := publicTryoutSecretName(derefString(harness.OpenAIAPIKeySecretName))
	switch domain.NormalizeAgentHarnessKind(harnessKind) {
	case domain.AgentHarnessKindClaudeE2B:
		env["ANTHROPIC_API_KEY"] = credential
	case domain.AgentHarnessKindOpenClawE2B:
		applyOpenClawSecretEnv(env, secretName, credential)
		applyOpenClawRunnerEnv(env, harness, defaultAgentHarnessTimeoutSeconds*time.Second)
	case domain.AgentHarnessKindHermesE2B:
		applyHermesSecretEnv(env, secretName, credential)
		applyHermesRunnerEnv(env, harness)
	default:
		env["OPENAI_API_KEY"] = credential
		env["CODEX_API_KEY"] = credential
	}
	return env
}

func publicTryoutSecretName(credentialRef string) string {
	switch {
	case strings.HasPrefix(credentialRef, "env://"):
		return strings.TrimPrefix(credentialRef, "env://")
	case strings.HasPrefix(credentialRef, "secret://"):
		return strings.TrimPrefix(credentialRef, "secret://")
	default:
		return "OPENAI_API_KEY"
	}
}

func publicTryoutSummary(code string, message string) json.RawMessage {
	payload, _ := json.Marshal(map[string]string{"code": code, "message": message})
	return payload
}

func publicTryoutCompletedSummary(outputs []map[string]any, scorecard map[string]any) json.RawMessage {
	payload, _ := json.Marshal(map[string]any{
		"code":      "completed",
		"message":   "The hosted agent finished this public tryout. Export the trace or sign in to save and rerun it.",
		"outputs":   outputs,
		"scorecard": scorecard,
	})
	return payload
}

// publicTryoutScorecard evaluates a template's lightweight evaluation spec
// (json_field / json_schema / artifact_produced validators) against the
// produced output previews and returns a self-contained scorecard.
func publicTryoutScorecard(evaluationSpec json.RawMessage, outputs []map[string]any, latencyMS int64) map[string]any {
	var spec struct {
		Validators []struct {
			Key         string `json:"key"`
			Type        string `json:"type"`
			Field       string `json:"field"`
			ArtifactKey string `json:"artifact_key"`
		} `json:"validators"`
		Scorecard struct {
			Dimensions []string `json:"dimensions"`
		} `json:"scorecard"`
	}
	_ = json.Unmarshal(evaluationSpec, &spec)

	// Decode every JSON output preview once so field checks can scan them all.
	decoded := make([]map[string]any, 0, len(outputs))
	for _, out := range outputs {
		if t, _ := out["type"].(string); t != "json" {
			continue
		}
		preview, _ := out["preview"].(string)
		var obj map[string]any
		if json.Unmarshal([]byte(preview), &obj) == nil {
			decoded = append(decoded, obj)
		}
	}

	checks := make([]map[string]any, 0, len(spec.Validators))
	passed := 0
	for _, v := range spec.Validators {
		ok := false
		switch strings.TrimSpace(v.Type) {
		case "json_schema":
			// Passes when at least one produced output is valid JSON.
			ok = len(decoded) > 0
		case "json_field":
			for _, obj := range decoded {
				if value, exists := obj[v.Field]; exists && !isEmptyScorecardValue(value) {
					ok = true
					break
				}
			}
		case "artifact_produced":
			target := strings.TrimSpace(v.ArtifactKey)
			for _, out := range outputs {
				key, _ := out["key"].(string)
				if key != target {
					continue
				}
				if size, okSize := out["size_bytes"].(int); okSize && size > 0 {
					ok = true
					break
				}
				if preview, okPreview := out["preview"].(string); okPreview && strings.TrimSpace(preview) != "" {
					ok = true
					break
				}
			}
		default:
			// Unknown validator types are reported but do not count for/against.
			checks = append(checks, map[string]any{"key": v.Key, "type": v.Type, "status": "skipped"})
			continue
		}
		if ok {
			passed++
		}
		status := "failed"
		if ok {
			status = "passed"
		}
		checks = append(checks, map[string]any{"key": v.Key, "type": v.Type, "status": status})
	}

	total := 0
	for _, c := range checks {
		if c["status"] != "skipped" {
			total++
		}
	}
	score := 0.0
	if total > 0 {
		score = float64(passed) / float64(total)
	}
	return map[string]any{
		"passed_validators": passed,
		"total_validators":  total,
		"score":             score,
		"passed":            total > 0 && passed == total,
		"dimensions":        spec.Scorecard.Dimensions,
		"checks":            checks,
		"latency_ms":        latencyMS,
		"outputs_count":     len(outputs),
	}
}

func isEmptyScorecardValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}

func (a *Activities) recordPublicTryoutEvent(ctx context.Context, tryoutID uuid.UUID, eventType runevents.Type, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = a.publicTryoutRepo.RecordAgentTryoutEvent(ctx, repository.RecordAgentTryoutEventParams{
		AgentTryoutID: tryoutID,
		EventType:     string(eventType),
		ActorType:     "worker",
		Payload:       encoded,
	})
	return err
}

func int64Ptr(value int64) *int64 {
	return &value
}
