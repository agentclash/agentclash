package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestPromptEvalExecutorSingleCall(t *testing.T) {
	client := &provider.FakeClient{
		Response: provider.Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-4.1-mini",
			FinishReason:    "stop",
			OutputText:      "Bonjour, monde.",
			Usage:           provider.Usage{InputTokens: 12, OutputTokens: 6, TotalTokens: 18},
		},
	}
	observer := &recordingObserver{}
	executor := NewPromptEvalExecutor(client, observer)

	result, err := executor.Execute(context.Background(), promptEvalExecutionContext())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if result.StopReason != StopReasonCompleted {
		t.Fatalf("stop reason = %s, want completed", result.StopReason)
	}
	if result.FinalOutput != "Bonjour, monde." {
		t.Fatalf("final output = %q, want %q", result.FinalOutput, "Bonjour, monde.")
	}
	if result.StepCount != 1 {
		t.Fatalf("step count = %d, want 1", result.StepCount)
	}
	if result.ToolCallCount != 0 {
		t.Fatalf("tool call count = %d, want 0", result.ToolCallCount)
	}
	if result.Usage.TotalTokens != 18 {
		t.Fatalf("total tokens = %d, want 18", result.Usage.TotalTokens)
	}

	if len(client.Requests) != 1 {
		t.Fatalf("provider call count = %d, want 1", len(client.Requests))
	}
	req := client.Requests[0]
	if len(req.Tools) != 0 {
		t.Fatalf("request tools = %d, want 0", len(req.Tools))
	}
	if len(req.Messages) != 2 {
		t.Fatalf("request message count = %d, want 2 (system + user)", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Fatalf("messages[0].role = %q, want system", req.Messages[0].Role)
	}
	if req.Messages[1].Role != "user" {
		t.Fatalf("messages[1].role = %q, want user", req.Messages[1].Role)
	}
	if !strings.Contains(req.Messages[1].Content, "French") {
		t.Fatalf("rendered user prompt %q missing variable substitution for language", req.Messages[1].Content)
	}
	if !strings.Contains(req.Messages[1].Content, "hello world") {
		t.Fatalf("rendered user prompt %q missing variable substitution for text", req.Messages[1].Content)
	}
	if strings.Contains(req.Messages[1].Content, "{{") {
		t.Fatalf("rendered user prompt %q still contains unrendered template tokens", req.Messages[1].Content)
	}

	if !observer.runComplete {
		t.Fatalf("observer did not receive OnRunComplete")
	}
	if observer.runFailure {
		t.Fatalf("observer received unexpected OnRunFailure")
	}
	if observer.providerCalls != 1 || observer.providerResponses != 1 {
		t.Fatalf("observer call counts: calls=%d responses=%d, want 1/1", observer.providerCalls, observer.providerResponses)
	}
	if observer.stepStarts != 1 || observer.stepEnds != 1 {
		t.Fatalf("observer step counts: start=%d end=%d, want 1/1", observer.stepStarts, observer.stepEnds)
	}
}

func TestPromptEvalExecutorProviderFailurePropagates(t *testing.T) {
	client := &provider.FakeClient{
		Err: provider.NewFailure("openai", provider.FailureCodeAuth, "bad key", false, nil),
	}
	observer := &recordingObserver{}
	executor := NewPromptEvalExecutor(client, observer)

	_, err := executor.Execute(context.Background(), promptEvalExecutionContext())
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := provider.AsFailure(err); !ok {
		t.Fatalf("expected provider.Failure, got %T", err)
	}
	if !observer.runFailure {
		t.Fatalf("observer did not record OnRunFailure")
	}
}

func TestPromptEvalExecutorUsesFirstCaseWhenInputSetHasMany(t *testing.T) {
	ctx := promptEvalExecutionContext()
	// Add two extra cases to confirm the executor ignores them without erroring.
	extra := repository.ChallengeCaseExecutionContext{
		ID:           uuid.New(),
		ChallengeKey: "translate-greeting",
		CaseKey:      "german-hello",
		Inputs: []challengepack.CaseInput{
			{Key: "text", Kind: "text", Value: "ignored text"},
			{Key: "language", Kind: "text", Value: "German"},
		},
	}
	ctx.ChallengeInputSet.Cases = append(ctx.ChallengeInputSet.Cases, extra, extra)

	client := &provider.FakeClient{Response: provider.Response{OutputText: "Bonjour"}}
	if _, err := NewPromptEvalExecutor(client, &recordingObserver{}).Execute(context.Background(), ctx); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(client.Requests) != 1 {
		t.Fatalf("provider call count = %d, want 1", len(client.Requests))
	}
	userMessage := client.Requests[0].Messages[len(client.Requests[0].Messages)-1].Content
	if !strings.Contains(userMessage, "French") {
		t.Fatalf("rendered prompt %q should reference first case language 'French'", userMessage)
	}
	if strings.Contains(userMessage, "German") {
		t.Fatalf("rendered prompt %q must not leak second case language 'German'", userMessage)
	}
}

