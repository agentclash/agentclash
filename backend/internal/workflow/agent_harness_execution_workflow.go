package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

const agentHarnessWorkspaceDir = "/workspace"

var (
	ErrAgentHarnessRepositoryMissing = errors.New("agent harness repository is not configured")
	ErrAgentHarnessSecretMissing     = errors.New("agent harness secret is missing")
)

type TransitionAgentHarnessExecutionStatusInput struct {
	ExecutionID uuid.UUID                              `json:"execution_id"`
	ToStatus    repository.AgentHarnessExecutionStatus `json:"to_status"`
	Reason      *string                                `json:"reason,omitempty"`
}

type ExecuteAgentHarnessExecutionInput struct {
	ExecutionID uuid.UUID `json:"execution_id"`
}

type agentHarnessExecutionConfig struct {
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

type agentHarnessSnapshot struct {
	ID                     uuid.UUID       `json:"id"`
	WorkspaceID            uuid.UUID       `json:"workspace_id"`
	OrganizationID         uuid.UUID       `json:"organization_id"`
	TaskPrompt             string          `json:"task_prompt"`
	CodexTemplate          string          `json:"codex_template"`
	CodexModel             *string         `json:"codex_model,omitempty"`
	AuthMode               string          `json:"auth_mode"`
	OpenAIAPIKeySecretName *string         `json:"openai_api_key_secret_name,omitempty"`
	RepositoryURL          *string         `json:"repository_url,omitempty"`
	BaseBranch             *string         `json:"base_branch,omitempty"`
	ExecutionConfig        json.RawMessage `json:"execution_config,omitempty"`
	EvaluationConfig       json.RawMessage `json:"evaluation_config,omitempty"`
}

func AgentHarnessExecutionWorkflow(ctx sdkworkflow.Context, input AgentHarnessExecutionWorkflowInput) error {
	ctx = sdkworkflow.WithActivityOptions(ctx, defaultActivityOptions)

	if err := transitionAgentHarnessExecutionStatus(ctx, input.ExecutionID, repository.AgentHarnessExecutionStatusProvisioning, nil); err != nil {
		return err
	}
	if err := transitionAgentHarnessExecutionStatus(ctx, input.ExecutionID, repository.AgentHarnessExecutionStatusRunning, nil); err != nil {
		return err
	}
	if err := executeAgentHarnessExecution(ctx, input.ExecutionID); err != nil {
		return markAgentHarnessExecutionFailed(ctx, input.ExecutionID, err)
	}
	if err := transitionAgentHarnessExecutionStatus(ctx, input.ExecutionID, repository.AgentHarnessExecutionStatusScoring, nil); err != nil {
		return markAgentHarnessExecutionFailed(ctx, input.ExecutionID, err)
	}
	if err := transitionAgentHarnessExecutionStatus(ctx, input.ExecutionID, repository.AgentHarnessExecutionStatusCompleted, nil); err != nil {
		return markAgentHarnessExecutionFailed(ctx, input.ExecutionID, err)
	}
	return nil
}

func transitionAgentHarnessExecutionStatus(ctx sdkworkflow.Context, executionID uuid.UUID, status repository.AgentHarnessExecutionStatus, reason *string) error {
	var execution repository.AgentHarnessExecution
	return sdkworkflow.ExecuteActivity(ctx, transitionAgentHarnessExecutionStatusActivityName, TransitionAgentHarnessExecutionStatusInput{
		ExecutionID: executionID,
		ToStatus:    status,
		Reason:      reason,
	}).Get(ctx, &execution)
}

func executeAgentHarnessExecution(ctx sdkworkflow.Context, executionID uuid.UUID) error {
	return sdkworkflow.ExecuteActivity(ctx, executeAgentHarnessExecutionActivityName, ExecuteAgentHarnessExecutionInput{
		ExecutionID: executionID,
	}).Get(ctx, nil)
}

func markAgentHarnessExecutionFailed(ctx sdkworkflow.Context, executionID uuid.UUID, cause error) error {
	disconnectedCtx, _ := sdkworkflow.NewDisconnectedContext(ctx)
	disconnectedCtx = sdkworkflow.WithActivityOptions(disconnectedCtx, defaultActivityOptions)
	reason := cause.Error()
	_ = transitionAgentHarnessExecutionStatus(disconnectedCtx, executionID, repository.AgentHarnessExecutionStatusFailed, &reason)
	return cause
}

func (a *Activities) TransitionAgentHarnessExecutionStatus(ctx context.Context, input TransitionAgentHarnessExecutionStatusInput) (repository.AgentHarnessExecution, error) {
	if a.agentHarnessRepo == nil {
		return repository.AgentHarnessExecution{}, ErrAgentHarnessRepositoryMissing
	}
	execution, err := a.agentHarnessRepo.TransitionAgentHarnessExecutionStatus(ctx, repository.TransitionAgentHarnessExecutionStatusParams{
		ExecutionID: input.ExecutionID,
		ToStatus:    input.ToStatus,
		Reason:      input.Reason,
	})
	return execution, wrapActivityError(err)
}

func (a *Activities) ExecuteAgentHarnessExecution(ctx context.Context, input ExecuteAgentHarnessExecutionInput) error {
	if a.agentHarnessRepo == nil {
		return ErrAgentHarnessRepositoryMissing
	}

	execution, err := a.agentHarnessRepo.GetAgentHarnessExecutionByID(ctx, input.ExecutionID)
	if err != nil {
		return wrapActivityError(err)
	}
	harness, err := a.agentHarnessSnapshot(ctx, execution)
	if err != nil {
		return wrapActivityError(err)
	}
	secrets, err := a.agentHarnessRepo.LoadWorkspaceSecrets(ctx, execution.WorkspaceID)
	if err != nil {
		return wrapActivityError(err)
	}
	env, err := agentHarnessEnv(harness, secrets)
	if err != nil {
		return wrapActivityError(err)
	}
	timeout := agentHarnessTimeout(execution.ExecutionConfigSnapshot)

	session, err := a.sandboxProvider.Create(ctx, sandbox.CreateRequest{
		RunID:      execution.ID,
		RunAgentID: harness.ID,
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
			"agentclash_kind":         "agent_harness_execution",
			"agent_harness_execution": execution.ID.String(),
			"agent_harness":           harness.ID.String(),
			"workspace":               execution.WorkspaceID.String(),
		},
		TemplateID: harness.CodexTemplate,
		EnvVars:    env,
	})
	if err != nil {
		return wrapActivityError(err)
	}
	defer func() {
		if destroyErr := session.Destroy(context.Background()); destroyErr != nil {
			_ = a.recordAgentHarnessEvent(context.Background(), execution.ID, "sandbox.destroy.failed", "worker", map[string]any{"error": destroyErr.Error()})
		}
	}()
	if err := a.recordAgentHarnessEvent(ctx, execution.ID, "sandbox.created", "worker", map[string]any{"sandbox_id": session.ID(), "template": harness.CodexTemplate}); err != nil {
		return err
	}

	workdir := agentHarnessWorkspaceDir
	if harness.RepositoryURL != nil && strings.TrimSpace(*harness.RepositoryURL) != "" {
		clone := []string{"git", "clone", strings.TrimSpace(*harness.RepositoryURL), workdir}
		if result, err := a.execHarnessCommand(ctx, execution.ID, session, "repository.clone", clone, "", timeout); err != nil {
			return err
		} else if result.ExitCode != 0 {
			return fmt.Errorf("repository clone failed with exit code %d", result.ExitCode)
		}
		if harness.BaseBranch != nil && strings.TrimSpace(*harness.BaseBranch) != "" {
			checkout := []string{"git", "checkout", strings.TrimSpace(*harness.BaseBranch)}
			if result, err := a.execHarnessCommand(ctx, execution.ID, session, "repository.checkout", checkout, workdir, timeout); err != nil {
				return err
			} else if result.ExitCode != 0 {
				return fmt.Errorf("repository checkout failed with exit code %d", result.ExitCode)
			}
		}
	} else {
		workdir = "/"
	}

	codexCommand := []string{"codex", "exec", "--full-auto", "--skip-git-repo-check", "--json", harness.TaskPrompt}
	codexResult, err := a.execHarnessCommand(ctx, execution.ID, session, "codex.exec", codexCommand, workdir, timeout)
	if err != nil {
		return err
	}
	if codexResult.ExitCode != 0 {
		return fmt.Errorf("codex exec failed with exit code %d", codexResult.ExitCode)
	}

	if result, err := a.execHarnessCommand(ctx, execution.ID, session, "git.diff", []string{"git", "diff", "--binary"}, workdir, 60*time.Second); err != nil {
		return err
	} else {
		_ = a.recordAgentHarnessEvent(ctx, execution.ID, "artifact.git_diff", "worker", map[string]any{"diff": result.Stdout})
	}
	if result, err := a.execHarnessCommand(ctx, execution.ID, session, "git.changed_files", []string{"git", "status", "--short"}, workdir, 60*time.Second); err != nil {
		return err
	} else {
		_ = a.recordAgentHarnessEvent(ctx, execution.ID, "artifact.changed_files", "worker", map[string]any{"changed_files": result.Stdout})
	}

	return nil
}

