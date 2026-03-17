package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

func TestNativeModelInvokerDelegatesToEngine(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-worker")
	client := &provider.FakeClient{
		Response: provider.Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-4.1",
			FinishReason:    "tool_calls",
			ToolCalls: []provider.ToolCall{
				{
					ID:        "submit-1",
					Name:      "submit",
					Arguments: []byte(`{"answer":"worker result"}`),
				},
			},
		},
	}

	invoker := NewNativeModelInvoker(client, &sandbox.FakeProvider{NextSession: session})
	result, err := invoker.InvokeNativeModel(context.Background(), nativeModelExecutionContext())
	if err != nil {
		t.Fatalf("InvokeNativeModel returned error: %v", err)
	}
	if result.StopReason != engine.StopReasonCompleted {
		t.Fatalf("stop reason = %s, want completed", result.StopReason)
	}
	if result.FinalOutput != "worker result" {
		t.Fatalf("final output = %q, want worker result", result.FinalOutput)
	}
	if len(client.Requests) != 1 {
		t.Fatalf("provider request count = %d, want 1", len(client.Requests))
	}
	if session.DestroyCalls() != 1 {
		t.Fatalf("destroy calls = %d, want 1", session.DestroyCalls())
	}
}

func TestNativeModelInvokerFailsClosedWhenSandboxProviderIsMissing(t *testing.T) {
	invoker := NewNativeModelInvoker(&provider.FakeClient{}, sandbox.UnconfiguredProvider{})

	_, err := invoker.InvokeNativeModel(context.Background(), nativeModelExecutionContext())
	if err == nil {
		t.Fatalf("expected sandbox failure")
	}

	failure, ok := engine.AsFailure(err)
	if !ok {
		t.Fatalf("expected engine failure, got %T", err)
	}
	if failure.StopReason != engine.StopReasonSandboxError {
		t.Fatalf("stop reason = %s, want sandbox_error", failure.StopReason)
	}
	if !errors.Is(err, sandbox.ErrProviderNotConfigured) {
		t.Fatalf("error = %v, want ErrProviderNotConfigured", err)
	}
}

func nativeModelExecutionContext() repository.RunAgentExecutionContext {
	runID := uuid.New()
	runAgentID := uuid.New()

	return repository.RunAgentExecutionContext{
		Run: domain.Run{
			ID: runID,
		},
		RunAgent: domain.RunAgent{
			ID:        runAgentID,
			RunID:     runID,
			Status:    domain.RunAgentStatusQueued,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			ID:       uuid.New(),
			Manifest: []byte(`{"challenge":"fixture","tool_policy":{"allowed_tool_kinds":["file"]}}`),
		},
		ChallengeInputSet: &repository.ChallengeInputSetExecutionContext{
			ID:            uuid.New(),
			InputKey:      "default",
			Name:          "Default Input",
			InputChecksum: "checksum",
		},
		Deployment: repository.AgentDeploymentExecutionContext{
			DeploymentType: "native",
			SnapshotConfig: []byte(`{"entrypoint":"runner"}`),
			AgentBuildVersion: repository.AgentBuildVersionExecutionContext{
				ID:         uuid.New(),
				AgentKind:  "llm_agent",
				AgentSpec:  []byte(`{"agent_kind":"llm_agent","policy_spec":{"instructions":"Use tools and submit when finished."}}`),
				PolicySpec: []byte(`{"instructions":"Use tools and submit when finished."}`),
			},
			RuntimeProfile: repository.RuntimeProfileExecutionContext{
				ExecutionTarget:    "native",
				TraceMode:          "preferred",
				MaxIterations:      2,
				MaxToolCalls:       2,
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
				ModelCatalogEntry: repository.ModelCatalogEntryExecutionContext{
					ProviderModelID: "gpt-4.1",
				},
			},
		},
	}
}
