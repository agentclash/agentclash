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

	"github.com/agentclash/agentclash/backend/internal/failurereview"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.temporal.io/api/serviceerror"
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
			ID:               runID,
			WorkspaceID:      workspaceID,
			OfficialPackMode: domain.OfficialPackModeSuiteOnly,
		},
		regressionCoverageCases: []repository.RunRegressionCoverageCase{
			{
				RegressionCaseID:    uuid.New(),
				RegressionCaseTitle: stringPtr("Replay drift"),
				SuiteID:             uuidPtr(uuid.New()),
				SuiteName:           stringPtr("Critical Regressions"),
				Outcome:             repository.RunRegressionCoverageOutcomePass,
			},
			{
				RegressionCaseID:    uuid.New(),
				RegressionCaseTitle: stringPtr("Missing tool output"),
				SuiteID:             uuidPtr(uuid.New()),
				SuiteName:           stringPtr("Edge Cases"),
				Outcome:             repository.RunRegressionCoverageOutcomeFail,
			},
		},
	})

	result, err := manager.GetRun(context.Background(), caller, runID)
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if result.Run.ID != runID {
		t.Fatalf("run id = %s, want %s", result.Run.ID, runID)
	}
	if result.RegressionCoverage == nil {
		t.Fatal("regression coverage = nil, want value")
	}
	if len(result.RegressionCoverage.Suites) != 2 {
		t.Fatalf("suite coverage count = %d, want 2", len(result.RegressionCoverage.Suites))
	}
	if result.RegressionCoverage.Suites[0].PassCount != 1 {
		t.Fatalf("first suite pass_count = %d, want 1", result.RegressionCoverage.Suites[0].PassCount)
	}
	if result.RegressionCoverage.Suites[1].FailCount != 1 {
		t.Fatalf("second suite fail_count = %d, want 1", result.RegressionCoverage.Suites[1].FailCount)
	}
}

func TestRunReadManagerCancelRunTransitionsActiveRunAndCancelsTemporal(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	runID := uuid.New()
	workflowID := "RunWorkflow/" + runID.String()
	temporalRunID := "temporal-run-1"
	cancelled := domain.Run{ID: runID, WorkspaceID: workspaceID, Status: domain.RunStatusCancelled}
	repo := &fakeRunReadRepository{
		run: domain.Run{
			ID:                 runID,
			WorkspaceID:        workspaceID,
			Status:             domain.RunStatusRunning,
			TemporalWorkflowID: &workflowID,
			TemporalRunID:      &temporalRunID,
		},
		transitionedRun: cancelled,
	}
	control := &fakeRunWorkflowControl{}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), repo).WithRunWorkflowControl(control)

	result, err := manager.CancelRun(context.Background(), Caller{
		UserID: userID,
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, runID)
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}

	if result.Run.Status != domain.RunStatusCancelled {
		t.Fatalf("status = %s, want cancelled", result.Run.Status)
	}
	if control.workflowID != workflowID || control.runID != temporalRunID {
		t.Fatalf("temporal cancel = (%q, %q), want (%q, %q)", control.workflowID, control.runID, workflowID, temporalRunID)
	}
	if repo.transitionRunStatusCalls != 1 || repo.transitionRunStatus != domain.RunStatusCancelled {
		t.Fatalf("transition calls/status = %d/%s, want 1/cancelled", repo.transitionRunStatusCalls, repo.transitionRunStatus)
	}
	if repo.transitionChangedBy == nil || *repo.transitionChangedBy != userID {
		t.Fatalf("changed by = %v, want %s", repo.transitionChangedBy, userID)
	}
	if repo.transitionReason == nil || *repo.transitionReason != "cancelled by user" {
		t.Fatalf("reason = %v, want cancelled by user", repo.transitionReason)
	}
}

func TestRunReadManagerCancelRunIsIdempotentForTerminalRun(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	repo := &fakeRunReadRepository{
		run: domain.Run{ID: runID, WorkspaceID: workspaceID, Status: domain.RunStatusCompleted},
	}
	control := &fakeRunWorkflowControl{}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), repo).WithRunWorkflowControl(control)

	result, err := manager.CancelRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, runID)
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}

	if result.Run.Status != domain.RunStatusCompleted {
		t.Fatalf("status = %s, want completed", result.Run.Status)
	}
	if control.cancelCalls != 0 || repo.transitionRunStatusCalls != 0 {
		t.Fatalf("cancel/transition calls = %d/%d, want 0/0", control.cancelCalls, repo.transitionRunStatusCalls)
	}
}

