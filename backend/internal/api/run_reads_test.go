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
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/failurereview"
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
					CreatedAt:          time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC),
					UpdatedAt:          time.Date(2026, 3, 13, 12, 1, 0, 0, time.UTC),
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

type fakeRunReadRepository struct {
	run                     domain.Run
	evalSession             repository.EvalSessionWithRuns
	evalSessions            []domain.EvalSession
	evalSessionRuns         map[uuid.UUID][]domain.Run
	runScorecard            repository.RunScorecard
	regressionCoverageCases []repository.RunRegressionCoverageCase
	runAgents               []domain.RunAgent
	failureItems            []failurereview.Item
	spendPolicies           []repository.SpendPolicyRow
	providerAccount         repository.ProviderAccountRow
	modelAlias              repository.ModelAliasRow
	modelCatalogEntry       repository.ModelCatalogEntryRow
	workspaceSecrets        map[string]string
	getRunErr               error
	getEvalSessionErr       error
	listEvalSessionRunsErr  error
	getRunScorecardErr      error
	listRunAgentsErr        error
	listRunFailuresErr      error
	listEvalSessionsErr     error
	getProviderAccountErr   error
	getModelAliasErr        error
	getModelCatalogErr      error
	listSpendPoliciesErr    error
	loadWorkspaceSecretsErr error
}

func (f *fakeRunReadRepository) GetRunByID(_ context.Context, _ uuid.UUID) (domain.Run, error) {
	return f.run, f.getRunErr
}

func (f *fakeRunReadRepository) GetEvalSessionWithRuns(_ context.Context, _ uuid.UUID) (repository.EvalSessionWithRuns, error) {
	return f.evalSession, f.getEvalSessionErr
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

func (f *fakeRunReadRepository) ListRunFailureReviewItems(_ context.Context, _ uuid.UUID, _ *uuid.UUID) ([]failurereview.Item, error) {
	return f.failureItems, f.listRunFailuresErr
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

func (f *fakeRunReadRepository) GetModelAliasByID(_ context.Context, _ uuid.UUID) (repository.ModelAliasRow, error) {
	return f.modelAlias, f.getModelAliasErr
}

func (f *fakeRunReadRepository) GetModelCatalogEntryByID(_ context.Context, _ uuid.UUID) (repository.ModelCatalogEntryRow, error) {
	return f.modelCatalogEntry, f.getModelCatalogErr
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

func uuidPtr(value uuid.UUID) *uuid.UUID {
	return &value
}
