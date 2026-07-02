package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/sandbox"
	"github.com/google/uuid"
)

func TestExecuteAgentHarnessExecutionRunsCodexAndRecordsTrace(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	runID := uuid.New()
	runAgentID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusQueued), fixtureRunAgent(runID, runAgentID, 0))
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
		ExecutionConfig:        json.RawMessage(`{"timeout_seconds":120,"setup_commands":[{"name":"deps","command":"go mod download","working_directory":"backend","timeout_seconds":30}]}`),
		EvaluationConfig:       json.RawMessage(`{"validators":[{"key":"tests","type":"command","command":"go test ./...","working_directory":"backend","timeout_seconds":60}],"llm_judges":[{"key":"autonomy","rubric":"Score whether the coding agent completed the requested task coherently."}]}`),
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:                      executionID,
		WorkspaceID:             workspaceID,
		AgentHarnessID:          harnessID,
		RunID:                   &runID,
		RunAgentID:              &runAgentID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120,"setup_commands":[{"name":"deps","command":"go mod download","working_directory":"backend","timeout_seconds":30}]}`),
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
		case len(request.Command) >= 3 && request.Command[0] == "bash" && request.Command[1] == "-lc" && strings.Contains(request.Command[2], ".devcontainer/devcontainer.json"):
			return sandbox.ExecResult{ExitCode: 0, Stdout: "go.mod\npackage.json\n"}, nil
		case len(request.Command) >= 3 && request.Command[0] == "bash" && request.Command[1] == "-lc" && request.Command[2] == "go mod download":
			if request.WorkingDirectory != agentHarnessWorkspaceDir+"/backend" {
				t.Fatalf("setup workdir = %q, want backend under workspace", request.WorkingDirectory)
			}
			return sandbox.ExecResult{ExitCode: 0, Stdout: "downloaded"}, nil
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
	sandboxProvider := &sandbox.FakeProvider{NextSession: session}
	judgeClient := &provider.FakeClient{
		Response: provider.Response{OutputText: `{"score":5,"confidence":"high","reasoning":"complete"}`},
	}
	activities := NewActivities(repo, FakeWorkHooks{}, judgeClient).WithSandboxProvider(sandboxProvider)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if err != nil {
		t.Fatalf("ExecuteAgentHarnessExecution error: %v", err)
	}

	if len(sandboxProvider.CreateRequests) != 1 {
		t.Fatalf("sandbox create calls = %d, want 1", len(sandboxProvider.CreateRequests))
	}
	createRequest := sandboxProvider.CreateRequests[0]
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
	setupIndex := commandIndex(calls, "bash", "-lc", "go mod download")
	codexIndex := commandIndex(calls, "codex", "exec")
	if addIntentIndex == -1 {
		t.Fatal("expected git add --intent-to-add before diff capture")
	}
	if diffIndex == -1 {
		t.Fatal("expected git diff --binary capture")
	}
	if addIntentIndex > diffIndex {
		t.Fatalf("git add --intent-to-add call index = %d, diff index = %d; want add before diff", addIntentIndex, diffIndex)
	}
	if setupIndex == -1 || codexIndex == -1 || setupIndex > codexIndex {
		t.Fatalf("setup command index = %d, codex index = %d; want setup before codex", setupIndex, codexIndex)
	}
	if got := len(repo.agentHarnessEvents[executionID]); got < 8 {
		t.Fatalf("recorded events = %d, want at least 8", got)
	}
	var sawCodexOutput bool
	var sawValidatorPassed bool
	var sawScoringCompleted bool
	var sawSetupCompleted bool
	var sawHints bool
	var sawScorecardPersisted bool
	var sawLLMJudgesSkipped bool
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
		case "setup.command.completed":
			sawSetupCompleted = true
		case "setup.hints.detected":
			sawHints = true
			var payload map[string]any
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatalf("decode setup hints payload: %v", err)
			}
			hints, ok := payload["hints"].([]any)
			if !ok || len(hints) != 2 {
				t.Fatalf("setup hints = %#v, want two structured hints", payload["hints"])
			}
			first, ok := hints[0].(map[string]any)
			if !ok || first["kind"] != "go" || first["path"] != "go.mod" {
				t.Fatalf("first setup hint = %#v, want go.mod kind go", hints[0])
			}
		case "setup.runtime.detected":
			var payload map[string]any
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatalf("decode setup runtime payload: %v", err)
			}
			if payload["template"] != "codex" || payload["agent_tool"] != "codex" || payload["metadata_version"] != float64(1) {
				t.Fatalf("runtime payload = %#v, want template/tool metadata", payload)
			}
		case "scorecard.persisted":
			sawScorecardPersisted = true
		case "llm_judges.skipped":
			sawLLMJudgesSkipped = true
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
	if !sawSetupCompleted {
		t.Fatal("expected setup command completed event")
	}
	if !sawHints {
		t.Fatal("expected setup hints detected event")
	}
	if !sawScorecardPersisted {
		t.Fatal("expected persisted scorecard event")
	}
	if sawLLMJudgesSkipped {
		t.Fatal("did not expect llm_judges.skipped after judge wiring")
	}
	evaluation, ok := repo.evaluations[runAgentID]
	if !ok {
		t.Fatal("expected stored run-agent evaluation")
	}
	if len(evaluation.ValidatorResults) != 1 || evaluation.ValidatorResults[0].Key != "tests" || evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("validator results = %#v, want passing tests validator", evaluation.ValidatorResults)
	}
	if len(evaluation.LLMJudgeResults) != 1 || evaluation.LLMJudgeResults[0].JudgeKey != "autonomy" {
		t.Fatalf("llm judge results = %#v, want autonomy judge result", evaluation.LLMJudgeResults)
	}
	if evaluation.LLMJudgeResults[0].NormalizedScore == nil || *evaluation.LLMJudgeResults[0].NormalizedScore != 1 {
		t.Fatalf("llm judge score = %#v, want normalized 1", evaluation.LLMJudgeResults[0].NormalizedScore)
	}
	if len(judgeClient.Requests) != 3 {
		t.Fatalf("judge requests = %d, want default 3 samples", len(judgeClient.Requests))
	}
	if !strings.Contains(judgeClient.Requests[0].Messages[0].Content, "done") {
		t.Fatalf("judge prompt = %q, want agent output evidence", judgeClient.Requests[0].Messages[0].Content)
	}
	if judgeClient.Requests[0].CredentialReference != "workspace-secret://OPENAI_API_KEY" {
		t.Fatalf("judge credential reference = %q, want harness workspace secret", judgeClient.Requests[0].CredentialReference)
	}
	if evaluation.OverallScore == nil || *evaluation.OverallScore != 1 {
		t.Fatalf("overall score = %#v, want 1 from command validator", evaluation.OverallScore)
	}
	if _, ok := evaluation.DimensionScores["latency"]; !ok {
		t.Fatalf("dimension scores = %#v, want latency dimension exposed", evaluation.DimensionScores)
	}
	if _, ok := evaluation.DimensionScores["cost"]; !ok {
		t.Fatalf("dimension scores = %#v, want cost dimension exposed", evaluation.DimensionScores)
	}
	if repo.callCountWithPrefix("CreateStandaloneEvaluationSpec:") != 1 {
		t.Fatalf("CreateStandaloneEvaluationSpec call count = %d, want 1", repo.callCountWithPrefix("CreateStandaloneEvaluationSpec:"))
	}
	updatedExecution, ok := repo.agentHarnessExecutions[executionID]
	if !ok || updatedExecution.EvaluationSpecID == nil || *updatedExecution.EvaluationSpecID != evaluation.EvaluationSpecID {
		t.Fatalf("execution evaluation_spec_id = %#v, want %s", updatedExecution.EvaluationSpecID, evaluation.EvaluationSpecID)
	}
	runEvents := repo.runEvents[runAgentID]
	if len(runEvents) == 0 {
		t.Fatal("expected agent harness events mirrored to canonical run_events")
	}
	if runEvents[0].RunID != runID {
		t.Fatalf("mirrored run_id = %s, want %s", runEvents[0].RunID, runID)
	}
	var mirroredPayload map[string]any
	if err := json.Unmarshal(runEvents[0].Payload, &mirroredPayload); err != nil {
		t.Fatalf("decode mirrored payload: %v", err)
	}
	if mirroredPayload["agent_harness_event_type"] == "" {
		t.Fatalf("mirrored payload missing harness event type: %#v", mirroredPayload)
	}
	for _, event := range runEvents {
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode mirrored run event payload: %v", err)
		}
		for _, rawKey := range []string{"stdout", "stderr", "diff", "raw"} {
			if _, ok := payload[rawKey]; ok {
				t.Fatalf("mirrored run event payload leaked raw %s: %#v", rawKey, payload)
			}
		}
	}
	if repo.callCountWithPrefix("BuildRunAgentReplay:"+runAgentID.String()) != 1 {
		t.Fatalf("BuildRunAgentReplay call count = %d, want 1", repo.callCountWithPrefix("BuildRunAgentReplay:"+runAgentID.String()))
	}
}

func TestExecuteAgentHarnessExecutionRedactsHiddenValidatorsAndRecordsPrivacyControls(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	runID := uuid.New()
	runAgentID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusQueued), fixtureRunAgent(runID, runAgentID, 0))
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		TaskPrompt:             "fix the private regression",
		CodexTemplate:          "codex",
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &openAISecret,
		RepositoryURL:          stringPtr("https://github.com/acme/private-repo"),
		BaseBranch:             stringPtr("main"),
		EvaluationConfig: json.RawMessage(`{
			"result":{"kind":"private_task_bank","benchmark_source":"customer-incident-bank","collection_date":"2026-05-01","allowed_public_context":["README.md"],"contamination":"hidden","publicity":"private"},
			"privacy":{"redact_replay":true,"redact_artifacts":true,"retention_days":14,"audit_log":true,"provider_data_use":"zero_retention","workspace_policy_key":"strict"},
			"validators":[{"key":"secret-tests","type":"command","command":"go test ./private/... --golden SECRET_EXPECTED","hidden":true}]
		}`),
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:             executionID,
		WorkspaceID:    workspaceID,
		AgentHarnessID: harnessID,
		RunID:          &runID,
		RunAgentID:     &runAgentID,
		Status:         string(repository.AgentHarnessExecutionStatusRunning),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{openAISecret: "sk-test"})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch {
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "clone":
			return sandbox.ExecResult{ExitCode: 0, Stdout: "cloned"}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "checkout":
			return sandbox.ExecResult{ExitCode: 0, Stdout: "checked out"}, nil
		case len(request.Command) >= 3 && request.Command[0] == "bash" && request.Command[1] == "-lc" && strings.Contains(request.Command[2], ".devcontainer/devcontainer.json"):
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "codex" && request.Command[1] == "exec":
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"final","message":"PRIVATE_AGENT_OUTPUT"}`}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "add" && request.Command[2] == "--intent-to-add":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "diff":
			return sandbox.ExecResult{ExitCode: 0, Stdout: "diff --git a/private b/private\n+PRIVATE_DIFF"}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "status":
			return sandbox.ExecResult{ExitCode: 0, Stdout: " M private/file.go"}, nil
		case len(request.Command) >= 3 && request.Command[0] == "bash" && request.Command[1] == "-lc" && strings.Contains(request.Command[2], "SECRET_EXPECTED"):
			return sandbox.ExecResult{ExitCode: 0, Stdout: "SECRET_EXPECTED passed", Stderr: "secret stderr"}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})
	activities := NewActivities(repo, FakeWorkHooks{}).WithSandboxProvider(&sandbox.FakeProvider{NextSession: session})
	if err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID}); err != nil {
		t.Fatalf("ExecuteAgentHarnessExecution error: %v", err)
	}

	for _, event := range repo.agentHarnessEvents[executionID] {
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode event payload: %v", err)
		}
		encoded := string(event.Payload)
		if containsAny(encoded, "SECRET_EXPECTED", "secret stderr", "PRIVATE_AGENT_OUTPUT", "PRIVATE_DIFF", "private/file.go") {
			t.Fatalf("event %s leaked private data: %s", event.EventType, encoded)
		}
		if event.EventType == "validator.command.passed" {
			if payload["command_hidden"] != true || payload["output_redacted"] != true || payload["visibility"] != "hidden" {
				t.Fatalf("hidden validator payload = %#v, want hidden redaction markers", payload)
			}
		}
	}
	if !agentHarnessEventRecorded(repo.agentHarnessEvents[executionID], "privacy.policy.applied") {
		t.Fatal("expected privacy policy event")
	}
	if !agentHarnessEventRecorded(repo.agentHarnessEvents[executionID], "privacy.audit.recorded") {
		t.Fatal("expected privacy audit event")
	}
	if !agentHarnessEventRecorded(repo.agentHarnessEvents[executionID], "benchmark.metadata.recorded") {
		t.Fatal("expected benchmark metadata event")
	}
	evaluation, ok := repo.evaluations[runAgentID]
	if !ok || len(evaluation.ValidatorResults) != 1 || evaluation.ValidatorResults[0].Verdict != "pass" {
		t.Fatalf("evaluation = %#v, want persisted passing hidden validator", evaluation)
	}
	if strings.Contains(string(evaluation.ValidatorResults[0].RawOutput), "SECRET_EXPECTED") {
		t.Fatalf("validator raw output leaked hidden command: %s", evaluation.ValidatorResults[0].RawOutput)
	}
	for _, event := range repo.runEvents[runAgentID] {
		encoded := string(event.Payload)
		if containsAny(encoded, "SECRET_EXPECTED", "PRIVATE_AGENT_OUTPUT", "PRIVATE_DIFF", "private/file.go") {
			t.Fatalf("canonical run event leaked private data: %s", event.Payload)
		}
	}
}