func TestRunReadManagerCancelRunReturnsTemporalFailure(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	workflowID := "RunWorkflow/" + runID.String()
	temporalErr := errors.New("temporal unavailable")
	repo := &fakeRunReadRepository{
		run: domain.Run{
			ID:                 runID,
			WorkspaceID:        workspaceID,
			Status:             domain.RunStatusRunning,
			TemporalWorkflowID: &workflowID,
		},
	}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), repo).WithRunWorkflowControl(&fakeRunWorkflowControl{err: temporalErr})

	_, err := manager.CancelRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, runID)
	if !errors.Is(err, temporalErr) {
		t.Fatalf("CancelRun error = %v, want temporal error", err)
	}
	if repo.transitionRunStatusCalls != 0 {
		t.Fatalf("transition calls = %d, want 0", repo.transitionRunStatusCalls)
	}
}

func TestRunReadManagerCancelRunUsesDeterministicWorkflowIDWhenNotPersisted(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	runID := uuid.New()
	repo := &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			Status:      domain.RunStatusQueued,
		},
	}
	control := &fakeRunWorkflowControl{}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), repo).WithRunWorkflowControl(control)

	_, err := manager.CancelRun(context.Background(), Caller{
		UserID: userID,
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, runID)
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}

	wantWorkflowID := workflow.RunWorkflowName + "/" + runID.String()
	if control.workflowID != wantWorkflowID || control.runID != "" {
		t.Fatalf("temporal cancel = (%q, %q), want (%q, \"\")", control.workflowID, control.runID, wantWorkflowID)
	}
	if repo.transitionRunStatusCalls != 1 {
		t.Fatalf("transition calls = %d, want 1", repo.transitionRunStatusCalls)
	}
}

func TestRunReadManagerCancelRunTransitionsWhenUnpersistedWorkflowIsNotFound(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	repo := &fakeRunReadRepository{
		run: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			Status:      domain.RunStatusQueued,
		},
	}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), repo).WithRunWorkflowControl(&fakeRunWorkflowControl{err: serviceerror.NewNotFound("workflow not found")})

	result, err := manager.CancelRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, runID)
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}
	if result.Run.Status != domain.RunStatusCancelled {
		t.Fatalf("status = %s, want cancelled", result.Run.Status)
	}
	if repo.transitionRunStatusCalls != 1 {
		t.Fatalf("transition calls = %d, want 1", repo.transitionRunStatusCalls)
	}
}

func TestRunReadManagerCancelRunReturnsLatestTerminalRunAfterTemporalFailure(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	workflowID := "RunWorkflow/" + runID.String()
	repo := &fakeRunReadRepository{
		run: domain.Run{
			ID:                 runID,
			WorkspaceID:        workspaceID,
			Status:             domain.RunStatusRunning,
			TemporalWorkflowID: &workflowID,
		},
		latestRun: domain.Run{
			ID:          runID,
			WorkspaceID: workspaceID,
			Status:      domain.RunStatusCompleted,
		},
	}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), repo).WithRunWorkflowControl(&fakeRunWorkflowControl{err: errors.New("workflow not found")})

	result, err := manager.CancelRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, runID)
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}
	if result.Run.Status != domain.RunStatusCompleted {
		t.Fatalf("status = %s, want completed", result.Run.Status)
	}
	if repo.transitionRunStatusCalls != 0 {
		t.Fatalf("transition calls = %d, want 0", repo.transitionRunStatusCalls)
	}
}

func TestRunReadManagerCancelRunRequiresWorkflowControlForTemporalRun(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	workflowID := "RunWorkflow/" + runID.String()
	repo := &fakeRunReadRepository{
		run: domain.Run{
			ID:                 runID,
			WorkspaceID:        workspaceID,
			Status:             domain.RunStatusRunning,
			TemporalWorkflowID: &workflowID,
		},
	}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), repo)

	_, err := manager.CancelRun(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, runID)
	var workflowErr RunCancellationWorkflowError
	if !errors.As(err, &workflowErr) {
		t.Fatalf("CancelRun error = %v, want RunCancellationWorkflowError", err)
	}
	if repo.transitionRunStatusCalls != 0 {
		t.Fatalf("transition calls = %d, want 0", repo.transitionRunStatusCalls)
	}
}

