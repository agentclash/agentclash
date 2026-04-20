package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/budget"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/failurereview"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type RunReadRepository interface {
	GetRunByID(ctx context.Context, id uuid.UUID) (domain.Run, error)
	GetEvalSessionWithRuns(ctx context.Context, id uuid.UUID) (repository.EvalSessionWithRuns, error)
	GetEvalSessionResultBySessionID(ctx context.Context, evalSessionID uuid.UUID) (repository.EvalSessionAggregateRecord, error)
	ListRunsByEvalSessionID(ctx context.Context, evalSessionID uuid.UUID) ([]domain.Run, error)
	GetRunScorecardByRunID(ctx context.Context, runID uuid.UUID) (repository.RunScorecard, error)
	ListRunRegressionCoverageCasesByRunID(ctx context.Context, runID uuid.UUID) ([]repository.RunRegressionCoverageCase, error)
	ListRunAgentsByRunID(ctx context.Context, runID uuid.UUID) ([]domain.RunAgent, error)
	ListRunFailureReviewItems(ctx context.Context, runID uuid.UUID, agentID *uuid.UUID) ([]failurereview.Item, error)
	ListEvalSessionsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit int32, offset int32) ([]domain.EvalSession, error)
	ListRunsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit int32, offset int32) ([]domain.Run, error)
	CountRunsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (int64, error)
	GetProviderAccountByID(ctx context.Context, id uuid.UUID) (repository.ProviderAccountRow, error)
	GetModelAliasByID(ctx context.Context, id uuid.UUID) (repository.ModelAliasRow, error)
	GetModelCatalogEntryByID(ctx context.Context, id uuid.UUID) (repository.ModelCatalogEntryRow, error)
	ListSpendPoliciesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.SpendPolicyRow, error)
	LoadWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error)
}

type RunReadService interface {
	GetRun(ctx context.Context, caller Caller, runID uuid.UUID) (GetRunResult, error)
	GetEvalSession(ctx context.Context, caller Caller, evalSessionID uuid.UUID) (GetEvalSessionResult, error)
	GetRunRanking(ctx context.Context, caller Caller, runID uuid.UUID, input GetRunRankingInput) (GetRunRankingResult, error)
	GenerateRunRankingInsights(ctx context.Context, caller Caller, runID uuid.UUID, input GenerateRunRankingInsightsInput) (GenerateRunRankingInsightsResult, error)
	ListEvalSessions(ctx context.Context, caller Caller, input ListEvalSessionsInput) (ListEvalSessionsResult, error)
	ListRunAgents(ctx context.Context, caller Caller, runID uuid.UUID) (ListRunAgentsResult, error)
	ListRunFailures(ctx context.Context, caller Caller, input ListRunFailuresInput) (ListRunFailuresResult, error)
	ListRuns(ctx context.Context, caller Caller, input ListRunsInput) (ListRunsResult, error)
}

type ListRunsInput struct {
	WorkspaceID uuid.UUID
	Limit       int32
	Offset      int32
}

type ListRunsResult struct {
	Runs  []domain.Run
	Total int64
}

type GetRunResult struct {
	Run                domain.Run
	RegressionCoverage *RunRegressionCoverage
}

type RunRegressionCoverage struct {
	Suites         []RunRegressionCoverageSuite
	UnmatchedCases []RunRegressionCoverageCase
}

type RunRegressionCoverageSuite struct {
	ID        uuid.UUID
	Name      string
	CaseCount int
	PassCount int
	FailCount int
}

type RunRegressionCoverageCase struct {
	ID      uuid.UUID
	Title   string
	Outcome string
}

type ListRunAgentsResult struct {
	Run       domain.Run
	RunAgents []domain.RunAgent
}

type WorkspaceRateLimiter interface {
	Allow(workspaceID uuid.UUID, group string) (bool, time.Duration)
}

type RunReadManager struct {
	authorizer      WorkspaceAuthorizer
	repo            RunReadRepository
	insightsClient  provider.Client
	budgetChecker   budget.BudgetChecker
	insightsLimiter WorkspaceRateLimiter
	insightsTimeout time.Duration
	now             func() time.Time
}

const rankingInsightsTimeout = 45 * time.Second

func NewRunReadManager(authorizer WorkspaceAuthorizer, repo RunReadRepository) *RunReadManager {
	return &RunReadManager{
		authorizer:      authorizer,
		repo:            repo,
		budgetChecker:   budget.NoopChecker{},
		insightsTimeout: rankingInsightsTimeout,
		now:             time.Now,
	}
}

