package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/maputil"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/agentclash/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

const (
	agentHarnessWorkspaceDir          = "/workspace"
	agentHarnessActivityTimeoutBuffer = 2 * time.Minute
	defaultAgentHarnessTimeoutSeconds = 1800
	agentHarnessReplayTextPreviewMax  = 2048
)

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
	TimeoutSeconds int                        `json:"timeout_seconds,omitempty"`
	SetupCommands  []agentHarnessSetupCommand `json:"setup_commands,omitempty"`
}

type agentHarnessSetupCommand struct {
	Name             string `json:"name,omitempty"`
	Command          string `json:"command"`
	WorkingDirectory string `json:"working_directory,omitempty"`
	TimeoutSeconds   int    `json:"timeout_seconds,omitempty"`
}

type agentHarnessEvaluationConfig struct {
	Validators []agentHarnessValidatorConfig `json:"validators,omitempty"`
	LLMJudges  []json.RawMessage             `json:"llm_judges,omitempty"`
	Scorecard  json.RawMessage               `json:"scorecard,omitempty"`
	Result     agentHarnessResultMetadata    `json:"result,omitempty"`
	Suite      agentHarnessSuiteMetadata     `json:"suite,omitempty"`
	Privacy    agentHarnessPrivacyConfig     `json:"privacy,omitempty"`
}

type agentHarnessValidatorConfig struct {
	Key              string `json:"key,omitempty"`
	Type             string `json:"type"`
	Command          string `json:"command,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`
	TimeoutSeconds   int    `json:"timeout_seconds,omitempty"`
	Required         *bool  `json:"required,omitempty"`
	Hidden           bool   `json:"hidden,omitempty"`
	Private          bool   `json:"private,omitempty"`
	RedactOutput     *bool  `json:"redact_output,omitempty"`
}

type agentHarnessResultMetadata struct {
	Kind                 string   `json:"kind,omitempty"`
	BenchmarkSource      string   `json:"benchmark_source,omitempty"`
	CollectionDate       string   `json:"collection_date,omitempty"`
	AllowedPublicContext []string `json:"allowed_public_context,omitempty"`
	Contamination        string   `json:"contamination,omitempty"`
	Publicity            string   `json:"publicity,omitempty"`
}

type agentHarnessSuiteMetadata struct {
	SuiteID        uuid.UUID       `json:"suite_id,omitempty"`
	SuiteVersion   int32           `json:"suite_version,omitempty"`
	SuiteVersionID uuid.UUID       `json:"suite_version_id,omitempty"`
	TaskID         uuid.UUID       `json:"task_id,omitempty"`
	TaskSource     string          `json:"task_source,omitempty"`
	TaskMetadata   json.RawMessage `json:"task_metadata,omitempty"`
	PublicPrompt   string          `json:"public_prompt,omitempty"`
}

type agentHarnessPrivacyConfig struct {
	RedactReplay       *bool  `json:"redact_replay,omitempty"`
	RedactArtifacts    *bool  `json:"redact_artifacts,omitempty"`
	RetentionDays      *int   `json:"retention_days,omitempty"`
	AuditLog           *bool  `json:"audit_log,omitempty"`
	ProviderDataUse    string `json:"provider_data_use,omitempty"`
	WorkspacePolicyKey string `json:"workspace_policy_key,omitempty"`
}

type agentHarnessSnapshot struct {
	ID                     uuid.UUID       `json:"id"`
	WorkspaceID            uuid.UUID       `json:"workspace_id"`
	OrganizationID         uuid.UUID       `json:"organization_id"`
	HarnessKind            string          `json:"harness_kind"`
	TaskPrompt             string          `json:"task_prompt"`
	CodexTemplate          string          `json:"codex_template"`
	CodexModel             *string         `json:"codex_model,omitempty"`
	AuthMode               string          `json:"auth_mode"`
	OpenAIAPIKeySecretName *string         `json:"openai_api_key_secret_name,omitempty"`
	RepositoryURL          *string         `json:"repository_url,omitempty"`
	RepositoryProvider     *string         `json:"repository_provider,omitempty"`
	GitHubRepositoryID     *int64          `json:"github_repository_id,omitempty"`
	GitHubInstallationID   *int64          `json:"github_installation_id,omitempty"`
	RepositoryFullName     *string         `json:"repository_full_name,omitempty"`
	RepositoryCloneURL     *string         `json:"repository_clone_url,omitempty"`
	BaseBranch             *string         `json:"base_branch,omitempty"`
	ExecutionConfig        json.RawMessage `json:"execution_config,omitempty"`
	EvaluationConfig       json.RawMessage `json:"evaluation_config,omitempty"`
}

func AgentHarnessExecutionWorkflow(ctx sdkworkflow.Context, input AgentHarnessExecutionWorkflowInput) error {
	ctx = sdkworkflow.WithActivityOptions(ctx, defaultActivityOptions)
	defer markAgentHarnessExecutionCancelledOnWorkflowCancel(ctx, input.ExecutionID)

	if err := transitionAgentHarnessExecutionStatus(ctx, input.ExecutionID, repository.AgentHarnessExecutionStatusProvisioning, nil); err != nil {
		return err
	}
	if err := transitionAgentHarnessExecutionStatus(ctx, input.ExecutionID, repository.AgentHarnessExecutionStatusRunning, nil); err != nil {
		return err
	}
	if err := executeAgentHarnessExecution(ctx, input.ExecutionID, input.TimeoutSeconds); err != nil {
		return markAgentHarnessExecutionFailed(ctx, input.ExecutionID, err)
	}
	if err := transitionAgentHarnessExecutionStatus(ctx, input.ExecutionID, repository.AgentHarnessExecutionStatusCompleted, nil); err != nil {
		return markAgentHarnessExecutionFailed(ctx, input.ExecutionID, err)
	}
	return nil
}

func markAgentHarnessExecutionCancelledOnWorkflowCancel(ctx sdkworkflow.Context, executionID uuid.UUID) {
	if ctx.Err() == nil {
		return
	}
	disconnectedCtx, _ := sdkworkflow.NewDisconnectedContext(ctx)
	disconnectedCtx = sdkworkflow.WithActivityOptions(disconnectedCtx, defaultActivityOptions)
	reason := "workflow cancelled"
	_ = transitionAgentHarnessExecutionStatus(disconnectedCtx, executionID, repository.AgentHarnessExecutionStatusCancelled, &reason)
}

func transitionAgentHarnessExecutionStatus(ctx sdkworkflow.Context, executionID uuid.UUID, status repository.AgentHarnessExecutionStatus, reason *string) error {
	var execution repository.AgentHarnessExecution
	return sdkworkflow.ExecuteActivity(ctx, transitionAgentHarnessExecutionStatusActivityName, TransitionAgentHarnessExecutionStatusInput{
		ExecutionID: executionID,
		ToStatus:    status,
		Reason:      reason,
	}).Get(ctx, &execution)
}

func executeAgentHarnessExecution(ctx sdkworkflow.Context, executionID uuid.UUID, timeoutSeconds int) error {
	executeCtx := sdkworkflow.WithActivityOptions(ctx, agentHarnessExecutionActivityOptions(timeoutSeconds))
	return sdkworkflow.ExecuteActivity(executeCtx, executeAgentHarnessExecutionActivityName, ExecuteAgentHarnessExecutionInput{
		ExecutionID: executionID,
	}).Get(ctx, nil)
}

