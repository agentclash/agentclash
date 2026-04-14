package api

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"log/slog"
)

func TestRunReadManagerGetRunRankingReturnsSortedRanking(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	winningRunAgentID := uuid.New()
	secondRunAgentID := uuid.New()

	scorecardDocument, err := json.Marshal(runScorecardRankingDocument{
		RunID:             runID,
		EvaluationSpecID:  uuid.New(),
		WinningRunAgentID: &winningRunAgentID,
		WinnerDetermination: runRankingWinnerSummary{
			Strategy:   "correctness_then_reliability",
			Status:     "winner",
			ReasonCode: "best_correctness",
		},
		Agents: []runRankingAgentDocument{
			{
				RunAgentID:       secondRunAgentID,
				LaneIndex:        1,
				Label:            "Beta",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				CorrectnessScore: float64PtrRunRankingTest(0.81),
				ReliabilityScore: float64PtrRunRankingTest(0.85),
				LatencyScore:     float64PtrRunRankingTest(0.60),
				CostScore:        float64PtrRunRankingTest(0.55),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(0.81)},
					"reliability": {State: "available", Score: float64PtrRunRankingTest(0.85)},
					"latency":     {State: "available", Score: float64PtrRunRankingTest(0.60)},
					"cost":        {State: "available", Score: float64PtrRunRankingTest(0.55)},
				},
			},
			{
				RunAgentID:       winningRunAgentID,
				LaneIndex:        0,
				Label:            "Alpha",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				CorrectnessScore: float64PtrRunRankingTest(0.92),
				ReliabilityScore: float64PtrRunRankingTest(0.80),
				LatencyScore:     float64PtrRunRankingTest(0.50),
				CostScore:        float64PtrRunRankingTest(0.45),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(0.92)},
					"reliability": {State: "available", Score: float64PtrRunRankingTest(0.80)},
					"latency":     {State: "available", Score: float64PtrRunRankingTest(0.50)},
					"cost":        {State: "available", Score: float64PtrRunRankingTest(0.45)},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal scorecard document: %v", err)
	}

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			Status:      domain.RunStatusCompleted,
		},
		runScorecard: repository.RunScorecard{
			ID:               uuid.New(),
			RunID:            runID,
			EvaluationSpecID: uuid.New(),
			Scorecard:        scorecardDocument,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		},
	})

	result, err := manager.GetRunRanking(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, runID, GetRunRankingInput{})
	if err != nil {
		t.Fatalf("GetRunRanking returned error: %v", err)
	}
	if result.State != RankingReadStateReady {
		t.Fatalf("state = %s, want ready", result.State)
	}
	if result.Ranking == nil {
		t.Fatalf("ranking = nil, want payload")
	}
	if len(result.Ranking.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Ranking.Items))
	}
	if result.Ranking.Items[0].RunAgentID != winningRunAgentID {
		t.Fatalf("first run_agent_id = %s, want %s", result.Ranking.Items[0].RunAgentID, winningRunAgentID)
	}
	if result.Ranking.Items[0].Rank == nil || *result.Ranking.Items[0].Rank != 1 {
		t.Fatalf("first rank = %v, want 1", result.Ranking.Items[0].Rank)
	}
	if result.Ranking.Items[0].CompositeScore == nil {
		t.Fatalf("first composite score = nil, want value")
	}
}

func TestRunReadManagerGetRunRankingReturnsPendingWithoutRunScorecard(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			Status:      domain.RunStatusRunning,
		},
		getRunScorecardErr: repository.ErrRunScorecardNotFound,
	})

	result, err := manager.GetRunRanking(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, runID, GetRunRankingInput{})
	if err != nil {
		t.Fatalf("GetRunRanking returned error: %v", err)
	}
	if result.State != RankingReadStatePending {
		t.Fatalf("state = %s, want pending", result.State)
	}
	if result.Ranking != nil {
		t.Fatalf("ranking = %v, want nil", result.Ranking)
	}
}

