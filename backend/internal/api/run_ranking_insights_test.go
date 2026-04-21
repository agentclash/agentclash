package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/budget"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRunReadManagerGenerateRankingInsightsRejectsSingleAgentRun(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:            runID,
			WorkspaceID:   workspaceID,
			Status:        domain.RunStatusCompleted,
			ExecutionMode: "single_agent",
		},
		runAgents: []domain.RunAgent{
			{ID: uuid.New(), RunID: runID, LaneIndex: 0, Label: "Solo", Status: domain.RunAgentStatusCompleted},
		},
	}).WithInsightsClient(&provider.FakeClient{})

	_, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(workspaceID), runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: uuid.New(),
		ModelAliasID:      uuid.New(),
	})

	var validationErr RunRankingInsightsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "invalid_run_for_insights" {
		t.Fatalf("validation code = %q, want invalid_run_for_insights", validationErr.Code)
	}
}

func TestRunReadManagerGenerateRankingInsightsRejectsUnavailableRanking(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:            runID,
			WorkspaceID:   workspaceID,
			Status:        domain.RunStatusCompleted,
			ExecutionMode: "comparison",
		},
		runAgents: []domain.RunAgent{
			{ID: uuid.New(), RunID: runID, LaneIndex: 0, Label: "Alpha", Status: domain.RunAgentStatusCompleted},
			{ID: uuid.New(), RunID: runID, LaneIndex: 1, Label: "Beta", Status: domain.RunAgentStatusCompleted},
		},
		getRunScorecardErr: repository.ErrRunScorecardNotFound,
	}).WithInsightsClient(&provider.FakeClient{})

	_, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(workspaceID), runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: uuid.New(),
		ModelAliasID:      uuid.New(),
	})

	var validationErr RunRankingInsightsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "ranking_unavailable" {
		t.Fatalf("validation code = %q, want ranking_unavailable", validationErr.Code)
	}
}