func TestCancelRunHandlerReturnsCancelledRun(t *testing.T) {
	runID := uuid.New()
	workspaceID := uuid.New()
	service := &fakeRunReadService{
		cancelRunResult: CancelRunResult{
			Run: domain.Run{ID: runID, WorkspaceID: workspaceID, Status: domain.RunStatusCancelled},
		},
	}
	router := chi.NewRouter()
	router.Post("/v1/runs/{runID}/cancel", cancelRunHandler(slog.Default(), service))
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+runID.String()+"/cancel", nil)
	req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, Caller{UserID: uuid.New()}))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var response getRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != runID || response.Status != domain.RunStatusCancelled {
		t.Fatalf("response = %+v, want cancelled run %s", response, runID)
	}
}

func TestCancelRunHandlerReturnsTemporalFailure(t *testing.T) {
	runID := uuid.New()
	workspaceID := uuid.New()
	workflowErr := errors.New("temporal unavailable")
	service := &fakeRunReadService{
		cancelRunErr: RunCancellationWorkflowError{
			Run:   domain.Run{ID: runID, WorkspaceID: workspaceID, Status: domain.RunStatusRunning},
			Cause: workflowErr,
		},
	}
	router := chi.NewRouter()
	router.Post("/v1/runs/{runID}/cancel", cancelRunHandler(slog.Default(), service))
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+runID.String()+"/cancel", nil)
	req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, Caller{UserID: uuid.New()}))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var response runWorkflowErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error.Code != "workflow_cancel_failed" || response.Run.ID != runID {
		t.Fatalf("response = %+v, want workflow_cancel_failed for %s", response, runID)
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
	prNumber := int32(23)

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String(), nil)
	req.Header.Set(headerUserID, userID.String())
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
			getRunResult: GetRunResult{
				Run: domain.Run{
					ID:                 runID,
					WorkspaceID:        workspaceID,
					Name:               "Run 2026-03-13T12:00:00Z",
					Status:             domain.RunStatusQueued,
					OfficialPackMode:   domain.OfficialPackModeSuiteOnly,
					ExecutionMode:      "comparison",
					TemporalWorkflowID: &workflowID,
					TemporalRunID:      &temporalRunID,
					CIMetadata: &domain.RunCIMetadata{
						Provider:          "github_actions",
						Repository:        "acme/agent",
						PullRequestNumber: &prNumber,
						WorkflowRunURL:    "https://github.com/acme/agent/actions/runs/123",
						DefaultBranch:     "main",
					},
					CreatedAt: time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
					UpdatedAt: time.Date(2026, 3, 13, 12, 1, 0, 0, time.UTC),
				},
				RegressionCoverage: &RunRegressionCoverage{
					Suites: []RunRegressionCoverageSuite{
						{
							ID:        uuid.New(),
							Name:      "Critical Regressions",
							CaseCount: 2,
							PassCount: 1,
							FailCount: 1,
						},
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

	var response getRunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.ID != runID {
		t.Fatalf("run id = %s, want %s", response.ID, runID)
	}
	if response.OfficialPackMode != string(domain.OfficialPackModeSuiteOnly) {
		t.Fatalf("official_pack_mode = %q, want %q", response.OfficialPackMode, domain.OfficialPackModeSuiteOnly)
	}
	if response.TemporalWorkflowID == nil || *response.TemporalWorkflowID != workflowID {
		t.Fatalf("temporal workflow id = %v, want %q", response.TemporalWorkflowID, workflowID)
	}
	if response.CIMetadata == nil || response.CIMetadata.Repository != "acme/agent" || response.CIMetadata.PullRequestNumber == nil || *response.CIMetadata.PullRequestNumber != prNumber || response.CIMetadata.DefaultBranch != "main" {
		t.Fatalf("ci metadata = %+v, want GitHub metadata", response.CIMetadata)
	}
	if response.RegressionCoverage == nil || len(response.RegressionCoverage.Suites) != 1 {
		t.Fatalf("regression_coverage = %#v, want one suite", response.RegressionCoverage)
	}
	if response.RegressionCoverage.Suites[0].FailCount != 1 {
		t.Fatalf("suite fail_count = %d, want 1", response.RegressionCoverage.Suites[0].FailCount)
	}
}

func TestGetRunEndpointReturnsNotFound(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String(), nil)
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
		&fakeRunReadService{getRunErr: repository.ErrRunNotFound},
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

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		&fakeRunReadService{},
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

func TestListRunAgentsEndpointReturnsOrderedItems(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	firstID := uuid.New()
	secondID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID.String()+"/agents", bytes.NewBuffer(nil))
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

	newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		&fakeRunReadService{listRunAgentsErr: ErrForbidden},
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

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestRunReadManagerListsRunEventStreamInTracePackOrder(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	agentLaneOne := uuid.New()
	agentLaneZero := uuid.New()
	occurredAt := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}
	manager := NewRunReadManager(NewCallerWorkspaceAuthorizer(), &fakeRunReadRepository{
		run: domain.Run{ID: runID, WorkspaceID: workspaceID},
		runAgents: []domain.RunAgent{
			{ID: agentLaneOne, RunID: runID, LaneIndex: 1},
			{ID: agentLaneZero, RunID: runID, LaneIndex: 0},
		},
		runEvents: map[uuid.UUID][]repository.RunEvent{
			agentLaneOne: {
				{ID: 30, RunID: runID, RunAgentID: agentLaneOne, SequenceNumber: 2, EventType: runevents.EventTypeSystemStepCompleted, Source: runevents.SourceNativeEngine, OccurredAt: occurredAt, Payload: []byte(`{"label":"lane-one-seq-two"}`)},
				{ID: 20, RunID: runID, RunAgentID: agentLaneOne, SequenceNumber: 1, EventType: runevents.EventTypeSystemStepStarted, Source: runevents.SourceNativeEngine, OccurredAt: occurredAt, Payload: []byte(`{"label":"lane-one-seq-one"}`)},
			},
			agentLaneZero: {
				{ID: 10, RunID: runID, RunAgentID: agentLaneZero, SequenceNumber: 1, EventType: runevents.EventTypeSystemRunStarted, Source: runevents.SourceNativeEngine, OccurredAt: occurredAt.Add(time.Second), Payload: []byte(`{"label":"later"}`)},
				{ID: 40, RunID: runID, RunAgentID: agentLaneZero, SequenceNumber: 2, EventType: runevents.EventTypeSystemStepStarted, Source: runevents.SourceNativeEngine, OccurredAt: occurredAt, Payload: []byte(`{"label":"lane-zero"}`)},
			},
		},
	})

	result, err := manager.ListRunEventStream(context.Background(), caller, runID)
	if err != nil {
		t.Fatalf("ListRunEventStream returned error: %v", err)
	}

	if len(result.Events) != 4 {
		t.Fatalf("event count = %d, want 4", len(result.Events))
	}
	got := []int64{result.Events[0].ID, result.Events[1].ID, result.Events[2].ID, result.Events[3].ID}
	want := []int64{40, 20, 30, 10}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event order = %v, want %v", got, want)
		}
	}
}