func TestExecuteAgentHarnessExecutionFailsRequiredValidator(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	runID := uuid.New()
	runAgentID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusQueued), fixtureRunAgent(runID, runAgentID, 0))
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
		RunID:                   &runID,
		RunAgentID:              &runAgentID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{openAISecret: "sk-test"})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch {
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "clone":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case isSetupHintsCommand(request.Command):
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
	evaluation, ok := repo.evaluations[runAgentID]
	if !ok {
		t.Fatal("expected stored scorecard evaluation for failed required validator")
	}
	if len(evaluation.ValidatorResults) != 2 || evaluation.ValidatorResults[1].Verdict != "fail" {
		t.Fatalf("validator results = %#v, want failed npm validator persisted", evaluation.ValidatorResults)
	}
	if evaluation.Passed == nil || *evaluation.Passed {
		t.Fatalf("scorecard passed = %#v, want false", evaluation.Passed)
	}
	if repo.callCountWithPrefix("BuildRunAgentReplay:"+runAgentID.String()) != 1 {
		t.Fatalf("BuildRunAgentReplay call count = %d, want 1", repo.callCountWithPrefix("BuildRunAgentReplay:"+runAgentID.String()))
	}
}

func TestExecuteAgentHarnessExecutionFailsSetupBeforeAgent(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	runID := uuid.New()
	runAgentID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusQueued), fixtureRunAgent(runID, runAgentID, 0))
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		TaskPrompt:             "implement issue 462",
		CodexTemplate:          "codex",
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &openAISecret,
		RepositoryURL:          stringPtr("https://github.com/acme/repo"),
		ExecutionConfig:        json.RawMessage(`{"timeout_seconds":120,"setup_commands":[{"name":"deps","command":"go mod download"}]}`),
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:                      executionID,
		WorkspaceID:             workspaceID,
		AgentHarnessID:          harnessID,
		RunID:                   &runID,
		RunAgentID:              &runAgentID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120,"setup_commands":[{"name":"deps","command":"go mod download"}]}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{openAISecret: "sk-test"})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch {
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "clone":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case isSetupHintsCommand(request.Command):
			return sandbox.ExecResult{ExitCode: 0, Stdout: "go.mod\n"}, nil
		case len(request.Command) >= 3 && request.Command[0] == "bash" && request.Command[1] == "-lc" && request.Command[2] == "go mod download":
			return sandbox.ExecResult{ExitCode: 1, Stderr: "module download failed"}, nil
		case len(request.Command) >= 2 && request.Command[0] == "codex" && request.Command[1] == "exec":
			t.Fatalf("codex should not run after setup failure")
			return sandbox.ExecResult{}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})
	provider := &sandbox.FakeProvider{NextSession: session}
	activities := NewActivities(repo, FakeWorkHooks{}).WithSandboxProvider(provider)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if err == nil || !strings.Contains(err.Error(), `setup command "deps" failed`) {
		t.Fatalf("ExecuteAgentHarnessExecution error = %v, want setup command failure", err)
	}
	if !agentHarnessEventRecorded(repo.agentHarnessEvents[executionID], "setup.command.failed") {
		t.Fatal("expected setup.command.failed event")
	}
	if agentHarnessEventRecorded(repo.agentHarnessEvents[executionID], "codex.exec.started") {
		t.Fatal("did not expect codex.exec.started after setup failure")
	}
	if repo.callCountWithPrefix("BuildRunAgentReplay:"+runAgentID.String()) != 1 {
		t.Fatalf("BuildRunAgentReplay call count = %d, want 1", repo.callCountWithPrefix("BuildRunAgentReplay:"+runAgentID.String()))
	}
}

