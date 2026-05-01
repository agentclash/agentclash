package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

func TestExecuteAgentHarnessExecutionRunsCodexAndRecordsTrace(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		TaskPrompt:             "implement issue 462",
		CodexTemplate:          "codex",
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &openAISecret,
		RepositoryURL:          stringPtr("https://github.com/acme/repo"),
		BaseBranch:             stringPtr("main"),
		ExecutionConfig:        json.RawMessage(`{"timeout_seconds":120}`),
		EvaluationConfig:       json.RawMessage(`{"validators":[{"type":"command","command":"go test ./...","working_directory":"backend","timeout_seconds":60}]}`),
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:                      executionID,
		WorkspaceID:             workspaceID,
		AgentHarnessID:          harnessID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{
		openAISecret: "sk-test",
	})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch {
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "clone":
			return sandbox.ExecResult{ExitCode: 0, Stdout: "cloned"}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "checkout":
			return sandbox.ExecResult{ExitCode: 0, Stdout: "checked out"}, nil
		case len(request.Command) >= 2 && request.Command[0] == "codex" && request.Command[1] == "exec":
			if request.Environment["CODEX_API_KEY"] != "sk-test" || request.Environment["OPENAI_API_KEY"] != "sk-test" {
				t.Fatalf("codex env = %#v, want OpenAI and Codex keys", request.Environment)
			}
			if !containsString(request.Command, "-C") || !containsString(request.Command, agentHarnessWorkspaceDir) {
				t.Fatalf("codex command = %#v, want -C workspace", request.Command)
			}
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"final","message":"done"}`}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "add" && request.Command[2] == "--intent-to-add":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "diff":
			return sandbox.ExecResult{ExitCode: 0, Stdout: "diff --git a/file b/file"}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "status":
			return sandbox.ExecResult{ExitCode: 0, Stdout: " M file"}, nil
		case len(request.Command) >= 3 && request.Command[0] == "bash" && request.Command[1] == "-lc" && request.Command[2] == "go test ./...":
			if request.WorkingDirectory != agentHarnessWorkspaceDir+"/backend" {
				t.Fatalf("validator workdir = %q, want backend under workspace", request.WorkingDirectory)
			}
			return sandbox.ExecResult{ExitCode: 0, Stdout: "ok"}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})
	provider := &sandbox.FakeProvider{NextSession: session}
	activities := NewActivities(repo, FakeWorkHooks{}).WithSandboxProvider(provider)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if err != nil {
		t.Fatalf("ExecuteAgentHarnessExecution error: %v", err)
	}

	if len(provider.CreateRequests) != 1 {
		t.Fatalf("sandbox create calls = %d, want 1", len(provider.CreateRequests))
	}
	createRequest := provider.CreateRequests[0]
	if createRequest.EnvVars["OPENAI_API_KEY"] != "sk-test" || createRequest.EnvVars["CODEX_API_KEY"] != "sk-test" {
		t.Fatalf("env vars = %#v, want OpenAI and Codex keys", createRequest.EnvVars)
	}
	if createRequest.TemplateID != "codex" {
		t.Fatalf("template id = %q, want codex", createRequest.TemplateID)
	}
	if session.DestroyCalls() != 1 {
		t.Fatalf("destroy calls = %d, want 1", session.DestroyCalls())
	}
	calls := session.ExecCalls()
	addIntentIndex := commandIndex(calls, "git", "add", "--intent-to-add")
	diffIndex := commandIndex(calls, "git", "diff", "--binary")
	if addIntentIndex == -1 {
		t.Fatal("expected git add --intent-to-add before diff capture")
	}
	if diffIndex == -1 {
		t.Fatal("expected git diff --binary capture")
	}
	if addIntentIndex > diffIndex {
		t.Fatalf("git add --intent-to-add call index = %d, diff index = %d; want add before diff", addIntentIndex, diffIndex)
	}
	if got := len(repo.agentHarnessEvents[executionID]); got < 8 {
		t.Fatalf("recorded events = %d, want at least 8", got)
	}
	var sawCodexOutput bool
	var sawValidatorPassed bool
	var sawScoringCompleted bool
	for _, event := range repo.agentHarnessEvents[executionID] {
		switch event.EventType {
		case "codex.exec.output":
			sawCodexOutput = true
			var payload map[string]any
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatalf("decode codex output payload: %v", err)
			}
			if payload["type"] != "final" || payload["message"] != "done" {
				t.Fatalf("codex output payload = %#v, want final done", payload)
			}
		case "validator.command.passed":
			sawValidatorPassed = true
		case "scoring.completed":
			sawScoringCompleted = true
		}
	}
	if !sawCodexOutput {
		t.Fatal("expected live codex output event")
	}
	if !sawValidatorPassed {
		t.Fatal("expected validator pass event")
	}
	if !sawScoringCompleted {
		t.Fatal("expected scoring completed event")
	}
}

func TestExecuteAgentHarnessExecutionFailsRequiredValidator(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		TaskPrompt:             "implement issue 462",
		CodexTemplate:          "codex",
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &openAISecret,
		RepositoryURL:          stringPtr("https://github.com/acme/repo"),
		ExecutionConfig:        json.RawMessage(`{"timeout_seconds":120}`),
		EvaluationConfig:       json.RawMessage(`{"validators":[{"type":"command","command":"go test ./..."},{"type":"command","command":"npm test"}]}`),
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:                      executionID,
		WorkspaceID:             workspaceID,
		AgentHarnessID:          harnessID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{openAISecret: "sk-test"})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch {
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "clone":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "codex" && request.Command[1] == "exec":
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"final","message":"done"}`}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "add" && request.Command[2] == "--intent-to-add":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "diff":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "status":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 3 && request.Command[0] == "bash" && request.Command[1] == "-lc" && request.Command[2] == "go test ./...":
			return sandbox.ExecResult{ExitCode: 0, Stdout: "ok"}, nil
		case len(request.Command) >= 3 && request.Command[0] == "bash" && request.Command[1] == "-lc" && request.Command[2] == "npm test":
			return sandbox.ExecResult{ExitCode: 1, Stderr: "failed"}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})
	provider := &sandbox.FakeProvider{NextSession: session}
	activities := NewActivities(repo, FakeWorkHooks{}).WithSandboxProvider(provider)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if err == nil {
		t.Fatal("expected validator failure")
	}
	validatorFailedCount := 0
	var sawPartialScore bool
	for _, event := range repo.agentHarnessEvents[executionID] {
		switch event.EventType {
		case "validator.command.failed":
			validatorFailedCount++
		case "scoring.completed":
			var payload map[string]any
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatalf("decode scoring payload: %v", err)
			}
			if payload["score"] == float64(0.5) {
				sawPartialScore = true
			}
		}
	}
	if validatorFailedCount != 1 {
		t.Fatalf("validator.command.failed events = %d, want 1", validatorFailedCount)
	}
	if !sawPartialScore {
		t.Fatal("expected partial score event")
	}
}