func (a *Activities) agentHarnessSnapshot(ctx context.Context, execution repository.AgentHarnessExecution) (agentHarnessSnapshot, error) {
	if len(execution.HarnessSnapshot) > 0 && string(execution.HarnessSnapshot) != "null" {
		var snapshot agentHarnessSnapshot
		if err := json.Unmarshal(execution.HarnessSnapshot, &snapshot); err != nil {
			return agentHarnessSnapshot{}, fmt.Errorf("parse harness snapshot: %w", err)
		}
		return snapshot, nil
	}

	harness, err := a.agentHarnessRepo.GetAgentHarnessByID(ctx, execution.AgentHarnessID)
	if err != nil {
		return agentHarnessSnapshot{}, err
	}
	return agentHarnessSnapshot{
		ID:                     harness.ID,
		WorkspaceID:            harness.WorkspaceID,
		OrganizationID:         harness.OrganizationID,
		TaskPrompt:             harness.TaskPrompt,
		CodexTemplate:          harness.CodexTemplate,
		CodexModel:             harness.CodexModel,
		AuthMode:               harness.AuthMode,
		OpenAIAPIKeySecretName: harness.OpenAIAPIKeySecretName,
		RepositoryURL:          harness.RepositoryURL,
		BaseBranch:             harness.BaseBranch,
		ExecutionConfig:        harness.ExecutionConfig,
		EvaluationConfig:       harness.EvaluationConfig,
	}, nil
}