func agentHarnessExecutionActivityOptions(timeoutSeconds int) sdkworkflow.ActivityOptions {
	return sdkworkflow.ActivityOptions{
		StartToCloseTimeout: agentHarnessTimeoutFromSeconds(timeoutSeconds) + agentHarnessActivityTimeoutBuffer,
		RetryPolicy:         defaultActivityOptions.RetryPolicy,
	}
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
	defer a.buildAgentHarnessReplayBestEffort(ctx, execution)
	harness, err := a.agentHarnessSnapshot(ctx, execution)
	if err != nil {
		return wrapActivityError(err)
	}
	if err := a.verifyAgentHarnessGitHubAccess(ctx, execution.ID, harness); err != nil {
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
	evaluationConfig, err := decodeAgentHarnessEvaluationConfig(harness.EvaluationConfig)
	if err != nil {
		return err
	}
	replayRedaction := agentHarnessReplayRedactionEnabled(evaluationConfig.Privacy)
	artifactRedaction := agentHarnessArtifactRedactionEnabled(evaluationConfig.Privacy)

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
	if err := a.recordAgentHarnessEvent(ctx, execution.ID, "setup.runtime.detected", "worker", map[string]any{
		"template":         harness.CodexTemplate,
		"harness_kind":     harness.HarnessKind,
		"agent_tool":       agentHarnessToolName(harness),
		"metadata_version": 1,
	}); err != nil {
		return err
	}

	workdir := agentHarnessWorkspaceDir
	hasRepository := harness.RepositoryURL != nil && strings.TrimSpace(*harness.RepositoryURL) != ""
	gitEnv := env
	var gitHubToken string
	if hasRepository {
		if isStructuredGitHubHarness(harness) {
			gitEnv, gitHubToken, err = a.prepareGitHubGitAuth(ctx, execution.ID, session, harness, env)
			if err != nil {
				return err
			}
		}
		clone := []string{"git", "clone", strings.TrimSpace(*harness.RepositoryURL), workdir}
		if result, err := a.execHarnessCommand(ctx, execution.ID, session, "repository.clone", clone, "", timeout, gitEnv); err != nil {
			return err
		} else if result.ExitCode != 0 {
			return fmt.Errorf("repository clone failed with exit code %d", result.ExitCode)
		}
		if harness.BaseBranch != nil && strings.TrimSpace(*harness.BaseBranch) != "" {
			checkout := []string{"git", "checkout", strings.TrimSpace(*harness.BaseBranch)}
			if result, err := a.execHarnessCommand(ctx, execution.ID, session, "repository.checkout", checkout, workdir, timeout, gitEnv); err != nil {
				return err
			} else if result.ExitCode != 0 {
				return fmt.Errorf("repository checkout failed with exit code %d", result.ExitCode)
			}
		}
		_ = a.detectAgentHarnessSetupHints(ctx, execution.ID, session, workdir, gitEnv)
	} else {
		workdir = "/"
	}

	if err := a.runAgentHarnessSetupCommands(ctx, execution.ID, session, execution.ExecutionConfigSnapshot, workdir, timeout, env); err != nil {
		return err
	}

	runner, err := agentHarnessRunnerFor(harness, workdir)
	if err != nil {
		return err
	}
	runnerEnv := maputil.CloneStringMap(env)
	applyOpenClawRunnerEnv(runnerEnv, harness, timeout)
	applyHermesRunnerEnv(runnerEnv, harness)
	runnerResult, err := a.execHarnessCommandWithOptions(ctx, execution.ID, session, runner.EventType, runner.Command, workdir, timeout, runnerEnv, agentHarnessCommandRecordOptions{Hidden: replayRedaction, RedactOutput: replayRedaction})
	if err != nil {
		return err
	}
	if runnerResult.ExitCode != 0 {
		return fmt.Errorf("%s failed with exit code %d", runner.DisplayName, runnerResult.ExitCode)
	}

	if hasRepository {
		if result, err := a.execHarnessCommand(ctx, execution.ID, session, "git.add_intent", []string{"git", "add", "--intent-to-add", "--all"}, workdir, 60*time.Second, env); err != nil {
			return err
		} else if result.ExitCode != 0 {
			return fmt.Errorf("git add intent failed with exit code %d", result.ExitCode)
		}
		baseRef := agentHarnessGitBaseRef(harness)
		diffCommand := []string{"git", "diff", "--binary"}
		if baseRef != "" {
			diffCommand = append(diffCommand, baseRef)
		}
		if result, err := a.execHarnessCommandWithOptions(ctx, execution.ID, session, "git.diff", diffCommand, workdir, 60*time.Second, env, agentHarnessCommandRecordOptions{RedactOutput: artifactRedaction}); err != nil {
			return err
		} else {
			_ = a.recordAgentHarnessEvent(ctx, execution.ID, "artifact.git_diff", "worker", agentHarnessArtifactDiffPayload(result.Stdout, artifactRedaction))
		}
		baseChangedFiles := ""
		changedFilesCommand := []string{"git", "diff", "--name-status"}
		if baseRef != "" {
			changedFilesCommand = append(changedFilesCommand, baseRef)
		}
		if result, err := a.execHarnessCommandWithOptions(ctx, execution.ID, session, "git.base_changed_files", changedFilesCommand, workdir, 60*time.Second, env, agentHarnessCommandRecordOptions{RedactOutput: artifactRedaction}); err != nil {
			return err
		} else {
			baseChangedFiles = result.Stdout
		}
		workingTreeFiles := ""
		if result, err := a.execHarnessCommandWithOptions(ctx, execution.ID, session, "git.changed_files", []string{"git", "status", "--short"}, workdir, 60*time.Second, env, agentHarnessCommandRecordOptions{RedactOutput: artifactRedaction}); err != nil {
			return err
		} else {
			workingTreeFiles = result.Stdout
			_ = a.recordAgentHarnessEvent(ctx, execution.ID, "artifact.changed_files", "worker", agentHarnessChangedFilesPayload(baseChangedFiles, workingTreeFiles, artifactRedaction))
		}
		if isStructuredGitHubHarness(harness) {
			changes := agentHarnessGitChanges{
				ChangedFiles:       combineGitChangeLists(baseChangedFiles, workingTreeFiles),
				WorkingTreeChanges: workingTreeFiles,
			}
			if err := a.createGitHubPullRequest(ctx, execution.ID, session, harness, workdir, timeout, gitEnv, gitHubToken, changes, artifactRedaction); err != nil {
				return err
			}
		}
	}

	if _, err := a.TransitionAgentHarnessExecutionStatus(ctx, TransitionAgentHarnessExecutionStatusInput{
		ExecutionID: execution.ID,
		ToStatus:    repository.AgentHarnessExecutionStatusScoring,
	}); err != nil {
		return err
	}
	if err := a.evaluateAgentHarnessExecution(ctx, execution.ID, session, harness, harness.EvaluationConfig, workdir, timeout, env); err != nil {
		return err
	}

	return nil
}

func (a *Activities) buildAgentHarnessReplayBestEffort(ctx context.Context, execution repository.AgentHarnessExecution) {
	if a.repo == nil || execution.RunAgentID == nil {
		return
	}
	if _, err := a.repo.BuildRunAgentReplay(ctx, *execution.RunAgentID); err != nil {
		_ = a.recordAgentHarnessEvent(context.Background(), execution.ID, "replay.build.failed", "worker", map[string]any{"error": err.Error()})
	}
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
		HarnessKind:            harness.HarnessKind,
		TaskPrompt:             harness.TaskPrompt,
		CodexTemplate:          harness.CodexTemplate,
		CodexModel:             harness.CodexModel,
		AuthMode:               harness.AuthMode,
		OpenAIAPIKeySecretName: harness.OpenAIAPIKeySecretName,
		RepositoryURL:          harness.RepositoryURL,
		RepositoryProvider:     harness.RepositoryProvider,
		GitHubRepositoryID:     harness.GitHubRepositoryID,
		GitHubInstallationID:   harness.GitHubInstallationID,
		RepositoryFullName:     harness.RepositoryFullName,
		RepositoryCloneURL:     harness.RepositoryCloneURL,
		BaseBranch:             harness.BaseBranch,
		ExecutionConfig:        harness.ExecutionConfig,
		EvaluationConfig:       harness.EvaluationConfig,
	}, nil
}

func (a *Activities) verifyAgentHarnessGitHubAccess(ctx context.Context, executionID uuid.UUID, harness agentHarnessSnapshot) error {
	if !isStructuredGitHubHarness(harness) {
		return nil
	}
	_, err := a.agentHarnessRepo.GetWorkspaceGitHubRepository(ctx, harness.WorkspaceID, *harness.GitHubRepositoryID, harness.GitHubInstallationID)
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrGitHubRepositoryNotInstalled) {
		_ = a.recordAgentHarnessEvent(ctx, executionID, "github.repository_access_revoked", "worker", map[string]any{
			"github_repository_id":   *harness.GitHubRepositoryID,
			"github_installation_id": harness.GitHubInstallationID,
			"repository_full_name":   harness.RepositoryFullName,
		})
		return fmt.Errorf("%w: github repository access was revoked or removed", repository.ErrGitHubRepositoryNotInstalled)
	}
	return err
}

func (a *Activities) prepareGitHubGitAuth(ctx context.Context, executionID uuid.UUID, session sandbox.Session, harness agentHarnessSnapshot, baseEnv map[string]string) (map[string]string, string, error) {
	if a.githubClient == nil || harness.GitHubInstallationID == nil {
		_ = a.recordAgentHarnessEvent(ctx, executionID, "github.pull_request.skipped", "worker", map[string]any{"reason": "github_app_client_not_configured"})
		return baseEnv, "", nil
	}
	token, err := a.githubClient.CreateInstallationToken(ctx, *harness.GitHubInstallationID)
	if err != nil {
		return nil, "", fmt.Errorf("create github installation token: %w", err)
	}
	askpassPath := "/tmp/agentclash-git-askpass.sh"
	askpass := []byte("#!/bin/sh\ncase \"$1\" in\n*Username*) echo x-access-token ;;\n*) echo \"$GITHUB_TOKEN\" ;;\nesac\n")
	if err := session.WriteFile(ctx, askpassPath, askpass); err != nil {
		return nil, "", fmt.Errorf("write github askpass helper: %w", err)
	}
	authEnv := maputil.CloneStringMap(baseEnv)
	authEnv["GIT_ASKPASS"] = askpassPath
	authEnv["GIT_TERMINAL_PROMPT"] = "0"
	authEnv["GITHUB_TOKEN"] = token
	if result, err := a.execHarnessCommand(ctx, executionID, session, "github.git_auth.prepare", []string{"chmod", "700", askpassPath}, "", 30*time.Second, baseEnv); err != nil {
		return nil, "", err
	} else if result.ExitCode != 0 {
		return nil, "", fmt.Errorf("github git auth prepare failed with exit code %d", result.ExitCode)
	}
	return authEnv, token, nil
}

type agentHarnessRunner struct {
	DisplayName string
	EventType   string
	Command     []string
}