func TestRunReadManagerGenerateRankingInsightsInvokesSelectedProvider(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	providerAccountID := uuid.New()
	modelCatalogEntryID := uuid.New()
	modelAliasID := uuid.New()
	alphaID := uuid.New()
	betaID := uuid.New()
	now := time.Date(2026, 4, 20, 8, 30, 0, 0, time.UTC)

	client := &provider.FakeClient{
		Response: provider.Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-5.4-mini",
			OutputText: `{
				"recommended_winner":{"run_agent_id":"` + alphaID.String() + `","label":"Alpha"},
				"why_it_won":"Alpha led on correctness while staying competitive elsewhere.",
				"tradeoffs":["Beta stayed closer on reliability than correctness."],
				"best_for_reliability":{"run_agent_id":"` + betaID.String() + `","label":"Beta","reason":"Beta had the highest reliability score."},
				"best_for_cost":{"run_agent_id":"` + alphaID.String() + `","label":"Alpha","reason":"Alpha kept cost lower in this run."},
				"best_for_latency":{"run_agent_id":"` + betaID.String() + `","label":"Beta","reason":"Beta was the fastest lane."},
				"model_summaries":[
					{"run_agent_id":"` + alphaID.String() + `","label":"Alpha","strongest_dimension":"correctness","weakest_dimension":"latency","summary":"Strongest overall performer."},
					{"run_agent_id":"` + betaID.String() + `","label":"Beta","strongest_dimension":"reliability","weakest_dimension":"cost","summary":"A viable fallback when reliability matters most."}
				],
				"recommended_next_step":"Run a tighter comparison focused on reliability-sensitive cases.",
				"confidence_notes":"Confidence is moderate because Beta remains close on reliability."
			}`,
		},
	}

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: buildInsightsRun(runID, workspaceID),
		runAgents: []domain.RunAgent{
			{ID: alphaID, RunID: runID, LaneIndex: 0, Label: "Alpha", Status: domain.RunAgentStatusCompleted},
			{ID: betaID, RunID: runID, LaneIndex: 1, Label: "Beta", Status: domain.RunAgentStatusCompleted},
		},
		runScorecard: buildInsightsScorecard(t, runID, alphaID, betaID),
		providerAccount: repository.ProviderAccountRow{
			ID:                  providerAccountID,
			WorkspaceID:         uuidPtr(workspaceID),
			ProviderKey:         "openai",
			CredentialReference: "env://OPENAI_API_KEY",
			Status:              "active",
		},
		modelAlias: repository.ModelAliasRow{
			ID:                  modelAliasID,
			WorkspaceID:         uuidPtr(workspaceID),
			ProviderAccountID:   uuidPtr(providerAccountID),
			ModelCatalogEntryID: modelCatalogEntryID,
			AliasKey:            "insights-default",
			DisplayName:         "Insights Default",
			Status:              "active",
		},
		modelCatalogEntry: repository.ModelCatalogEntryRow{
			ID:              modelCatalogEntryID,
			ProviderKey:     "openai",
			ProviderModelID: "gpt-5.4-mini",
			DisplayName:     "GPT-5.4 Mini",
		},
	})
	manager = manager.WithInsightsClient(client)
	manager.now = func() time.Time { return now }

	result, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(workspaceID), runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: providerAccountID,
		ModelAliasID:      modelAliasID,
	})
	if err != nil {
		t.Fatalf("GenerateRunRankingInsights returned error: %v", err)
	}
	if len(client.Requests) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(client.Requests))
	}
	request := client.Requests[0]
	if request.ProviderKey != "openai" {
		t.Fatalf("provider key = %q, want openai", request.ProviderKey)
	}
	if request.ProviderAccountID != providerAccountID.String() {
		t.Fatalf("provider account id = %q, want %q", request.ProviderAccountID, providerAccountID.String())
	}
	if request.Model != "gpt-5.4-mini" {
		t.Fatalf("model = %q, want gpt-5.4-mini", request.Model)
	}
	if request.CredentialReference != "env://OPENAI_API_KEY" {
		t.Fatalf("credential reference = %q, want env://OPENAI_API_KEY", request.CredentialReference)
	}

	var metadata map[string]any
	if err := json.Unmarshal(request.Metadata, &metadata); err != nil {
		t.Fatalf("metadata json: %v", err)
	}
	if metadata["feature"] != "run_ranking_insights" {
		t.Fatalf("metadata feature = %#v, want run_ranking_insights", metadata["feature"])
	}

	if result.Insights.ProviderKey != "openai" {
		t.Fatalf("provider key = %q, want openai", result.Insights.ProviderKey)
	}
	if result.Insights.ProviderModelID != "gpt-5.4-mini" {
		t.Fatalf("provider model id = %q, want gpt-5.4-mini", result.Insights.ProviderModelID)
	}
	if !result.Insights.GeneratedAt.Equal(now) {
		t.Fatalf("generated at = %s, want %s", result.Insights.GeneratedAt, now)
	}
	if result.Insights.RecommendedWinner.RunAgentID != alphaID {
		t.Fatalf("recommended winner = %s, want %s", result.Insights.RecommendedWinner.RunAgentID, alphaID)
	}
}

