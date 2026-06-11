package workflow

import (
	"context"
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

	redaction := repository.AgentTryoutRedactionPassed
	if _, err := a.publicTryoutRepo.UpdateAgentTryoutStatus(ctx, repository.UpdateAgentTryoutStatusParams{
		ID:              tryout.ID,
		Status:          repository.AgentTryoutStatusCompleted,
		Summary:         publicTryoutCompletedSummary(outputs),
		LatencyMS:       int64Ptr(time.Since(started).Milliseconds()),
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
	timeout := time.Duration(tryout.MaxDurationSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultAgentHarnessTimeoutSeconds * time.Second
	}

	runAgentID := uuid.New()
	session, err := a.sandboxProvider.Create(ctx, sandbox.CreateRequest{
		RunID:      tryout.ID,
		RunAgentID: runAgentID,
		Timeout:    timeout,
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

	runner, err := agentHarnessRunnerFor(harness, agentHarnessWorkspaceDir)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	if err := a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeSandboxCommandStarted, map[string]any{
		"provider_key":   config.Provider,
		"harness_kind":   harnessKind,
		"sandbox_action": runner.DisplayName,
	}); err != nil {
		return nil, err
	}
	result, err := session.Exec(ctx, sandbox.ExecRequest{
		Command:          runner.Command,
		WorkingDirectory: agentHarnessWorkspaceDir,
		Timeout:          timeout,
		Environment:      env,
	})
	durationMS := time.Since(started).Milliseconds()
	eventType := runevents.EventTypeSandboxCommandCompleted
	if err != nil || result.ExitCode != 0 {
		eventType = runevents.EventTypeSandboxCommandFailed
	}
	if recordErr := a.recordPublicTryoutEvent(ctx, tryout.ID, eventType, map[string]any{
		"provider_key":   config.Provider,
		"harness_kind":   harnessKind,
		"sandbox_action": runner.DisplayName,
		"exit_code":      result.ExitCode,
		"duration_ms":    durationMS,
	}); recordErr != nil {
		return nil, recordErr
	}
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("public tryout runner exited with code %d", result.ExitCode)
	}
	outputs := a.publicTryoutOutputPreviews(ctx, tryout.ID, session, harness.ExecutionConfig)
	if err := a.recordPublicTryoutEvent(ctx, tryout.ID, runevents.EventTypeSystemOutputFinalized, map[string]any{
		"status":      "finalized",
		"duration_ms": durationMS,
	}); err != nil {
		return nil, err
	}
	return outputs, nil
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
		if len(data) > 32*1024 {
			data = data[:32*1024]
		}
		outputs = append(outputs, map[string]any{
			"key":           spec.Key,
			"type":          spec.Type,
			"relative_path": rel,
			"preview":       string(data),
			"truncated":     len(data) == 32*1024,
		})
		_ = a.recordPublicTryoutEvent(ctx, tryoutID, runevents.EventTypeSandboxFileWritten, map[string]any{
			"relative_path": rel,
			"file_path":     rel,
		})
	}
	return outputs
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

func publicTryoutCompletedSummary(outputs []map[string]any) json.RawMessage {
	payload, _ := json.Marshal(map[string]any{
		"code":    "completed",
		"message": "The hosted agent finished this public tryout. Export the trace or sign in to save and rerun it.",
		"outputs": outputs,
	})
	return payload
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