func agentHarnessRunnerFor(h agentHarnessSnapshot, workdir string) (agentHarnessRunner, error) {
	switch domain.NormalizeAgentHarnessKind(h.HarnessKind) {
	case "codex_e2b":
		return agentHarnessRunner{
			DisplayName: "codex exec",
			EventType:   "codex.exec",
			Command:     []string{"codex", "exec", "--full-auto", "--skip-git-repo-check", "--json", "-C", workdir, h.TaskPrompt},
		}, nil
	case domain.AgentHarnessKindOpenClawE2B:
		script := strings.Join([]string{
			"set -euo pipefail",
			`if [ -n "${OPENAI_API_KEY:-}" ]; then AUTH_CHOICE=openai-api-key`,
			`elif [ -n "${ANTHROPIC_API_KEY:-}" ]; then AUTH_CHOICE=apiKey`,
			`elif [ -n "${OPENROUTER_API_KEY:-}" ]; then AUTH_CHOICE=openrouter-api-key`,
			`else echo "missing OpenClaw provider API key" >&2; exit 1; fi`,
			`openclaw setup --workspace "$PWD" --mode local --non-interactive --accept-risk`,
			`openclaw onboard --non-interactive --mode local --auth-choice "$AUTH_CHOICE" --secret-input-mode ref --accept-risk --skip-bootstrap --skip-health`,
			`AGENT_ARGS=(--local --session-id agentclash-harness --json --timeout "${AGENTCLASH_HARNESS_TIMEOUT_SECONDS:-1800}" -m "$AGENTCLASH_HARNESS_TASK")`,
			`if [ -n "${AGENTCLASH_HARNESS_MODEL:-}" ]; then AGENT_ARGS+=(--model "$AGENTCLASH_HARNESS_MODEL"); fi`,
			`exec openclaw agent "${AGENT_ARGS[@]}"`,
		}, "\n")
		return agentHarnessRunner{
			DisplayName: "openclaw exec",
			EventType:   "openclaw.exec",
			Command:     []string{"bash", "-lc", script},
		}, nil
	case domain.AgentHarnessKindClaudeE2B:
		command := []string{"claude", "-p", "--output-format", "stream-json", "--verbose", "--permission-mode", "bypassPermissions"}
		if h.CodexModel != nil && strings.TrimSpace(*h.CodexModel) != "" {
			command = append(command, "--model", strings.TrimSpace(*h.CodexModel))
		}
		command = append(command, h.TaskPrompt)
		return agentHarnessRunner{
			DisplayName: "claude exec",
			EventType:   "claude.exec",
			Command:     command,
		}, nil
	case domain.AgentHarnessKindHermesE2B:
		script := strings.Join([]string{
			"set -euo pipefail",
			`hermes setup model --non-interactive`,
			`CHAT_ARGS=(--ignore-user-config --ignore-rules --toolsets terminal,skills --provider "${AGENTCLASH_HARNESS_PROVIDER:-openrouter}" -q "$AGENTCLASH_HARNESS_TASK")`,
			`if [ -n "${AGENTCLASH_HARNESS_MODEL:-}" ]; then CHAT_ARGS+=(--model "$AGENTCLASH_HARNESS_MODEL"); fi`,
			`exec hermes chat "${CHAT_ARGS[@]}"`,
		}, "\n")
		return agentHarnessRunner{
			DisplayName: "hermes exec",
			EventType:   "hermes.exec",
			Command:     []string{"bash", "-lc", script},
		}, nil
	default:
		return agentHarnessRunner{}, fmt.Errorf("unsupported agent harness kind %q", h.HarnessKind)
	}
}

func agentHarnessToolName(h agentHarnessSnapshot) string {
	switch domain.NormalizeAgentHarnessKind(h.HarnessKind) {
	case domain.AgentHarnessKindClaudeE2B:
		return "claude"
	case domain.AgentHarnessKindOpenClawE2B:
		return "openclaw"
	case domain.AgentHarnessKindHermesE2B:
		return "hermes"
	default:
		return "codex"
	}
}

type agentHarnessGitChanges struct {
	ChangedFiles       string
	WorkingTreeChanges string
}

type agentHarnessGitCommand struct {
	event   string
	command []string
}

func (c agentHarnessGitChanges) hasChanges() bool {
	return strings.TrimSpace(c.ChangedFiles) != ""
}

func (c agentHarnessGitChanges) hasWorkingTreeChanges() bool {
	return strings.TrimSpace(c.WorkingTreeChanges) != ""
}

func agentHarnessGitBaseRef(harness agentHarnessSnapshot) string {
	baseBranch := strings.TrimSpace(derefString(harness.BaseBranch))
	if baseBranch == "" {
		baseBranch = "main"
	}
	return "origin/" + baseBranch
}

func combineGitChangeLists(lists ...string) string {
	parts := make([]string, 0, len(lists))
	for _, list := range lists {
		if trimmed := strings.TrimSpace(list); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "\n")
}