func TestExecuteAgentHarnessExecutionReportsAgentFailureSeparately(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	runID := uuid.New()
	runAgentID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusQueued), fixtureRunAgent(runID, runAgentID, 0))
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		TaskPrompt:             "implement issue 462",
		CodexTemplate:          "codex",
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &openAISecret,
		ExecutionConfig:        json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:                      executionID,
		WorkspaceID:             workspaceID,
		AgentHarnessID:          harnessID,
		RunID:                   &runID,
		RunAgentID:              &runAgentID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{openAISecret: "sk-test"})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch {
		case len(request.Command) >= 2 && request.Command[0] == "codex" && request.Command[1] == "exec":
			return sandbox.ExecResult{ExitCode: 1, Stderr: "agent failed"}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})
	provider := &sandbox.FakeProvider{NextSession: session}
	activities := NewActivities(repo, FakeWorkHooks{}).WithSandboxProvider(provider)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if err == nil || !strings.Contains(err.Error(), "codex exec failed with exit code 1") {
		t.Fatalf("ExecuteAgentHarnessExecution error = %v, want agent command failure", err)
	}
	if !agentHarnessEventRecorded(repo.agentHarnessEvents[executionID], "setup.skipped") {
		t.Fatal("expected setup.skipped event before agent failure")
	}
	if !agentHarnessEventRecorded(repo.agentHarnessEvents[executionID], "codex.exec.failed") {
		t.Fatal("expected codex.exec.failed event")
	}
	if agentHarnessEventRecorded(repo.agentHarnessEvents[executionID], "validator.command.failed") {
		t.Fatal("did not expect validator failure event for agent failure")
	}
	if repo.callCountWithPrefix("BuildRunAgentReplay:"+runAgentID.String()) != 1 {
		t.Fatalf("BuildRunAgentReplay call count = %d, want 1", repo.callCountWithPrefix("BuildRunAgentReplay:"+runAgentID.String()))
	}
}

