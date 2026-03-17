package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

func TestNativeExecutorHappyPathWritesFileThenSubmits(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-happy")
	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				validate: func(t *testing.T, request provider.Request) {
					if len(request.Messages) != 2 {
						t.Fatalf("message count = %d, want 2", len(request.Messages))
					}
					if len(request.Tools) != 4 {
						t.Fatalf("tool count = %d, want 4", len(request.Tools))
					}
				},
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-write",
							Name:      writeFileToolName,
							Arguments: []byte(`{"path":"/workspace/result.txt","content":"done"}`),
						},
					},
				},
			},
			{
				validate: func(t *testing.T, request provider.Request) {
					if len(request.Messages) != 4 {
						t.Fatalf("message count = %d, want 4", len(request.Messages))
					}
					last := request.Messages[len(request.Messages)-1]
					if last.Role != "tool" || last.ToolCallID != "call-write" {
						t.Fatalf("last message = %#v, want tool result for call-write", last)
					}
					if last.IsError {
						t.Fatalf("tool result unexpectedly marked as error")
					}
				},
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-submit",
							Name:      submitToolName,
							Arguments: []byte(`{"answer":"final answer"}`),
						},
					},
				},
			},
		},
	}

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, NoopObserver{})
	result, err := executor.Execute(context.Background(), nativeExecutionContext())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("stop reason = %s, want completed", result.StopReason)
	}
	if result.FinalOutput != "final answer" {
		t.Fatalf("final output = %q, want final answer", result.FinalOutput)
	}
	if result.ToolCallCount != 1 {
		t.Fatalf("tool call count = %d, want 1", result.ToolCallCount)
	}
	if session.DestroyCalls() != 1 {
		t.Fatalf("destroy calls = %d, want 1", session.DestroyCalls())
	}
	files := session.Files()
	if string(files["/workspace/result.txt"]) != "done" {
		t.Fatalf("result file = %q, want done", string(files["/workspace/result.txt"]))
	}
	if _, ok := files["/workspace/agentclash/run-context.json"]; !ok {
		t.Fatalf("expected run-context.json to be staged")
	}
	if _, ok := files["/workspace/agentclash/challenge-pack-manifest.json"]; !ok {
		t.Fatalf("expected challenge-pack-manifest.json to be staged")
	}
	if _, ok := files["/workspace/agentclash/challenges.json"]; !ok {
		t.Fatalf("expected challenges.json to be staged")
	}
	if _, ok := files["/workspace/agentclash/challenge-input-set.json"]; !ok {
		t.Fatalf("expected challenge-input-set.json to be staged")
	}
}

func TestNativeExecutorReturnsObserverErrorWhenObserverWriteFails(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-observer-error")
	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-submit",
							Name:      submitToolName,
							Arguments: []byte(`{"answer":"done"}`),
						},
					},
				},
			},
		},
	}

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, failingObserver{})
	_, err := executor.Execute(context.Background(), nativeExecutionContext())
	if err == nil {
		t.Fatalf("expected observer error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected engine failure, got %T", err)
	}
	if failure.StopReason != StopReasonObserverError {
		t.Fatalf("stop reason = %s, want %s", failure.StopReason, StopReasonObserverError)
	}
}

func TestNativeExecutorReturnsObserverErrorWhenRunCompleteWriteFails(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-complete-observer-error")
	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-submit",
							Name:      submitToolName,
							Arguments: []byte(`{"answer":"done"}`),
						},
					},
				},
			},
		},
	}

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, runCompleteFailingObserver{})
	result, err := executor.Execute(context.Background(), nativeExecutionContext())
	if err == nil {
		t.Fatalf("expected observer completion error")
	}
	if result != (Result{}) {
		t.Fatalf("result = %#v, want zero value", result)
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected engine failure, got %T", err)
	}
	if failure.StopReason != StopReasonObserverError {
		t.Fatalf("stop reason = %s, want %s", failure.StopReason, StopReasonObserverError)
	}
	if !strings.Contains(err.Error(), "record native terminal completion event") {
		t.Fatalf("error = %v, want terminal completion context", err)
	}
}

func TestNativeExecutorJoinsObserverFailureWhenRunFailureWriteFails(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-failure-observer-error")
	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				err: provider.NewFailure("openai", provider.FailureCodeAuth, "upstream unavailable", false, errors.New("boom")),
			},
		},
	}

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, runFailureFailingObserver{})
	_, err := executor.Execute(context.Background(), nativeExecutionContext())
	if err == nil {
		t.Fatalf("expected joined failure")
	}
	if !strings.Contains(err.Error(), "upstream unavailable") {
		t.Fatalf("error = %v, want original provider failure", err)
	}
	if !strings.Contains(err.Error(), "record native terminal failure event") {
		t.Fatalf("error = %v, want observer failure context", err)
	}

	var providerFailure provider.Failure
	if !errors.As(err, &providerFailure) {
		t.Fatalf("expected joined provider failure, got %v", err)
	}
	var observerFailure Failure
	if !errors.As(err, &observerFailure) {
		t.Fatalf("expected joined observer failure, got %v", err)
	}
	if observerFailure.StopReason != StopReasonObserverError {
		t.Fatalf("observer stop reason = %s, want %s", observerFailure.StopReason, StopReasonObserverError)
	}
}

