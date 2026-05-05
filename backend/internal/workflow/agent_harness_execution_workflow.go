package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/maputil"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

const (
	agentHarnessWorkspaceDir          = "/workspace"
	agentHarnessActivityTimeoutBuffer = 2 * time.Minute
	defaultAgentHarnessTimeoutSeconds = 1800
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
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

type agentHarnessEvaluationConfig struct {
	Validators []agentHarnessValidatorConfig `json:"validators,omitempty"`
	LLMJudges  []json.RawMessage             `json:"llm_judges,omitempty"`
}

type agentHarnessValidatorConfig struct {
	Type             string `json:"type"`
	Command          string `json:"command,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`
	TimeoutSeconds   int    `json:"timeout_seconds,omitempty"`
	Required         *bool  `json:"required,omitempty"`
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
	} else {
		workdir = "/"
	}

	runner, err := agentHarnessRunnerFor(harness, workdir)
	if err != nil {
		return err
	}
	runnerResult, err := a.execHarnessCommand(ctx, execution.ID, session, runner.EventType, runner.Command, workdir, timeout, env)
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
		if result, err := a.execHarnessCommand(ctx, execution.ID, session, "git.diff", []string{"git", "diff", "--binary"}, workdir, 60*time.Second, env); err != nil {
			return err
		} else {
			_ = a.recordAgentHarnessEvent(ctx, execution.ID, "artifact.git_diff", "worker", map[string]any{"diff": result.Stdout})
		}
		changedFiles := ""
		if result, err := a.execHarnessCommand(ctx, execution.ID, session, "git.changed_files", []string{"git", "status", "--short"}, workdir, 60*time.Second, env); err != nil {
			return err
		} else {
			changedFiles = result.Stdout
			_ = a.recordAgentHarnessEvent(ctx, execution.ID, "artifact.changed_files", "worker", map[string]any{"changed_files": result.Stdout})
		}
		if isStructuredGitHubHarness(harness) {
			if err := a.createGitHubPullRequest(ctx, execution.ID, session, harness, workdir, timeout, gitEnv, gitHubToken, changedFiles); err != nil {
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
	if err := a.evaluateAgentHarnessExecution(ctx, execution.ID, session, harness.EvaluationConfig, workdir, timeout, env); err != nil {
		return err
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
	switch normalizeAgentHarnessKind(h.HarnessKind) {
	case "codex_e2b":
		return agentHarnessRunner{
			DisplayName: "codex exec",
			EventType:   "codex.exec",
			Command:     []string{"codex", "exec", "--full-auto", "--skip-git-repo-check", "--json", "-C", workdir, h.TaskPrompt},
		}, nil
	case "claude_e2b":
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
	default:
		return agentHarnessRunner{}, fmt.Errorf("unsupported agent harness kind %q", h.HarnessKind)
	}
}

func normalizeAgentHarnessKind(kind string) string {
	trimmed := strings.TrimSpace(kind)
	if trimmed == "" {
		return "codex_e2b"
	}
	return trimmed
}

func (a *Activities) createGitHubPullRequest(ctx context.Context, executionID uuid.UUID, session sandbox.Session, harness agentHarnessSnapshot, workdir string, timeout time.Duration, gitEnv map[string]string, token string, changedFiles string) error {
	if strings.TrimSpace(changedFiles) == "" {
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
	commands := []struct {
		event   string
		command []string
	}{
		{"git.config_user_email", []string{"git", "config", "user.email", "agentclash[bot]@users.noreply.github.com"}},
		{"git.config_user_name", []string{"git", "config", "user.name", "agentclash[bot]"}},
		{"git.create_branch", []string{"git", "checkout", "-B", branch}},
		{"git.add_all", []string{"git", "add", "--all"}},
		{"git.commit", []string{"git", "-c", "core.hooksPath=/dev/null", "commit", "-m", "AgentClash harness changes"}},
		{"git.push_branch", []string{"git", "-c", "core.hooksPath=/dev/null", "-c", "credential.helper=", "push", pushURL, "HEAD:refs/heads/" + branch}},
	}
	for _, step := range commands {
		stepEnv := map[string]string(nil)
		if step.event == "git.push_branch" {
			stepEnv = gitEnv
		}
		if result, err := a.execHarnessCommand(ctx, executionID, session, step.event, step.command, workdir, timeout, stepEnv); err != nil {
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

func (a *Activities) execHarnessCommand(ctx context.Context, executionID uuid.UUID, session sandbox.Session, eventType string, command []string, workdir string, timeout time.Duration, env map[string]string) (sandbox.ExecResult, error) {
	if err := a.recordAgentHarnessEvent(ctx, executionID, eventType+".started", "worker", map[string]any{"command": command, "working_directory": workdir}); err != nil {
		return sandbox.ExecResult{}, err
	}
	stdoutRemainder := ""
	outputEventType, outputActor, streamOutput := agentHarnessOutputStream(eventType)
	onStdout := func(chunk []byte) error {
		if !streamOutput {
			return nil
		}
		remainder, err := a.recordAgentRunnerOutputEvents(ctx, executionID, outputEventType, outputActor, stdoutRemainder+string(chunk), false)
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
		if _, parseErr := a.recordAgentRunnerOutputEvents(ctx, executionID, outputEventType, outputActor, stdoutRemainder, true); err == nil && parseErr != nil {
			return sandbox.ExecResult{}, parseErr
		}
	}
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

func (a *Activities) evaluateAgentHarnessExecution(ctx context.Context, executionID uuid.UUID, session sandbox.Session, rawConfig json.RawMessage, workdir string, defaultTimeout time.Duration, env map[string]string) error {
	config, err := decodeAgentHarnessEvaluationConfig(rawConfig)
	if err != nil {
		_ = a.recordAgentHarnessEvent(ctx, executionID, "scoring.failed", "worker", map[string]any{"error": err.Error()})
		return err
	}
	if len(config.Validators) == 0 && len(config.LLMJudges) == 0 {
		return a.recordAgentHarnessEvent(ctx, executionID, "scoring.skipped", "worker", map[string]any{"reason": "evaluation_config has no validators or llm_judges"})
	}

	passed := 0
	failed := 0
	skipped := 0
	for index, validator := range config.Validators {
		switch strings.TrimSpace(validator.Type) {
		case "command":
			ok, err := a.evaluateCommandValidator(ctx, executionID, session, validator, index, workdir, defaultTimeout, env)
			if err != nil {
				failed++
				if validatorRequired(validator) {
					_ = a.recordAgentHarnessEvent(ctx, executionID, "scoring.completed", "worker", map[string]any{"passed": passed, "failed": failed, "skipped": skipped, "score": agentHarnessScore(passed, failed)})
					return err
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
	if len(config.LLMJudges) > 0 {
		skipped += len(config.LLMJudges)
		if err := a.recordAgentHarnessEvent(ctx, executionID, "llm_judges.skipped", "worker", map[string]any{"count": len(config.LLMJudges), "reason": "agent harness LLM judge scoring is not wired yet"}); err != nil {
			return err
		}
	}

	return a.recordAgentHarnessEvent(ctx, executionID, "scoring.completed", "worker", map[string]any{"passed": passed, "failed": failed, "skipped": skipped, "score": agentHarnessScore(passed, failed)})
}

func (a *Activities) evaluateCommandValidator(ctx context.Context, executionID uuid.UUID, session sandbox.Session, validator agentHarnessValidatorConfig, index int, defaultWorkdir string, defaultTimeout time.Duration, env map[string]string) (bool, error) {
	command := strings.TrimSpace(validator.Command)
	if command == "" {
		err := fmt.Errorf("command validator %d is missing command", index)
		_ = a.recordAgentHarnessEvent(ctx, executionID, "validator.command.failed", "worker", map[string]any{"index": index, "error": err.Error()})
		return false, err
	}
	workdir := agentHarnessValidatorWorkdir(defaultWorkdir, validator.WorkingDirectory)
	timeout := defaultTimeout
	if validator.TimeoutSeconds > 0 {
		timeout = time.Duration(validator.TimeoutSeconds) * time.Second
	}

	result, err := a.execHarnessCommand(ctx, executionID, session, "validator.command.exec", []string{"bash", "-lc", command}, workdir, timeout, env)
	payload := map[string]any{
		"index":             index,
		"command":           command,
		"working_directory": workdir,
	}
	if err != nil {
		payload["error"] = err.Error()
		_ = a.recordAgentHarnessEvent(ctx, executionID, "validator.command.failed", "worker", payload)
		return false, err
	}
	payload["exit_code"] = result.ExitCode
	payload["stdout"] = result.Stdout
	payload["stderr"] = result.Stderr
	if result.ExitCode == 0 {
		return true, a.recordAgentHarnessEvent(ctx, executionID, "validator.command.passed", "worker", payload)
	}
	err = fmt.Errorf("command validator %d failed with exit code %d", index, result.ExitCode)
	payload["error"] = err.Error()
	_ = a.recordAgentHarnessEvent(ctx, executionID, "validator.command.failed", "worker", payload)
	return false, err
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
	case "claude.exec":
		return "claude.exec.output", "claude", true
	default:
		return "", "", false
	}
}

func (a *Activities) recordAgentRunnerOutputEvents(ctx context.Context, executionID uuid.UUID, eventType string, actorType string, raw string, flush bool) (string, error) {
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
		switch normalizeAgentHarnessKind(h.HarnessKind) {
		case "codex_e2b":
			env["OPENAI_API_KEY"] = openAIKey
			env["CODEX_API_KEY"] = openAIKey
		case "claude_e2b":
			env["ANTHROPIC_API_KEY"] = openAIKey
		default:
			return nil, fmt.Errorf("unsupported agent harness kind %q", h.HarnessKind)
		}
	default:
		return nil, fmt.Errorf("unsupported agent harness auth mode %q", h.AuthMode)
	}
	return env, nil
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
