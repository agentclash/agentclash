package api

import (
	"bytes"
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

func TestRunReadManagerReturnsRunForAuthorizedCaller(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}

	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
		},
	})

	result, err := manager.GetRun(context.Background(), caller, runID)
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if result.Run.ID != runID {
		t.Fatalf("run id = %s, want %s", result.Run.ID, runID)
	}
}

func TestRunReadManagerReturnsNotFound(t *testing.T) {
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		getRunErr: repository.ErrRunNotFound,
	})

	_, err := manager.GetRun(context.Background(), Caller{UserID: uuid.New()}, uuid.New())
	if !errors.Is(err, repository.ErrRunNotFound) {
		t.Fatalf("error = %v, want ErrRunNotFound", err)
	}
}

func TestRunReadManagerRejectsForbiddenWorkspaceAccess(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
		},
	})

	_, err := manager.GetRun(context.Background(), Caller{
		UserID:               uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{},
	}, uuid.New())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

func TestRunReadManagerReturnsRepositoryErrorWhenListingAgents(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
		},
		listRunAgentsErr: errors.New("database unavailable"),
	})

	_, err := manager.ListRunAgents(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}, uuid.New())
	if err == nil {
		t.Fatalf("expected repository error")
	}
	if err.Error() != "list run agents: database unavailable" {
		t.Fatalf("error = %q, want wrapped repository error", err.Error())
	}
}

func TestGetRunEndpointReturnsRun(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	runID := uuid.New()
	workflowID := "RunWorkflow/" + runID.String()
	temporalRunID := "temporal-run-id"

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String(), nil)
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		&fakeRunReadService{
			getRunResult: GetRunResult{
				Run: domain.Run{
					ID:                 runID,
					WorkspaceID:        workspaceID,
					Name:               "Run 2026-03-13T12:00:00Z",
					Status:             domain.RunStatusQueued,
					ExecutionMode:      "comparison",
					TemporalWorkflowID: &workflowID,
					TemporalRunID:      &temporalRunID,
					CreatedAt:          time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
					UpdatedAt:          time.Date(2026, 3, 13, 12, 1, 0, 0, time.UTC),
				},
			},
		},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response getRunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.ID != runID {
		t.Fatalf("run id = %s, want %s", response.ID, runID)
	}
	if response.TemporalWorkflowID == nil || *response.TemporalWorkflowID != workflowID {
		t.Fatalf("temporal workflow id = %v, want %q", response.TemporalWorkflowID, workflowID)
	}
}

func TestGetRunEndpointReturnsNotFound(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String(), nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		&fakeRunReadService{getRunErr: repository.ErrRunNotFound},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestGetRunEndpointRejectsMalformedRunID(t *testing.T) {
	workspaceID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/runs/not-a-uuid", nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		&fakeRunReadService{},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestListRunAgentsEndpointReturnsOrderedItems(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	firstID := uuid.New()
	secondID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String()+"/agents", bytes.NewBuffer(nil))
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		&fakeRunReadService{
			listRunAgentsResult: ListRunAgentsResult{
				Run: domain.Run{
					ID:          runID,
					WorkspaceID: workspaceID,
				},
				RunAgents: []domain.RunAgent{
					{ID: firstID, RunID: runID, LaneIndex: 0, Label: "Alpha", AgentDeploymentID: uuid.New(), AgentDeploymentSnapshotID: uuid.New(), Status: domain.RunAgentStatusQueued, CreatedAt: time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)},
					{ID: secondID, RunID: runID, LaneIndex: 1, Label: "Beta", AgentDeploymentID: uuid.New(), AgentDeploymentSnapshotID: uuid.New(), Status: domain.RunAgentStatusQueued, CreatedAt: time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)},
				},
			},
		},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response listRunAgentsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(response.Items))
	}
	if response.Items[0].ID != firstID || response.Items[1].ID != secondID {
		t.Fatalf("run agent ordering = [%s, %s], want [%s, %s]", response.Items[0].ID, response.Items[1].ID, firstID, secondID)
	}
}

func TestListRunAgentsEndpointReturnsForbidden(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String()+"/agents", nil)
	req.Header.Set(headerUserID, uuid.New().String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	recorder := httptest.NewRecorder()

	newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		stubRunCreationService{},
		&fakeRunReadService{listRunAgentsErr: ErrForbidden},
		&fakeReplayReadService{},
		stubHostedRunIngestionService{},
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
	).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

type fakeRunReadRepository struct {
	run              domain.Run
	runAgents        []domain.RunAgent
	getRunErr        error
	listRunAgentsErr error
}

func (f *fakeRunReadRepository) GetRunByID(_ context.Context, _ uuid.UUID) (domain.Run, error) {
	return f.run, f.getRunErr
}

func (f *fakeRunReadRepository) ListRunAgentsByRunID(_ context.Context, _ uuid.UUID) ([]domain.RunAgent, error) {
	return f.runAgents, f.listRunAgentsErr
}

func (f *fakeRunReadRepository) ListRunsByWorkspaceID(_ context.Context, _ uuid.UUID, _ int32, _ int32) ([]domain.Run, error) {
	return nil, nil
}

func (f *fakeRunReadRepository) CountRunsByWorkspaceID(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

type fakeRunReadService struct {
	getRunResult        GetRunResult
	getRunErr           error
	listRunAgentsResult ListRunAgentsResult
	listRunAgentsErr    error
}

func (f *fakeRunReadService) GetRun(_ context.Context, _ Caller, _ uuid.UUID) (GetRunResult, error) {
	return f.getRunResult, f.getRunErr
}

func (f *fakeRunReadService) ListRunAgents(_ context.Context, _ Caller, _ uuid.UUID) (ListRunAgentsResult, error) {
	return f.listRunAgentsResult, f.listRunAgentsErr
}

func (f *fakeRunReadService) ListRuns(_ context.Context, _ Caller, _ ListRunsInput) (ListRunsResult, error) {
	return ListRunsResult{}, nil
}