func TestNativeExecutorRecoversFromToolErrorAndEventuallySubmits(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-recover")
	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-read",
							Name:      readFileToolName,
							Arguments: []byte(`{"path":"/workspace/missing.txt"}`),
						},
					},
				},
			},
			{
				validate: func(t *testing.T, request provider.Request) {
					last := request.Messages[len(request.Messages)-1]
					if !last.IsError {
						t.Fatalf("expected tool error message, got %#v", last)
					}
					if !strings.Contains(last.Content, "missing.txt") {
						t.Fatalf("tool error content = %q, want missing file context", last.Content)
					}
				},
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-write",
							Name:      writeFileToolName,
							Arguments: []byte(`{"path":"/workspace/missing.txt","content":"fixed"}`),
						},
					},
				},
			},
			{
				validate: func(t *testing.T, request provider.Request) {
					last := request.Messages[len(request.Messages)-1]
					if last.IsError {
						t.Fatalf("expected successful tool result, got %#v", last)
					}
				},
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-submit",
							Name:      submitToolName,
							Arguments: []byte(`{"answer":"recovered"}`),
						},
					},
				},
			},
		},
	}

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, NoopObserver{})
	result, err := executor.Execute(context.Background(), nativeExecutionContext())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.FinalOutput != "recovered" {
		t.Fatalf("final output = %q, want recovered", result.FinalOutput)
	}
}

func TestNativeExecutorRetriesTransientProviderFailure(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-retry")
	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				err: provider.NewFailure("openai", provider.FailureCodeRateLimit, "too many requests", true, nil),
			},
			{
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-submit",
							Name:      submitToolName,
							Arguments: []byte(`{"answer":"after retry"}`),
						},
					},
				},
			},
		},
	}

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, NoopObserver{})
	result, err := executor.Execute(context.Background(), nativeExecutionContext())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.FinalOutput != "after retry" {
		t.Fatalf("final output = %q, want after retry", result.FinalOutput)
	}
	if len(client.requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(client.requests))
	}
}

func TestNativeExecutorFailsOnRuntimeTimeout(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-timeout")
	executionContext := nativeExecutionContext()
	executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds = 1

	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{waitForContext: true},
		},
	}

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, NoopObserver{})
	_, err := executor.Execute(context.Background(), executionContext)
	if err == nil {
		t.Fatalf("expected timeout error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected engine failure, got %T", err)
	}
	if failure.StopReason != StopReasonTimeout {
		t.Fatalf("stop reason = %s, want timeout", failure.StopReason)
	}
}

func TestNativeExecutorFailsOnStepLimit(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-step-limit")
	executionContext := nativeExecutionContext()
	executionContext.Deployment.RuntimeProfile.MaxIterations = 1

	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-list",
							Name:      listFilesToolName,
							Arguments: []byte(`{"prefix":"/workspace"}`),
						},
					},
				},
			},
		},
	}

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, NoopObserver{})
	_, err := executor.Execute(context.Background(), executionContext)
	if err == nil {
		t.Fatalf("expected step limit failure")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected engine failure, got %T", err)
	}
	if failure.StopReason != StopReasonStepLimit {
		t.Fatalf("stop reason = %s, want step_limit", failure.StopReason)
	}
}

func TestNativeExecutorFailsWhenSandboxSetupFails(t *testing.T) {
	executor := NewNativeExecutor(&scriptedProviderClient{t: t}, &sandbox.FakeProvider{
		CreateErr: errors.New("sandbox unavailable"),
	}, NoopObserver{})

	_, err := executor.Execute(context.Background(), nativeExecutionContext())
	if err == nil {
		t.Fatalf("expected sandbox error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected engine failure, got %T", err)
	}
	if failure.StopReason != StopReasonSandboxError {
		t.Fatalf("stop reason = %s, want sandbox_error", failure.StopReason)
	}
}