func TestExecuteAgentHarnessExecutionContinuesWhenSetupHintDetectionFails(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	runID := uuid.New()
	runAgentID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusQueued), fixtureRunAgent(runID, runAgentID, 0))
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
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:                      executionID,
		WorkspaceID:             workspaceID,
		AgentHarnessID:          harnessID,
		RunID:                   &runID,
		RunAgentID:              &runAgentID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{openAISecret: "sk-test"})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch {
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "clone":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case isSetupHintsCommand(request.Command):
			return sandbox.ExecResult{ExitCode: 127, Stderr: "bash unavailable"}, nil
		case len(request.Command) >= 2 && request.Command[0] == "codex" && request.Command[1] == "exec":
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"final","message":"done"}`}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "add" && request.Command[2] == "--intent-to-add":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "diff":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "status":
			return sandbox.ExecResult{ExitCode: 0}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})
	provider := &sandbox.FakeProvider{NextSession: session}
	activities := NewActivities(repo, FakeWorkHooks{}).WithSandboxProvider(provider)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if err != nil {
		t.Fatalf("ExecuteAgentHarnessExecution error = %v, want setup hint failure to be non-fatal", err)
	}
	if !agentHarnessEventRecorded(repo.agentHarnessEvents[executionID], "setup.hints.detect.failed") {
		t.Fatal("expected setup.hints.detect.failed event")
	}
	if !agentHarnessEventRecorded(repo.agentHarnessEvents[executionID], "codex.exec.started") {
		t.Fatal("expected codex to run after setup hint failure")
	}
}

func TestExecuteAgentHarnessExecutionTreatsReplayBuildFailureAsNonFatal(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	runID := uuid.New()
	runAgentID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusQueued), fixtureRunAgent(runID, runAgentID, 0))
	repo.buildReplayErr = errors.New("replay index unavailable")
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
		RunID:                   &runID,
		RunAgentID:              &runAgentID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{openAISecret: "sk-test"})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		if len(request.Command) >= 2 && request.Command[0] == "codex" && request.Command[1] == "exec" {
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"final","message":"done"}`}, nil
		}
		t.Fatalf("unexpected command: %#v", request.Command)
		return sandbox.ExecResult{}, nil
	})
	provider := &sandbox.FakeProvider{NextSession: session}
	activities := NewActivities(repo, FakeWorkHooks{}).WithSandboxProvider(provider)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if err != nil {
		t.Fatalf("ExecuteAgentHarnessExecution error = %v, want replay build failure to be non-fatal", err)
	}
	if repo.callCountWithPrefix("BuildRunAgentReplay:"+runAgentID.String()) != 1 {
		t.Fatalf("BuildRunAgentReplay call count = %d, want 1", repo.callCountWithPrefix("BuildRunAgentReplay:"+runAgentID.String()))
	}
	if !agentHarnessEventRecorded(repo.agentHarnessEvents[executionID], "replay.build.failed") {
		t.Fatal("expected replay.build.failed harness event")
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

func TestExecuteAgentHarnessExecutionRunsClaudeAndRecordsTrace(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	anthropicSecret := "ANTHROPIC_API_KEY"
	model := "claude-sonnet-4-6"
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		HarnessKind:            "claude_e2b",
		TaskPrompt:             "implement issue 462",
		CodexTemplate:          "agentclash-claude-fullstack",
		CodexModel:             &model,
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &anthropicSecret,
		ExecutionConfig:        json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:                      executionID,
		WorkspaceID:             workspaceID,
		AgentHarnessID:          harnessID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{anthropicSecret: "sk-ant-test"})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch {
		case len(request.Command) >= 2 && request.Command[0] == "claude" && request.Command[1] == "-p":
			if request.Environment["ANTHROPIC_API_KEY"] != "sk-ant-test" {
				t.Fatalf("claude env = %#v, want Anthropic key", request.Environment)
			}
			if containsString(request.Command, "OPENAI_API_KEY") {
				t.Fatalf("claude command leaked secret name: %#v", request.Command)
			}
			if !containsString(request.Command, "--output-format") || !containsString(request.Command, "stream-json") {
				t.Fatalf("claude command = %#v, want stream-json output", request.Command)
			}
			if !containsString(request.Command, "--verbose") {
				t.Fatalf("claude command = %#v, want --verbose (required when stream-json is paired with -p)", request.Command)
			}
			if !containsString(request.Command, "--permission-mode") || !containsString(request.Command, "bypassPermissions") {
				t.Fatalf("claude command = %#v, want bypass permission mode", request.Command)
			}
			if !containsString(request.Command, "--model") || !containsString(request.Command, model) {
				t.Fatalf("claude command = %#v, want model override", request.Command)
			}
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"assistant","message":"done"}`}, nil
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
	if provider.CreateRequests[0].TemplateID != "agentclash-claude-fullstack" {
		t.Fatalf("template id = %q, want Claude template", provider.CreateRequests[0].TemplateID)
	}
	if provider.CreateRequests[0].EnvVars["ANTHROPIC_API_KEY"] != "sk-ant-test" {
		t.Fatalf("sandbox env vars = %#v, want Anthropic key", provider.CreateRequests[0].EnvVars)
	}
	var sawClaudeOutput bool
	for _, event := range repo.agentHarnessEvents[executionID] {
		if event.EventType == "claude.exec.output" && event.ActorType == "claude" {
			sawClaudeOutput = true
			break
		}
	}
	if !sawClaudeOutput {
		t.Fatal("expected live claude output event")
	}
}

func TestExecuteAgentHarnessExecutionRunsOpenClawAndRecordsTrace(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	openClawSecret := "OPENAI_API_KEY"
	model := "openai/gpt-5.4"
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		HarnessKind:            "openclaw_e2b",
		TaskPrompt:             "implement issue 462",
		CodexTemplate:          "agentclash-openclaw-fullstack",
		CodexModel:             &model,
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &openClawSecret,
		ExecutionConfig:        json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:                      executionID,
		WorkspaceID:             workspaceID,
		AgentHarnessID:          harnessID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{openClawSecret: "sk-openai-test"})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch {
		case len(request.Command) >= 3 && request.Command[0] == "bash" && request.Command[1] == "-lc":
			if request.Environment["OPENAI_API_KEY"] != "sk-openai-test" {
				t.Fatalf("openclaw env = %#v, want OpenAI key", request.Environment)
			}
			if request.Environment["AGENTCLASH_HARNESS_TASK"] != "implement issue 462" {
				t.Fatalf("openclaw task env = %#v", request.Environment)
			}
			if request.Environment["AGENTCLASH_HARNESS_MODEL"] != model {
				t.Fatalf("openclaw model env = %#v", request.Environment)
			}
			if request.Environment["AGENTCLASH_HARNESS_TIMEOUT_SECONDS"] != "120" {
				t.Fatalf("openclaw timeout env = %#v", request.Environment)
			}
			if containsString(request.Command, "OPENAI_API_KEY") {
				t.Fatalf("openclaw command leaked secret name: %#v", request.Command)
			}
			script := request.Command[2]
			if !strings.Contains(script, "openclaw setup --workspace \"$PWD\"") || !strings.Contains(script, "--accept-risk") {
				t.Fatalf("openclaw script = %q, want workspace setup with accept-risk", script)
			}
			if !strings.Contains(script, "openclaw onboard --non-interactive") || !strings.Contains(script, "--secret-input-mode ref") {
				t.Fatalf("openclaw script = %q, want non-interactive onboard with secret refs", script)
			}
			if !strings.Contains(script, "--skip-bootstrap") || !strings.Contains(script, "--skip-health") {
				t.Fatalf("openclaw script = %q, want headless onboard flags", script)
			}
			if !strings.Contains(script, "AUTH_CHOICE=openai-api-key") {
				t.Fatalf("openclaw script = %q, want OpenAI auth choice", script)
			}
			if !strings.Contains(script, "AGENT_ARGS+=(--model \"$AGENTCLASH_HARNESS_MODEL\")") {
				t.Fatalf("openclaw script = %q, want model override on agent command", script)
			}
			if strings.Contains(script, "openclaw models set") {
				t.Fatalf("openclaw script = %q, should not call models set separately", script)
			}
			if strings.Contains(script, ">/tmp/openclaw-setup.log") || strings.Contains(script, "|| true") {
				t.Fatalf("openclaw script = %q, want visible fail-fast setup", script)
			}
			if !strings.Contains(script, "exec openclaw agent \"${AGENT_ARGS[@]}\"") {
				t.Fatalf("openclaw script = %q, want local json agent run", script)
			}
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"assistant","message":"done"}`}, nil
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
	if provider.CreateRequests[0].TemplateID != "agentclash-openclaw-fullstack" {
		t.Fatalf("template id = %q, want OpenClaw template", provider.CreateRequests[0].TemplateID)
	}
	if provider.CreateRequests[0].EnvVars["OPENAI_API_KEY"] != "sk-openai-test" {
		t.Fatalf("sandbox env vars = %#v, want OpenAI key", provider.CreateRequests[0].EnvVars)
	}
	var sawOpenClawOutput bool
	for _, event := range repo.agentHarnessEvents[executionID] {
		if event.EventType == "openclaw.exec.output" && event.ActorType == "openclaw" {
			sawOpenClawOutput = true
			break
		}
	}
	if !sawOpenClawOutput {
		t.Fatal("expected live openclaw output event")
	}
}

