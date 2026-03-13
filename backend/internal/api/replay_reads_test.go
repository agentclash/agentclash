package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestReplayReadManagerReturnsReplayForAuthorizedCaller(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	manager := NewReplayReadManager(NewCallerWorkspaceAuthorizer(), &fakeReplayReadRepository{
		runAgent: domain.RunAgent{
			ID:          runAgentID,
			WorkspaceID: workspaceID,
		},
		replay: repository.RunAgentReplay{
			ID:         uuid.New(),
			RunAgentID: runAgentID,
		},
	})

	result, err := manager.GetRunAgentReplay(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, runAgentID)
	if err != nil {
		t.Fatalf("GetRunAgentReplay returned error: %v", err)
	}
	if result.Replay.RunAgentID != runAgentID {
		t.Fatalf("replay run_agent_id = %s, want %s", result.Replay.RunAgentID, runAgentID)
	}
}

func TestReplayReadManagerReturnsNotFoundWhenReplayMissing(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewReplayReadManager(NewCallerWorkspaceAuthorizer(), &fakeReplayReadRepository{
		runAgent: domain.RunAgent{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
		},
		replayErr: repository.ErrRunAgentReplayNotFound,
	})

	_, err := manager.GetRunAgentReplay(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, uuid.New())
	if !errors.Is(err, repository.ErrRunAgentReplayNotFound) {
		t.Fatalf("error = %v, want ErrRunAgentReplayNotFound", err)
	}
}

func TestGetRunAgentReplayEndpointReturnsReplay(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/replays/"+runAgentID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
		&fakeReplayReadService{
			replayResult: GetRunAgentReplayResult{
				RunAgent: domain.RunAgent{
					ID:          runAgentID,
					RunID:       uuid.New(),
					WorkspaceID: workspaceID,
				},
				Replay: repository.RunAgentReplay{
					ID:                   uuid.New(),
					RunAgentID:           runAgentID,
					ArtifactID:           nil,
					Summary:              []byte(`{"headline":"trace ready"}`),
					LatestSequenceNumber: int64Ptr(42),
					EventCount:           42,
					CreatedAt:            time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
					UpdatedAt:            time.Date(2026, 3, 13, 12, 1, 0, 0, time.UTC),
				},
			},
		},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response getRunAgentReplayResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.RunAgentID != runAgentID {
		t.Fatalf("run_agent_id = %s, want %s", response.RunAgentID, runAgentID)
	}
	if response.LatestSequenceNumber == nil || *response.LatestSequenceNumber != 42 {
		t.Fatalf("latest_sequence_number = %v, want 42", response.LatestSequenceNumber)
	}
}

func TestGetRunAgentReplayEndpointReturnsNotFoundWhenReplayMissing(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/replays/"+runAgentID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
		&fakeReplayReadService{replayErr: repository.ErrRunAgentReplayNotFound},
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

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
		&fakeReplayReadService{replayErr: ErrForbidden},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestGetRunAgentReplayEndpointReturnsNotFoundWhenRunAgentMissing(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/replays/"+runAgentID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
		&fakeReplayReadService{replayErr: repository.ErrRunAgentNotFound},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestGetRunAgentScorecardEndpointReturnsScorecard(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/scorecards/"+runAgentID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
		&fakeReplayReadService{
			scorecardResult: GetRunAgentScorecardResult{
				RunAgent: domain.RunAgent{
					ID:          runAgentID,
					RunID:       uuid.New(),
					WorkspaceID: workspaceID,
				},
				Scorecard: repository.RunAgentScorecard{
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
}

func TestGetRunAgentScorecardEndpointReturnsForbidden(t *testing.T) {
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/scorecards/"+runAgentID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, uuid.New().String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
		&fakeReplayReadService{scorecardErr: ErrForbidden},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestGetRunAgentScorecardEndpointReturnsNotFoundWhenScorecardMissing(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/scorecards/"+runAgentID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
		&fakeReplayReadService{scorecardErr: repository.ErrRunAgentScorecardNotFound},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestGetRunAgentScorecardEndpointReturnsNotFoundWhenRunAgentMissing(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/scorecards/"+runAgentID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		stubRunReadService{},
		&fakeReplayReadService{scorecardErr: repository.ErrRunAgentNotFound},
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

func (f *fakeReplayReadService) GetRunAgentReplay(_ context.Context, _ Caller, _ uuid.UUID) (GetRunAgentReplayResult, error) {
	return f.replayResult, f.replayErr
}

func (f *fakeReplayReadService) GetRunAgentScorecard(_ context.Context, _ Caller, _ uuid.UUID) (GetRunAgentScorecardResult, error) {
	return f.scorecardResult, f.scorecardErr
}

func int64Ptr(v int64) *int64       { return &v }
func float64Ptr(v float64) *float64 { return &v }