func TestRunReadManagerGetRunRankingSortsPartialAgentsLast(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	availableAgentID := uuid.New()
	partialAgentID := uuid.New()

	scorecardDocument, err := json.Marshal(runScorecardRankingDocument{
		RunID:            runID,
		EvaluationSpecID: uuid.New(),
		Agents: []runRankingAgentDocument{
			{
				RunAgentID:       partialAgentID,
				LaneIndex:        0,
				Label:            "Partial",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "partial",
				CorrectnessScore: float64PtrRunRankingTest(0.55),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(0.55)},
					"reliability": {State: "unavailable"},
				},
			},
			{
				RunAgentID:       availableAgentID,
				LaneIndex:        1,
				Label:            "Available",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				ReliabilityScore: float64PtrRunRankingTest(0.77),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"reliability": {State: "available", Score: float64PtrRunRankingTest(0.77)},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal scorecard document: %v", err)
	}

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			Status:      domain.RunStatusCompleted,
		},
		runScorecard: repository.RunScorecard{
			ID:               uuid.New(),
			RunID:            runID,
			EvaluationSpecID: uuid.New(),
			Scorecard:        scorecardDocument,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		},
	})

	result, err := manager.GetRunRanking(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, runID, GetRunRankingInput{SortBy: RunRankingSortFieldReliability})
	if err != nil {
		t.Fatalf("GetRunRanking returned error: %v", err)
	}
	if result.Ranking == nil {
		t.Fatalf("ranking = nil, want payload")
	}
	if result.Ranking.Items[0].RunAgentID != availableAgentID {
		t.Fatalf("first run_agent_id = %s, want %s", result.Ranking.Items[0].RunAgentID, availableAgentID)
	}
	if result.Ranking.Items[0].Rank == nil || *result.Ranking.Items[0].Rank != 1 {
		t.Fatalf("available rank = %v, want 1", result.Ranking.Items[0].Rank)
	}
	if result.Ranking.Items[1].RunAgentID != partialAgentID {
		t.Fatalf("second run_agent_id = %s, want %s", result.Ranking.Items[1].RunAgentID, partialAgentID)
	}
	if result.Ranking.Items[1].Rank != nil {
		t.Fatalf("partial rank = %v, want nil", result.Ranking.Items[1].Rank)
	}
	if result.Ranking.Items[1].CompositeScore == nil {
		t.Fatalf("partial composite score = nil, want fallback from available dimensions")
	}
}

func TestRunReadManagerGetRunRankingSortsByCompositeAndComputesDelta(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	firstAgentID := uuid.New()
	secondAgentID := uuid.New()

	scorecardDocument, err := json.Marshal(runScorecardRankingDocument{
		RunID:            runID,
		EvaluationSpecID: uuid.New(),
		Agents: []runRankingAgentDocument{
			{
				RunAgentID:       secondAgentID,
				LaneIndex:        1,
				Label:            "Second",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				CorrectnessScore: float64PtrRunRankingTest(0.80),
				ReliabilityScore: float64PtrRunRankingTest(0.70),
				LatencyScore:     float64PtrRunRankingTest(0.60),
				CostScore:        float64PtrRunRankingTest(0.50),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(0.80)},
					"reliability": {State: "available", Score: float64PtrRunRankingTest(0.70)},
					"latency":     {State: "available", Score: float64PtrRunRankingTest(0.60)},
					"cost":        {State: "available", Score: float64PtrRunRankingTest(0.50)},
				},
			},
			{
				RunAgentID:       firstAgentID,
				LaneIndex:        0,
				Label:            "First",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				CorrectnessScore: float64PtrRunRankingTest(1.0),
				ReliabilityScore: float64PtrRunRankingTest(1.0),
				LatencyScore:     float64PtrRunRankingTest(0.8),
				CostScore:        float64PtrRunRankingTest(1.0),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(1.0)},
					"reliability": {State: "available", Score: float64PtrRunRankingTest(1.0)},
					"latency":     {State: "available", Score: float64PtrRunRankingTest(0.8)},
					"cost":        {State: "available", Score: float64PtrRunRankingTest(1.0)},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal scorecard document: %v", err)
	}

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			Status:      domain.RunStatusCompleted,
		},
		runScorecard: repository.RunScorecard{
			ID:               uuid.New(),
			RunID:            runID,
			EvaluationSpecID: uuid.New(),
			Scorecard:        scorecardDocument,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		},
	})

	result, err := manager.GetRunRanking(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, runID, GetRunRankingInput{SortBy: RunRankingSortFieldComposite})
	if err != nil {
		t.Fatalf("GetRunRanking returned error: %v", err)
	}
	if result.Ranking == nil {
		t.Fatalf("ranking = nil, want payload")
	}
	if result.Ranking.Sort.Field != "composite" {
		t.Fatalf("sort field = %q, want composite", result.Ranking.Sort.Field)
	}
	if result.Ranking.Items[0].RunAgentID != firstAgentID {
		t.Fatalf("first run_agent_id = %s, want %s", result.Ranking.Items[0].RunAgentID, firstAgentID)
	}
	if result.Ranking.Items[0].CompositeScore == nil || math.Abs(*result.Ranking.Items[0].CompositeScore-0.95) > 1e-9 {
		t.Fatalf("first composite score = %v, want 0.95", result.Ranking.Items[0].CompositeScore)
	}
	if result.Ranking.Items[0].DeltaFromTop == nil || math.Abs(*result.Ranking.Items[0].DeltaFromTop) > 1e-9 {
		t.Fatalf("first delta_from_top = %v, want 0", result.Ranking.Items[0].DeltaFromTop)
	}
	if result.Ranking.Items[1].CompositeScore == nil || math.Abs(*result.Ranking.Items[1].CompositeScore-0.65) > 1e-9 {
		t.Fatalf("second composite score = %v, want 0.65", result.Ranking.Items[1].CompositeScore)
	}
	if result.Ranking.Items[1].DeltaFromTop == nil || math.Abs(*result.Ranking.Items[1].DeltaFromTop-0.30) > 1e-9 {
		t.Fatalf("second delta_from_top = %v, want 0.30", result.Ranking.Items[1].DeltaFromTop)
	}
}