type fakeRunReadRepository struct {
	run                      domain.Run
	latestRun                domain.Run
	transitionedRun          domain.Run
	evalSession              repository.EvalSessionWithRuns
	evalSessions             []domain.EvalSession
	evalSessionRuns          map[uuid.UUID][]domain.Run
	evalSessionResults       map[uuid.UUID]repository.EvalSessionAggregateRecord
	runScorecard             repository.RunScorecard
	regressionCoverageCases  []repository.RunRegressionCoverageCase
	runAgents                []domain.RunAgent
	runEvents                map[uuid.UUID][]repository.RunEvent
	failureItems             []failurereview.Item
	failureItemsByRunID      map[uuid.UUID][]failurereview.Item
	recentComparableRuns     []domain.Run
	recentComparableRunCalls int
	spendPolicies            []repository.SpendPolicyRow
	providerAccount          repository.ProviderAccountRow
	workspaceSecrets         map[string]string
	getRunErr                error
	getEvalSessionErr        error
	getEvalSessionResultErr  error
	listEvalSessionRunsErr   error
	getRunScorecardErr       error
	listRunAgentsErr         error
	listRunFailuresErr       error
	listRecentRunsErr        error
	listEvalSessionsErr      error
	getProviderAccountErr    error
	listSpendPoliciesErr     error
	loadWorkspaceSecretsErr  error
	transitionRunStatusErr   error
	transitionRunStatusCalls int
	transitionRunStatus      domain.RunStatus
	transitionReason         *string
	transitionChangedBy      *uuid.UUID
	getRunByIDCalls          int
}

