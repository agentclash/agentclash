package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestReplayReadManagerReturnsPaginatedReplayForAuthorizedCaller(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	manager := NewReplayReadManager(NewCallerWorkspaceAuthorizer(), &fakeReplayReadRepository{
		runAgent: domain.RunAgent{
			ID:          runAgentID,
			RunID:       uuid.New(),
			WorkspaceID: workspaceID,
			Status:      domain.RunAgentStatusCompleted,
		},
		replay: repository.RunAgentReplay{
			ID:         uuid.New(),
			RunAgentID: runAgentID,
			Summary: []byte(`{
				"headline":"trace ready",
				"status":"completed",
				"steps":[
					{"type":"run","headline":"Run started"},
					{"type":"model_call","headline":"Model call"},
					{"type":"tool_call","headline":"Tool call"}
				]
			}`),
		},
	})

	result, err := manager.GetRunAgentReplay(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, runAgentID, ReplayStepPageParams{Cursor: 1, Limit: 1})
	if err != nil {
		t.Fatalf("GetRunAgentReplay returned error: %v", err)
	}
	if result.State != ReplayStateReady {
		t.Fatalf("state = %q, want %q", result.State, ReplayStateReady)
	}
	if result.Replay == nil || result.Replay.RunAgentID != runAgentID {
		t.Fatalf("replay run_agent_id = %v, want %s", result.Replay, runAgentID)
	}
	if result.StepPage.TotalSteps != 3 {
		t.Fatalf("total_steps = %d, want 3", result.StepPage.TotalSteps)
	}
	if len(result.StepPage.Steps) != 1 {
		t.Fatalf("step page size = %d, want 1", len(result.StepPage.Steps))
	}
	if result.StepPage.NextCursor == nil || *result.StepPage.NextCursor != "2" {
		t.Fatalf("next_cursor = %v, want 2", result.StepPage.NextCursor)
	}

	summary := decodeReplayPayload(t, result.Summary)
	if _, ok := summary["steps"]; ok {
		t.Fatalf("paginated summary should not include inline steps")
	}
}

func TestReplayReadManagerReturnsPendingWhenReplayGenerationHasNotMaterialized(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewReplayReadManager(NewCallerWorkspaceAuthorizer(), &fakeReplayReadRepository{
		runAgent: domain.RunAgent{
			ID:          uuid.New(),
			RunID:       uuid.New(),
			WorkspaceID: workspaceID,
			Status:      domain.RunAgentStatusExecuting,
		},
		replayErr: repository.ErrRunAgentReplayNotFound,
	})

	result, err := manager.GetRunAgentReplay(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, uuid.New(), ReplayStepPageParams{})
	if err != nil {
		t.Fatalf("GetRunAgentReplay returned error: %v", err)
	}
	if result.State != ReplayStatePending {
		t.Fatalf("state = %q, want %q", result.State, ReplayStatePending)
	}
	if result.Message == "" {
		t.Fatalf("expected pending message")
	}
}

func TestReplayReadManagerReturnsErroredWhenReplayIsMissingAfterTerminalState(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewReplayReadManager(NewCallerWorkspaceAuthorizer(), &fakeReplayReadRepository{
		runAgent: domain.RunAgent{
			ID:          uuid.New(),
			RunID:       uuid.New(),
			WorkspaceID: workspaceID,
			Status:      domain.RunAgentStatusCompleted,
		},
		replayErr: repository.ErrRunAgentReplayNotFound,
	})

	result, err := manager.GetRunAgentReplay(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, uuid.New(), ReplayStepPageParams{})
	if err != nil {
		t.Fatalf("GetRunAgentReplay returned error: %v", err)
	}
	if result.State != ReplayStateErrored {
		t.Fatalf("state = %q, want %q", result.State, ReplayStateErrored)
	}
	if result.Message == "" {
		t.Fatalf("expected errored message")
	}
}

func TestReplayReadManagerRejectsNegativeCursorOutsideHTTPHandler(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	manager := NewReplayReadManager(NewCallerWorkspaceAuthorizer(), &fakeReplayReadRepository{
		runAgent: domain.RunAgent{
			ID:          runAgentID,
			RunID:       uuid.New(),
			WorkspaceID: workspaceID,
			Status:      domain.RunAgentStatusCompleted,
		},
		replay: repository.RunAgentReplay{
			ID:         uuid.New(),
			RunAgentID: runAgentID,
			Summary:    []byte(`{"headline":"trace ready","steps":[{"type":"run"}]}`),
		},
	})

	_, err := manager.GetRunAgentReplay(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, runAgentID, ReplayStepPageParams{Cursor: -1, Limit: 1})
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "paginate run-agent replay summary: cursor must be a non-negative integer" {
		t.Fatalf("error = %v, want negative cursor validation", err)
	}
}