func TestRunReadManagerGetRunRankingAssignsDenseRanksForTies(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	firstAgentID := uuid.New()
	secondAgentID := uuid.New()
	thirdAgentID := uuid.New()

	scorecardDocument, err := json.Marshal(runScorecardRankingDocument{
		RunID:            runID,
		EvaluationSpecID: uuid.New(),
		Agents: []runRankingAgentDocument{
			{
				RunAgentID:       thirdAgentID,
				LaneIndex:        2,
				Label:            "Third",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				CorrectnessScore: float64PtrRunRankingTest(0.7),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(0.7)},
				},
			},
			{
				RunAgentID:       secondAgentID,
				LaneIndex:        1,
				Label:            "Second",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				CorrectnessScore: float64PtrRunRankingTest(1.0),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(1.0)},
				},
			},
			{
				RunAgentID:       firstAgentID,
				LaneIndex:        0,
				Label:            "First",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				CorrectnessScore: float64PtrRunRankingTest(1.0),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(1.0)},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal scorecard document: %v", err)
	}

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			Status:      domain.RunStatusCompleted,
		},
		runScorecard: repository.RunScorecard{
			ID:               uuid.New(),
			RunID:            runID,
			EvaluationSpecID: uuid.New(),
			Scorecard:        scorecardDocument,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		},
	})

	result, err := manager.GetRunRanking(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, runID, GetRunRankingInput{})
	if err != nil {
		t.Fatalf("GetRunRanking returned error: %v", err)
	}
	if result.Ranking == nil {
		t.Fatalf("ranking = nil, want payload")
	}
	if result.Ranking.Items[0].Rank == nil || *result.Ranking.Items[0].Rank != 1 {
		t.Fatalf("first rank = %v, want 1", result.Ranking.Items[0].Rank)
	}
	if result.Ranking.Items[1].Rank == nil || *result.Ranking.Items[1].Rank != 1 {
		t.Fatalf("second rank = %v, want 1", result.Ranking.Items[1].Rank)
	}
	if result.Ranking.Items[2].Rank == nil || *result.Ranking.Items[2].Rank != 2 {
		t.Fatalf("third rank = %v, want 2", result.Ranking.Items[2].Rank)
	}
	if result.Ranking.Items[1].DeltaFromTop == nil || math.Abs(*result.Ranking.Items[1].DeltaFromTop) > 1e-9 {
		t.Fatalf("second delta_from_top = %v, want 0", result.Ranking.Items[1].DeltaFromTop)
	}
}

func TestGetRunRankingEndpointReturnsSortedPayload(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	firstID := uuid.New()
	secondID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String()+"/ranking?sort_by=reliability", nil)
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
			getRunRankingResult: GetRunRankingResult{
				State: RankingReadStateReady,
				Ranking: &runRankingPayload{
					RunID:            runID,
					EvaluationSpecID: uuid.New(),
					Sort: runRankingSortResponse{
						Field:        "reliability",
						Direction:    "desc",
						DefaultOrder: false,
					},
					Items: []runRankingItemResponse{
						{RunAgentID: firstID, Rank: intPtrRunRankingTest(1), SortState: "available"},
						{RunAgentID: secondID, Rank: intPtrRunRankingTest(2), SortState: "available"},
					},
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

	var response getRunRankingResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Ranking == nil {
		t.Fatalf("ranking = nil, want payload")
	}
	if response.Ranking.Sort.Field != "reliability" {
		t.Fatalf("sort field = %q, want reliability", response.Ranking.Sort.Field)
	}
	if response.Ranking.Items[0].RunAgentID != firstID {
		t.Fatalf("first run_agent_id = %s, want %s", response.Ranking.Items[0].RunAgentID, firstID)
	}
}

func TestGetRunRankingEndpointRejectsInvalidSortField(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String()+"/ranking?sort_by=overall", nil)
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
		&fakeRunReadService{getRunRankingErr: ErrInvalidRunRankingSort},
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
}

