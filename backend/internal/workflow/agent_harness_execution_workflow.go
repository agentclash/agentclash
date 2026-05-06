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
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
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
		baseRef := agentHarnessGitBaseRef(harness)
		diffCommand := []string{"git", "diff", "--binary"}
		if baseRef != "" {
			diffCommand = append(diffCommand, baseRef)
		}
		if result, err := a.execHarnessCommand(ctx, execution.ID, session, "git.diff", diffCommand, workdir, 60*time.Second, env); err != nil {
			return err
		} else {
			_ = a.recordAgentHarnessEvent(ctx, execution.ID, "artifact.git_diff", "worker", map[string]any{"diff": result.Stdout})
		}
		baseChangedFiles := ""
		changedFilesCommand := []string{"git", "diff", "--name-status"}
		if baseRef != "" {
			changedFilesCommand = append(changedFilesCommand, baseRef)
		}
		if result, err := a.execHarnessCommand(ctx, execution.ID, session, "git.base_changed_files", changedFilesCommand, workdir, 60*time.Second, env); err != nil {
			return err
		} else {
			baseChangedFiles = result.Stdout
		}
		workingTreeFiles := ""
		if result, err := a.execHarnessCommand(ctx, execution.ID, session, "git.changed_files", []string{"git", "status", "--short"}, workdir, 60*time.Second, env); err != nil {
			return err
		} else {
			workingTreeFiles = result.Stdout
			_ = a.recordAgentHarnessEvent(ctx, execution.ID, "artifact.changed_files", "worker", map[string]any{
				"changed_files":        combineGitChangeLists(baseChangedFiles, workingTreeFiles),
				"base_changed_files":   baseChangedFiles,
				"working_tree_changes": workingTreeFiles,
			})
		}
		if isStructuredGitHubHarness(harness) {
			changes := agentHarnessGitChanges{
				ChangedFiles:       combineGitChangeLists(baseChangedFiles, workingTreeFiles),
				WorkingTreeChanges: workingTreeFiles,
			}
			if err := a.createGitHubPullRequest(ctx, execution.ID, session, harness, workdir, timeout, gitEnv, gitHubToken, changes); err != nil {
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

func agentHarnessToolName(h agentHarnessSnapshot) string {
	switch normalizeAgentHarnessKind(h.HarnessKind) {
	case "claude_e2b":
		return "claude"
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

func (a *Activities) createGitHubPullRequest(ctx context.Context, executionID uuid.UUID, session sandbox.Session, harness agentHarnessSnapshot, workdir string, timeout time.Duration, gitEnv map[string]string, token string, changes agentHarnessGitChanges) error {
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

func (a *Activities) execAgentHarnessShellCommand(ctx context.Context, executionID uuid.UUID, session sandbox.Session, eventType string, command string, configuredWorkdir string, timeoutSeconds int, defaultWorkdir string, defaultTimeout time.Duration, env map[string]string) (sandbox.ExecResult, string, error) {
	workdir := agentHarnessValidatorWorkdir(defaultWorkdir, configuredWorkdir)
	timeout := defaultTimeout
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}
	result, err := a.execHarnessCommand(ctx, executionID, session, eventType, []string{"bash", "-lc", command}, workdir, timeout, env)
	return result, workdir, err
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

func (a *Activities) evaluateCommandValidator(ctx context.Context, executionID uuid.UUID, session sandbox.Session, validator agentHarnessValidatorConfig, index int, defaultWorkdir string, defaultTimeout time.Duration, env map[string]string) (bool, error) {
	command := strings.TrimSpace(validator.Command)
	if command == "" {
		err := fmt.Errorf("command validator %d is missing command", index)
		_ = a.recordAgentHarnessEvent(ctx, executionID, "validator.command.failed", "worker", map[string]any{"index": index, "error": err.Error()})
		return false, err
	}
	result, workdir, err := a.execAgentHarnessShellCommand(ctx, executionID, session, "validator.command.exec", command, validator.WorkingDirectory, validator.TimeoutSeconds, defaultWorkdir, defaultTimeout, env)
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
	case eventType == "codex.exec.output" || eventType == "claude.exec.output":
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