func TestNativeExecutorExecutesMultipleToolCallsSequentially(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-multi")
	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-write",
							Name:      writeFileToolName,
							Arguments: []byte(`{"path":"/workspace/sequence.txt","content":"hello"}`),
						},
						{
							ID:        "call-read",
							Name:      readFileToolName,
							Arguments: []byte(`{"path":"/workspace/sequence.txt"}`),
						},
					},
				},
			},
			{
				validate: func(t *testing.T, request provider.Request) {
					if len(request.Messages) != 5 {
						t.Fatalf("message count = %d, want 5", len(request.Messages))
					}
					if request.Messages[3].ToolCallID != "call-write" || request.Messages[4].ToolCallID != "call-read" {
						t.Fatalf("tool result order = %#v, %#v", request.Messages[3], request.Messages[4])
					}
					if request.Messages[4].IsError {
						t.Fatalf("read_file should have observed the write from the same turn")
					}
				},
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-submit",
							Name:      submitToolName,
							Arguments: []byte(`{"answer":"sequential"}`),
						},
					},
				},
			},
		},
	}

	executionContext := nativeExecutionContext()
	executionContext.Deployment.RuntimeProfile.MaxToolCalls = 4

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, NoopObserver{})
	result, err := executor.Execute(context.Background(), executionContext)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.FinalOutput != "sequential" {
		t.Fatalf("final output = %q, want sequential", result.FinalOutput)
	}
	if result.ToolCallCount != 2 {
		t.Fatalf("tool call count = %d, want 2", result.ToolCallCount)
	}
}

func TestNativeExecutorKeepsSuccessfulResultWhenSandboxDestroyFails(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-destroy-success")
	session.SetDestroyError(errors.New("destroy failed"))

	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-submit",
							Name:      submitToolName,
							Arguments: []byte(`{"answer":"final answer"}`),
						},
					},
				},
			},
		},
	}

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, NoopObserver{})
	result, err := executor.Execute(context.Background(), nativeExecutionContext())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("stop reason = %s, want completed", result.StopReason)
	}
}

func TestNativeExecutorJoinsDestroyFailureWhenExecutionAlreadyFailed(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-destroy-failure")
	session.SetDestroyError(errors.New("destroy failed"))

	client := &scriptedProviderClient{
		t: t,
		steps: []providerStep{
			{
				err: provider.NewFailure("openai", provider.FailureCodeUnavailable, "provider down", false, nil),
			},
		},
	}

	executor := NewNativeExecutor(client, &sandbox.FakeProvider{NextSession: session}, NoopObserver{})
	_, err := executor.Execute(context.Background(), nativeExecutionContext())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "destroy native sandbox") {
		t.Fatalf("error = %v, want destroy failure joined", err)
	}
}

func TestSandboxTTLDefaultsWhenRunTimeoutIsUnset(t *testing.T) {
	executionContext := nativeExecutionContext()
	executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds = 0

	got := sandboxTTL(executionContext)
	if got != defaultSandboxTTL {
		t.Fatalf("sandbox TTL = %s, want %s", got, defaultSandboxTTL)
	}
}

type scriptedProviderClient struct {
	t        *testing.T
	steps    []providerStep
	requests []provider.Request
}

type failingObserver struct{}

func (failingObserver) OnStepStart(context.Context, int) error {
	return errors.New("observer unavailable")
}
func (failingObserver) OnProviderCall(context.Context, provider.Request) error      { return nil }
func (failingObserver) OnProviderResponse(context.Context, provider.Response) error { return nil }
func (failingObserver) OnToolExecution(context.Context, provider.ToolCall, provider.ToolResult) error {
	return nil
}
func (failingObserver) OnStepEnd(context.Context, int) error        { return nil }
func (failingObserver) OnRunComplete(context.Context, Result) error { return nil }
func (failingObserver) OnRunFailure(context.Context, error) error   { return nil }

type runCompleteFailingObserver struct{}

func (runCompleteFailingObserver) OnStepStart(context.Context, int) error                 { return nil }
func (runCompleteFailingObserver) OnProviderCall(context.Context, provider.Request) error { return nil }
func (runCompleteFailingObserver) OnProviderResponse(context.Context, provider.Response) error {
	return nil
}
func (runCompleteFailingObserver) OnToolExecution(context.Context, provider.ToolCall, provider.ToolResult) error {
	return nil
}
func (runCompleteFailingObserver) OnStepEnd(context.Context, int) error { return nil }
func (runCompleteFailingObserver) OnRunComplete(context.Context, Result) error {
	return errors.New("observer completion write failed")
}
func (runCompleteFailingObserver) OnRunFailure(context.Context, error) error { return nil }

type runFailureFailingObserver struct{}

func (runFailureFailingObserver) OnStepStart(context.Context, int) error                 { return nil }
func (runFailureFailingObserver) OnProviderCall(context.Context, provider.Request) error { return nil }
func (runFailureFailingObserver) OnProviderResponse(context.Context, provider.Response) error {
	return nil
}
func (runFailureFailingObserver) OnToolExecution(context.Context, provider.ToolCall, provider.ToolResult) error {
	return nil
}
func (runFailureFailingObserver) OnStepEnd(context.Context, int) error        { return nil }
func (runFailureFailingObserver) OnRunComplete(context.Context, Result) error { return nil }
func (runFailureFailingObserver) OnRunFailure(context.Context, error) error {
	return errors.New("observer failure write failed")
}