func splitNonEmptyLines(raw string) []string {
	lines := strings.Split(raw, "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func agentHarnessSetupHints(paths []string) []map[string]string {
	hints := make([]map[string]string, 0, len(paths))
	for _, path := range paths {
		kind := "unknown"
		switch {
		case path == ".devcontainer/devcontainer.json" || path == "devcontainer.json":
			kind = "devcontainer"
		case strings.HasPrefix(path, ".github/workflows/"):
			kind = "github_workflow"
		case path == "go.mod":
			kind = "go"
		case path == "Cargo.toml":
			kind = "rust"
		case path == "package.json":
			kind = "node"
		case path == "pyproject.toml":
			kind = "python"
		case path == "flake.nix":
			kind = "nix"
		}
		hints = append(hints, map[string]string{"path": path, "kind": kind})
	}
	return hints
}

func (a *Activities) createGitHubPullRequest(ctx context.Context, executionID uuid.UUID, session sandbox.Session, harness agentHarnessSnapshot, workdir string, timeout time.Duration, gitEnv map[string]string, token string, changes agentHarnessGitChanges, redactArtifacts bool) error {
	if !changes.hasChanges() {
		return a.recordAgentHarnessEvent(ctx, executionID, "github.pull_request.skipped", "worker", map[string]any{"reason": "no_changes"})
	}
	if a.githubClient == nil || token == "" {
		return a.recordAgentHarnessEvent(ctx, executionID, "github.pull_request.skipped", "worker", map[string]any{"reason": "github_app_client_not_configured"})
	}
	owner, repo, ok := parseGitHubFullName(derefString(harness.RepositoryFullName))
	if !ok {
		return a.recordAgentHarnessEvent(ctx, executionID, "github.pull_request.skipped", "worker", map[string]any{"reason": "missing_repository_full_name"})
	}
	baseBranch := strings.TrimSpace(derefString(harness.BaseBranch))
	if baseBranch == "" {
		baseBranch = "main"
	}
	branch := "agentclash/harness/" + strings.ReplaceAll(executionID.String()[:8], "-", "")
	pushURL := "https://github.com/" + owner + "/" + repo + ".git"
	commands := []agentHarnessGitCommand{
		{"git.config_user_email", []string{"git", "config", "user.email", "agentclash[bot]@users.noreply.github.com"}},
		{"git.config_user_name", []string{"git", "config", "user.name", "agentclash[bot]"}},
		{"git.create_branch", []string{"git", "checkout", "-B", branch}},
	}
	if changes.hasWorkingTreeChanges() {
		commands = append(commands,
			agentHarnessGitCommand{"git.add_all", []string{"git", "add", "--all"}},
			agentHarnessGitCommand{"git.commit", []string{"git", "-c", "core.hooksPath=/dev/null", "commit", "-m", "AgentClash harness changes"}},
		)
	}
	commands = append(commands, agentHarnessGitCommand{"git.push_branch", []string{"git", "-c", "core.hooksPath=/dev/null", "-c", "credential.helper=", "push", pushURL, "HEAD:refs/heads/" + branch}})
	for _, step := range commands {
		stepEnv := map[string]string(nil)
		if step.event == "git.push_branch" {
			stepEnv = gitEnv
		}
		if result, err := a.execHarnessCommandWithOptions(ctx, executionID, session, step.event, step.command, workdir, timeout, stepEnv, agentHarnessCommandRecordOptions{RedactOutput: redactArtifacts}); err != nil {
			return err
		} else if result.ExitCode != 0 {
			return fmt.Errorf("%s failed with exit code %d", step.event, result.ExitCode)
		}
	}
	pr, err := a.githubClient.CreatePullRequest(ctx, CreateGitHubPullRequestInput{
		Token: token,
		Owner: owner,
		Repo:  repo,
		Title: "AgentClash harness changes",
		Head:  owner + ":" + branch,
		Base:  baseBranch,
		Body:  "Created automatically by an AgentClash agent harness execution.\n\nExecution: " + executionID.String(),
		Draft: true,
	})
	if err != nil {
		return fmt.Errorf("create github pull request: %w", err)
	}
	return a.recordAgentHarnessEvent(ctx, executionID, "github.pull_request.created", "worker", map[string]any{
		"number": pr.Number,
		"url":    pr.HTMLURL,
		"state":  pr.State,
		"draft":  pr.Draft,
		"branch": branch,
		"base":   baseBranch,
	})
}

func isStructuredGitHubHarness(harness agentHarnessSnapshot) bool {
	return derefString(harness.RepositoryProvider) == "github" && harness.GitHubRepositoryID != nil
}

func parseGitHubFullName(fullName string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(fullName), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

type agentHarnessCommandRecordOptions struct {
	Hidden       bool
	RedactOutput bool
}

func (a *Activities) execHarnessCommand(ctx context.Context, executionID uuid.UUID, session sandbox.Session, eventType string, command []string, workdir string, timeout time.Duration, env map[string]string) (sandbox.ExecResult, error) {
	return a.execHarnessCommandWithOptions(ctx, executionID, session, eventType, command, workdir, timeout, env, agentHarnessCommandRecordOptions{})
}

func (a *Activities) execHarnessCommandWithOptions(ctx context.Context, executionID uuid.UUID, session sandbox.Session, eventType string, command []string, workdir string, timeout time.Duration, env map[string]string, options agentHarnessCommandRecordOptions) (sandbox.ExecResult, error) {
	if err := a.recordAgentHarnessEvent(ctx, executionID, eventType+".started", "worker", agentHarnessCommandStartPayload(command, workdir, options)); err != nil {
		return sandbox.ExecResult{}, err
	}
	stdoutRemainder := ""
	outputEventType, outputActor, streamOutput := agentHarnessOutputStream(eventType)
	onStdout := func(chunk []byte) error {
		if !streamOutput {
			return nil
		}
		remainder, err := a.recordAgentRunnerOutputEventsWithOptions(ctx, executionID, outputEventType, outputActor, stdoutRemainder+string(chunk), false, options)
		stdoutRemainder = remainder
		return err
	}
	result, err := session.Exec(ctx, sandbox.ExecRequest{
		Command:          command,
		WorkingDirectory: workdir,
		Timeout:          timeout,
		Environment:      maputil.CloneStringMap(env),
		OnStdout:         onStdout,
	})
	if streamOutput && stdoutRemainder != "" {
		if _, parseErr := a.recordAgentRunnerOutputEventsWithOptions(ctx, executionID, outputEventType, outputActor, stdoutRemainder, true, options); err == nil && parseErr != nil {
			return sandbox.ExecResult{}, parseErr
		}
	}
	payload := agentHarnessCommandPayload(command, workdir, result, err, options)
	if err != nil {
		_ = a.recordAgentHarnessEvent(ctx, executionID, eventType+".failed", "worker", payload)
		return sandbox.ExecResult{}, err
	}
	if result.ExitCode == 0 {
		return result, a.recordAgentHarnessEvent(ctx, executionID, eventType+".completed", "worker", payload)
	}
	return result, a.recordAgentHarnessEvent(ctx, executionID, eventType+".failed", "worker", payload)
}

func agentHarnessCommandStartPayload(command []string, workdir string, options agentHarnessCommandRecordOptions) map[string]any {
	payload := map[string]any{"working_directory": workdir}
	if options.Hidden {
		payload["command_hidden"] = true
		payload["visibility"] = "hidden"
		return payload
	}
	payload["command"] = command
	return payload
}

func agentHarnessCommandPayload(command []string, workdir string, result sandbox.ExecResult, err error, options agentHarnessCommandRecordOptions) map[string]any {
	payload := map[string]any{"working_directory": workdir}
	if options.Hidden {
		payload["command_hidden"] = true
		payload["visibility"] = "hidden"
	} else {
		payload["command"] = command
	}
	if err != nil {
		payload["error"] = err.Error()
		return payload
	}
	payload["exit_code"] = result.ExitCode
	if options.Hidden || options.RedactOutput {
		payload["output_redacted"] = true
		return payload
	}
	payload["stdout"] = result.Stdout
	payload["stderr"] = result.Stderr
	return payload
}

func agentHarnessReplayRedactionEnabled(config agentHarnessPrivacyConfig) bool {
	return config.RedactReplay != nil && *config.RedactReplay
}

func agentHarnessArtifactRedactionEnabled(config agentHarnessPrivacyConfig) bool {
	return config.RedactArtifacts != nil && *config.RedactArtifacts
}

func agentHarnessArtifactDiffPayload(diff string, redact bool) map[string]any {
	if redact {
		return map[string]any{
			"artifact_redacted": true,
			"diff_summary": map[string]any{
				"size_bytes": len(diff),
				"truncated":  true,
				"preview":    "[redacted]",
			},
		}
	}
	return map[string]any{"diff": diff}
}

func agentHarnessChangedFilesPayload(baseChangedFiles string, workingTreeFiles string, redact bool) map[string]any {
	combined := combineGitChangeLists(baseChangedFiles, workingTreeFiles)
	if redact {
		return map[string]any{
			"artifact_redacted": true,
			"changed_files_summary": map[string]any{
				"count": len(splitNonEmptyLines(combined)),
			},
		}
	}
	return map[string]any{
		"changed_files":        combined,
		"base_changed_files":   baseChangedFiles,
		"working_tree_changes": workingTreeFiles,
	}
}

func (a *Activities) execAgentHarnessShellCommand(ctx context.Context, executionID uuid.UUID, session sandbox.Session, eventType string, command string, configuredWorkdir string, timeoutSeconds int, defaultWorkdir string, defaultTimeout time.Duration, env map[string]string) (sandbox.ExecResult, string, error) {
	return a.execAgentHarnessShellCommandWithOptions(ctx, executionID, session, eventType, command, configuredWorkdir, timeoutSeconds, defaultWorkdir, defaultTimeout, env, agentHarnessCommandRecordOptions{})
}

func (a *Activities) execAgentHarnessShellCommandWithOptions(ctx context.Context, executionID uuid.UUID, session sandbox.Session, eventType string, command string, configuredWorkdir string, timeoutSeconds int, defaultWorkdir string, defaultTimeout time.Duration, env map[string]string, options agentHarnessCommandRecordOptions) (sandbox.ExecResult, string, error) {
	workdir := agentHarnessValidatorWorkdir(defaultWorkdir, configuredWorkdir)
	timeout := defaultTimeout
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}
	result, err := a.execHarnessCommandWithOptions(ctx, executionID, session, eventType, []string{"bash", "-lc", command}, workdir, timeout, env, options)
	return result, workdir, err
}

func (a *Activities) evaluateAgentHarnessExecution(ctx context.Context, executionID uuid.UUID, session sandbox.Session, harness agentHarnessSnapshot, rawConfig json.RawMessage, workdir string, defaultTimeout time.Duration, env map[string]string) error {
	config, err := decodeAgentHarnessEvaluationConfig(rawConfig)
	if err != nil {
		_ = a.recordAgentHarnessEvent(ctx, executionID, "scoring.failed", "worker", map[string]any{"error": err.Error()})
		return err
	}
	if len(config.Validators) == 0 && len(config.LLMJudges) == 0 {
		return a.recordAgentHarnessEvent(ctx, executionID, "scoring.skipped", "worker", map[string]any{"reason": "evaluation_config has no validators or llm_judges"})
	}
	if err := a.recordAgentHarnessEvaluationControls(ctx, executionID, config); err != nil {
		return err
	}

	passed := 0
	failed := 0
	skipped := 0
	var requiredValidatorErr error
	validatorResults := make([]scoring.ValidatorResult, 0, len(config.Validators))
	for index, validator := range config.Validators {
		switch strings.TrimSpace(validator.Type) {
		case "command":
			result, ok, err := a.evaluateCommandValidator(ctx, executionID, session, validator, index, workdir, defaultTimeout, env)
			validatorResults = append(validatorResults, result)
			if err != nil {
				failed++
				if validatorRequired(validator) {
					requiredValidatorErr = err
				}
			}
			if ok {
				passed++
			}
		default:
			skipped++
			if err := a.recordAgentHarnessEvent(ctx, executionID, "validator.skipped", "worker", map[string]any{"index": index, "type": validator.Type, "reason": "unsupported validator type"}); err != nil {
				return err
			}
		}
	}

	if a.repo != nil {
		evaluation, err := a.buildAndStoreAgentHarnessScorecard(ctx, executionID, harness, config, validatorResults, passed, failed, skipped)
		if err != nil {
			_ = a.recordAgentHarnessEvent(ctx, executionID, "scoring.failed", "worker", map[string]any{"error": err.Error()})
			return err
		}
		if evaluation.RunAgentID != uuid.Nil {
			if err := a.recordAgentHarnessEvent(ctx, executionID, "scorecard.persisted", "worker", map[string]any{
				"evaluation_spec_id": evaluation.EvaluationSpecID,
				"status":             evaluation.Status,
				"passed":             evaluation.Passed,
				"overall_score":      evaluation.OverallScore,
				"llm_judges":         len(evaluation.LLMJudgeResults),
				"dimensions":         evaluation.DimensionScores,
				"result":             config.Result,
				"suite":              config.Suite,
				"privacy":            config.Privacy,
			}); err != nil {
				return err
			}
		}
	}

	if err := a.recordAgentHarnessEvent(ctx, executionID, "scoring.completed", "worker", map[string]any{"passed": passed, "failed": failed, "skipped": skipped, "score": agentHarnessScore(passed, failed)}); err != nil {
		return err
	}
	return requiredValidatorErr
}

func (a *Activities) recordAgentHarnessEvaluationControls(ctx context.Context, executionID uuid.UUID, config agentHarnessEvaluationConfig) error {
	if !agentHarnessPrivacyConfigEmpty(config.Privacy) {
		if err := a.recordAgentHarnessEvent(ctx, executionID, "privacy.policy.applied", "worker", map[string]any{
			"redact_replay":        config.Privacy.RedactReplay,
			"redact_artifacts":     config.Privacy.RedactArtifacts,
			"retention_days":       config.Privacy.RetentionDays,
			"audit_log":            config.Privacy.AuditLog,
			"provider_data_use":    strings.TrimSpace(config.Privacy.ProviderDataUse),
			"workspace_policy_key": strings.TrimSpace(config.Privacy.WorkspacePolicyKey),
		}); err != nil {
			return err
		}
		if config.Privacy.AuditLog == nil || *config.Privacy.AuditLog {
			if err := a.recordAgentHarnessEvent(ctx, executionID, "privacy.audit.recorded", "worker", map[string]any{
				"policy_event": "privacy.policy.applied",
			}); err != nil {
				return err
			}
		}
	}
	if !agentHarnessResultMetadataEmpty(config.Result) || !agentHarnessSuiteMetadataEmpty(config.Suite) {
		return a.recordAgentHarnessEvent(ctx, executionID, "benchmark.metadata.recorded", "worker", map[string]any{
			"kind":                   strings.TrimSpace(config.Result.Kind),
			"benchmark_source":       strings.TrimSpace(config.Result.BenchmarkSource),
			"collection_date":        strings.TrimSpace(config.Result.CollectionDate),
			"allowed_public_context": config.Result.AllowedPublicContext,
			"contamination":          strings.TrimSpace(config.Result.Contamination),
			"publicity":              strings.TrimSpace(config.Result.Publicity),
			"suite":                  config.Suite,
		})
	}
	return nil
}

func agentHarnessPrivacyConfigEmpty(config agentHarnessPrivacyConfig) bool {
	return config.RedactReplay == nil &&
		config.RedactArtifacts == nil &&
		config.RetentionDays == nil &&
		config.AuditLog == nil &&
		strings.TrimSpace(config.ProviderDataUse) == "" &&
		strings.TrimSpace(config.WorkspacePolicyKey) == ""
}

func agentHarnessResultMetadataEmpty(metadata agentHarnessResultMetadata) bool {
	return strings.TrimSpace(metadata.Kind) == "" &&
		strings.TrimSpace(metadata.BenchmarkSource) == "" &&
		strings.TrimSpace(metadata.CollectionDate) == "" &&
		len(metadata.AllowedPublicContext) == 0 &&
		strings.TrimSpace(metadata.Contamination) == "" &&
		strings.TrimSpace(metadata.Publicity) == ""
}

func agentHarnessSuiteMetadataEmpty(metadata agentHarnessSuiteMetadata) bool {
	return metadata.SuiteID == uuid.Nil &&
		metadata.SuiteVersion == 0 &&
		metadata.SuiteVersionID == uuid.Nil &&
		metadata.TaskID == uuid.Nil &&
		strings.TrimSpace(metadata.TaskSource) == "" &&
		len(metadata.TaskMetadata) == 0 &&
		strings.TrimSpace(metadata.PublicPrompt) == ""
}

func (a *Activities) detectAgentHarnessSetupHints(ctx context.Context, executionID uuid.UUID, session sandbox.Session, workdir string, env map[string]string) error {
	result, err := a.execHarnessCommand(ctx, executionID, session, "setup.hints.detect", []string{"bash", "-lc", "for f in .devcontainer/devcontainer.json devcontainer.json .github/workflows/*.yml .github/workflows/*.yaml go.mod Cargo.toml package.json pyproject.toml flake.nix; do [ -e \"$f\" ] && echo \"$f\"; done"}, workdir, 30*time.Second, env)
	if err != nil {
		return a.recordAgentHarnessEvent(ctx, executionID, "setup.hints.detected", "worker", map[string]any{"hints": []map[string]string{}, "error": err.Error()})
	}
	if result.ExitCode != 0 {
		return a.recordAgentHarnessEvent(ctx, executionID, "setup.hints.detected", "worker", map[string]any{"hints": []map[string]string{}, "error": fmt.Sprintf("setup hint detection failed with exit code %d", result.ExitCode)})
	}
	hints := agentHarnessSetupHints(splitNonEmptyLines(result.Stdout))
	return a.recordAgentHarnessEvent(ctx, executionID, "setup.hints.detected", "worker", map[string]any{"hints": hints})
}

func (a *Activities) runAgentHarnessSetupCommands(ctx context.Context, executionID uuid.UUID, session sandbox.Session, rawConfig json.RawMessage, defaultWorkdir string, defaultTimeout time.Duration, env map[string]string) error {
	cfg := agentHarnessExecutionConfig{}
	if len(rawConfig) > 0 && string(rawConfig) != "null" {
		if err := json.Unmarshal(rawConfig, &cfg); err != nil {
			_ = a.recordAgentHarnessEvent(ctx, executionID, "setup.config.failed", "worker", map[string]any{"error": err.Error()})
			return fmt.Errorf("decode execution_config setup commands: %w", err)
		}
	}
	if len(cfg.SetupCommands) == 0 {
		return a.recordAgentHarnessEvent(ctx, executionID, "setup.skipped", "worker", map[string]any{"reason": "execution_config has no setup_commands"})
	}
	for index, setup := range cfg.SetupCommands {
		command := strings.TrimSpace(setup.Command)
		if command == "" {
			err := fmt.Errorf("setup command %d is missing command", index)
			_ = a.recordAgentHarnessEvent(ctx, executionID, "setup.command.failed", "worker", map[string]any{"index": index, "error": err.Error()})
			return err
		}
		name := strings.TrimSpace(setup.Name)
		if name == "" {
			name = fmt.Sprintf("setup-%d", index+1)
		}
		result, workdir, err := a.execAgentHarnessShellCommand(ctx, executionID, session, "setup.command.exec", command, setup.WorkingDirectory, setup.TimeoutSeconds, defaultWorkdir, defaultTimeout, env)
		if err != nil {
			_ = a.recordAgentHarnessEvent(ctx, executionID, "setup.command.failed", "worker", map[string]any{"index": index, "name": name, "command": command, "error": err.Error()})
			return err
		}
		payload := map[string]any{"index": index, "name": name, "command": command, "working_directory": workdir, "exit_code": result.ExitCode}
		if result.ExitCode != 0 {
			payload["stdout"] = result.Stdout
			payload["stderr"] = result.Stderr
			err := fmt.Errorf("setup command %q failed with exit code %d", name, result.ExitCode)
			payload["error"] = err.Error()
			_ = a.recordAgentHarnessEvent(ctx, executionID, "setup.command.failed", "worker", payload)
			return err
		}
		if err := a.recordAgentHarnessEvent(ctx, executionID, "setup.command.completed", "worker", payload); err != nil {
			return err
		}
	}
	return a.recordAgentHarnessEvent(ctx, executionID, "setup.completed", "worker", map[string]any{"commands": len(cfg.SetupCommands)})
}

func (a *Activities) evaluateCommandValidator(ctx context.Context, executionID uuid.UUID, session sandbox.Session, validator agentHarnessValidatorConfig, index int, defaultWorkdir string, defaultTimeout time.Duration, env map[string]string) (scoring.ValidatorResult, bool, error) {
	command := strings.TrimSpace(validator.Command)
	validatorResult := scoring.ValidatorResult{
		Key:          agentHarnessValidatorKey(validator, index),
		Type:         scoring.ValidatorTypeBooleanAssert,
		Target:       "agent_harness.command_validator",
		ExpectedFrom: "exit_code_zero",
	}
	if command == "" {
		err := fmt.Errorf("command validator %d is missing command", index)
		_ = a.recordAgentHarnessEvent(ctx, executionID, "validator.command.failed", "worker", map[string]any{"index": index, "error": err.Error()})
		validatorResult.State = scoring.OutputStateError
		validatorResult.Verdict = "error"
		validatorResult.OutcomeClass = scoring.ValidatorOutcomePackError
		validatorResult.Reason = err.Error()
		validatorResult.RawOutput = mustMarshalJSON(map[string]any{"error": err.Error()})
		return validatorResult, false, err
	}
	hidden := agentHarnessValidatorHidden(validator)
	result, workdir, err := a.execAgentHarnessShellCommandWithOptions(ctx, executionID, session, "validator.command.exec", command, validator.WorkingDirectory, validator.TimeoutSeconds, defaultWorkdir, defaultTimeout, env, agentHarnessCommandRecordOptions{
		Hidden:       hidden,
		RedactOutput: agentHarnessValidatorRedactOutput(validator),
	})
	payload := agentHarnessValidatorEventPayload(validator, index, command, workdir, result, nil)
	if err != nil {
		payload["error"] = err.Error()
		_ = a.recordAgentHarnessEvent(ctx, executionID, "validator.command.failed", "worker", payload)
		validatorResult.State = scoring.OutputStateError
		validatorResult.Verdict = "error"
		validatorResult.OutcomeClass = scoring.ValidatorOutcomeInfraError
		validatorResult.Reason = err.Error()
		validatorResult.RawOutput = mustMarshalJSON(agentHarnessValidatorRawOutput(payload, hidden))
		return validatorResult, false, err
	}
	score := 0.0
	validatorResult.State = scoring.OutputStateAvailable
	validatorResult.RawOutput = mustMarshalJSON(agentHarnessValidatorRawOutput(payload, hidden))
	if result.ExitCode == 0 {
		score = 1
		validatorResult.Verdict = "pass"
		validatorResult.OutcomeClass = scoring.ValidatorOutcomePass
		validatorResult.NormalizedScore = &score
		return validatorResult, true, a.recordAgentHarnessEvent(ctx, executionID, "validator.command.passed", "worker", payload)
	}
	err = fmt.Errorf("command validator %d failed with exit code %d", index, result.ExitCode)
	payload["error"] = err.Error()
	_ = a.recordAgentHarnessEvent(ctx, executionID, "validator.command.failed", "worker", payload)
	validatorResult.Verdict = "fail"
	validatorResult.OutcomeClass = scoring.ValidatorOutcomeFail
	validatorResult.NormalizedScore = &score
	validatorResult.Reason = err.Error()
	validatorResult.RawOutput = mustMarshalJSON(agentHarnessValidatorRawOutput(payload, hidden))
	return validatorResult, false, err
}

func agentHarnessValidatorKey(validator agentHarnessValidatorConfig, index int) string {
	if key := strings.TrimSpace(validator.Key); key != "" {
		return key
	}
	return fmt.Sprintf("command_%d", index+1)
}

func (a *Activities) buildAndStoreAgentHarnessScorecard(ctx context.Context, executionID uuid.UUID, harness agentHarnessSnapshot, config agentHarnessEvaluationConfig, validatorResults []scoring.ValidatorResult, passed int, failed int, skipped int) (scoring.RunAgentEvaluation, error) {
	execution, err := a.agentHarnessRepo.GetAgentHarnessExecutionByID(ctx, executionID)
	if err != nil {
		return scoring.RunAgentEvaluation{}, wrapActivityError(err)
	}
	if execution.RunID == nil || execution.RunAgentID == nil {
		return scoring.RunAgentEvaluation{}, nil
	}
	spec, err := agentHarnessEvaluationSpec(executionID, config, validatorResults)
	if err != nil {
		return scoring.RunAgentEvaluation{}, err
	}
	definition, err := scoring.MarshalDefinition(spec)
	if err != nil {
		return scoring.RunAgentEvaluation{}, err
	}
	normalizedSpec, err := scoring.DecodeDefinition(definition)
	if err != nil {
		return scoring.RunAgentEvaluation{}, err
	}
	specRecord, err := a.repo.CreateStandaloneEvaluationSpec(ctx, repository.CreateStandaloneEvaluationSpecParams{
		Name:          normalizedSpec.Name,
		VersionNumber: normalizedSpec.VersionNumber,
		JudgeMode:     string(normalizedSpec.JudgeMode),
		Definition:    definition,
	})
	if err != nil {
		return scoring.RunAgentEvaluation{}, err
	}
	if _, err := a.agentHarnessRepo.SetAgentHarnessExecutionEvaluationSpec(ctx, repository.SetAgentHarnessExecutionEvaluationSpecParams{
		ExecutionID:      executionID,
		EvaluationSpecID: specRecord.ID,
	}); err != nil {
		return scoring.RunAgentEvaluation{}, err
	}
	runEvents, err := a.repo.ListRunEventsByRunAgentID(ctx, *execution.RunAgentID)
	if err != nil {
		return scoring.RunAgentEvaluation{}, err
	}
	evaluationEvents := agentHarnessEvaluationEvents(runEvents)
	input := scoring.EvaluationInput{
		RunAgentID:       *execution.RunAgentID,
		EvaluationSpecID: specRecord.ID,
		Events:           evaluationEvents,
	}
	executionContext := repository.RunAgentExecutionContext{
		Run: domain.Run{
			ID:          *execution.RunID,
			WorkspaceID: execution.WorkspaceID,
		},
		RunAgent: domain.RunAgent{
			ID: *execution.RunAgentID,
		},
		Deployment: agentHarnessJudgeDeploymentContext(harness),
	}
	llmJudgeResults, judgeWarnings := evaluateLLMJudges(ctx, a.judgeClient, a.repo, executionContext, input, normalizedSpec)
	evaluation, err := scoring.EvaluateRunAgentWithPrecomputedResults(input, normalizedSpec, validatorResults, nil, llmJudgeResults)
	if err != nil {
		return scoring.RunAgentEvaluation{}, err
	}
	if !agentHarnessSuiteMetadataEmpty(config.Suite) || !agentHarnessResultMetadataEmpty(config.Result) {
		evaluation.Metadata = mustMarshalJSON(map[string]any{
			"agent_harness_suite": config.Suite,
			"result":              config.Result,
		})
	}
	evaluation.Warnings = append(evaluation.Warnings, judgeWarnings...)
	if err := a.repo.StoreRunAgentEvaluationResults(ctx, evaluation); err != nil {
		return scoring.RunAgentEvaluation{}, err
	}
	if err := recordScoringEvents(ctx, a.repo, *execution.RunID, evaluation); err != nil {
		evaluation.Warnings = append(evaluation.Warnings, fmt.Sprintf("record scoring events: %v", err))
	}
	return evaluation, nil
}

func agentHarnessEvaluationSpec(executionID uuid.UUID, config agentHarnessEvaluationConfig, validatorResults []scoring.ValidatorResult) (scoring.EvaluationSpec, error) {
	judges, err := agentHarnessLLMJudges(config.LLMJudges)
	if err != nil {
		return scoring.EvaluationSpec{}, err
	}
	spec := scoring.EvaluationSpec{
		Name:          "agent-harness-" + executionID.String(),
		VersionNumber: 1,
		JudgeMode:     scoring.JudgeModeDeterministic,
		Validators:    make([]scoring.ValidatorDeclaration, 0, len(validatorResults)),
		LLMJudges:     judges,
		Scorecard: scoring.ScorecardDeclaration{
			Strategy: scoring.ScoringStrategyWeighted,
		},
	}
	passThreshold := 1.0
	spec.Scorecard.PassThreshold = &passThreshold
	if len(judges) > 0 {
		spec.JudgeMode = scoring.JudgeModeHybrid
	}
	validatorKeys := make([]string, 0, len(validatorResults))
	for index, result := range validatorResults {
		key := strings.TrimSpace(result.Key)
		if key == "" {
			key = fmt.Sprintf("command_%d", index+1)
		}
		validatorKeys = append(validatorKeys, key)
		spec.Validators = append(spec.Validators, scoring.ValidatorDeclaration{
			Key:          key,
			Type:         scoring.ValidatorTypeBooleanAssert,
			Target:       "literal:true",
			ExpectedFrom: "literal:true",
		})
	}
	if len(spec.Validators) == 0 {
		spec.Validators = append(spec.Validators, scoring.ValidatorDeclaration{
			Key:          "harness_execution",
			Type:         scoring.ValidatorTypeBooleanAssert,
			Target:       "literal:true",
			ExpectedFrom: "literal:true",
		})
	}
	if len(config.Scorecard) > 0 && string(config.Scorecard) != "null" {
		if err := json.Unmarshal(config.Scorecard, &spec.Scorecard); err != nil {
			return scoring.EvaluationSpec{}, fmt.Errorf("decode harness scorecard: %w", err)
		}
	} else {
		targetLatency := 60_000.0
		maxLatency := 30 * 60_000.0
		targetCost := 0.0
		maxCost := 1.0
		spec.Scorecard.Dimensions = append(spec.Scorecard.Dimensions, scoring.DimensionDeclaration{
			Key:             "correctness",
			Source:          scoring.DimensionSourceValidators,
			Validators:      validatorKeys,
			BetterDirection: "higher",
		})
		spec.Scorecard.Dimensions = append(spec.Scorecard.Dimensions, scoring.DimensionDeclaration{
			Key:             "latency",
			Source:          scoring.DimensionSourceLatency,
			BetterDirection: "lower",
			Normalization:   &scoring.DimensionNormalization{Target: &targetLatency, Max: &maxLatency},
		})
		spec.Scorecard.Dimensions = append(spec.Scorecard.Dimensions, scoring.DimensionDeclaration{
			Key:             "cost",
			Source:          scoring.DimensionSourceCost,
			BetterDirection: "lower",
			Normalization:   &scoring.DimensionNormalization{Target: &targetCost, Max: &maxCost},
		})
		for _, judge := range judges {
			spec.Scorecard.Dimensions = append(spec.Scorecard.Dimensions, scoring.DimensionDeclaration{
				Key:             judge.Key,
				Source:          scoring.DimensionSourceLLMJudge,
				JudgeKey:        judge.Key,
				BetterDirection: "higher",
			})
		}
	}
	return spec, nil
}

func agentHarnessJudgeDeploymentContext(harness agentHarnessSnapshot) repository.AgentDeploymentExecutionContext {
	providerKey := ""
	switch domain.NormalizeAgentHarnessKind(harness.HarnessKind) {
	case domain.AgentHarnessKindClaudeE2B:
		providerKey = "anthropic"
	case domain.AgentHarnessKindOpenClawE2B:
		providerKey = openClawProviderKey(derefString(harness.OpenAIAPIKeySecretName))
	case domain.AgentHarnessKindHermesE2B:
		providerKey = hermesProviderKey(derefString(harness.OpenAIAPIKeySecretName))
	default:
		providerKey = "openai"
	}
	secretName := strings.TrimSpace(derefString(harness.OpenAIAPIKeySecretName))
	if secretName == "" || providerKey == "" {
		return repository.AgentDeploymentExecutionContext{}
	}
	return repository.AgentDeploymentExecutionContext{
		ProviderAccount: &repository.ProviderAccountExecutionContext{
			ProviderKey:         providerKey,
			CredentialReference: "workspace-secret://" + secretName,
		},
	}
}

func agentHarnessEvaluationEvents(runEvents []repository.RunEvent) []scoring.Event {
	events := mapRunEvents(runEvents)
	if len(runEvents) == 0 {
		now := time.Now().UTC()
		return append(events, scoring.Event{
			Type:       "system.run.completed",
			Source:     string(runevents.SourceAgentHarnessWorker),
			OccurredAt: now,
			Payload:    mustMarshalJSON(map[string]any{"final_output": "Agent Harness execution produced no canonical events."}),
		})
	}
	startedAt := runEvents[0].OccurredAt
	completedAt := runEvents[len(runEvents)-1].OccurredAt
	finalOutput := agentHarnessJudgeTranscript(runEvents)
	return append(events,
		scoring.Event{
			Type:       "system.run.started",
			Source:     string(runevents.SourceAgentHarnessWorker),
			OccurredAt: startedAt,
			Payload:    mustMarshalJSON(map[string]any{}),
		},
		scoring.Event{
			Type:       "system.run.completed",
			Source:     string(runevents.SourceAgentHarnessWorker),
			OccurredAt: completedAt,
			Payload:    mustMarshalJSON(map[string]any{"final_output": finalOutput}),
		},
	)
}

func agentHarnessJudgeTranscript(runEvents []repository.RunEvent) string {
	lines := []string{"Agent Harness execution evidence:"}
	for _, event := range runEvents {
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			continue
		}
		eventType, _ := payload["agent_harness_event_type"].(string)
		if eventType == "" {
			eventType = string(event.EventType)
		}
		switch {
		case eventType == "codex.exec.output" || eventType == "claude.exec.output" || eventType == "openclaw.exec.output" || eventType == "hermes.exec.output":
			if preview := summaryPreview(payload, "message_summary"); preview != "" {
				lines = append(lines, fmt.Sprintf("- %s: %s", eventType, preview))
			}
		case eventType == "artifact.git_diff":
			if preview := summaryPreview(payload, "diff_summary"); preview != "" {
				lines = append(lines, fmt.Sprintf("- git diff summary: %s", preview))
			}
		case eventType == "artifact.changed_files":
			if changed, ok := payload["changed_files"].(string); ok && strings.TrimSpace(changed) != "" {
				lines = append(lines, fmt.Sprintf("- changed files: %s", strings.TrimSpace(changed)))
			}
		case strings.HasPrefix(eventType, "validator."):
			if exitCode, ok := payload["exit_code"]; ok {
				lines = append(lines, fmt.Sprintf("- %s exit_code=%v", eventType, exitCode))
			}
		}
	}
	if len(lines) == 1 {
		lines = append(lines, "No agent output, diff, or validator evidence was available.")
	}
	return strings.Join(lines, "\n")
}