func (m *RunReadManager) WithInsightsClient(client provider.Client) *RunReadManager {
	m.insightsClient = client
	return m
}

func (m *RunReadManager) WithBudgetChecker(checker budget.BudgetChecker) *RunReadManager {
	if checker == nil {
		checker = budget.NoopChecker{}
	}
	m.budgetChecker = checker
	return m
}

func (m *RunReadManager) WithInsightsRateLimiter(limiter WorkspaceRateLimiter) *RunReadManager {
	m.insightsLimiter = limiter
	return m
}

func (m *RunReadManager) WithInsightsTimeout(timeout time.Duration) *RunReadManager {
	if timeout > 0 {
		m.insightsTimeout = timeout
	}
	return m
}

func (m *RunReadManager) InsightsConfigured() bool {
	return m.insightsClient != nil
}

func (m *RunReadManager) GetRun(ctx context.Context, caller Caller, runID uuid.UUID) (GetRunResult, error) {
	run, err := m.repo.GetRunByID(ctx, runID)
	if err != nil {
		return GetRunResult{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, run.WorkspaceID); err != nil {
		return GetRunResult{}, err
	}

	coverageCases, err := m.repo.ListRunRegressionCoverageCasesByRunID(ctx, run.ID)
	if err != nil {
		return GetRunResult{}, fmt.Errorf("list run regression coverage: %w", err)
	}

	coverage := buildRunRegressionCoverage(coverageCases)
	return GetRunResult{
		Run:                run,
		RegressionCoverage: &coverage,
	}, nil
}

func (m *RunReadManager) ListRuns(ctx context.Context, caller Caller, input ListRunsInput) (ListRunsResult, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, input.WorkspaceID); err != nil {
		return ListRunsResult{}, err
	}

	runs, err := m.repo.ListRunsByWorkspaceID(ctx, input.WorkspaceID, input.Limit, input.Offset)
	if err != nil {
		return ListRunsResult{}, fmt.Errorf("list runs: %w", err)
	}

	total, err := m.repo.CountRunsByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return ListRunsResult{}, fmt.Errorf("count runs: %w", err)
	}

	return ListRunsResult{
		Runs:  runs,
		Total: total,
	}, nil
}

func (m *RunReadManager) ListRunAgents(ctx context.Context, caller Caller, runID uuid.UUID) (ListRunAgentsResult, error) {
	run, err := m.repo.GetRunByID(ctx, runID)
	if err != nil {
		return ListRunAgentsResult{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, run.WorkspaceID); err != nil {
		return ListRunAgentsResult{}, err
	}

	runAgents, err := m.repo.ListRunAgentsByRunID(ctx, runID)
	if err != nil {
		return ListRunAgentsResult{}, fmt.Errorf("list run agents: %w", err)
	}

	return ListRunAgentsResult{
		Run:       run,
		RunAgents: runAgents,
	}, nil
}

type getRunResponse struct {
	ID                     uuid.UUID                      `json:"id"`
	WorkspaceID            uuid.UUID                      `json:"workspace_id"`
	ChallengePackVersionID uuid.UUID                      `json:"challenge_pack_version_id"`
	ChallengeInputSetID    *uuid.UUID                     `json:"challenge_input_set_id,omitempty"`
	OfficialPackMode       string                         `json:"official_pack_mode"`
	Name                   string                         `json:"name"`
	Status                 domain.RunStatus               `json:"status"`
	ExecutionMode          string                         `json:"execution_mode"`
	TemporalWorkflowID     *string                        `json:"temporal_workflow_id,omitempty"`
	TemporalRunID          *string                        `json:"temporal_run_id,omitempty"`
	QueuedAt               *time.Time                     `json:"queued_at,omitempty"`
	StartedAt              *time.Time                     `json:"started_at,omitempty"`
	FinishedAt             *time.Time                     `json:"finished_at,omitempty"`
	CancelledAt            *time.Time                     `json:"cancelled_at,omitempty"`
	FailedAt               *time.Time                     `json:"failed_at,omitempty"`
	CreatedAt              time.Time                      `json:"created_at"`
	UpdatedAt              time.Time                      `json:"updated_at"`
	RegressionCoverage     *runRegressionCoverageResponse `json:"regression_coverage,omitempty"`
	Links                  runLinksResponse               `json:"links"`
}

type runRegressionCoverageResponse struct {
	Suites         []runRegressionCoverageSuiteResponse `json:"suites"`
	UnmatchedCases []runRegressionCoverageCaseResponse  `json:"unmatched_cases"`
}

type runRegressionCoverageSuiteResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CaseCount int       `json:"case_count"`
	PassCount int       `json:"pass_count"`
	FailCount int       `json:"fail_count"`
}