func TestExecuteAgentHarnessExecutionRunsHermesAndRecordsTrace(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	hermesSecret := "OPENROUTER_API_KEY"
	model := "anthropic/claude-sonnet-4"
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		HarnessKind:            "hermes_e2b",
		TaskPrompt:             "implement issue 462",
		CodexTemplate:          "agentclash-hermes-fullstack",
		CodexModel:             &model,
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &hermesSecret,
		ExecutionConfig:        json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setAgentHarnessExecution(repository.AgentHarnessExecution{
		ID:                      executionID,
		WorkspaceID:             workspaceID,
		AgentHarnessID:          harnessID,
		Status:                  string(repository.AgentHarnessExecutionStatusRunning),
		ExecutionConfigSnapshot: json.RawMessage(`{"timeout_seconds":120}`),
	})
	repo.setWorkspaceSecrets(workspaceID, map[string]string{hermesSecret: "sk-or-test"})

	session := sandbox.NewFakeSession("sandbox-1")
	session.SetExecFunc(func(request sandbox.ExecRequest, _ map[string][]byte) (sandbox.ExecResult, error) {
		switch {
		case len(request.Command) >= 3 && request.Command[0] == "bash" && request.Command[1] == "-lc":
			if request.Environment["OPENROUTER_API_KEY"] != "sk-or-test" {
				t.Fatalf("hermes env = %#v, want OpenRouter key", request.Environment)
			}
			if request.Environment["AGENTCLASH_HARNESS_TASK"] != "implement issue 462" {
				t.Fatalf("hermes task env = %#v", request.Environment)
			}
			if request.Environment["AGENTCLASH_HARNESS_MODEL"] != model {
				t.Fatalf("hermes model env = %#v", request.Environment)
			}
			if request.Environment["AGENTCLASH_HARNESS_PROVIDER"] != "openrouter" {
				t.Fatalf("hermes provider env = %#v", request.Environment)
			}
			if containsString(request.Command, "OPENROUTER_API_KEY") {
				t.Fatalf("hermes command leaked secret name: %#v", request.Command)
			}
			script := request.Command[2]
			if !strings.Contains(script, "hermes setup model --non-interactive") {
				t.Fatalf("hermes script = %q, want non-interactive model setup", script)
			}
			if !strings.Contains(script, "--ignore-user-config") || !strings.Contains(script, "--ignore-rules") {
				t.Fatalf("hermes script = %q, want isolated chat flags", script)
			}
			if !strings.Contains(script, "--toolsets terminal,skills") {
				t.Fatalf("hermes script = %q, want terminal and skills toolsets", script)
			}
			if !strings.Contains(script, "CHAT_ARGS+=(--model \"$AGENTCLASH_HARNESS_MODEL\")") {
				t.Fatalf("hermes script = %q, want model override on chat command", script)
			}
			if !strings.Contains(script, "exec hermes chat \"${CHAT_ARGS[@]}\"") {
				t.Fatalf("hermes script = %q, want chat exec", script)
			}
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"assistant","message":"done"}`}, nil
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
	if provider.CreateRequests[0].TemplateID != "agentclash-hermes-fullstack" {
		t.Fatalf("template id = %q, want Hermes template", provider.CreateRequests[0].TemplateID)
	}
	if provider.CreateRequests[0].EnvVars["OPENROUTER_API_KEY"] != "sk-or-test" {
		t.Fatalf("sandbox env vars = %#v, want OpenRouter key", provider.CreateRequests[0].EnvVars)
	}
	var sawHermesOutput bool
	for _, event := range repo.agentHarnessEvents[executionID] {
		if event.EventType == "hermes.exec.output" && event.ActorType == "hermes" {
			sawHermesOutput = true
			break
		}
	}
	if !sawHermesOutput {
		t.Fatal("expected live hermes output event")
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

func TestExecuteAgentHarnessExecutionCreatesDraftPullRequestForGitHubHarness(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repositoryID := int64(100)
	installationID := int64(42)
	provider := "github"
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.githubRepo = repository.GitHubInstallationRepository{
		GitHubInstallationID: installationID,
		GitHubRepositoryID:   repositoryID,
		FullName:             "acme/repo",
		DefaultBranch:        "main",
		Status:               "active",
	}
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		TaskPrompt:             "make a sample change",
		CodexTemplate:          "codex",
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &openAISecret,
		RepositoryURL:          stringPtr("https://github.com/acme/repo"),
		RepositoryProvider:     &provider,
		GitHubRepositoryID:     &repositoryID,
		GitHubInstallationID:   &installationID,
		RepositoryFullName:     stringPtr("acme/repo"),
		BaseBranch:             stringPtr("main"),
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
		if containsString(request.Command, "ghs_test_token") {
			t.Fatalf("command leaked github token: %#v", request.Command)
		}
		switch {
		case len(request.Command) >= 2 && request.Command[0] == "chmod":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "clone":
			if request.Environment["GITHUB_TOKEN"] != "ghs_test_token" {
				t.Fatalf("clone env missing github token")
			}
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "checkout" && request.Command[2] == "main":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case isSetupHintsCommand(request.Command):
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "codex" && request.Command[1] == "exec":
			if _, ok := request.Environment["GITHUB_TOKEN"]; ok {
				t.Fatal("codex env must not include github token")
			}
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"final","message":"done"}`}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "add" && request.Command[2] == "--intent-to-add":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "diff":
			return sandbox.ExecResult{ExitCode: 0, Stdout: "diff --git a/README.md b/README.md"}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "status":
			return sandbox.ExecResult{ExitCode: 0, Stdout: " M README.md\n"}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "config":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "checkout" && request.Command[2] == "-B":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "add":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case isGitSubcommand(request.Command, "commit"):
			if !containsString(request.Command, "core.hooksPath=/dev/null") {
				t.Fatalf("commit command did not disable hooks: %#v", request.Command)
			}
			return sandbox.ExecResult{ExitCode: 0}, nil
		case isGitSubcommand(request.Command, "push"):
			if request.Environment["GITHUB_TOKEN"] != "ghs_test_token" {
				t.Fatalf("push env missing github token")
			}
			if !containsString(request.Command, "core.hooksPath=/dev/null") || !containsString(request.Command, "credential.helper=") {
				t.Fatalf("push command did not disable hooks and credential helpers: %#v", request.Command)
			}
			if !containsString(request.Command, "https://github.com/acme/repo.git") {
				t.Fatalf("push command did not use explicit github url: %#v", request.Command)
			}
			return sandbox.ExecResult{ExitCode: 0}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})
	providerStub := &sandbox.FakeProvider{NextSession: session}
	githubClient := &fakeGitHubPullRequestClient{token: "ghs_test_token"}
	activities := NewActivities(repo, FakeWorkHooks{}).
		WithSandboxProvider(providerStub).
		WithGitHubPullRequestClient(githubClient)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if err != nil {
		t.Fatalf("ExecuteAgentHarnessExecution error: %v", err)
	}
	if githubClient.tokenInstallationID != installationID {
		t.Fatalf("token installation id = %d, want %d", githubClient.tokenInstallationID, installationID)
	}
	if githubClient.pullRequestInput.Owner != "acme" || githubClient.pullRequestInput.Repo != "repo" {
		t.Fatalf("pull request repo = %s/%s, want acme/repo", githubClient.pullRequestInput.Owner, githubClient.pullRequestInput.Repo)
	}
	if !githubClient.pullRequestInput.Draft {
		t.Fatal("pull request should be draft")
	}
	if githubClient.pullRequestInput.Head != "acme:agentclash/harness/"+strings.ReplaceAll(executionID.String()[:8], "-", "") {
		t.Fatalf("pull request head = %q", githubClient.pullRequestInput.Head)
	}
	var sawPREvent bool
	for _, event := range repo.agentHarnessEvents[executionID] {
		if event.EventType == "github.pull_request.created" {
			sawPREvent = true
			break
		}
	}
	if !sawPREvent {
		t.Fatal("expected github.pull_request.created event")
	}
}