func summaryPreview(payload map[string]any, key string) string {
	summary, ok := payload[key].(map[string]any)
	if !ok {
		return ""
	}
	preview, _ := summary["preview"].(string)
	return strings.TrimSpace(preview)
}

func agentHarnessLLMJudges(rawJudges []json.RawMessage) ([]scoring.LLMJudgeDeclaration, error) {
	judges := make([]scoring.LLMJudgeDeclaration, 0, len(rawJudges))
	for index, raw := range rawJudges {
		var judge scoring.LLMJudgeDeclaration
		if err := json.Unmarshal(raw, &judge); err != nil {
			return nil, fmt.Errorf("decode llm_judges[%d]: %w", index, err)
		}
		if judge.Mode == "" {
			judge.Mode = scoring.JudgeMethodRubric
		}
		if strings.TrimSpace(judge.Key) == "" {
			judge.Key = fmt.Sprintf("llm_judge_%d", index+1)
		}
		if strings.TrimSpace(judge.Model) == "" && len(judge.Models) == 0 {
			judge.Model = "gpt-4.1-mini"
		}
		if len(judge.ContextFrom) == 0 {
			judge.ContextFrom = []string{"final_output"}
		}
		judges = append(judges, judge)
	}
	return judges, nil
}

func agentHarnessValidatorWorkdir(defaultWorkdir string, configured string) string {
	workdir := strings.TrimSpace(configured)
	if workdir == "" {
		return defaultWorkdir
	}
	if filepath.IsAbs(workdir) {
		return filepath.Clean(workdir)
	}
	return filepath.Join(defaultWorkdir, workdir)
}