func TestRunReadManagerGenerateRankingInsightsLoadsWorkspaceSecrets(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	providerAccountID := uuid.New()
	modelCatalogEntryID := uuid.New()
	modelAliasID := uuid.New()
	alphaID := uuid.New()
	betaID := uuid.New()

	client := &workspaceSecretAssertingClient{
		t:                     t,
		expectedCredentialRef: "workspace-secret://OPENAI_API_KEY",
		expectedSecret:        "super-secret",
		response: provider.Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-5.4-mini",
			OutputText: `{
				"recommended_winner":{"run_agent_id":"` + alphaID.String() + `","label":"Alpha"},
				"why_it_won":"Alpha won this run.",
				"tradeoffs":["Beta stayed close on latency."],
				"model_summaries":[
					{"run_agent_id":"` + alphaID.String() + `","label":"Alpha","strongest_dimension":"correctness","weakest_dimension":"latency","summary":"Strong overall."}
				],
				"recommended_next_step":"Retry with a latency-focused pack.",
				"confidence_notes":"Confidence is moderate."
			}`,
		},
	}

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: buildInsightsRun(runID, workspaceID),
		runAgents: []domain.RunAgent{
			{ID: alphaID, RunID: runID, LaneIndex: 0, Label: "Alpha", Status: domain.RunAgentStatusCompleted},
			{ID: betaID, RunID: runID, LaneIndex: 1, Label: "Beta", Status: domain.RunAgentStatusCompleted},
		},
		runScorecard: buildInsightsScorecard(t, runID, alphaID, betaID),
		providerAccount: repository.ProviderAccountRow{
			ID:                  providerAccountID,
			WorkspaceID:         uuidPtr(workspaceID),
			ProviderKey:         "openai",
			CredentialReference: "workspace-secret://OPENAI_API_KEY",
			Status:              "active",
		},
		modelAlias: repository.ModelAliasRow{
			ID:                  modelAliasID,
			WorkspaceID:         uuidPtr(workspaceID),
			ModelCatalogEntryID: modelCatalogEntryID,
			AliasKey:            "insights-default",
			DisplayName:         "Insights Default",
			Status:              "active",
		},
		modelCatalogEntry: repository.ModelCatalogEntryRow{
			ID:              modelCatalogEntryID,
			ProviderKey:     "openai",
			ProviderModelID: "gpt-5.4-mini",
			DisplayName:     "GPT-5.4 Mini",
		},
		workspaceSecrets: map[string]string{
			"OPENAI_API_KEY": "super-secret",
		},
	}).WithInsightsClient(client)

	_, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(workspaceID), runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: providerAccountID,
		ModelAliasID:      modelAliasID,
	})
	if err != nil {
		t.Fatalf("GenerateRunRankingInsights returned error: %v", err)
	}
	if !client.called {
		t.Fatalf("expected provider client to be called")
	}
}

func TestRunReadManagerGenerateRankingInsightsRejectsForbiddenCallerWithoutInvokingProvider(t *testing.T) {
	fixture := newRankingInsightsFixture(t)
	client := &provider.FakeClient{}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), fixture.repo).WithInsightsClient(client)

	_, err := manager.GenerateRunRankingInsights(context.Background(), Caller{
		UserID:               uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{},
	}, fixture.runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: fixture.providerAccountID,
		ModelAliasID:      fixture.modelAliasID,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
	if len(client.Requests) != 0 {
		t.Fatalf("provider requests = %d, want 0", len(client.Requests))
	}
}

func TestRunReadManagerGenerateRankingInsightsRejectsInactiveProviderAccount(t *testing.T) {
	fixture := newRankingInsightsFixture(t)
	fixture.repo.providerAccount.Status = "archived"
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), fixture.repo).WithInsightsClient(&provider.FakeClient{})

	_, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(fixture.workspaceID), fixture.runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: fixture.providerAccountID,
		ModelAliasID:      fixture.modelAliasID,
	})

	var validationErr RunRankingInsightsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "invalid_provider_account_id" {
		t.Fatalf("validation code = %q, want invalid_provider_account_id", validationErr.Code)
	}
	if validationErr.Message != "provider_account_id must reference an active provider account visible to the run workspace" {
		t.Fatalf("validation message = %q", validationErr.Message)
	}
}

func TestRunReadManagerGenerateRankingInsightsRejectsModelAliasBoundToDifferentProviderAccount(t *testing.T) {
	fixture := newRankingInsightsFixture(t)
	fixture.repo.modelAlias.ProviderAccountID = uuidPtr(uuid.New())
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), fixture.repo).WithInsightsClient(&provider.FakeClient{})

	_, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(fixture.workspaceID), fixture.runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: fixture.providerAccountID,
		ModelAliasID:      fixture.modelAliasID,
	})

	var validationErr RunRankingInsightsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "invalid_model_alias_id" {
		t.Fatalf("validation code = %q, want invalid_model_alias_id", validationErr.Code)
	}
	if validationErr.Message != "model_alias_id must reference an active model alias visible to the run workspace" {
		t.Fatalf("validation message = %q", validationErr.Message)
	}
}

