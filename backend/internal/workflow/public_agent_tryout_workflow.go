package workflow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/agentclash/agentclash/backend/internal/scoring"
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
	if judgeSection := a.runPublicTryoutJudges(ctx, tryout, outputs); judgeSection != nil {
		scorecard["judge"] = judgeSection
	}
	scoringPayload := map[string]any{
		"passed": scorecard["passed_validators"],
		"total":  scorecard["total_validators"],
		"score":  scorecard["score"],
	}
	if judgeSection, ok := scorecard["judge"].(map[string]any); ok {
		if verdict, ok := judgeSection["verdict"].(string); ok {
			scoringPayload["verdict"] = verdict
		}
		if model, ok := judgeSection["model"].(string); ok {
			scoringPayload["provider_model_id"] = model
		}
	}
	if err := a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeScoringCompleted, scoringPayload); err != nil {
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
	selectedModel := publicTryoutSelectedModel(tryout.SelectedModelPolicy)
	if selectedModel != "" {
		env["AGENTCLASH_SELECTED_MODEL"] = selectedModel
	}

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

	if err := a.stagePublicTryoutInputAttachments(ctx, session, tryout.InputSnapshot); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(sessionCap)
	var outputs []map[string]any

	// Opening turn: the task prompt assembled from the template + user input.
	if err := a.runPublicTryoutTurn(ctx, tryout, session, harnessKind, env, harness.TaskPrompt, true); err != nil {
		return nil, err
	}
	outputs = a.publicTryoutOutputPreviews(ctx, tryout.ID, session, harness.ExecutionConfig)
	if len(outputs) > 0 {
		if err := a.updatePublicTryoutOutputSnapshot(ctx, tryout, outputs, harnessKind, selectedModel); err != nil {
			return nil, err
		}
	}

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
		if len(outputs) > 0 {
			if err := a.updatePublicTryoutOutputSnapshot(ctx, tryout, outputs, harnessKind, selectedModel); err != nil {
				return nil, err
			}
		}
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
	command, err := publicTurnCommand(harnessKind, agentHarnessWorkspaceDir, message, firstTurn, env["AGENTCLASH_SELECTED_MODEL"])
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
func publicTurnCommand(harnessKind, workdir, message string, firstTurn bool, selectedModel string) ([]string, error) {
	selectedModel = strings.TrimSpace(selectedModel)
	switch domain.NormalizeAgentHarnessKind(harnessKind) {
	case domain.AgentHarnessKindClaudeE2B:
		cmd := []string{"claude", "-p", "--output-format", "stream-json", "--verbose", "--permission-mode", "bypassPermissions"}
		if selectedModel != "" {
			cmd = append(cmd, "--model", selectedModel)
		}
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
				`MODEL_ARGS=()`,
				`if [ -n "${AGENTCLASH_SELECTED_MODEL:-}" ]; then MODEL_ARGS=(--model "$AGENTCLASH_SELECTED_MODEL"); fi`,
				`openclaw setup --workspace "$PWD" --mode local --non-interactive --accept-risk`,
				`openclaw onboard --non-interactive --mode local --auth-choice "$AUTH_CHOICE" --secret-input-mode ref --accept-risk --skip-bootstrap --skip-health`,
				`exec openclaw agent --local --session-id agentclash-tryout --json --timeout "${AGENTCLASH_HARNESS_TIMEOUT_SECONDS:-1800}" "${MODEL_ARGS[@]}" -m "$AGENTCLASH_HARNESS_TASK"`,
			}, "\n")
			return []string{"bash", "-lc", script}, nil
		}
		script := `set -euo pipefail
MODEL_ARGS=()
if [ -n "${AGENTCLASH_SELECTED_MODEL:-}" ]; then MODEL_ARGS=(--model "$AGENTCLASH_SELECTED_MODEL"); fi
exec openclaw agent --local --session-id agentclash-tryout --json --timeout "${AGENTCLASH_HARNESS_TIMEOUT_SECONDS:-1800}" "${MODEL_ARGS[@]}" -m "$AGENTCLASH_TURN_MESSAGE"`
		return []string{"bash", "-lc", script}, nil
	default: // codex_e2b
		// E2B already isolates the sandbox; Codex's own OS sandbox fails nested
		// (it can't set up landlock around /workspace), so bypass it. Verified
		// live: --full-auto cannot write files inside the E2B container.
		if !firstTurn {
			// `codex exec resume` restores the session's original cwd and does
			// NOT accept -C (passing it exits 2). Verified live.
			cmd := []string{
				"codex", "exec", "resume", "--last",
				"--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check",
				"--json",
			}
			if selectedModel != "" {
				cmd = append(cmd, "--model", selectedModel)
			}
			return append(cmd, message), nil
		}
		cmd := []string{
			"codex", "exec",
			"--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check",
			"--json",
		}
		if selectedModel != "" {
			cmd = append(cmd, "--model", selectedModel)
		}
		return append(cmd, "-C", workdir, message), nil
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