func TestGetRunRankingEndpointReturnsCompositeSortPayload(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	runAgentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String()+"/ranking?sort_by=composite", nil)
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
			getRunRankingResult: GetRunRankingResult{
				State: RankingReadStateReady,
				Ranking: &runRankingPayload{
					RunID:            runID,
					EvaluationSpecID: uuid.New(),
					Sort: runRankingSortResponse{
						Field:        "composite",
						Direction:    "desc",
						DefaultOrder: false,
					},
					Items: []runRankingItemResponse{
						{RunAgentID: runAgentID, Rank: intPtrRunRankingTest(1), SortState: "available", CompositeScore: float64PtrRunRankingTest(0.91), DeltaFromTop: float64PtrRunRankingTest(0)},
					},
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

	var response getRunRankingResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Ranking == nil {
		t.Fatalf("ranking = nil, want payload")
	}
	if response.Ranking.Sort.Field != "composite" {
		t.Fatalf("sort field = %q, want composite", response.Ranking.Sort.Field)
	}
	if response.Ranking.Items[0].CompositeScore == nil || math.Abs(*response.Ranking.Items[0].CompositeScore-0.91) > 1e-9 {
		t.Fatalf("composite score = %v, want 0.91", response.Ranking.Items[0].CompositeScore)
	}
}

func TestGetRunRankingEndpointReturnsAcceptedWhenPending(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String()+"/ranking", nil)
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
			getRunRankingResult: GetRunRankingResult{
				State:   RankingReadStatePending,
				Message: "ranking is not ready yet",
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

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
}

func TestGetRunRankingEndpointReturnsConflictWhenErrored(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String()+"/ranking", nil)
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
			getRunRankingResult: GetRunRankingResult{
				State:   RankingReadStateErrored,
				Message: "run is completed but the ranking is unavailable",
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

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func float64PtrRunRankingTest(value float64) *float64 {
	return &value
}

func intPtrRunRankingTest(value int) *int {
	return &value
}

func boolPtrRunRankingTest(value bool) *bool {
	return &value
}

// Phase 3: custom sort key lookup. A user-declared dimension "safety" must be
// accepted as sort_by and drive ordering via the JSONB score.
func TestRunReadManagerGetRunRankingAcceptsCustomDimensionSortKey(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	winnerID := uuid.New()
	loserID := uuid.New()

	scorecardDocument, err := json.Marshal(runScorecardRankingDocument{
		RunID:            runID,
		EvaluationSpecID: uuid.New(),
		Agents: []runRankingAgentDocument{
			{
				RunAgentID:       loserID,
				LaneIndex:        0,
				Label:            "Loser",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				Dimensions: map[string]runRankingDimensionScorePayload{
					"safety": {State: "available", Score: float64PtrRunRankingTest(0.40), BetterDirection: "higher"},
				},
			},
			{
				RunAgentID:       winnerID,
				LaneIndex:        1,
				Label:            "Winner",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				Dimensions: map[string]runRankingDimensionScorePayload{
					"safety": {State: "available", Score: float64PtrRunRankingTest(0.90), BetterDirection: "higher"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal scorecard document: %v", err)
	}

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			Status:      domain.RunStatusCompleted,
		},
		runScorecard: repository.RunScorecard{
			ID:               uuid.New(),
			RunID:            runID,
			EvaluationSpecID: uuid.New(),
			Scorecard:        scorecardDocument,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		},
	})

	result, err := manager.GetRunRanking(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, runID, GetRunRankingInput{SortBy: RunRankingSortField("safety")})
	if err != nil {
		t.Fatalf("GetRunRanking returned error: %v", err)
	}
	if result.Ranking == nil {
		t.Fatalf("ranking = nil, want payload")
	}
	if result.Ranking.Sort.Field != "safety" {
		t.Fatalf("sort field = %q, want safety", result.Ranking.Sort.Field)
	}
	if result.Ranking.Items[0].RunAgentID != winnerID {
		t.Fatalf("first run_agent_id = %s, want %s", result.Ranking.Items[0].RunAgentID, winnerID)
	}
	if result.Ranking.Items[0].SortValue == nil || math.Abs(*result.Ranking.Items[0].SortValue-0.90) > 1e-9 {
		t.Fatalf("first sort_value = %v, want 0.90", result.Ranking.Items[0].SortValue)
	}
	if result.Ranking.Items[0].Dimensions["safety"].BetterDirection != "higher" {
		t.Fatalf("better_direction = %q, want higher", result.Ranking.Items[0].Dimensions["safety"].BetterDirection)
	}
}

// Phase 3: unknown sort key rejection. A sort_by that neither matches a legacy
// built-in nor appears in any agent's Dimensions must surface as a 400 via
// ErrInvalidRunRankingSort.
func TestRunReadManagerGetRunRankingRejectsUnknownCustomSortKey(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()

	scorecardDocument, err := json.Marshal(runScorecardRankingDocument{
		RunID:            runID,
		EvaluationSpecID: uuid.New(),
		Agents: []runRankingAgentDocument{
			{
				RunAgentID:       uuid.New(),
				LaneIndex:        0,
				Label:            "Only",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(0.5)},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal scorecard document: %v", err)
	}

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			Status:      domain.RunStatusCompleted,
		},
		runScorecard: repository.RunScorecard{
			ID:               uuid.New(),
			RunID:            runID,
			EvaluationSpecID: uuid.New(),
			Scorecard:        scorecardDocument,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		},
	})

	_, err = manager.GetRunRanking(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, runID, GetRunRankingInput{SortBy: RunRankingSortField("mystery_dim")})
	if err == nil {
		t.Fatalf("GetRunRanking error = nil, want ErrInvalidRunRankingSort")
	}
	if !errors.Is(err, ErrInvalidRunRankingSort) {
		t.Fatalf("GetRunRanking error = %v, want ErrInvalidRunRankingSort", err)
	}
}

// Phase 3: Strategy, Passed, and OverallReason on agent documents must surface
// into the ranking item response alongside legacy score columns.
func TestRunReadManagerGetRunRankingSurfacesStrategyPassedAndReason(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	runAgentID := uuid.New()

	scorecardDocument, err := json.Marshal(runScorecardRankingDocument{
		RunID:            runID,
		EvaluationSpecID: uuid.New(),
		Agents: []runRankingAgentDocument{
			{
				RunAgentID:       runAgentID,
				LaneIndex:        0,
				Label:            "Solo",
				Status:           domain.RunAgentStatusCompleted,
				HasScorecard:     true,
				EvaluationStatus: "complete",
				Strategy:         "binary",
				OverallScore:     float64PtrRunRankingTest(1.0),
				Passed:           boolPtrRunRankingTest(true),
				OverallReason:    "all validators passed",
				CorrectnessScore: float64PtrRunRankingTest(0.9),
				Dimensions: map[string]runRankingDimensionScorePayload{
					"correctness": {State: "available", Score: float64PtrRunRankingTest(0.9)},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal scorecard document: %v", err)
	}

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			Status:      domain.RunStatusCompleted,
		},
		runScorecard: repository.RunScorecard{
			ID:               uuid.New(),
			RunID:            runID,
			EvaluationSpecID: uuid.New(),
			Scorecard:        scorecardDocument,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		},
	})

	result, err := manager.GetRunRanking(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, runID, GetRunRankingInput{})
	if err != nil {
		t.Fatalf("GetRunRanking returned error: %v", err)
	}
	if result.Ranking == nil || len(result.Ranking.Items) != 1 {
		t.Fatalf("ranking items = %v, want 1", result.Ranking)
	}
	item := result.Ranking.Items[0]
	if item.Strategy != "binary" {
		t.Fatalf("strategy = %q, want binary", item.Strategy)
	}
	if item.Passed == nil || !*item.Passed {
		t.Fatalf("passed = %v, want true", item.Passed)
	}
	if item.OverallReason != "all validators passed" {
		t.Fatalf("overall_reason = %q, want all validators passed", item.OverallReason)
	}
	if item.OverallScore == nil || math.Abs(*item.OverallScore-1.0) > 1e-9 {
		t.Fatalf("overall_score = %v, want 1.0", item.OverallScore)
	}
}