func TestRunReadManagerGenerateRankingInsightsRejectsModelCatalogProviderMismatch(t *testing.T) {
	fixture := newRankingInsightsFixture(t)
	fixture.repo.modelCatalogEntry.ProviderKey = "anthropic"
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), fixture.repo).WithInsightsClient(&provider.FakeClient{})

	_, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(fixture.workspaceID), fixture.runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: fixture.providerAccountID,
		ModelAliasID:      fixture.modelAliasID,
	})

	var validationErr RunRankingInsightsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "invalid_model_alias_id" {
		t.Fatalf("validation code = %q, want invalid_model_alias_id", validationErr.Code)
	}
}

func TestRunReadManagerGenerateRankingInsightsRejectsBudgetExceeded(t *testing.T) {
	fixture := newRankingInsightsFixture(t)
	policyID := uuid.New()
	fixture.repo.spendPolicies = []repository.SpendPolicyRow{{ID: policyID, WorkspaceID: uuidPtr(fixture.workspaceID)}}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), fixture.repo).
		WithInsightsClient(&provider.FakeClient{}).
		WithBudgetChecker(fakeBudgetChecker{
			results: map[uuid.UUID]budget.BudgetCheckResult{
				policyID: {Allowed: false},
			},
		})

	_, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(fixture.workspaceID), fixture.runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: fixture.providerAccountID,
		ModelAliasID:      fixture.modelAliasID,
	})

	var validationErr RunRankingInsightsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "budget_exceeded" {
		t.Fatalf("validation code = %q, want budget_exceeded", validationErr.Code)
	}
}

func TestRunReadManagerGenerateRankingInsightsRejectsRateLimitedRun(t *testing.T) {
	fixture := newRankingInsightsFixture(t)
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), fixture.repo).
		WithInsightsClient(&provider.FakeClient{}).
		WithInsightsRateLimiter(fakeWorkspaceRateLimiter{
			allowed:    false,
			retryAfter: 3 * time.Second,
		})

	_, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(fixture.workspaceID), fixture.runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: fixture.providerAccountID,
		ModelAliasID:      fixture.modelAliasID,
	})

	var rateLimitErr RunRankingInsightsRateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("error = %v, want rate limit error", err)
	}
	if rateLimitErr.RetryAfter != 3*time.Second {
		t.Fatalf("retry after = %s, want 3s", rateLimitErr.RetryAfter)
	}
}

func TestRunReadManagerGenerateRankingInsightsRejectsInvalidModelJSON(t *testing.T) {
	fixture := newRankingInsightsFixture(t)
	client := &provider.FakeClient{
		Response: provider.Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-5.4-mini",
			OutputText:      `not json at all`,
		},
	}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), fixture.repo).WithInsightsClient(client)

	_, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(fixture.workspaceID), fixture.runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: fixture.providerAccountID,
		ModelAliasID:      fixture.modelAliasID,
	})

	var validationErr RunRankingInsightsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "invalid_insights_output" {
		t.Fatalf("validation code = %q, want invalid_insights_output", validationErr.Code)
	}
}

func TestRunReadManagerGenerateRankingInsightsRejectsWinnerOutsideRun(t *testing.T) {
	fixture := newRankingInsightsFixture(t)
	client := &provider.FakeClient{
		Response: provider.Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-5.4-mini",
			OutputText: `{
				"recommended_winner":{"run_agent_id":"` + uuid.New().String() + `","label":"Injected"},
				"why_it_won":"Not real.",
				"tradeoffs":["Fake result."],
				"model_summaries":[
					{"run_agent_id":"` + fixture.alphaID.String() + `","label":"Alpha","strongest_dimension":"correctness","weakest_dimension":"latency","summary":"Strong overall."}
				],
				"recommended_next_step":"Ignore this output.",
				"confidence_notes":"Confidence is low."
			}`,
		},
	}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), fixture.repo).WithInsightsClient(client)

	_, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(fixture.workspaceID), fixture.runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: fixture.providerAccountID,
		ModelAliasID:      fixture.modelAliasID,
	})

	var validationErr RunRankingInsightsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "invalid_insights_output" {
		t.Fatalf("validation code = %q, want invalid_insights_output", validationErr.Code)
	}
}