func TestExecuteAgentHarnessExecutionCreatesPullRequestForAgentCommittedChanges(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repositoryID := int64(100)
	installationID := int64(42)
	provider := "github"
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.githubRepo = repository.GitHubInstallationRepository{
		GitHubInstallationID: installationID,
		GitHubRepositoryID:   repositoryID,
		FullName:             "acme/repo",
		DefaultBranch:        "main",
		Status:               "active",
	}
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		TaskPrompt:             "make and commit a sample change",
		CodexTemplate:          "codex",
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &openAISecret,
		RepositoryURL:          stringPtr("https://github.com/acme/repo"),
		RepositoryProvider:     &provider,
		GitHubRepositoryID:     &repositoryID,
		GitHubInstallationID:   &installationID,
		RepositoryFullName:     stringPtr("acme/repo"),
		BaseBranch:             stringPtr("main"),
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
		case len(request.Command) >= 2 && request.Command[0] == "chmod":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "clone":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "checkout" && request.Command[2] == "main":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case isSetupHintsCommand(request.Command):
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "codex" && request.Command[1] == "exec":
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"final","message":"committed"}`}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "add" && request.Command[2] == "--intent-to-add":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "diff" && request.Command[2] == "--binary":
			if !containsString(request.Command, "origin/main") {
				t.Fatalf("binary diff command = %#v, want origin/main", request.Command)
			}
			return sandbox.ExecResult{ExitCode: 0, Stdout: "diff --git a/README.md b/README.md"}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "diff" && request.Command[2] == "--name-status":
			if !containsString(request.Command, "origin/main") {
				t.Fatalf("name-status diff command = %#v, want origin/main", request.Command)
			}
			return sandbox.ExecResult{ExitCode: 0, Stdout: "M\tREADME.md\n"}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "status":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "config":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "checkout" && request.Command[2] == "-B":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "add":
			t.Fatalf("unexpected git add for already-committed agent changes: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		case isGitSubcommand(request.Command, "commit"):
			t.Fatalf("unexpected git commit for already-committed agent changes: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		case isGitSubcommand(request.Command, "push"):
			if request.Environment["GITHUB_TOKEN"] != "ghs_test_token" {
				t.Fatalf("push env missing github token")
			}
			return sandbox.ExecResult{ExitCode: 0}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})
	providerStub := &sandbox.FakeProvider{NextSession: session}
	githubClient := &fakeGitHubPullRequestClient{token: "ghs_test_token"}
	activities := NewActivities(repo, FakeWorkHooks{}).
		WithSandboxProvider(providerStub).
		WithGitHubPullRequestClient(githubClient)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if err != nil {
		t.Fatalf("ExecuteAgentHarnessExecution error: %v", err)
	}
	if githubClient.pullRequestInput.Head != "acme:agentclash/harness/"+strings.ReplaceAll(executionID.String()[:8], "-", "") {
		t.Fatalf("pull request head = %q", githubClient.pullRequestInput.Head)
	}
	var sawPREvent bool
	var sawBaseChanges bool
	for _, event := range repo.agentHarnessEvents[executionID] {
		switch event.EventType {
		case "github.pull_request.created":
			sawPREvent = true
		case "artifact.changed_files":
			var payload map[string]any
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatalf("decode changed files payload: %v", err)
			}
			if payload["changed_files"] == "M\tREADME.md" {
				sawBaseChanges = true
			}
		}
	}
	if !sawPREvent {
		t.Fatal("expected github.pull_request.created event")
	}
	if !sawBaseChanges {
		t.Fatal("expected artifact.changed_files to include base diff files")
	}
}

func TestExecuteAgentHarnessExecutionSkipsPullRequestWhenNoChanges(t *testing.T) {
	workspaceID := uuid.New()
	harnessID := uuid.New()
	executionID := uuid.New()
	openAISecret := "OPENAI_API_KEY"
	repositoryID := int64(100)
	installationID := int64(42)
	provider := "github"
	repo := newFakeRunRepository(fixtureRun(uuid.New(), domain.RunStatusQueued))
	repo.githubRepo = repository.GitHubInstallationRepository{
		GitHubInstallationID: installationID,
		GitHubRepositoryID:   repositoryID,
		FullName:             "acme/repo",
		DefaultBranch:        "main",
		Status:               "active",
	}
	repo.setAgentHarness(repository.AgentHarness{
		ID:                     harnessID,
		OrganizationID:         uuid.New(),
		WorkspaceID:            workspaceID,
		TaskPrompt:             "make a sample change",
		CodexTemplate:          "codex",
		AuthMode:               "api_key_secret",
		OpenAIAPIKeySecretName: &openAISecret,
		RepositoryURL:          stringPtr("https://github.com/acme/repo"),
		RepositoryProvider:     &provider,
		GitHubRepositoryID:     &repositoryID,
		GitHubInstallationID:   &installationID,
		RepositoryFullName:     stringPtr("acme/repo"),
		BaseBranch:             stringPtr("main"),
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
		case len(request.Command) >= 2 && request.Command[0] == "chmod":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "clone":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "checkout" && request.Command[2] == "main":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case isSetupHintsCommand(request.Command):
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "codex" && request.Command[1] == "exec":
			return sandbox.ExecResult{ExitCode: 0, Stdout: `{"type":"final","message":"done"}`}, nil
		case len(request.Command) >= 3 && request.Command[0] == "git" && request.Command[1] == "add" && request.Command[2] == "--intent-to-add":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "diff":
			return sandbox.ExecResult{ExitCode: 0}, nil
		case len(request.Command) >= 2 && request.Command[0] == "git" && request.Command[1] == "status":
			return sandbox.ExecResult{ExitCode: 0}, nil
		default:
			t.Fatalf("unexpected command: %#v", request.Command)
			return sandbox.ExecResult{}, nil
		}
	})
	providerStub := &sandbox.FakeProvider{NextSession: session}
	githubClient := &fakeGitHubPullRequestClient{token: "ghs_test_token"}
	activities := NewActivities(repo, FakeWorkHooks{}).
		WithSandboxProvider(providerStub).
		WithGitHubPullRequestClient(githubClient)

	err := activities.ExecuteAgentHarnessExecution(context.Background(), ExecuteAgentHarnessExecutionInput{ExecutionID: executionID})
	if err != nil {
		t.Fatalf("ExecuteAgentHarnessExecution error: %v", err)
	}
	if githubClient.pullRequestInput.Owner != "" {
		t.Fatalf("pull request was created unexpectedly: %#v", githubClient.pullRequestInput)
	}
	var sawSkip bool
	for _, event := range repo.agentHarnessEvents[executionID] {
		if event.EventType == "github.pull_request.skipped" {
			sawSkip = true
			break
		}
	}
	if !sawSkip {
		t.Fatal("expected github.pull_request.skipped event")
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

func agentHarnessEventRecorded(events []repository.AgentHarnessExecutionEvent, eventType string) bool {
	for _, event := range events {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func isGitSubcommand(command []string, subcommand string) bool {
	if len(command) == 0 || command[0] != "git" {
		return false
	}
	for _, part := range command[1:] {
		if part == subcommand {
			return true
		}
	}
	return false
}

func isSetupHintsCommand(command []string) bool {
	return len(command) >= 3 &&
		command[0] == "bash" &&
		command[1] == "-lc" &&
		strings.Contains(command[2], ".devcontainer/devcontainer.json") &&
		strings.Contains(command[2], "Cargo.toml")
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

type fakeGitHubPullRequestClient struct {
	token               string
	tokenInstallationID int64
	pullRequestInput    CreateGitHubPullRequestInput
}

func (f *fakeGitHubPullRequestClient) CreateInstallationToken(_ context.Context, installationID int64) (string, error) {
	f.tokenInstallationID = installationID
	return f.token, nil
}

func (f *fakeGitHubPullRequestClient) CreatePullRequest(_ context.Context, input CreateGitHubPullRequestInput) (GitHubPullRequest, error) {
	f.pullRequestInput = input
	return GitHubPullRequest{Number: 12, HTMLURL: "https://github.com/acme/repo/pull/12", State: "open", Draft: true}, nil
}

func TestExpectedArtifactsFromExecutionConfig(t *testing.T) {
	config := []byte(`{"timeout_seconds":180,"agent_tryout":{"template_slug":"slide-deck","runtime":{"expected_artifacts":[{"key":"deck_outline","type":"markdown","path":"deck.md"},{"key":"structured_deck","type":"json","path":"deck.json"}]}}}`)
	specs := expectedArtifactsFromExecutionConfig(config)
	if len(specs) != 2 {
		t.Fatalf("specs = %d, want 2", len(specs))
	}
	if specs[0].Key != "deck_outline" || specs[0].Path != "deck.md" || specs[0].Type != "markdown" {
		t.Fatalf("first spec = %+v, want deck_outline/deck.md/markdown", specs[0])
	}

	// Non-tryout runs (no agent_tryout block) yield no capture work.
	if got := expectedArtifactsFromExecutionConfig([]byte(`{"timeout_seconds":120}`)); got != nil {
		t.Fatalf("non-tryout config should yield nil specs, got %+v", got)
	}
	if got := expectedArtifactsFromExecutionConfig(nil); got != nil {
		t.Fatalf("empty config should yield nil specs, got %+v", got)
	}
}

func TestCapturedArtifactContentType(t *testing.T) {
	cases := map[string]string{
		"deck.md":      "text/markdown",
		"deck.json":    "application/json",
		"data.csv":     "text/csv",
		"changes.diff": "text/x-patch",
		"mystery.xyz":  "application/octet-stream",
	}
	for path, wantPrefix := range cases {
		got := capturedArtifactContentType(path)
		if !strings.HasPrefix(got, wantPrefix) {
			t.Fatalf("content type for %q = %q, want prefix %q", path, got, wantPrefix)
		}
	}
}
