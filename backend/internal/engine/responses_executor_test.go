package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/sandbox"
)

func TestResponsesExecutorSingleResearchCall(t *testing.T) {
	client := &provider.FakeResearchClient{
		FakeClient: provider.FakeClient{
			Response: provider.Response{
				ProviderKey:     "openai",
				ProviderModelID: "o4-mini-deep-research",
				FinishReason:    "completed",
				OutputText:      `{"title":"CS224N"}`,
				Usage:           provider.Usage{InputTokens: 100, OutputTokens: 200, TotalTokens: 300},
			},
		},
	}
	observer := &recordingObserver{}
	executor := NewResponsesExecutor(client, observer)

	ctx := promptEvalExecutionContext()
	ctx.ChallengePackVersion.Manifest = []byte(`{"version":{"execution_mode":"responses"}}`)

	result, err := executor.Execute(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.FinalOutput != `{"title":"CS224N"}` {
		t.Fatalf("final output = %q", result.FinalOutput)
	}
	if len(client.ResearchRequests) != 1 {
		t.Fatalf("research call count = %d, want 1", len(client.ResearchRequests))
	}
	req := client.ResearchRequests[0]
	if !strings.Contains(req.Input, "French") {
		t.Fatalf("research input missing rendered template vars: %q", req.Input)
	}
	if !strings.Contains(req.Instructions, "precise translator") {
		t.Fatalf("research instructions missing policy text: %q", req.Instructions)
	}
	if !req.Background {
		t.Fatalf("expected background research request")
	}
}

func TestResponsesExecutorRejectsNonOpenAIProvider(t *testing.T) {
	client := &provider.FakeResearchClient{}
	executor := NewResponsesExecutor(client, &recordingObserver{})

	ctx := promptEvalExecutionContext()
	ctx.Deployment.ProviderAccount.ProviderKey = "anthropic"

	_, err := executor.Execute(context.Background(), ctx)
	if err == nil {
		t.Fatal("expected error for non-openai provider")
	}
	failure, ok := provider.AsFailure(err)
	if !ok || failure.Code != provider.FailureCodeUnsupportedCapability {
		t.Fatalf("failure = %#v, want unsupported_capability", failure)
	}
}

func TestResponsesExecutorProvisionsSandboxWhenConfigured(t *testing.T) {
	client := &provider.FakeResearchClient{
		FakeClient: provider.FakeClient{
			Response: provider.Response{
				ProviderKey:  "openai",
				FinishReason: "completed",
				OutputText:   `{"ok":true}`,
			},
		},
	}
	fakeSandbox := &sandbox.FakeProvider{}
	executor := NewResponsesExecutor(client, &recordingObserver{}).WithSandboxProvider(fakeSandbox)

	ctx := promptEvalExecutionContext()
	ctx.ChallengePackVersion.Manifest = []byte(`{"version":{"execution_mode":"responses"},"sandbox":{"network_access":true}}`)

	if _, err := executor.Execute(context.Background(), ctx); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(fakeSandbox.CreateRequests) != 1 {
		t.Fatalf("sandbox create requests = %d, want 1", len(fakeSandbox.CreateRequests))
	}
	if !fakeSandbox.CreateRequests[0].ToolPolicy.AllowNetwork {
		t.Fatal("expected sandbox network access from manifest")
	}
}

func TestResponsesExecutorSkipsSandboxWithoutManifestConfig(t *testing.T) {
	client := &provider.FakeResearchClient{
		FakeClient: provider.FakeClient{
			Response: provider.Response{
				ProviderKey:  "openai",
				FinishReason: "completed",
				OutputText:   `{"ok":true}`,
			},
		},
	}
	fakeSandbox := &sandbox.FakeProvider{}
	executor := NewResponsesExecutor(client, &recordingObserver{}).WithSandboxProvider(fakeSandbox)

	ctx := promptEvalExecutionContext()
	ctx.ChallengePackVersion.Manifest = []byte(`{"version":{"execution_mode":"responses"}}`)

	if _, err := executor.Execute(context.Background(), ctx); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(fakeSandbox.CreateRequests) != 0 {
		t.Fatalf("sandbox create requests = %d, want 0", len(fakeSandbox.CreateRequests))
	}
}