func TestRunReadManagerGenerateRankingInsightsRejectsWinnerDivergingFromDeterministicRanking(t *testing.T) {
	fixture := newRankingInsightsFixture(t)
	client := &provider.FakeClient{
		Response: provider.Response{
			ProviderKey:     "openai",
			ProviderModelID: "gpt-5.4-mini",
			OutputText: `{
				"recommended_winner":{"run_agent_id":"` + fixture.betaID.String() + `","label":"Beta"},
				"why_it_won":"Injected override.",
				"tradeoffs":["Beta is faster."],
				"model_summaries":[
					{"run_agent_id":"` + fixture.alphaID.String() + `","label":"Alpha","strongest_dimension":"correctness","weakest_dimension":"latency","summary":"Strong overall."},
					{"run_agent_id":"` + fixture.betaID.String() + `","label":"Beta","strongest_dimension":"latency","weakest_dimension":"cost","summary":"Fast but not the winner."}
				],
				"recommended_next_step":"Run another benchmark.",
				"confidence_notes":"Confidence is low."
			}`,
		},
	}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), fixture.repo).WithInsightsClient(client)

	_, err := manager.GenerateRunRankingInsights(context.Background(), newRunInsightsCaller(fixture.workspaceID), fixture.runID, GenerateRunRankingInsightsInput{
		ProviderAccountID: fixture.providerAccountID,
		ModelAliasID:      fixture.modelAliasID,
	})

	var validationErr RunRankingInsightsValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "invalid_insights_output" {
		t.Fatalf("validation code = %q, want invalid_insights_output", validationErr.Code)
	}
	if !strings.Contains(validationErr.Message, "deterministic ranking winner") {
		t.Fatalf("validation message = %q, want deterministic winner reference", validationErr.Message)
	}
}

