package worker

import (
	"context"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestResponsesInvokerDelegatesToEngine(t *testing.T) {
	client := &provider.FakeResearchClient{
		FakeClient: provider.FakeClient{
			Response: provider.Response{
				ProviderKey:     "openai",
				ProviderModelID: "o4-mini-deep-research",
				FinishReason:    "completed",
				OutputText:      `{"ok":true}`,
				Usage:           provider.Usage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14},
			},
		},
	}
	invoker := NewResponsesInvoker(client)
	result, err := invoker.InvokeResponses(context.Background(), responsesInvokerExecutionContext())
	if err != nil {
		t.Fatalf("InvokeResponses returned error: %v", err)
	}
	if result.StopReason != engine.StopReasonCompleted {
		t.Fatalf("stop reason = %s, want completed", result.StopReason)
	}
	if result.FinalOutput != `{"ok":true}` {
		t.Fatalf("final output = %q", result.FinalOutput)
	}
	if len(client.ResearchRequests) != 1 {
		t.Fatalf("research requests = %d, want 1", len(client.ResearchRequests))
	}
	if !client.ResearchRequests[0].Background {
		t.Fatalf("expected background research request")
	}
}

func responsesInvokerExecutionContext() repository.RunAgentExecutionContext {
	runID := uuid.New()
	runAgentID := uuid.New()

	return repository.RunAgentExecutionContext{
		Run:      domain.Run{ID: runID},
		RunAgent: domain.RunAgent{ID: runAgentID, RunID: runID, Status: domain.RunAgentStatusQueued, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			ID:       uuid.New(),
			Manifest: []byte(`{"version":{"execution_mode":"responses"}}`),
			Challenges: []repository.ChallengeDefinitionExecutionContext{
				{
					ID:           uuid.New(),
					ChallengeKey: "translate",
					Title:        "Translate",
					Definition:   []byte(`{"instructions":"Translate {{text}} to French."}`),
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
					ChallengeKey: "translate",
					CaseKey:      "hello",
					Inputs: []challengepack.CaseInput{
						{Key: "text", Kind: "text", Value: "hello"},
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
				PolicySpec: []byte(`{}`),
			},
			RuntimeProfile: repository.RuntimeProfileExecutionContext{
				ExecutionTarget:   "responses",
				TraceMode:         "preferred",
				RunTimeoutSeconds: 900,
				ProfileConfig:     []byte(`{}`),
			},
			ProviderAccount: &repository.ProviderAccountExecutionContext{
				ID:                  uuid.New(),
				ProviderKey:         "openai",
				CredentialReference: "env://OPENAI_API_KEY",
			},
			ModelID: "o4-mini-deep-research",
		},
	}
}