func TestGetRunAgentReplayEndpointReturnsPaginatedReplay(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/replays/"+runAgentID.String()+"?limit=1&cursor=1", nil)
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
		stubRunReadService{},
		&fakeReplayReadService{
			replayResult: GetRunAgentReplayResult{
				RunAgent: domain.RunAgent{
					ID:          runAgentID,
					RunID:       uuid.New(),
					WorkspaceID: workspaceID,
					Status:      domain.RunAgentStatusCompleted,
				},
				State:   ReplayStateReady,
				Summary: []byte(`{"headline":"trace ready","status":"completed"}`),
				Replay: &repository.RunAgentReplay{
					ID:                   uuid.New(),
					RunAgentID:           runAgentID,
					LatestSequenceNumber: int64Ptr(42),
					EventCount:           42,
					CreatedAt:            time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
					UpdatedAt:            time.Date(2026, 3, 13, 12, 1, 0, 0, time.UTC),
				},
				StepPage: ReplayStepPage{
					Steps:      []json.RawMessage{json.RawMessage(`{"type":"model_call","headline":"Model call"}`)},
					NextCursor: replayStringPtr("2"),
					Limit:      1,
					TotalSteps: 3,
					HasMore:    true,
				},
			},
		},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response getRunAgentReplayResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.State != ReplayStateReady {
		t.Fatalf("state = %q, want %q", response.State, ReplayStateReady)
	}
	if response.Replay == nil || response.Replay.LatestSequenceNumber == nil || *response.Replay.LatestSequenceNumber != 42 {
		t.Fatalf("latest_sequence_number = %v, want 42", response.Replay)
	}
	if len(response.Steps) != 1 {
		t.Fatalf("step count = %d, want 1", len(response.Steps))
	}
	if response.Pagination.NextCursor == nil || *response.Pagination.NextCursor != "2" {
		t.Fatalf("next_cursor = %v, want 2", response.Pagination.NextCursor)
	}
}

func TestGetRunAgentReplayEndpointReturnsPendingState(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/replays/"+runAgentID.String(), nil)
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
		stubRunReadService{},
		&fakeReplayReadService{
			replayResult: GetRunAgentReplayResult{
				RunAgent: domain.RunAgent{
					ID:          runAgentID,
					RunID:       uuid.New(),
					WorkspaceID: workspaceID,
					Status:      domain.RunAgentStatusExecuting,
				},
				State:   ReplayStatePending,
				Message: "replay generation is pending",
				StepPage: ReplayStepPage{
					Steps:      []json.RawMessage{},
					Limit:      defaultReplayStepPageLimit,
					TotalSteps: 0,
				},
			},
		},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}

	var response getRunAgentReplayResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.State != ReplayStatePending {
		t.Fatalf("state = %q, want %q", response.State, ReplayStatePending)
	}
}