func decodeAgentHarnessEvaluationConfig(raw json.RawMessage) (agentHarnessEvaluationConfig, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return agentHarnessEvaluationConfig{}, nil
	}
	var config agentHarnessEvaluationConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return agentHarnessEvaluationConfig{}, fmt.Errorf("decode evaluation_config: %w", err)
	}
	return config, nil
}

func validatorRequired(validator agentHarnessValidatorConfig) bool {
	return validator.Required == nil || *validator.Required
}

func agentHarnessValidatorHidden(validator agentHarnessValidatorConfig) bool {
	return validator.Hidden || validator.Private
}

func agentHarnessValidatorRedactOutput(validator agentHarnessValidatorConfig) bool {
	if agentHarnessValidatorHidden(validator) {
		return true
	}
	return validator.RedactOutput != nil && *validator.RedactOutput
}

func agentHarnessValidatorEventPayload(validator agentHarnessValidatorConfig, index int, command string, workdir string, result sandbox.ExecResult, err error) map[string]any {
	hidden := agentHarnessValidatorHidden(validator)
	redactOutput := agentHarnessValidatorRedactOutput(validator)
	payload := map[string]any{
		"index":             index,
		"working_directory": workdir,
		"exit_code":         result.ExitCode,
	}
	if hidden {
		payload["visibility"] = "hidden"
		payload["command_hidden"] = true
	} else {
		payload["command"] = command
	}
	if hidden || redactOutput {
		payload["output_redacted"] = true
	} else {
		payload["stdout"] = result.Stdout
		payload["stderr"] = result.Stderr
	}
	if err != nil {
		payload["error"] = err.Error()
	}
	return payload
}