func (a *Activities) updatePublicTryoutOutputSnapshot(ctx context.Context, tryout repository.AgentTryout, outputs []map[string]any, harnessKind string, selectedModel string) error {
	_, err := a.publicTryoutRepo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:      tryout.ID,
		Status:  repository.AgentTryoutStatusRunning,
		Summary: publicTryoutRunningSummary(outputs, harnessKind, selectedModel),
	})
	return err
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

func publicTryoutSelectedModel(policy json.RawMessage) string {
	var parsed struct {
		Models []struct {
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.Unmarshal(policy, &parsed); err != nil {
		return ""
	}
	if len(parsed.Models) == 0 {
		return ""
	}
	return strings.TrimSpace(parsed.Models[0].Model)
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
	lines := []string{
		"You are running a public AgentClash tryout.",
		"Task: " + name + " (" + tryout.TemplateSlug + ")",
		"Goal: " + description,
		"Use the sandbox to produce concise, inspectable office-work outputs for the user.",
		"Do not reveal secrets or environment variables.",
		"User input JSON:",
		"```json",
		string(tryout.InputSnapshot),
		"```",
	}
	lines = append(lines, publicTryoutEvalSetupPrompt(tryout.InputSnapshot)...)
	lines = append(lines, publicTryoutInputAttachmentsPrompt(tryout.InputSnapshot)...)
	lines = append(lines,
		"Runtime instructions JSON:",
		"```json",
		string(template.Runtime),
		"```",
	)
	return strings.Join(lines, "\n")
}

func publicTryoutEvalSetupPrompt(input json.RawMessage) []string {
	var object map[string]any
	if err := json.Unmarshal(input, &object); err != nil {
		return nil
	}
	value, ok := object["eval_setup"]
	if !ok || value == nil {
		return nil
	}
	setup, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	normalized, err := json.MarshalIndent(setup, "", "  ")
	if err != nil {
		return nil
	}
	return []string{
		"Business eval setup:",
		"Treat this as the user's acceptance criteria. Optimize for the rubric, avoid the named failure modes, and make outputs easy for a human evaluator to inspect.",
		"```json",
		string(normalized),
		"```",
	}
}

func publicTryoutInputAttachmentsPrompt(input json.RawMessage) []string {
	attachments := publicTryoutResolvedInputAttachments(input)
	if len(attachments) == 0 {
		return nil
	}
	lines := []string{
		"User-provided input files are staged in the sandbox workspace. Read and use them as source material for this task.",
	}
	for _, attachment := range attachments {
		filename, _ := attachment["filename"].(string)
		workspacePath, _ := attachment["workspace_path"].(string)
		mediaType, _ := attachment["media_type"].(string)
		if strings.TrimSpace(workspacePath) == "" {
			continue
		}
		label := strings.TrimSpace(filename)
		if label == "" {
			label = workspacePath
		}
		typeLabel := strings.TrimSpace(mediaType)
		if typeLabel != "" {
			label += " (" + typeLabel + ")"
		}
		lines = append(lines, fmt.Sprintf("- %s at /workspace/%s", label, strings.TrimPrefix(workspacePath, "/")))
	}
	return lines
}

func publicTryoutResolvedInputAttachments(input json.RawMessage) []map[string]any {
	var object map[string]any
	if err := json.Unmarshal(input, &object); err != nil {
		return nil
	}
	value, ok := object["input_attachments"]
	if !ok || value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		attachment, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringFromAny(attachment["storage_key"])) == "" {
			continue
		}
		out = append(out, attachment)
	}
	return out
}

func stringFromAny(value any) string {
	raw, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(raw)
}