type runRegressionCoverageCaseResponse struct {
	ID      uuid.UUID `json:"id"`
	Title   string    `json:"title"`
	Outcome string    `json:"outcome"`
}

type listRunAgentsResponse struct {
	Items []runAgentResponse `json:"items"`
}

type runAgentResponse struct {
	ID                        uuid.UUID             `json:"id"`
	RunID                     uuid.UUID             `json:"run_id"`
	LaneIndex                 int32                 `json:"lane_index"`
	Label                     string                `json:"label"`
	AgentDeploymentID         uuid.UUID             `json:"agent_deployment_id"`
	AgentDeploymentSnapshotID uuid.UUID             `json:"agent_deployment_snapshot_id"`
	Status                    domain.RunAgentStatus `json:"status"`
	QueuedAt                  *time.Time            `json:"queued_at,omitempty"`
	StartedAt                 *time.Time            `json:"started_at,omitempty"`
	FinishedAt                *time.Time            `json:"finished_at,omitempty"`
	FailureReason             *string               `json:"failure_reason,omitempty"`
	CreatedAt                 time.Time             `json:"created_at"`
	UpdatedAt                 time.Time             `json:"updated_at"`
}

func getRunHandler(logger *slog.Logger, service RunReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		runID, err := runIDFromURLParam("runID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_id", err.Error())
			return
		}

		result, err := service.GetRun(r.Context(), caller, runID)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrRunNotFound):
				writeError(w, http.StatusNotFound, "run_not_found", "run not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("get run request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"run_id", runID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		writeJSON(w, http.StatusOK, buildGetRunResponse(result.Run, result.RegressionCoverage))
	}
}

func listRunAgentsHandler(logger *slog.Logger, service RunReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		runID, err := runIDFromURLParam("runID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_id", err.Error())
			return
		}

		result, err := service.ListRunAgents(r.Context(), caller, runID)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrRunNotFound):
				writeError(w, http.StatusNotFound, "run_not_found", "run not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("list run agents request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"run_id", runID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		responseItems := make([]runAgentResponse, 0, len(result.RunAgents))
		for _, runAgent := range result.RunAgents {
			responseItems = append(responseItems, buildRunAgentResponse(runAgent))
		}

		writeJSON(w, http.StatusOK, listRunAgentsResponse{Items: responseItems})
	}
}

type listRunsResponse struct {
	Items  []getRunResponse `json:"items"`
	Total  int64            `json:"total"`
	Limit  int32            `json:"limit"`
	Offset int32            `json:"offset"`
}