func TestExecuteAgentHarnessExecutionWithoutRepositorySkipsGitArtifactCapture(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		TaskPrompt:             "write a file without a repository",
		CodexTemplate:          "codex",
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &openAISecret,
		ExecutionConfig:        json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:                      executionID,
		WorkspaceID:             workspaceID,
		AgentHarnessID:          harnessID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{openAISecret: "sk-test"})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch {
		case len(request.Command) >= 2 && request.Command[0] == "codex" && request.Command[1] == "exec":
			if request.WorkingDirectory != "/" {
				t.Fatalf("codex workdir = %q, want root for no-repo harness", request.WorkingDirectory)
			}
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"final","message":"done"}`}, nil
		default:
			t.Fatalf("unexpected command for no-repo harness: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})
	provider := &sandbox.FakeProvider{NextSession: session}
	activities := NewActivities(repo, FakeWorkHooks{}).WithSandboxProvider(provider)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if err != nil {
		t.Fatalf("ExecuteAgentHarnessExecution error: %v", err)
	}

	for _, call := range session.ExecCalls() {
		if len(call.Command) > 0 && call.Command[0] == "git" {
			t.Fatalf("no-repo harness should not run git artifact commands, saw %#v", call.Command)
		}
	}
}

func TestExecuteAgentHarnessExecutionFailsEarlyWhenGitHubAccessRevoked(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repositoryID := int64(100)
	installationID := int64(42)
	provider := "github"
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.githubRepoErr = repository.ErrGitHubRepositoryNotInstalled
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		TaskPrompt:             "implement issue 462",
		CodexTemplate:          "codex",
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &openAISecret,
		RepositoryURL:          stringPtr("https://github.com/acme/repo"),
		RepositoryProvider:     &provider,
		GitHubRepositoryID:     &repositoryID,
		GitHubInstallationID:   &installationID,
		RepositoryFullName:     stringPtr("acme/repo"),
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:                      executionID,
		WorkspaceID:             workspaceID,
		AgentHarnessID:          harnessID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{openAISecret: "sk-test"})

	providerStub := &sandbox.FakeProvider{NextSession: sandbox.NewFakeSession("sandbox-1")}
	activities := NewActivities(repo, FakeWorkHooks{}).WithSandboxProvider(providerStub)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if !errors.Is(err, repository.ErrGitHubRepositoryNotInstalled) {
		t.Fatalf("ExecuteAgentHarnessExecution error = %v, want ErrGitHubRepositoryNotInstalled", err)
	}
	if len(providerStub.CreateRequests) != 0 {
		t.Fatalf("sandbox creates = %d, want none before access preflight passes", len(providerStub.CreateRequests))
	}
	var sawRevokedEvent bool
	for _, event := range repo.agentHarnessEvents[executionID] {
		if event.EventType == "github.repository_access_revoked" {
			sawRevokedEvent = true
			break
		}
	}
	if !sawRevokedEvent {
		t.Fatal("expected github.repository_access_revoked event")
	}
}

func TestAgentHarnessTimeoutDefaults(t *testing.T) {
	if got := agentHarnessTimeout(nil); got != 30*time.Minute {
		t.Fatalf("default timeout = %s, want 30m", got)
	}
	if got := agentHarnessTimeout(json.RawMessage(`{"timeout_seconds":5}`)); got != 5*time.Second {
		t.Fatalf("configured timeout = %s, want 5s", got)
	}
}

func TestAgentHarnessExecutionActivityOptionsUseHarnessTimeout(t *testing.T) {
	options := agentHarnessExecutionActivityOptions(120)
	want := 120*time.Second + agentHarnessActivityTimeoutBuffer
	if options.StartToCloseTimeout != want {
		t.Fatalf("start to close timeout = %s, want %s", options.StartToCloseTimeout, want)
	}
	if options.RetryPolicy == nil || options.RetryPolicy.MaximumAttempts != 1 {
		t.Fatalf("retry policy maximum attempts = %#v, want 1", options.RetryPolicy)
	}

	defaultOptions := agentHarnessExecutionActivityOptions(0)
	defaultWant := 30*time.Minute + agentHarnessActivityTimeoutBuffer
	if defaultOptions.StartToCloseTimeout != defaultWant {
		t.Fatalf("default start to close timeout = %s, want %s", defaultOptions.StartToCloseTimeout, defaultWant)
	}
}

func TestAgentHarnessValidatorWorkdir(t *testing.T) {
	tests := []struct {
		name           string
		defaultWorkdir string
		configured     string
		want           string
	}{
		{
			name:           "empty uses default",
			defaultWorkdir: "/workspace",
			configured:     "",
			want:           "/workspace",
		},
		{
			name:           "relative joins default",
			defaultWorkdir: "/workspace",
			configured:     "cli",
			want:           "/workspace/cli",
		},
		{
			name:           "relative cleans path",
			defaultWorkdir: "/workspace/repo",
			configured:     "./backend/../cli",
			want:           "/workspace/repo/cli",
		},
		{
			name:           "absolute preserved",
			defaultWorkdir: "/workspace",
			configured:     "/tmp/project",
			want:           "/tmp/project",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := agentHarnessValidatorWorkdir(tt.defaultWorkdir, tt.configured); got != tt.want {
				t.Fatalf("agentHarnessValidatorWorkdir(%q, %q) = %q, want %q", tt.defaultWorkdir, tt.configured, got, tt.want)
			}
		})
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func commandIndex(calls []sandbox.ExecRequest, parts ...string) int {
	for index, call := range calls {
		if len(call.Command) < len(parts) {
			continue
		}
		matches := true
		for partIndex, part := range parts {
			if call.Command[partIndex] != part {
				matches = false
				break
			}
		}
		if matches {
			return index
		}
	}
	return -1
}