func (f *fakeRunReadRepository) GetRunByID(_ context.Context, _ uuid.UUID) (domain.Run, error) {
	f.getRunByIDCalls++
	if f.getRunByIDCalls > 1 && f.latestRun.ID != uuid.Nil {
		return f.latestRun, f.getRunErr
	}
	return f.run, f.getRunErr
}

func (f *fakeRunReadRepository) TransitionRunStatus(_ context.Context, params repository.TransitionRunStatusParams) (domain.Run, error) {
	f.transitionRunStatusCalls++
	f.transitionRunStatus = params.ToStatus
	f.transitionReason = params.Reason
	f.transitionChangedBy = params.ChangedByUserID
	if f.transitionRunStatusErr != nil {
		return domain.Run{}, f.transitionRunStatusErr
	}
	if f.transitionedRun.ID != uuid.Nil {
		return f.transitionedRun, nil
	}
	run := f.run
	run.Status = params.ToStatus
	return run, nil
}

func (f *fakeRunReadRepository) GetEvalSessionWithRuns(_ context.Context, _ uuid.UUID) (repository.EvalSessionWithRuns, error) {
	return f.evalSession, f.getEvalSessionErr
}

func (f *fakeRunReadRepository) GetEvalSessionResultBySessionID(_ context.Context, evalSessionID uuid.UUID) (repository.EvalSessionAggregateRecord, error) {
	if f.getEvalSessionResultErr != nil {
		return repository.EvalSessionAggregateRecord{}, f.getEvalSessionResultErr
	}
	if f.evalSessionResults == nil {
		return repository.EvalSessionAggregateRecord{}, repository.ErrEvalSessionResultNotFound
	}
	result, ok := f.evalSessionResults[evalSessionID]
	if !ok {
		return repository.EvalSessionAggregateRecord{}, repository.ErrEvalSessionResultNotFound
	}
	return result, nil
}

func (f *fakeRunReadRepository) ListRunsByEvalSessionID(_ context.Context, evalSessionID uuid.UUID) ([]domain.Run, error) {
	if f.listEvalSessionRunsErr != nil {
		return nil, f.listEvalSessionRunsErr
	}
	return append([]domain.Run(nil), f.evalSessionRuns[evalSessionID]...), nil
}

func (f *fakeRunReadRepository) GetRunScorecardByRunID(_ context.Context, _ uuid.UUID) (repository.RunScorecard, error) {
	return f.runScorecard, f.getRunScorecardErr
}

func (f *fakeRunReadRepository) ListRunRegressionCoverageCasesByRunID(_ context.Context, _ uuid.UUID) ([]repository.RunRegressionCoverageCase, error) {
	return f.regressionCoverageCases, nil
}

func (f *fakeRunReadRepository) ListRunAgentsByRunID(_ context.Context, _ uuid.UUID) ([]domain.RunAgent, error) {
	return f.runAgents, f.listRunAgentsErr
}

func (f *fakeRunReadRepository) ListRunEventsByRunAgentID(_ context.Context, runAgentID uuid.UUID) ([]repository.RunEvent, error) {
	return append([]repository.RunEvent(nil), f.runEvents[runAgentID]...), nil
}

func (f *fakeRunReadRepository) ListRunFailureReviewItems(_ context.Context, runID uuid.UUID, _ *uuid.UUID) ([]failurereview.Item, error) {
	if f.failureItemsByRunID != nil {
		if items, ok := f.failureItemsByRunID[runID]; ok {
			return append([]failurereview.Item(nil), items...), f.listRunFailuresErr
		}
	}
	return f.failureItems, f.listRunFailuresErr
}

func (f *fakeRunReadRepository) ListRecentComparableScoredRunsBeforeRunID(_ context.Context, _ uuid.UUID, _ int32) ([]domain.Run, error) {
	f.recentComparableRunCalls++
	return append([]domain.Run(nil), f.recentComparableRuns...), f.listRecentRunsErr
}

