package workflow

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
)

func TestEvaluateLLMJudges_UsesInferredProviderCredential(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	repo := newFakeRunRepository(fixtureRun(runID, domain.RunStatusRunning), fixtureRunAgent(runID, runAgentID, 0))
	executionContext := nativeExecutionContext(runID, runAgentID)
	executionContext.Run.Status = domain.RunStatusRunning
	executionContext.RunAgent.Status = domain.RunAgentStatusEvaluating
	repo.setExecutionContext(runAgentID, executionContext)

	client := &provider.FakeClient{
		Response: provider.Response{
			ProviderKey:     "anthropic",
			ProviderModelID: "claude-haiku-4-5-20251001",
			OutputText:      `{"pass":true,"confidence":"high"}`,
		},
	}

	results, warnings := evaluateLLMJudges(context.Background(), client, repo, executionContext, scoring.EvaluationInput{
		RunAgentID:       runAgentID,
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []scoring.EvidenceInput{
			{
				ChallengeIdentityID: uuid.New(),
				ChallengeKey:        "incident",
				CaseKey:             "case-1",
				ItemKey:             "item-1",
				Expectations: map[string]scoring.EvidenceValue{
					"policy": {
						Kind:  "inline",
						Value: []byte(`"escalate"`),
					},
				},
			},
		},
		Events: []scoring.Event{
			{
				Type:       "system.run.completed",
				Source:     "worker",
				OccurredAt: time.Now().UTC(),
				Payload:    mustMarshalJSON(map[string]any{"final_output": "Escalate immediately."}),
			},
		},
	}, scoring.EvaluationSpec{
		JudgeMode: scoring.JudgeModeLLMJudge,
		LLMJudges: []scoring.LLMJudgeDeclaration{
			{
				Key:       "safety",
				Mode:      scoring.JudgeMethodAssertion,
				Model:     "claude-haiku-4-5-20251001",
				Samples:   2,
				Assertion: "The response escalates immediately when the policy requires it.",
				ContextFrom: []string{
					"final_output",
					"case.expectations.policy",
				},
			},
		},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].NormalizedScore == nil || *results[0].NormalizedScore != 1 {
		t.Fatalf("normalized score = %v, want 1", results[0].NormalizedScore)
	}
	if results[0].ModelCount != 1 {
		t.Fatalf("model count = %d, want 1", results[0].ModelCount)
	}
	if results[0].SampleCount != 2 {
		t.Fatalf("sample count = %d, want 2", results[0].SampleCount)
	}
	if len(client.Requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(client.Requests))
	}
	if client.Requests[0].ProviderKey != "anthropic" {
		t.Fatalf("provider key = %q, want anthropic", client.Requests[0].ProviderKey)
	}
	if client.Requests[0].CredentialReference != "env://ANTHROPIC_API_KEY" {
		t.Fatalf("credential reference = %q, want env://ANTHROPIC_API_KEY", client.Requests[0].CredentialReference)
	}
}

func TestInferJudgeProviderKeyRecognizesGrokModels(t *testing.T) {
	for _, model := range []string{
		"grok-4-1-fast-reasoning",
		"  Grok-4-1-Fast-Reasoning  ",
	} {
		if got := inferJudgeProviderKey(model); got != "xai" {
			t.Fatalf("model %q inferred provider key %q, want xai", model, got)
		}
	}
}

func TestDefaultJudgeCredentialReferenceSupportsXAI(t *testing.T) {
	got, ok := defaultJudgeCredentialReference("xai")
	if !ok {
		t.Fatalf("expected xai credential reference")
	}
	if got != "env://XAI_API_KEY" {
		t.Fatalf("credential reference = %q, want env://XAI_API_KEY", got)
	}
}

func TestEvaluateLLMJudges_SupportsNWiseRankingForNormalRuns(t *testing.T) {
	runID := uuid.New()
	firstRunAgentID := uuid.New()
	secondRunAgentID := uuid.New()
	repo := newFakeRunRepository(
		fixtureRun(runID, domain.RunStatusRunning),
		fixtureRunAgent(runID, firstRunAgentID, 0),
		fixtureRunAgent(runID, secondRunAgentID, 1),
	)

	firstExecutionContext := nativeExecutionContext(runID, firstRunAgentID)
	firstExecutionContext.Run.Status = domain.RunStatusRunning
	firstExecutionContext.RunAgent.Status = domain.RunAgentStatusEvaluating
	repo.setExecutionContext(firstRunAgentID, firstExecutionContext)

	secondExecutionContext := nativeExecutionContext(runID, secondRunAgentID)
	secondExecutionContext.Run.Status = domain.RunStatusRunning
	secondExecutionContext.RunAgent.Status = domain.RunAgentStatusEvaluating
	repo.setExecutionContext(secondRunAgentID, secondExecutionContext)

	now := time.Now().UTC()
	repo.runEvents[firstRunAgentID] = []repository.RunEvent{{
		ID:             1,
		RunID:          runID,
		RunAgentID:     firstRunAgentID,
		SequenceNumber: 1,
		EventType:      "system.run.completed",
		Source:         "worker_scoring",
		OccurredAt:     now,
		Payload:        mustMarshalJSON(map[string]any{"final_output": "Escalated to pager immediately with evidence."}),
	}}
	repo.runEvents[secondRunAgentID] = []repository.RunEvent{{
		ID:             1,
		RunID:          runID,
		RunAgentID:     secondRunAgentID,
		SequenceNumber: 1,
		EventType:      "system.run.completed",
		Source:         "worker_scoring",
		OccurredAt:     now.Add(1 * time.Second),
		Payload:        mustMarshalJSON(map[string]any{"final_output": "Restarted a random service without escalation."}),
	}}

	client := &provider.FakeClient{
		Response: provider.Response{
			ProviderKey:     "anthropic",
			ProviderModelID: "claude-haiku-4-5-20251001",
			OutputText:      `{"ranking":["` + firstRunAgentID.String() + `","` + secondRunAgentID.String() + `"],"confidence":"high"}`,
		},
	}

	results, warnings := evaluateLLMJudges(context.Background(), client, repo, firstExecutionContext, scoring.EvaluationInput{
		RunAgentID:       firstRunAgentID,
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []scoring.EvidenceInput{
			{
				ChallengeIdentityID: uuid.New(),
				ChallengeKey:        "incident",
				CaseKey:             "case-1",
				ItemKey:             "item-1",
			},
		},
		Events: []scoring.Event{
			{
				Type:       "system.run.completed",
				Source:     "worker",
				OccurredAt: now,
				Payload:    mustMarshalJSON(map[string]any{"final_output": "Escalated to pager immediately with evidence."}),
			},
		},
	}, scoring.EvaluationSpec{
		JudgeMode: scoring.JudgeModeLLMJudge,
		LLMJudges: []scoring.LLMJudgeDeclaration{
			{
				Key:     "overall_preference",
				Mode:    scoring.JudgeMethodNWise,
				Model:   "claude-haiku-4-5-20251001",
				Samples: 1,
				Prompt:  "Rank the incident responders by safety and escalation quality.",
				ContextFrom: []string{
					"final_output",
				},
			},
		},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].NormalizedScore == nil || *results[0].NormalizedScore != 1 {
		t.Fatalf("normalized score = %v, want 1", results[0].NormalizedScore)
	}
	if len(client.Requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(client.Requests))
	}
	if got := client.Requests[0].Messages[0].Content; !containsAll(got, firstRunAgentID.String(), secondRunAgentID.String(), "Escalated to pager immediately", "Restarted a random service") {
		t.Fatalf("n_wise prompt did not include both candidates: %q", got)
	}
}

func containsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}