func (a *Activities) stagePublicTryoutInputAttachments(
	ctx context.Context,
	session sandbox.Session,
	inputSnapshot json.RawMessage,
) error {
	attachments := publicTryoutResolvedInputAttachments(inputSnapshot)
	if len(attachments) == 0 {
		return nil
	}
	if a.artifactStore == nil {
		return fmt.Errorf("artifact store is not configured for tryout input attachments")
	}
	for _, attachment := range attachments {
		storageKey := stringFromAny(attachment["storage_key"])
		workspacePath := stringFromAny(attachment["workspace_path"])
		if storageKey == "" || workspacePath == "" {
			return fmt.Errorf("tryout input attachment is missing staging metadata")
		}
		reader, _, err := a.artifactStore.OpenObject(ctx, storageKey)
		if err != nil {
			return fmt.Errorf("open tryout input attachment %q: %w", storageKey, err)
		}
		content, err := io.ReadAll(io.LimitReader(reader, defaultAgentTryoutInputAttachmentMaxBytes+1))
		closeErr := reader.Close()
		if err != nil {
			return fmt.Errorf("read tryout input attachment %q: %w", storageKey, err)
		}
		if closeErr != nil {
			return fmt.Errorf("close tryout input attachment %q: %w", storageKey, closeErr)
		}
		if int64(len(content)) > defaultAgentTryoutInputAttachmentMaxBytes {
			return fmt.Errorf("tryout input attachment %q exceeds maximum size", storageKey)
		}
		targetPath := path.Join(agentHarnessWorkspaceDir, workspacePath)
		if err := session.UploadFile(ctx, targetPath, content); err != nil {
			return fmt.Errorf("stage tryout input attachment to %s: %w", targetPath, err)
		}
	}
	return nil
}

const defaultAgentTryoutInputAttachmentMaxBytes = 15 << 20

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

func publicTryoutRunningSummary(outputs []map[string]any, harnessKind string, selectedModel string) json.RawMessage {
	payload, _ := json.Marshal(map[string]any{
		"code":                  "outputs_ready",
		"message":               "The hosted agent produced downloadable outputs. You can keep chatting to request edits, or end the session to finalize scoring.",
		"outputs":               outputs,
		"selected_harness_kind": strings.TrimSpace(harnessKind),
		"selected_model":        strings.TrimSpace(selectedModel),
	})
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

// Judge-output digest bounds. Output previews are truncated before they are
// handed to the LLM judge so anonymous tryouts cannot run up platform-key
// spend with giant artifacts.
const (
	publicJudgeMaxBytesPerOutput = 6_000
	publicJudgeMaxDigestBytes    = 18_000
)

// runPublicTryoutJudges executes the llm_judges baked into the tryout's
// evaluation spec snapshot against the produced outputs, reusing the same
// judge machinery as harness runs (evaluateLLMJudges). Anonymous tryouts carry
// no deployment context, so credential resolution always lands on platform env
// keys. Returns nil when no judges are configured; judge failures degrade to a
// section with skipped criteria instead of failing the tryout.
func (a *Activities) runPublicTryoutJudges(ctx context.Context, tryout repository.AgentTryout, outputs []map[string]any) map[string]any {
	var spec struct {
		LLMJudges []json.RawMessage `json:"llm_judges"`
		JudgeMeta struct {
			Model      string            `json:"model"`
			Strictness string            `json:"strictness"`
			Labels     map[string]string `json:"labels"`
		} `json:"judge_meta"`
	}
	if err := json.Unmarshal(tryout.EvaluationSpecSnapshot, &spec); err != nil || len(spec.LLMJudges) == 0 {
		return nil
	}
	judges, err := agentHarnessLLMJudges(spec.LLMJudges)
	if err != nil || len(judges) == 0 {
		return nil
	}

	judgeModel := strings.TrimSpace(spec.JudgeMeta.Model)
	if judgeModel == "" {
		judgeModel = judges[0].Model
	}
	_ = a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeScoringStarted, map[string]any{
		"status":            "judging",
		"provider_model_id": judgeModel,
	})

	digest := publicTryoutJudgeDigest(outputs)
	if digest == "" {
		_ = a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeScoringFailed, map[string]any{
			"status": "no_output_to_judge",
		})
		return map[string]any{
			"model":      judgeModel,
			"strictness": spec.JudgeMeta.Strictness,
			"verdict":    "not_judged",
			"reason":     "the run produced no output the judge could grade",
		}
	}

	input := scoring.EvaluationInput{
		Events: []scoring.Event{{
			Type:       string(runevents.EventTypeSystemOutputFinalized),
			Source:     "worker",
			OccurredAt: time.Now().UTC(),
			Payload:    mustMarshalJSON(map[string]any{"final_output": digest}),
		}},
	}
	results, warnings := evaluateLLMJudges(ctx, a.judgeClient, a.repo, repository.RunAgentExecutionContext{}, input, scoring.EvaluationSpec{LLMJudges: judges})

	criteria := make([]map[string]any, 0, len(results))
	var overallScore *float64
	anyScored := false
	instantFailHit := false
	for _, result := range results {
		entry := map[string]any{
			"key":   result.JudgeKey,
			"label": publicJudgeLabel(spec.JudgeMeta.Labels, result.JudgeKey),
			"mode":  result.Mode,
		}
		if result.NormalizedScore == nil {
			entry["status"] = "skipped"
			if result.Reason != "" {
				entry["reason"] = result.Reason
			}
		} else {
			anyScored = true
			score := *result.NormalizedScore
			entry["score"] = score
			if score >= 0.5 {
				entry["status"] = "passed"
			} else {
				entry["status"] = "failed"
			}
			if result.JudgeKey == "overall_quality" {
				overallScore = result.NormalizedScore
			} else if score < 0.5 && result.JudgeKey == "instant_fail" {
				instantFailHit = true
			}
		}
		if result.Confidence != nil {
			entry["confidence"] = *result.Confidence
		}
		if reasoning := publicJudgeReasoning(result.Payload); reasoning != "" {
			entry["reasoning"] = reasoning
		}
		criteria = append(criteria, entry)
	}

	section := map[string]any{
		"model":      judgeModel,
		"strictness": spec.JudgeMeta.Strictness,
		"criteria":   criteria,
	}
	if len(warnings) > 0 {
		section["warnings"] = warnings
	}
	if !anyScored {
		section["verdict"] = "not_judged"
		section["reason"] = "the judge could not grade this run"
		return section
	}
	if overallScore != nil {
		section["score"] = *overallScore
	}
	switch {
	case instantFailHit:
		section["verdict"] = "rejected"
	case overallScore != nil && *overallScore >= 0.75:
		section["verdict"] = "approved"
	case overallScore != nil && *overallScore >= 0.4:
		section["verdict"] = "needs_edits"
	case overallScore != nil:
		section["verdict"] = "rejected"
	default:
		section["verdict"] = "needs_edits"
	}
	return section
}