func TestGetRunAgentReplayEndpointReturnsErroredState(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/replays/"+runAgentID.String(), nil)
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
		stubRunReadService{},
		&fakeReplayReadService{
			replayResult: GetRunAgentReplayResult{
				RunAgent: domain.RunAgent{
					ID:          runAgentID,
					RunID:       uuid.New(),
					WorkspaceID: workspaceID,
					Status:      domain.RunAgentStatusCompleted,
				},
				State:   ReplayStateErrored,
				Message: "replay generation failed or replay data is unavailable",
				StepPage: ReplayStepPage{
					Steps:      []json.RawMessage{},
					Limit:      defaultReplayStepPageLimit,
					TotalSteps: 0,
				},
			},
		},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func TestGetRunAgentReplayEndpointReturnsNotFoundWhenRunAgentMissing(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/replays/"+runAgentID.String(), nil)
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
		stubRunReadService{},
		&fakeReplayReadService{replayErr: repository.ErrRunAgentNotFound},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestGetRunAgentReplayEndpointReturnsForbidden(t *testing.T) {
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/replays/"+runAgentID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, uuid.New().String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		&fakeReplayReadService{replayErr: ErrForbidden},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestGetRunAgentReplayEndpointRejectsMalformedRunAgentID(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/replays/not-a-uuid", nil)
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
		stubRunReadService{},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestGetRunAgentReplayEndpointRejectsMalformedPagination(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/replays/"+runAgentID.String()+"?cursor=-1", nil)
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
		stubRunReadService{},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestGetRunAgentScorecardEndpointReturnsScorecard(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/scorecards/"+runAgentID.String(), nil)
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
		stubRunReadService{},
		&fakeReplayReadService{
			scorecardResult: GetRunAgentScorecardResult{
				RunAgent: domain.RunAgent{
					ID:          runAgentID,
					RunID:       uuid.New(),
					WorkspaceID: workspaceID,
				},
				State: ReplayStateReady,
				Scorecard: &repository.RunAgentScorecard{
					ID:               uuid.New(),
					RunAgentID:       runAgentID,
					EvaluationSpecID: uuid.New(),
					OverallScore:     float64Ptr(0.91),
					Scorecard:        []byte(`{"winner":true}`),
					CreatedAt:        time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
					UpdatedAt:        time.Date(2026, 3, 13, 12, 1, 0, 0, time.UTC),
				},
			},
		},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response getRunAgentScorecardResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.RunAgentID != runAgentID {
		t.Fatalf("run_agent_id = %s, want %s", response.RunAgentID, runAgentID)
	}
	if response.OverallScore == nil || *response.OverallScore != 0.91 {
		t.Fatalf("overall_score = %v, want 0.91", response.OverallScore)
	}
	if response.State != ReplayStateReady {
		t.Fatalf("state = %q, want %q", response.State, ReplayStateReady)
	}
}

func TestGetRunAgentScorecardEndpointReturnsForbidden(t *testing.T) {
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/scorecards/"+runAgentID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, uuid.New().String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		&fakeReplayReadService{scorecardErr: ErrForbidden},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestGetRunAgentScorecardEndpointReturnsPendingWhenScorecardIsPending(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/scorecards/"+runAgentID.String(), nil)
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
		stubRunReadService{},
		&fakeReplayReadService{scorecardResult: GetRunAgentScorecardResult{
			RunAgent: domain.RunAgent{
				ID:          runAgentID,
				RunID:       uuid.New(),
				WorkspaceID: workspaceID,
				Status:      domain.RunAgentStatusEvaluating,
			},
			State:   ReplayStatePending,
			Message: "scorecard generation is pending",
		}},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}

	var response getRunAgentScorecardResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.State != ReplayStatePending {
		t.Fatalf("state = %q, want %q", response.State, ReplayStatePending)
	}
}

func TestGetRunAgentScorecardEndpointReturnsConflictWhenScorecardIsMissingAfterTerminalState(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/scorecards/"+runAgentID.String(), nil)
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
		stubRunReadService{},
		&fakeReplayReadService{scorecardResult: GetRunAgentScorecardResult{
			RunAgent: domain.RunAgent{
				ID:          runAgentID,
				RunID:       uuid.New(),
				WorkspaceID: workspaceID,
				Status:      domain.RunAgentStatusCompleted,
			},
			State:   ReplayStateErrored,
			Message: "scorecard generation failed or scorecard data is unavailable",
		}},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func TestGetRunAgentScorecardEndpointReturnsNotFoundWhenRunAgentMissing(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/scorecards/"+runAgentID.String(), nil)
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
		stubRunReadService{},
		&fakeReplayReadService{scorecardErr: repository.ErrRunAgentNotFound},
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
		nil,
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

type fakeReplayReadRepository struct {
	runAgent     domain.RunAgent
	runAgentErr  error
	replay       repository.RunAgentReplay
	replayErr    error
	scorecard    repository.RunAgentScorecard
	scorecardErr error
}

func (f *fakeReplayReadRepository) GetRunAgentByID(_ context.Context, _ uuid.UUID) (domain.RunAgent, error) {
	return f.runAgent, f.runAgentErr
}

func (f *fakeReplayReadRepository) GetRunAgentReplayByRunAgentID(_ context.Context, _ uuid.UUID) (repository.RunAgentReplay, error) {
	return f.replay, f.replayErr
}

func (f *fakeReplayReadRepository) GetRunAgentScorecardByRunAgentID(_ context.Context, _ uuid.UUID) (repository.RunAgentScorecard, error) {
	return f.scorecard, f.scorecardErr
}

type fakeReplayReadService struct {
	replayResult    GetRunAgentReplayResult
	replayErr       error
	scorecardResult GetRunAgentScorecardResult
	scorecardErr    error
}

func (f *fakeReplayReadService) GetRunAgentReplay(_ context.Context, _ Caller, _ uuid.UUID, _ ReplayStepPageParams) (GetRunAgentReplayResult, error) {
	return f.replayResult, f.replayErr
}

func (f *fakeReplayReadService) GetRunAgentScorecard(_ context.Context, _ Caller, _ uuid.UUID) (GetRunAgentScorecardResult, error) {
	return f.scorecardResult, f.scorecardErr
}

func decodeReplayPayload(t *testing.T, payload []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode replay payload: %v", err)
	}
	return decoded
}

func int64Ptr(v int64) *int64          { return &v }
func float64Ptr(v float64) *float64    { return &v }
func replayStringPtr(v string) *string { return &v }