func agentHarnessValidatorRawOutput(payload map[string]any, hidden bool) map[string]any {
	if !hidden {
		return payload
	}
	redacted := map[string]any{}
	for key, value := range payload {
		switch key {
		case "command", "stdout", "stderr":
			continue
		default:
			redacted[key] = value
		}
	}
	redacted["visibility"] = "hidden"
	redacted["command_hidden"] = true
	redacted["output_redacted"] = true
	return redacted
}

func agentHarnessScore(passed int, failed int) float64 {
	totalScored := passed + failed
	if totalScored == 0 {
		return 1
	}
	return float64(passed) / float64(totalScored)
}

func agentHarnessOutputStream(eventType string) (string, string, bool) {
	switch eventType {
	case "codex.exec":
		return "codex.exec.output", "codex", true
	case "openclaw.exec":
		return "openclaw.exec.output", "openclaw", true
	case "claude.exec":
		return "claude.exec.output", "claude", true
	case "hermes.exec":
		return "hermes.exec.output", "hermes", true
	default:
		return "", "", false
	}
}

func (a *Activities) recordAgentRunnerOutputEvents(ctx context.Context, executionID uuid.UUID, eventType string, actorType string, raw string, flush bool) (string, error) {
	return a.recordAgentRunnerOutputEventsWithOptions(ctx, executionID, eventType, actorType, raw, flush, agentHarnessCommandRecordOptions{})
}

func (a *Activities) recordAgentRunnerOutputEventsWithOptions(ctx context.Context, executionID uuid.UUID, eventType string, actorType string, raw string, flush bool, options agentHarnessCommandRecordOptions) (string, error) {
	lines := strings.Split(raw, "\n")
	remainder := ""
	if !flush {
		remainder = lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if options.RedactOutput {
			if err := a.recordAgentHarnessEvent(ctx, executionID, eventType, actorType, map[string]any{
				"stream":          "stdout",
				"output_redacted": true,
				"raw_summary": map[string]any{
					"size_bytes": len(line),
					"redacted":   true,
				},
			}); err != nil {
				return remainder, err
			}
			continue
		}
		payload := map[string]any{"stream": "stdout", "raw": line}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err == nil {
			for key, value := range decoded {
				payload[key] = value
			}
		} else {
			payload["message"] = line
		}
		if err := a.recordAgentHarnessEvent(ctx, executionID, eventType, actorType, payload); err != nil {
			return remainder, err
		}
	}
	return remainder, nil
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
	if err != nil {
		return wrapActivityError(err)
	}
	return a.recordAgentHarnessRunEvent(ctx, executionID, eventType, actorType, raw)
}