// publicTryoutJudgeDigest flattens output previews into one bounded text block
// the judge can grade. Binary previews (base64) are summarized, not inlined.
func publicTryoutJudgeDigest(outputs []map[string]any) string {
	sections := make([]string, 0, len(outputs))
	total := 0
	for _, output := range outputs {
		preview, _ := output["preview"].(string)
		name, _ := output["relative_path"].(string)
		if name == "" {
			name, _ = output["key"].(string)
		}
		encoding, _ := output["encoding"].(string)
		if strings.TrimSpace(preview) == "" {
			continue
		}
		if encoding == "base64" {
			kind, _ := output["type"].(string)
			sections = append(sections, fmt.Sprintf("=== %s ===\n(binary %s artifact was produced; grade based on the other outputs)", name, kind))
			continue
		}
		if len(preview) > publicJudgeMaxBytesPerOutput {
			runes := []rune(preview)
			if len(runes) > publicJudgeMaxBytesPerOutput {
				preview = string(runes[:publicJudgeMaxBytesPerOutput]) + "\n…(truncated)"
			}
		}
		section := fmt.Sprintf("=== %s ===\n%s", name, preview)
		if total+len(section) > publicJudgeMaxDigestBytes {
			break
		}
		total += len(section)
		sections = append(sections, section)
	}
	return strings.Join(sections, "\n\n")
}

func publicJudgeLabel(labels map[string]string, key string) string {
	if label, ok := labels[key]; ok && strings.TrimSpace(label) != "" {
		return label
	}
	spaced := strings.ReplaceAll(key, "_", " ")
	if spaced == "" {
		return "Check"
	}
	return strings.ToUpper(spaced[:1]) + spaced[1:]
}

// publicJudgeReasoning extracts the judge's own "reasoning" text from the
// first successful call recorded in an LLMJudgeResult payload.
func publicJudgeReasoning(payload json.RawMessage) string {
	var decoded struct {
		Calls []struct {
			Error        string `json:"error"`
			ResponseText string `json:"response_text"`
		} `json:"calls"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return ""
	}
	for _, call := range decoded.Calls {
		if call.Error != "" || strings.TrimSpace(call.ResponseText) == "" {
			continue
		}
		var response struct {
			Reasoning string `json:"reasoning"`
		}
		if err := json.Unmarshal([]byte(sanitizeJudgeJSON(call.ResponseText)), &response); err != nil {
			continue
		}
		if reasoning := strings.TrimSpace(response.Reasoning); reasoning != "" {
			return reasoning
		}
	}
	return ""
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
