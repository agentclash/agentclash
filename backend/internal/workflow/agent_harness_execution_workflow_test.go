package workflow

import (
	"context"
	"encoding/json"
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
		EvaluationConfig:       json.RawMessage(`{}`),
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
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"final","message":"done"}`}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "diff":
			return sandbox.ExecResult{ExitCode: 0, Stdout: "diff --git a/file b/file"}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "status":
			return sandbox.ExecResult{ExitCode: 0, Stdout: " M file"}, nil
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
	if got := len(repo.agentHarnessEvents[executionID]); got < 8 {
		t.Fatalf("recorded events = %d, want at least 8", got)
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
