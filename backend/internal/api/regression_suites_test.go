package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRegressionManagerRejectsCrossWorkspaceSuiteAccess(t *testing.T) {
	requestWorkspaceID := uuid.New()
	actualWorkspaceID := uuid.New()
	suiteID := uuid.New()

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		suite: repository.RegressionSuite{
			ID:          suiteID,
			WorkspaceID: actualWorkspaceID,
			Status:      domain.RegressionSuiteStatusActive,
		},
	})

	_, err := manager.GetRegressionSuite(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			requestWorkspaceID: {WorkspaceID: requestWorkspaceID, Role: RoleWorkspaceMember},
		},
	}, GetRegressionSuiteInput{
		WorkspaceID: requestWorkspaceID,
		SuiteID:     suiteID,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("GetRegressionSuite error = %v, want ErrForbidden", err)
	}
}

func TestRegressionManagerRejectsCrossWorkspaceCasePatch(t *testing.T) {
	requestWorkspaceID := uuid.New()
	actualWorkspaceID := uuid.New()
	caseID := uuid.New()

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		regressionCase: repository.RegressionCase{
			ID:          caseID,
			WorkspaceID: actualWorkspaceID,
			Status:      domain.RegressionCaseStatusActive,
		},
	})

	status := domain.RegressionCaseStatusMuted
	_, err := manager.PatchRegressionCase(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			requestWorkspaceID: {WorkspaceID: requestWorkspaceID, Role: RoleWorkspaceMember},
		},
	}, PatchRegressionCaseInput{
		WorkspaceID: requestWorkspaceID,
		CaseID:      caseID,
		Status:      &status,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("PatchRegressionCase error = %v, want ErrForbidden", err)
	}
}

func TestRegressionManagerRejectsInvisibleChallengePackOnCreate(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		challengePacks: []repository.ChallengePackSummary{{ID: uuid.New()}},
	})

	_, err := manager.CreateRegressionSuite(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, CreateRegressionSuiteInput{
		WorkspaceID:           workspaceID,
		SourceChallengePackID: uuid.New(),
		Name:                  "Critical regressions",
		DefaultGateSeverity:   domain.RegressionSeverityWarning,
	})
	if !errors.Is(err, ErrChallengePackNotFound) {
		t.Fatalf("CreateRegressionSuite error = %v, want ErrChallengePackNotFound", err)
	}
}