func (f *fakeRunReadRepository) ListEvalSessionsByWorkspaceID(_ context.Context, _ uuid.UUID, _ int32, _ int32) ([]domain.EvalSession, error) {
	return append([]domain.EvalSession(nil), f.evalSessions...), f.listEvalSessionsErr
}

func (f *fakeRunReadRepository) ListRunsByWorkspaceID(_ context.Context, _ uuid.UUID, _ int32, _ int32) ([]domain.Run, error) {
	return nil, nil
}

func (f *fakeRunReadRepository) CountRunsByWorkspaceID(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

func (f *fakeRunReadRepository) GetProviderAccountByID(_ context.Context, _ uuid.UUID) (repository.ProviderAccountRow, error) {
	return f.providerAccount, f.getProviderAccountErr
}

func (f *fakeRunReadRepository) ListSpendPoliciesByWorkspaceID(_ context.Context, _ uuid.UUID) ([]repository.SpendPolicyRow, error) {
	return f.spendPolicies, f.listSpendPoliciesErr
}

func (f *fakeRunReadRepository) LoadWorkspaceSecrets(_ context.Context, _ uuid.UUID) (map[string]string, error) {
	return f.workspaceSecrets, f.loadWorkspaceSecretsErr
}

type fakeRunReadService struct {
	getRunResult           GetRunResult
	getRunErr              error
	cancelRunResult        CancelRunResult
	cancelRunErr           error
	getEvalSessionResult   GetEvalSessionResult
	getEvalSessionErr      error
	getRunRankingResult    GetRunRankingResult
	getRunRankingErr       error
	insightsResult         GenerateRunRankingInsightsResult
	insightsErr            error
	listEvalSessionsResult ListEvalSessionsResult
	listEvalSessionsErr    error
	listRunAgentsResult    ListRunAgentsResult
	listRunAgentsErr       error
	listRunFailuresResult  ListRunFailuresResult
	listRunFailuresErr     error
}

func (f *fakeRunReadService) GetRun(_ context.Context, _ Caller, _ uuid.UUID) (GetRunResult, error) {
	return f.getRunResult, f.getRunErr
}

func (f *fakeRunReadService) CancelRun(_ context.Context, _ Caller, _ uuid.UUID) (CancelRunResult, error) {
	return f.cancelRunResult, f.cancelRunErr
}

func (f *fakeRunReadService) GetEvalSession(_ context.Context, _ Caller, _ uuid.UUID) (GetEvalSessionResult, error) {
	return f.getEvalSessionResult, f.getEvalSessionErr
}

func (f *fakeRunReadService) GetRunRanking(_ context.Context, _ Caller, _ uuid.UUID, _ GetRunRankingInput) (GetRunRankingResult, error) {
	return f.getRunRankingResult, f.getRunRankingErr
}

func (f *fakeRunReadService) GenerateRunRankingInsights(_ context.Context, _ Caller, _ uuid.UUID, _ GenerateRunRankingInsightsInput) (GenerateRunRankingInsightsResult, error) {
	return f.insightsResult, f.insightsErr
}

func (f *fakeRunReadService) ListEvalSessions(_ context.Context, _ Caller, _ ListEvalSessionsInput) (ListEvalSessionsResult, error) {
	return f.listEvalSessionsResult, f.listEvalSessionsErr
}

func (f *fakeRunReadService) ListRunAgents(_ context.Context, _ Caller, _ uuid.UUID) (ListRunAgentsResult, error) {
	return f.listRunAgentsResult, f.listRunAgentsErr
}

func (f *fakeRunReadService) ListRunFailures(_ context.Context, _ Caller, _ ListRunFailuresInput) (ListRunFailuresResult, error) {
	return f.listRunFailuresResult, f.listRunFailuresErr
}

func (f *fakeRunReadService) ListRuns(_ context.Context, _ Caller, _ ListRunsInput) (ListRunsResult, error) {
	return ListRunsResult{}, nil
}

type fakeRunWorkflowControl struct {
	workflowID  string
	runID       string
	cancelCalls int
	err         error
}

func (f *fakeRunWorkflowControl) CancelRunWorkflow(_ context.Context, workflowID string, runID string) error {
	f.cancelCalls++
	f.workflowID = workflowID
	f.runID = runID
	return f.err
}

func uuidPtr(value uuid.UUID) *uuid.UUID {
	return &value
}