func listRunsHandler(logger *slog.Logger, service RunReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		limit := int32(20)
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, parseErr := strconv.Atoi(raw)
			if parseErr == nil && parsed > 0 {
				limit = int32(parsed)
			}
		}
		if limit > 100 {
			limit = 100
		}

		offset := int32(0)
		if raw := r.URL.Query().Get("offset"); raw != "" {
			parsed, parseErr := strconv.Atoi(raw)
			if parseErr == nil && parsed >= 0 {
				offset = int32(parsed)
			}
		}

		result, err := service.ListRuns(r.Context(), caller, ListRunsInput{
			WorkspaceID: workspaceID,
			Limit:       limit,
			Offset:      offset,
		})
		if err != nil {
			switch {
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("list runs request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"workspace_id", workspaceID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		responseItems := make([]getRunResponse, 0, len(result.Runs))
		for _, run := range result.Runs {
			responseItems = append(responseItems, buildGetRunResponse(run, nil))
		}

		writeJSON(w, http.StatusOK, listRunsResponse{
			Items:  responseItems,
			Total:  result.Total,
			Limit:  limit,
			Offset: offset,
		})
	}
}

func buildGetRunResponse(run domain.Run, regressionCoverage *RunRegressionCoverage) getRunResponse {
	response := getRunResponse{
		ID:                     run.ID,
		WorkspaceID:            run.WorkspaceID,
		ChallengePackVersionID: run.ChallengePackVersionID,
		ChallengeInputSetID:    run.ChallengeInputSetID,
		OfficialPackMode:       string(run.OfficialPackMode),
		Name:                   run.Name,
		Status:                 run.Status,
		ExecutionMode:          run.ExecutionMode,
		TemporalWorkflowID:     run.TemporalWorkflowID,
		TemporalRunID:          run.TemporalRunID,
		QueuedAt:               run.QueuedAt,
		StartedAt:              run.StartedAt,
		FinishedAt:             run.FinishedAt,
		CancelledAt:            run.CancelledAt,
		FailedAt:               run.FailedAt,
		CreatedAt:              run.CreatedAt,
		UpdatedAt:              run.UpdatedAt,
		Links:                  buildRunLinks(run.ID),
	}
	if regressionCoverage != nil {
		response.RegressionCoverage = buildRunRegressionCoverageResponse(*regressionCoverage)
	}
	return response
}

func buildRunAgentResponse(runAgent domain.RunAgent) runAgentResponse {
	return runAgentResponse{
		ID:                        runAgent.ID,
		RunID:                     runAgent.RunID,
		LaneIndex:                 runAgent.LaneIndex,
		Label:                     runAgent.Label,
		AgentDeploymentID:         runAgent.AgentDeploymentID,
		AgentDeploymentSnapshotID: runAgent.AgentDeploymentSnapshotID,
		Status:                    runAgent.Status,
		QueuedAt:                  runAgent.QueuedAt,
		StartedAt:                 runAgent.StartedAt,
		FinishedAt:                runAgent.FinishedAt,
		FailureReason:             runAgent.FailureReason,
		CreatedAt:                 runAgent.CreatedAt,
		UpdatedAt:                 runAgent.UpdatedAt,
	}
}

func buildRunRegressionCoverage(rows []repository.RunRegressionCoverageCase) RunRegressionCoverage {
	suitesByID := make(map[uuid.UUID]*RunRegressionCoverageSuite)
	suiteOrder := make([]uuid.UUID, 0)
	unmatchedCases := make([]RunRegressionCoverageCase, 0)

	for _, row := range rows {
		title := ""
		if row.RegressionCaseTitle != nil {
			title = *row.RegressionCaseTitle
		}

		if row.SuiteID == nil || row.SuiteName == nil {
			unmatchedCases = append(unmatchedCases, RunRegressionCoverageCase{
				ID:      row.RegressionCaseID,
				Title:   title,
				Outcome: string(row.Outcome),
			})
			continue
		}

		suite, ok := suitesByID[*row.SuiteID]
		if !ok {
			suite = &RunRegressionCoverageSuite{
				ID:   *row.SuiteID,
				Name: *row.SuiteName,
			}
			suitesByID[*row.SuiteID] = suite
			suiteOrder = append(suiteOrder, *row.SuiteID)
		}
		suite.CaseCount++
		switch row.Outcome {
		case repository.RunRegressionCoverageOutcomePass:
			suite.PassCount++
		case repository.RunRegressionCoverageOutcomeFail:
			suite.FailCount++
		}
	}

	suites := make([]RunRegressionCoverageSuite, 0, len(suiteOrder))
	for _, suiteID := range suiteOrder {
		suites = append(suites, *suitesByID[suiteID])
	}

	return RunRegressionCoverage{
		Suites:         suites,
		UnmatchedCases: unmatchedCases,
	}
}

func buildRunRegressionCoverageResponse(coverage RunRegressionCoverage) *runRegressionCoverageResponse {
	response := &runRegressionCoverageResponse{
		Suites:         make([]runRegressionCoverageSuiteResponse, 0, len(coverage.Suites)),
		UnmatchedCases: make([]runRegressionCoverageCaseResponse, 0, len(coverage.UnmatchedCases)),
	}
	for _, suite := range coverage.Suites {
		response.Suites = append(response.Suites, runRegressionCoverageSuiteResponse{
			ID:        suite.ID,
			Name:      suite.Name,
			CaseCount: suite.CaseCount,
			PassCount: suite.PassCount,
			FailCount: suite.FailCount,
		})
	}
	for _, unmatchedCase := range coverage.UnmatchedCases {
		response.UnmatchedCases = append(response.UnmatchedCases, runRegressionCoverageCaseResponse{
			ID:      unmatchedCase.ID,
			Title:   unmatchedCase.Title,
			Outcome: unmatchedCase.Outcome,
		})
	}
	return response
}

func runIDFromURLParam(name string) func(*http.Request) (uuid.UUID, error) {
	return func(r *http.Request) (uuid.UUID, error) {
		raw := chi.URLParam(r, name)
		if raw == "" {
			return uuid.Nil, errors.New("run id is required")
		}

		runID, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, errors.New("run id must be a valid UUID")
		}

		return runID, nil
	}
}