func TestRegressionSuiteEndpointsRoundTrip(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	sourceChallengePackID := uuid.New()
	suiteID := uuid.New()
	caseID := uuid.New()
	createdAt := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(5 * time.Minute)

	service := &fakeRegressionService{
		suite: repository.RegressionSuite{
			ID:                    suiteID,
			WorkspaceID:           workspaceID,
			SourceChallengePackID: sourceChallengePackID,
			Name:                  "Critical regressions",
			Description:           "Seed suite",
			Status:                domain.RegressionSuiteStatusActive,
			SourceMode:            "derived_only",
			DefaultGateSeverity:   domain.RegressionSeverityWarning,
			CreatedByUserID:       userID,
			CreatedAt:             createdAt,
			UpdatedAt:             updatedAt,
		},
		regressionCase: repository.RegressionCase{
			ID:                           caseID,
			SuiteID:                      suiteID,
			WorkspaceID:                  workspaceID,
			Title:                        "Case one",
			Description:                  "First regression case",
			Status:                       domain.RegressionCaseStatusActive,
			Severity:                     domain.RegressionSeverityBlocking,
			PromotionMode:                domain.RegressionPromotionModeFullExecutable,
			SourceChallengePackVersionID: uuid.New(),
			SourceChallengeIdentityID:    uuid.New(),
			SourceCaseKey:                "case-1",
			EvidenceTier:                 "replay",
			FailureClass:                 "behavioral_regression",
			FailureSummary:               "Regressed",
			PayloadSnapshot:              json.RawMessage(`{"payload":"snapshot"}`),
			ExpectedContract:             json.RawMessage(`{"contract":"expected"}`),
			Metadata:                     json.RawMessage(`{"origin":"test"}`),
			CreatedAt:                    createdAt,
			UpdatedAt:                    updatedAt,
		},
	}

	router := buildRouter(routerOptions{
		authMode:                   "dev",
		logger:                     testLogger(t),
		authenticator:              NewDevelopmentAuthenticator(),
		authorizer:                 NewCallerWorkspaceAuthorizer(),
		runCreationService:         stubRunCreationService{},
		runReadService:             stubRunReadService{},
		replayReadService:          stubReplayReadService{},
		hostedRunIngestionService:  stubHostedRunIngestionService{},
		compareReadService:         stubCompareReadService{},
		agentDeploymentReadService: stubAgentDeploymentReadService{},
		challengePackReadService:   stubChallengePackReadService{},
		agentBuildService:          stubAgentBuildService{},
		releaseGateService:         noopReleaseGateService{},
		regressionService:          service,
	})

	postReq := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/regression-suites", bytes.NewBufferString(`{
		"source_challenge_pack_id":"`+sourceChallengePackID.String()+`",
		"name":"Critical regressions",
		"description":"Seed suite",
		"default_gate_severity":"warning"
	}`))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set(headerUserID, userID.String())
	postReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	postRec := httptest.NewRecorder()
	router.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201", postRec.Code)
	}

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/regression-suites/"+suiteID.String(), nil)
	getReq.Header.Set(headerUserID, userID.String())
	getReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET suite status = %d, want 200", getRec.Code)
	}

	patchRec := httptest.NewRecorder()
	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/workspaces/"+workspaceID.String()+"/regression-suites/"+suiteID.String(), bytes.NewBufferString(`{
		"description":"Updated suite",
		"status":"archived",
		"default_gate_severity":"blocking"
	}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set(headerUserID, userID.String())
	patchReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	router.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH suite status = %d, want 200", patchRec.Code)
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/regression-suites?limit=10&offset=0", nil)
	listReq.Header.Set(headerUserID, userID.String())
	listReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("LIST suites status = %d, want 200", listRec.Code)
	}

	var listResponse listRegressionSuitesResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("json.Unmarshal list response returned error: %v", err)
	}
	if len(listResponse.Items) != 1 || listResponse.Items[0].Status != domain.RegressionSuiteStatusArchived {
		t.Fatalf("list suites = %+v, want archived item", listResponse.Items)
	}

	casesRec := httptest.NewRecorder()
	casesReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/regression-suites/"+suiteID.String()+"/cases", nil)
	casesReq.Header.Set(headerUserID, userID.String())
	casesReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	router.ServeHTTP(casesRec, casesReq)
	if casesRec.Code != http.StatusOK {
		t.Fatalf("LIST cases status = %d, want 200", casesRec.Code)
	}

	casePatchRec := httptest.NewRecorder()
	casePatchReq := httptest.NewRequest(http.MethodPatch, "/v1/workspaces/"+workspaceID.String()+"/regression-cases/"+caseID.String(), bytes.NewBufferString(`{
		"title":"Muted case",
		"status":"muted",
		"severity":"warning"
	}`))
	casePatchReq.Header.Set("Content-Type", "application/json")
	casePatchReq.Header.Set(headerUserID, userID.String())
	casePatchReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	router.ServeHTTP(casePatchRec, casePatchReq)
	if casePatchRec.Code != http.StatusOK {
		t.Fatalf("PATCH case status = %d, want 200", casePatchRec.Code)
	}

	var patchedCase regressionCaseResponse
	if err := json.Unmarshal(casePatchRec.Body.Bytes(), &patchedCase); err != nil {
		t.Fatalf("json.Unmarshal patched case returned error: %v", err)
	}
	if patchedCase.Status != domain.RegressionCaseStatusMuted {
		t.Fatalf("patched case status = %s, want muted", patchedCase.Status)
	}
	if service.patchSuiteInput == nil || service.patchCaseInput == nil {
		t.Fatalf("expected patch inputs to be captured")
	}
}