func TestCreateRunRankingInsightsEndpointReturnsInsights(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	providerAccountID := uuid.New()
	modelAliasID := uuid.New()
	winnerID := uuid.New()

	body, err := json.Marshal(createRunRankingInsightsRequest{
		ProviderAccountID: providerAccountID.String(),
		ModelAliasID:      modelAliasID.String(),
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+runID.String()+"/ranking-insights", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		&fakeRunReadService{
			insightsResult: GenerateRunRankingInsightsResult{
				Run: domain.Run{ID: runID, WorkspaceID: workspaceID},
				Insights: runRankingInsightsResponse{
					GeneratedAt:         time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC),
					GroundingScope:      "current_run_only",
					ProviderKey:         "openai",
					ProviderModelID:     "gpt-5.4-mini",
					RecommendedWinner:   runRankingInsightCandidate{RunAgentID: winnerID, Label: "Alpha"},
					WhyItWon:            "Alpha delivered the best overall mix for this run.",
					Tradeoffs:           []string{"Beta stayed close on latency."},
					ModelSummaries:      []runRankingModelInsight{{RunAgentID: winnerID, Label: "Alpha", StrongestDimension: "correctness", WeakestDimension: "latency", Summary: "Strong overall."}},
					RecommendedNextStep: "Run a reliability-focused follow-up.",
					ConfidenceNotes:     "Confidence is moderate.",
				},
			},
		},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response runRankingInsightsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.RecommendedWinner.RunAgentID != winnerID {
		t.Fatalf("winner = %s, want %s", response.RecommendedWinner.RunAgentID, winnerID)
	}
}

func TestCreateRunRankingInsightsEndpointMapsProviderAuthFailureTo400(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	providerAccountID := uuid.New()
	modelAliasID := uuid.New()

	body, err := json.Marshal(createRunRankingInsightsRequest{
		ProviderAccountID: providerAccountID.String(),
		ModelAliasID:      modelAliasID.String(),
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+runID.String()+"/ranking-insights", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		&fakeRunReadService{
			insightsErr: provider.Failure{
				Code:    provider.FailureCodeAuth,
				Message: "raw upstream auth body should not leak",
			},
		},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), "invalid_provider_credentials") {
		t.Fatalf("body = %s, want invalid_provider_credentials", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "raw upstream auth body should not leak") {
		t.Fatalf("body leaked raw provider message: %s", recorder.Body.String())
	}
}

func TestCreateRunRankingInsightsEndpointMapsProviderRateLimitTo429(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	providerAccountID := uuid.New()
	modelAliasID := uuid.New()

	body, err := json.Marshal(createRunRankingInsightsRequest{
		ProviderAccountID: providerAccountID.String(),
		ModelAliasID:      modelAliasID.String(),
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+runID.String()+"/ranking-insights", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		&fakeRunReadService{
			insightsErr: provider.Failure{
				Code:       provider.FailureCodeRateLimit,
				RetryAfter: 5 * time.Second,
			},
		},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if recorder.Header().Get("Retry-After") != "5" {
		t.Fatalf("Retry-After = %q, want 5", recorder.Header().Get("Retry-After"))
	}
}

func TestCreateRunRankingInsightsEndpointMapsManagerRateLimitTo429(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	providerAccountID := uuid.New()
	modelAliasID := uuid.New()

	body, err := json.Marshal(createRunRankingInsightsRequest{
		ProviderAccountID: providerAccountID.String(),
		ModelAliasID:      modelAliasID.String(),
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+runID.String()+"/ranking-insights", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		&fakeRunReadService{
			insightsErr: RunRankingInsightsRateLimitError{RetryAfter: 7 * time.Second},
		},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if recorder.Header().Get("Retry-After") != "7" {
		t.Fatalf("Retry-After = %q, want 7", recorder.Header().Get("Retry-After"))
	}
}

type workspaceSecretAssertingClient struct {
	t                     *testing.T
	expectedCredentialRef string
	expectedSecret        string
	response              provider.Response
	called                bool
}

func (c *workspaceSecretAssertingClient) InvokeModel(ctx context.Context, request provider.Request) (provider.Response, error) {
	c.called = true
	if request.CredentialReference != c.expectedCredentialRef {
		c.t.Fatalf("credential reference = %q, want %q", request.CredentialReference, c.expectedCredentialRef)
	}
	secret, err := provider.EnvCredentialResolver{}.Resolve(ctx, request.CredentialReference)
	if err != nil {
		c.t.Fatalf("resolve credential: %v", err)
	}
	if secret != c.expectedSecret {
		c.t.Fatalf("secret = %q, want %q", secret, c.expectedSecret)
	}
	return c.response, nil
}

func buildInsightsRun(runID uuid.UUID, workspaceID uuid.UUID) domain.Run {
	return domain.Run{
		ID:            runID,
		WorkspaceID:   workspaceID,
		Name:          "Ranking Insights Run",
		Status:        domain.RunStatusCompleted,
		ExecutionMode: "comparison",
	}
}

func buildInsightsScorecard(t *testing.T, runID uuid.UUID, alphaID uuid.UUID, betaID uuid.UUID) repository.RunScorecard {
	t.Helper()

	scorecardDocument, err := json.Marshal(runScorecardRankingDocument{
		RunID:             runID,
		EvaluationSpecID:  uuid.New(),
		WinningRunAgentID: &alphaID,
		WinnerDetermination: runRankingWinnerSummary{
			Strategy:   "weighted_score",
			Status:     "winner",
			ReasonCode: "highest_composite",
		},
		Agents: []runRankingAgentDocument{
			{
				RunAgentID:       alphaID,
				LaneIndex:        0,
				Label:            "Alpha",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				OverallScore:     float64PtrRunRankingTest(0.91),
				CorrectnessScore: float64PtrRunRankingTest(0.93),
				ReliabilityScore: float64PtrRunRankingTest(0.85),
				LatencyScore:     float64PtrRunRankingTest(0.62),
				CostScore:        float64PtrRunRankingTest(0.70),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(0.93)},
					"reliability": {State: "available", Score: float64PtrRunRankingTest(0.85)},
					"latency":     {State: "available", Score: float64PtrRunRankingTest(0.62)},
					"cost":        {State: "available", Score: float64PtrRunRankingTest(0.70)},
				},
			},
			{
				RunAgentID:       betaID,
				LaneIndex:        1,
				Label:            "Beta",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				OverallScore:     float64PtrRunRankingTest(0.84),
				CorrectnessScore: float64PtrRunRankingTest(0.82),
				ReliabilityScore: float64PtrRunRankingTest(0.88),
				LatencyScore:     float64PtrRunRankingTest(0.76),
				CostScore:        float64PtrRunRankingTest(0.59),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(0.82)},
					"reliability": {State: "available", Score: float64PtrRunRankingTest(0.88)},
					"latency":     {State: "available", Score: float64PtrRunRankingTest(0.76)},
					"cost":        {State: "available", Score: float64PtrRunRankingTest(0.59)},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal scorecard document: %v", err)
	}

	return repository.RunScorecard{
		ID:               uuid.New(),
		RunID:            runID,
		EvaluationSpecID: uuid.New(),
		Scorecard:        scorecardDocument,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
}

func newRunInsightsCaller(workspaceID uuid.UUID) Caller {
	return Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}
}

type rankingInsightsFixture struct {
	workspaceID         uuid.UUID
	runID               uuid.UUID
	providerAccountID   uuid.UUID
	modelCatalogEntryID uuid.UUID
	modelAliasID        uuid.UUID
	alphaID             uuid.UUID
	betaID              uuid.UUID
	repo                *fakeRunReadRepository
}

func newRankingInsightsFixture(t *testing.T) rankingInsightsFixture {
	t.Helper()

	workspaceID := uuid.New()
	runID := uuid.New()
	providerAccountID := uuid.New()
	modelCatalogEntryID := uuid.New()
	modelAliasID := uuid.New()
	alphaID := uuid.New()
	betaID := uuid.New()

	return rankingInsightsFixture{
		workspaceID:         workspaceID,
		runID:               runID,
		providerAccountID:   providerAccountID,
		modelCatalogEntryID: modelCatalogEntryID,
		modelAliasID:        modelAliasID,
		alphaID:             alphaID,
		betaID:              betaID,
		repo: &fakeRunReadRepository{
			run: buildInsightsRun(runID, workspaceID),
			runAgents: []domain.RunAgent{
				{ID: alphaID, RunID: runID, LaneIndex: 0, Label: "Alpha", Status: domain.RunAgentStatusCompleted},
				{ID: betaID, RunID: runID, LaneIndex: 1, Label: "Beta", Status: domain.RunAgentStatusCompleted},
			},
			runScorecard: buildInsightsScorecard(t, runID, alphaID, betaID),
			providerAccount: repository.ProviderAccountRow{
				ID:                  providerAccountID,
				WorkspaceID:         uuidPtr(workspaceID),
				ProviderKey:         "openai",
				CredentialReference: "env://OPENAI_API_KEY",
				Status:              "active",
			},
			modelAlias: repository.ModelAliasRow{
				ID:                  modelAliasID,
				WorkspaceID:         uuidPtr(workspaceID),
				ProviderAccountID:   uuidPtr(providerAccountID),
				ModelCatalogEntryID: modelCatalogEntryID,
				AliasKey:            "insights-default",
				DisplayName:         "Insights Default",
				Status:              "active",
			},
			modelCatalogEntry: repository.ModelCatalogEntryRow{
				ID:              modelCatalogEntryID,
				ProviderKey:     "openai",
				ProviderModelID: "gpt-5.4-mini",
				DisplayName:     "GPT-5.4 Mini",
			},
		},
	}
}

type fakeBudgetChecker struct {
	results map[uuid.UUID]budget.BudgetCheckResult
	err     error
}

func (f fakeBudgetChecker) CheckPreRunBudget(_ context.Context, _ uuid.UUID, spendPolicyID uuid.UUID) (budget.BudgetCheckResult, error) {
	if f.err != nil {
		return budget.BudgetCheckResult{}, f.err
	}
	if result, ok := f.results[spendPolicyID]; ok {
		return result, nil
	}
	return budget.BudgetCheckResult{Allowed: true}, nil
}

type fakeWorkspaceRateLimiter struct {
	allowed    bool
	retryAfter time.Duration
}

func (f fakeWorkspaceRateLimiter) Allow(_ uuid.UUID, _ string) (bool, time.Duration) {
	return f.allowed, f.retryAfter
}
