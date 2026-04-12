package worker

import (
	"context"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestPromptEvalInvokerDelegatesToEngine(t *testing.T) {
	client := &provider.FakeClient{
		Response: provider.Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-4.1-mini",
			FinishReason:    "stop",
			OutputText:      "Bonjour",
			Usage:           provider.Usage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14},
		},
	}

	invoker := NewPromptEvalInvoker(client)
	result, err := invoker.InvokePromptEval(context.Background(), promptEvalInvokerExecutionContext())
	if err != nil {
		t.Fatalf("InvokePromptEval returned error: %v", err)
	}
	if result.StopReason != engine.StopReasonCompleted {
		t.Fatalf("stop reason = %s, want completed", result.StopReason)
	}
	if result.FinalOutput != "Bonjour" {
		t.Fatalf("final output = %q, want Bonjour", result.FinalOutput)
	}
	if len(client.Requests) != 1 {
		t.Fatalf("provider request count = %d, want 1", len(client.Requests))
	}
	if len(client.Requests[0].Tools) != 0 {
		t.Fatalf("prompt_eval request must not declare tools, got %d", len(client.Requests[0].Tools))
	}
}

func promptEvalInvokerExecutionContext() repository.RunAgentExecutionContext {
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
				ExecutionTarget:   "prompt_eval",
				TraceMode:         "preferred",
				RunTimeoutSeconds: 30,
				ProfileConfig:     []byte(`{}`),
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