func TestRegressionSuiteEndpointsRejectMalformedPagination(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	router := buildRouter(routerOptions{
		authMode:                   "dev",
		logger:                     testLogger(t),
		authenticator:              NewDevelopmentAuthenticator(),
		authorizer:                 NewCallerWorkspaceAuthorizer(),
		runCreationService:         stubRunCreationService{},
		runReadService:             stubRunReadService{},
		replayReadService:          stubReplayReadService{},
		hostedRunIngestionService:  stubHostedRunIngestionService{},
		compareReadService:         stubCompareReadService{},
		agentDeploymentReadService: stubAgentDeploymentReadService{},
		challengePackReadService:   stubChallengePackReadService{},
		agentBuildService:          stubAgentBuildService{},
		releaseGateService:         noopReleaseGateService{},
		regressionService:          &fakeRegressionService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/regression-suites?limit=abc&offset=-1", nil)
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed pagination status = %d, want 400", rec.Code)
	}
}

func TestRegressionSuitePatchReturnsConflictOnTransitionConflict(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	suiteID := uuid.New()
	router := buildRouter(routerOptions{
		authMode:                   "dev",
		logger:                     testLogger(t),
		authenticator:              NewDevelopmentAuthenticator(),
		authorizer:                 NewCallerWorkspaceAuthorizer(),
		runCreationService:         stubRunCreationService{},
		runReadService:             stubRunReadService{},
		replayReadService:          stubReplayReadService{},
		hostedRunIngestionService:  stubHostedRunIngestionService{},
		compareReadService:         stubCompareReadService{},
		agentDeploymentReadService: stubAgentDeploymentReadService{},
		challengePackReadService:   stubChallengePackReadService{},
		agentBuildService:          stubAgentBuildService{},
		releaseGateService:         noopReleaseGateService{},
		regressionService: &fakeRegressionService{
			patchSuiteErr: repository.TransitionConflictError{
				Entity:   "regression_suite",
				ID:       suiteID,
				Expected: "active",
			},
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/v1/workspaces/"+workspaceID.String()+"/regression-suites/"+suiteID.String(), bytes.NewBufferString(`{"status":"archived"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("transition conflict status = %d, want 409", rec.Code)
	}
}

type fakeRegressionRepository struct {
	challengePacks     []repository.ChallengePackSummary
	challengePacksErr  error
	suite             repository.RegressionSuite
	suiteErr          error
	regressionCase    repository.RegressionCase
	regressionCaseErr error
	listSuites        []repository.RegressionSuite
	listSuitesErr     error
	listCases         []repository.RegressionCase
	listCasesErr      error
	countSuites       int64
	countSuitesErr    error
	patchSuiteResult  repository.RegressionSuite
	patchSuiteErr     error
	patchCaseResult   repository.RegressionCase
	patchCaseErr      error
}

func (f *fakeRegressionRepository) ListVisibleChallengePacks(_ context.Context, _ uuid.UUID) ([]repository.ChallengePackSummary, error) {
	return f.challengePacks, f.challengePacksErr
}

func (f *fakeRegressionRepository) CreateRegressionSuite(_ context.Context, params repository.CreateRegressionSuiteParams) (repository.RegressionSuite, error) {
	return repository.RegressionSuite{
		ID:                    uuid.New(),
		WorkspaceID:           params.WorkspaceID,
		SourceChallengePackID: params.SourceChallengePackID,
		Name:                  params.Name,
		Description:           params.Description,
		Status:                params.Status,
		SourceMode:            params.SourceMode,
		DefaultGateSeverity:   params.DefaultGateSeverity,
		CreatedByUserID:       params.CreatedByUserID,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
	}, nil
}

func (f *fakeRegressionRepository) GetRegressionSuiteByID(_ context.Context, _ uuid.UUID) (repository.RegressionSuite, error) {
	if f.suiteErr != nil {
		return repository.RegressionSuite{}, f.suiteErr
	}
	return f.suite, nil
}

func (f *fakeRegressionRepository) ListRegressionSuitesByWorkspaceID(_ context.Context, _ uuid.UUID, _, _ int32) ([]repository.RegressionSuite, error) {
	return f.listSuites, f.listSuitesErr
}

func (f *fakeRegressionRepository) CountRegressionSuitesByWorkspaceID(_ context.Context, _ uuid.UUID) (int64, error) {
	return f.countSuites, f.countSuitesErr
}

func (f *fakeRegressionRepository) PatchRegressionSuite(_ context.Context, _ repository.PatchRegressionSuiteParams) (repository.RegressionSuite, error) {
	return f.patchSuiteResult, f.patchSuiteErr
}

func (f *fakeRegressionRepository) GetRegressionCaseByID(_ context.Context, _ uuid.UUID) (repository.RegressionCase, error) {
	if f.regressionCaseErr != nil {
		return repository.RegressionCase{}, f.regressionCaseErr
	}
	return f.regressionCase, nil
}

func (f *fakeRegressionRepository) ListRegressionCasesBySuiteID(_ context.Context, _ uuid.UUID) ([]repository.RegressionCase, error) {
	return f.listCases, f.listCasesErr
}

func (f *fakeRegressionRepository) PatchRegressionCase(_ context.Context, _ repository.PatchRegressionCaseParams) (repository.RegressionCase, error) {
	return f.patchCaseResult, f.patchCaseErr
}

type fakeRegressionService struct {
	suite           repository.RegressionSuite
	regressionCase  repository.RegressionCase
	createSuiteErr  error
	listSuitesErr   error
	getSuiteErr     error
	patchSuiteErr   error
	listCasesErr    error
	patchCaseErr    error
	patchSuiteInput *PatchRegressionSuiteInput
	patchCaseInput  *PatchRegressionCaseInput
}

func (f *fakeRegressionService) CreateRegressionSuite(_ context.Context, _ Caller, input CreateRegressionSuiteInput) (repository.RegressionSuite, error) {
	if f.createSuiteErr != nil {
		return repository.RegressionSuite{}, f.createSuiteErr
	}
	f.suite.WorkspaceID = input.WorkspaceID
	f.suite.SourceChallengePackID = input.SourceChallengePackID
	f.suite.Name = input.Name
	f.suite.Description = input.Description
	f.suite.DefaultGateSeverity = input.DefaultGateSeverity
	return f.suite, nil
}

func (f *fakeRegressionService) ListRegressionSuites(_ context.Context, _ Caller, input ListRegressionSuitesInput) (ListRegressionSuitesResult, error) {
	if f.listSuitesErr != nil {
		return ListRegressionSuitesResult{}, f.listSuitesErr
	}
	return ListRegressionSuitesResult{
		Items:  []repository.RegressionSuite{f.suite},
		Total:  1,
		Limit:  input.Limit,
		Offset: input.Offset,
	}, nil
}

func (f *fakeRegressionService) GetRegressionSuite(_ context.Context, _ Caller, _ GetRegressionSuiteInput) (repository.RegressionSuite, error) {
	if f.getSuiteErr != nil {
		return repository.RegressionSuite{}, f.getSuiteErr
	}
	return f.suite, nil
}

func (f *fakeRegressionService) PatchRegressionSuite(_ context.Context, _ Caller, input PatchRegressionSuiteInput) (repository.RegressionSuite, error) {
	if f.patchSuiteErr != nil {
		return repository.RegressionSuite{}, f.patchSuiteErr
	}
	f.patchSuiteInput = &input
	if input.Description != nil {
		f.suite.Description = *input.Description
	}
	if input.Status != nil {
		f.suite.Status = *input.Status
	}
	if input.DefaultGateSeverity != nil {
		f.suite.DefaultGateSeverity = *input.DefaultGateSeverity
	}
	return f.suite, nil
}

func (f *fakeRegressionService) ListRegressionCases(_ context.Context, _ Caller, _ ListRegressionCasesInput) ([]repository.RegressionCase, error) {
	if f.listCasesErr != nil {
		return nil, f.listCasesErr
	}
	return []repository.RegressionCase{f.regressionCase}, nil
}

func (f *fakeRegressionService) PatchRegressionCase(_ context.Context, _ Caller, input PatchRegressionCaseInput) (repository.RegressionCase, error) {
	if f.patchCaseErr != nil {
		return repository.RegressionCase{}, f.patchCaseErr
	}
	f.patchCaseInput = &input
	if input.Title != nil {
		f.regressionCase.Title = *input.Title
	}
	if input.Status != nil {
		f.regressionCase.Status = *input.Status
	}
	if input.Severity != nil {
		f.regressionCase.Severity = *input.Severity
	}
	return f.regressionCase, nil
}