func (a *Activities) recordAgentHarnessRunEvent(ctx context.Context, executionID uuid.UUID, eventType string, actorType string, raw json.RawMessage) error {
	if a.repo == nil {
		return nil
	}
	execution, err := a.agentHarnessRepo.GetAgentHarnessExecutionByID(ctx, executionID)
	if err != nil {
		return wrapActivityError(err)
	}
	if execution.RunID == nil || execution.RunAgentID == nil {
		return nil
	}
	payload, err := agentHarnessRunEventPayload(eventType, actorType, raw)
	if err != nil {
		return err
	}
	_, err = a.repo.RecordRunEvent(ctx, repository.RecordRunEventParams{Event: runevents.Envelope{
		EventID:       fmt.Sprintf("agent-harness:%s:%s", executionID, eventType),
		SchemaVersion: runevents.SchemaVersionV1,
		RunID:         *execution.RunID,
		RunAgentID:    *execution.RunAgentID,
		EventType:     agentHarnessRunEventType(eventType),
		Source:        runevents.SourceAgentHarnessWorker,
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
		Summary: runevents.SummaryMetadata{
			SandboxAction: eventType,
			EvidenceLevel: runevents.EvidenceLevelHostedStructured,
		},
	}})
	return wrapActivityError(err)
}

func agentHarnessRunEventPayload(eventType string, actorType string, raw json.RawMessage) (json.RawMessage, error) {
	var rawPayload map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &rawPayload); err != nil {
			return nil, fmt.Errorf("decode agent harness event payload for replay: %w", err)
		}
	}
	payload := summarizeAgentHarnessRunEventPayload(rawPayload)
	payload["agent_harness_event_type"] = eventType
	payload["agent_harness_actor_type"] = actorType
	return json.Marshal(payload)
}

func summarizeAgentHarnessRunEventPayload(raw map[string]any) map[string]any {
	payload := map[string]any{}
	for key, value := range raw {
		switch key {
		case "stdout", "stderr", "diff", "raw", "message":
			if text, ok := value.(string); ok {
				payload[key+"_summary"] = summarizeAgentHarnessText(text)
			}
		case "command":
			payload[key] = summarizeAgentHarnessCommand(value)
		default:
			payload[key] = summarizeAgentHarnessValue(value)
		}
	}
	return payload
}

func summarizeAgentHarnessValue(value any) any {
	switch typed := value.(type) {
	case string:
		return truncateAgentHarnessText(redactAgentHarnessText(typed))
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, summarizeAgentHarnessValue(item))
		}
		return items
	case map[string]any:
		return summarizeAgentHarnessRunEventPayload(typed)
	default:
		return value
	}
}

func summarizeAgentHarnessCommand(value any) any {
	items, ok := value.([]any)
	if !ok {
		return summarizeAgentHarnessValue(value)
	}
	command := make([]any, 0, len(items))
	for _, item := range items {
		command = append(command, summarizeAgentHarnessValue(item))
	}
	return command
}

func summarizeAgentHarnessText(text string) map[string]any {
	redacted := redactAgentHarnessText(text)
	return map[string]any{
		"size_bytes": len(text),
		"truncated":  len(redacted) > agentHarnessReplayTextPreviewMax,
		"preview":    truncateAgentHarnessText(redacted),
	}
}

func truncateAgentHarnessText(text string) string {
	if len(text) <= agentHarnessReplayTextPreviewMax {
		return text
	}
	return text[:agentHarnessReplayTextPreviewMax]
}

func redactAgentHarnessText(text string) string {
	parts := strings.Fields(text)
	for _, part := range parts {
		if looksLikeSecret(part) {
			text = strings.ReplaceAll(text, part, "[redacted]")
		}
	}
	return text
}

func looksLikeSecret(value string) bool {
	trimmed := strings.Trim(value, `"'.,;:()[]{}<>`)
	if len(trimmed) < 16 {
		return false
	}
	secretPrefixes := []string{"sk-", "ghp_", "github_pat_", "ghs_", "glpat-", "xoxb-", "xoxp-", "AKIA"}
	for _, prefix := range secretPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(trimmed), "api_key=") || strings.Contains(strings.ToLower(trimmed), "token=")
}

func agentHarnessRunEventType(eventType string) runevents.Type {
	switch {
	case strings.HasPrefix(eventType, "scoring."):
		switch eventType {
		case "scoring.started":
			return runevents.EventTypeScoringStarted
		case "scoring.failed":
			return runevents.EventTypeScoringFailed
		default:
			return runevents.EventTypeScoringCompleted
		}
	case strings.HasSuffix(eventType, ".started"):
		return runevents.EventTypeSandboxCommandStarted
	case strings.HasSuffix(eventType, ".failed"):
		return runevents.EventTypeSandboxCommandFailed
	case eventType == "codex.exec.output" || eventType == "claude.exec.output" || eventType == "openclaw.exec.output" || eventType == "hermes.exec.output":
		return runevents.EventTypeModelOutputDelta
	case strings.HasPrefix(eventType, "artifact."):
		return runevents.EventTypeSandboxFileWritten
	default:
		return runevents.EventTypeSandboxCommandCompleted
	}
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
		switch domain.NormalizeAgentHarnessKind(h.HarnessKind) {
		case "codex_e2b":
			env["OPENAI_API_KEY"] = openAIKey
			env["CODEX_API_KEY"] = openAIKey
		case domain.AgentHarnessKindOpenClawE2B:
			applyOpenClawSecretEnv(env, openAISecretName, openAIKey)
		case domain.AgentHarnessKindClaudeE2B:
			env["ANTHROPIC_API_KEY"] = openAIKey
		case domain.AgentHarnessKindHermesE2B:
			applyHermesSecretEnv(env, openAISecretName, openAIKey)
		default:
			return nil, fmt.Errorf("unsupported agent harness kind %q", h.HarnessKind)
		}
	default:
		return nil, fmt.Errorf("unsupported agent harness auth mode %q", h.AuthMode)
	}
	return env, nil
}

func applyOpenClawSecretEnv(env map[string]string, secretName string, secretValue string) {
	upperName := strings.ToUpper(secretName)
	switch {
	case strings.Contains(upperName, "ANTHROPIC"):
		env["ANTHROPIC_API_KEY"] = secretValue
	case strings.Contains(upperName, "OPENROUTER"):
		env["OPENROUTER_API_KEY"] = secretValue
	default:
		env["OPENAI_API_KEY"] = secretValue
	}
}

func openClawProviderKey(secretName string) string {
	upperName := strings.ToUpper(strings.TrimSpace(secretName))
	switch {
	case strings.Contains(upperName, "ANTHROPIC"):
		return "anthropic"
	case strings.Contains(upperName, "OPENROUTER"):
		return "openrouter"
	default:
		return "openai"
	}
}

func applyOpenClawRunnerEnv(env map[string]string, harness agentHarnessSnapshot, timeout time.Duration) {
	if domain.NormalizeAgentHarnessKind(harness.HarnessKind) != domain.AgentHarnessKindOpenClawE2B {
		return
	}
	env["AGENTCLASH_HARNESS_TASK"] = harness.TaskPrompt
	timeoutSeconds := int(timeout / time.Second)
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultAgentHarnessTimeoutSeconds
	}
	env["AGENTCLASH_HARNESS_TIMEOUT_SECONDS"] = fmt.Sprintf("%d", timeoutSeconds)
	if harness.CodexModel != nil && strings.TrimSpace(*harness.CodexModel) != "" {
		env["AGENTCLASH_HARNESS_MODEL"] = strings.TrimSpace(*harness.CodexModel)
	}
}

func applyHermesSecretEnv(env map[string]string, secretName string, secretValue string) {
	upperName := strings.ToUpper(secretName)
	switch {
	case strings.Contains(upperName, "ANTHROPIC"):
		env["ANTHROPIC_API_KEY"] = secretValue
	case strings.Contains(upperName, "OPENROUTER"):
		env["OPENROUTER_API_KEY"] = secretValue
	case strings.Contains(upperName, "OPENAI"):
		env["OPENAI_API_KEY"] = secretValue
	default:
		env["OPENROUTER_API_KEY"] = secretValue
	}
}

func hermesProviderKey(secretName string) string {
	upperName := strings.ToUpper(strings.TrimSpace(secretName))
	switch {
	case strings.Contains(upperName, "ANTHROPIC"):
		return "anthropic"
	case strings.Contains(upperName, "OPENROUTER"):
		return "openrouter"
	case strings.Contains(upperName, "OPENAI"):
		return "openai-codex"
	default:
		return "openrouter"
	}
}

func applyHermesRunnerEnv(env map[string]string, harness agentHarnessSnapshot) {
	if domain.NormalizeAgentHarnessKind(harness.HarnessKind) != domain.AgentHarnessKindHermesE2B {
		return
	}
	env["AGENTCLASH_HARNESS_TASK"] = harness.TaskPrompt
	env["AGENTCLASH_HARNESS_PROVIDER"] = hermesProviderKey(derefString(harness.OpenAIAPIKeySecretName))
	if harness.CodexModel != nil && strings.TrimSpace(*harness.CodexModel) != "" {
		env["AGENTCLASH_HARNESS_MODEL"] = strings.TrimSpace(*harness.CodexModel)
	}
}

func agentHarnessTimeout(raw json.RawMessage) time.Duration {
	cfg := agentHarnessExecutionConfig{TimeoutSeconds: defaultAgentHarnessTimeoutSeconds}
	_ = json.Unmarshal(raw, &cfg)
	return agentHarnessTimeoutFromSeconds(cfg.TimeoutSeconds)
}

func agentHarnessTimeoutFromSeconds(timeoutSeconds int) time.Duration {
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultAgentHarnessTimeoutSeconds
	}
	return time.Duration(timeoutSeconds) * time.Second
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