func TestPromptEvalExecutorPassesThroughUnresolvedTokens(t *testing.T) {
	ctx := promptEvalExecutionContext()
	ctx.ChallengePackVersion.Challenges[0].Definition = []byte(`{"instructions":"Use {{text}} and respect {{missing_var}}."}`)

	client := &provider.FakeClient{Response: provider.Response{OutputText: "ok"}}
	if _, err := NewPromptEvalExecutor(client, &recordingObserver{}).Execute(context.Background(), ctx); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(client.Requests) != 1 {
		t.Fatalf("provider call count = %d, want 1", len(client.Requests))
	}
	userMessage := client.Requests[0].Messages[len(client.Requests[0].Messages)-1].Content
	if !strings.Contains(userMessage, "hello world") {
		t.Fatalf("rendered prompt %q should substitute resolved var", userMessage)
	}
	if !strings.Contains(userMessage, "{{missing_var}}") {
		t.Fatalf("rendered prompt %q should pass unresolved token through verbatim", userMessage)
	}
}

func TestPromptEvalExecutorRejectsMissingInstructions(t *testing.T) {
	ctx := promptEvalExecutionContext()
	ctx.ChallengePackVersion.Challenges[0].Definition = []byte(`{}`)

	client := &provider.FakeClient{Response: provider.Response{OutputText: "ignored"}}
	_, err := NewPromptEvalExecutor(client, &recordingObserver{}).Execute(context.Background(), ctx)
	if err == nil {
		t.Fatalf("expected error when instructions missing")
	}
	if _, ok := provider.AsFailure(err); !ok {
		t.Fatalf("expected provider.Failure, got %T", err)
	}
	if len(client.Requests) != 0 {
		t.Fatalf("provider should not be called when instructions missing; got %d requests", len(client.Requests))
	}
}

func promptEvalExecutionContext() repository.RunAgentExecutionContext {
	runID := uuid.New()
	runAgentID := uuid.New()

	return repository.RunAgentExecutionContext{
		Run:      domain.Run{ID: runID},
		RunAgent: domain.RunAgent{ID: runAgentID, RunID: runID, Status: domain.RunAgentStatusQueued, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			ID:       uuid.New(),
			Manifest: []byte(`{"version":{"execution_mode":"prompt_eval"}}`),
			Challenges: []repository.ChallengeDefinitionExecutionContext{
				{
					ID:           uuid.New(),
					ChallengeKey: "translate-greeting",
					Title:        "Translate a greeting",
					Definition:   []byte(`{"instructions":"Translate {{text}} to {{language}}."}`),
				},
			},
		},
		ChallengeInputSet: &repository.ChallengeInputSetExecutionContext{
			ID:       uuid.New(),
			InputKey: "default",
			Name:     "Default",
			Cases: []repository.ChallengeCaseExecutionContext{
				{
					ID:           uuid.New(),
					ChallengeKey: "translate-greeting",
					CaseKey:      "french-hello",
					Inputs: []challengepack.CaseInput{
						{Key: "text", Kind: "text", Value: "hello world"},
						{Key: "language", Kind: "text", Value: "French"},
					},
				},
			},
		},
		Deployment: repository.AgentDeploymentExecutionContext{
			DeploymentType: "native",
			SnapshotConfig: []byte(`{}`),
			AgentBuildVersion: repository.AgentBuildVersionExecutionContext{
				ID:         uuid.New(),
				AgentKind:  "llm_agent",
				AgentSpec:  []byte(`{}`),
				PolicySpec: []byte(`{"instructions":"You are a precise translator."}`),
			},
			RuntimeProfile: repository.RuntimeProfileExecutionContext{
				ExecutionTarget:    "prompt_eval",
				TraceMode:          "preferred",
				StepTimeoutSeconds: 30,
				RunTimeoutSeconds:  60,
				ProfileConfig:      []byte(`{}`),
			},
			ProviderAccount: &repository.ProviderAccountExecutionContext{
				ID:                  uuid.New(),
				ProviderKey:         "openai",
				CredentialReference: "env://OPENAI_API_KEY",
			},
			ModelAlias: &repository.ModelAliasExecutionContext{
				ModelCatalogEntry: repository.ModelCatalogEntryExecutionContext{
					ProviderModelID: "gpt-4.1-mini",
				},
			},
		},
	}
}

type recordingObserver struct {
	stepStarts        int
	stepEnds          int
	providerCalls     int
	providerResponses int
	runComplete       bool
	runFailure        bool
}

func (o *recordingObserver) OnStepStart(context.Context, int) error {
	o.stepStarts++
	return nil
}
func (o *recordingObserver) OnProviderCall(context.Context, provider.Request) error {
	o.providerCalls++
	return nil
}
func (o *recordingObserver) OnProviderOutput(context.Context, provider.Request, provider.StreamDelta) error {
	return nil
}
func (o *recordingObserver) OnProviderResponse(context.Context, provider.Response) error {
	o.providerResponses++
	return nil
}
func (o *recordingObserver) OnToolExecution(context.Context, ToolExecutionRecord) error { return nil }
func (o *recordingObserver) OnStepEnd(context.Context, int) error {
	o.stepEnds++
	return nil
}
func (o *recordingObserver) OnRunComplete(context.Context, Result) error {
	o.runComplete = true
	return nil
}
func (o *recordingObserver) OnRunFailure(context.Context, error) error {
	o.runFailure = true
	return nil
}