type providerStep struct {
	validate       func(t *testing.T, request provider.Request)
	response       provider.Response
	err            error
	waitForContext bool
}

func (c *scriptedProviderClient) InvokeModel(ctx context.Context, request provider.Request) (provider.Response, error) {
	index := len(c.requests)
	c.requests = append(c.requests, request)
	if index >= len(c.steps) {
		c.t.Fatalf("unexpected provider invocation %d", index+1)
	}

	step := c.steps[index]
	if step.validate != nil {
		step.validate(c.t, request)
	}
	if step.waitForContext {
		<-ctx.Done()
		return provider.Response{}, ctx.Err()
	}
	if step.err != nil {
		return provider.Response{}, step.err
	}
	return step.response, nil
}

func nativeExecutionContext() repository.RunAgentExecutionContext {
	runID := uuid.New()
	runAgentID := uuid.New()

	return repository.RunAgentExecutionContext{
		Run: domain.Run{
			ID:   runID,
			Name: "Native Loop Test",
		},
		RunAgent: domain.RunAgent{
			ID:        runAgentID,
			RunID:     runID,
			Status:    domain.RunAgentStatusQueued,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			ID:               uuid.New(),
			ChallengePackID:  uuid.New(),
			VersionNumber:    1,
			ManifestChecksum: "manifest",
			Manifest:         []byte(`{"challenge":"fixture","tool_policy":{"allowed_tool_kinds":["file"]}}`),
			Challenges: []repository.ChallengeDefinitionExecutionContext{
				{
					ID:                  uuid.New(),
					ChallengeIdentityID: uuid.New(),
					ChallengeKey:        "coding-fix",
					ExecutionOrder:      0,
					Title:               "Fix the workspace",
					Category:            "coding",
					Difficulty:          "medium",
					Definition:          []byte(`{"goal":"produce the requested output"}`),
				},
			},
		},
		ChallengeInputSet: &repository.ChallengeInputSetExecutionContext{
			ID:                     uuid.New(),
			ChallengePackVersionID: uuid.New(),
			InputKey:               "default",
			Name:                   "Default Inputs",
			InputChecksum:          "checksum",
			Items: []repository.ChallengeInputItemExecutionContext{
				{
					ID:                  uuid.New(),
					ChallengeIdentityID: uuid.New(),
					ChallengeKey:        "coding-fix",
					ItemKey:             "task",
					Payload:             []byte(`{"instruction":"fix the workspace"}`),
				},
			},
		},
		Deployment: repository.AgentDeploymentExecutionContext{
			AgentDeploymentID:         uuid.New(),
			AgentDeploymentSnapshotID: uuid.New(),
			AgentBuildID:              uuid.New(),
			AgentBuildVersionID:       uuid.New(),
			DeploymentType:            "native",
			SnapshotHash:              "snapshot",
			SnapshotConfig:            []byte(`{"entrypoint":"runner"}`),
			AgentBuildVersion: repository.AgentBuildVersionExecutionContext{
				ID:           uuid.New(),
				AgentKind:    "llm_agent",
				AgentSpec:    []byte(`{"agent_kind":"llm_agent","policy_spec":{"instructions":"Use tools and submit when finished."},"output_schema":{"type":"object","properties":{"answer":{"type":"string"}}}}`),
				PolicySpec:   []byte(`{"instructions":"Use tools and submit when finished."}`),
				OutputSchema: []byte(`{"type":"object","properties":{"answer":{"type":"string"}}}`),
			},
			RuntimeProfile: repository.RuntimeProfileExecutionContext{
				ID:                 uuid.New(),
				Name:               "Native Runtime",
				Slug:               "native-runtime",
				ExecutionTarget:    "native",
				TraceMode:          "preferred",
				MaxIterations:      4,
				MaxToolCalls:       5,
				StepTimeoutSeconds: 1,
				RunTimeoutSeconds:  5,
				ProfileConfig:      []byte(`{"sandbox":{"working_directory":"/workspace","readable_roots":["/workspace"],"writable_roots":["/workspace"]}}`),
			},
			ProviderAccount: &repository.ProviderAccountExecutionContext{
				ID:                  uuid.New(),
				ProviderKey:         "openai",
				CredentialReference: "env://OPENAI_API_KEY",
			},
			ModelAlias: &repository.ModelAliasExecutionContext{
				ID:          uuid.New(),
				AliasKey:    "primary-model",
				DisplayName: "Primary Model",
				ModelCatalogEntry: repository.ModelCatalogEntryExecutionContext{
					ID:              uuid.New(),
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					DisplayName:     "GPT-4.1",
				},
			},
		},
	}
}