func (a *Activities) execHarnessCommand(ctx context.Context, executionID uuid.UUID, session sandbox.Session, eventType string, command []string, workdir string, timeout time.Duration) (sandbox.ExecResult, error) {
	if err := a.recordAgentHarnessEvent(ctx, executionID, eventType+".started", "worker", map[string]any{"command": command, "working_directory": workdir}); err != nil {
		return sandbox.ExecResult{}, err
	}
	result, err := session.Exec(ctx, sandbox.ExecRequest{
		Command:          command,
		WorkingDirectory: workdir,
		Timeout:          timeout,
	})
	payload := map[string]any{"command": command, "working_directory": workdir}
	if err != nil {
		payload["error"] = err.Error()
		_ = a.recordAgentHarnessEvent(ctx, executionID, eventType+".failed", "worker", payload)
		return sandbox.ExecResult{}, err
	}
	payload["exit_code"] = result.ExitCode
	payload["stdout"] = result.Stdout
	payload["stderr"] = result.Stderr
	if result.ExitCode == 0 {
		return result, a.recordAgentHarnessEvent(ctx, executionID, eventType+".completed", "worker", payload)
	}
	return result, a.recordAgentHarnessEvent(ctx, executionID, eventType+".failed", "worker", payload)
}

func (a *Activities) recordAgentHarnessEvent(ctx context.Context, executionID uuid.UUID, eventType string, actorType string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = a.agentHarnessRepo.RecordAgentHarnessExecutionEvent(ctx, repository.RecordAgentHarnessExecutionEventParams{
		ExecutionID: executionID,
		EventType:   eventType,
		ActorType:   actorType,
		OccurredAt:  time.Now().UTC(),
		Payload:     raw,
	})
	return wrapActivityError(err)
}

func agentHarnessEnv(h agentHarnessSnapshot, secrets map[string]string) (map[string]string, error) {
	env := map[string]string{}
	openAISecretName := strings.TrimSpace(derefString(h.OpenAIAPIKeySecretName))
	switch h.AuthMode {
	case "api_key_secret":
		if openAISecretName == "" {
			return nil, fmt.Errorf("%w: openai_api_key_secret_name is required", ErrAgentHarnessSecretMissing)
		}
		openAIKey := strings.TrimSpace(secrets[openAISecretName])
		if openAIKey == "" {
			return nil, fmt.Errorf("%w: %s", ErrAgentHarnessSecretMissing, openAISecretName)
		}
		env["OPENAI_API_KEY"] = openAIKey
		env["CODEX_API_KEY"] = openAIKey
	default:
		return nil, fmt.Errorf("unsupported agent harness auth mode %q", h.AuthMode)
	}
	return env, nil
}

func agentHarnessTimeout(raw json.RawMessage) time.Duration {
	cfg := agentHarnessExecutionConfig{TimeoutSeconds: 1800}
	_ = json.Unmarshal(raw, &cfg)
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 1800
	}
	return time.Duration(cfg.TimeoutSeconds) * time.Second
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
