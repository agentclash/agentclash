package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/failurereview"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type RunReadRepository interface {
	GetRunByID(ctx context.Context, id uuid.UUID) (domain.Run, error)
	GetRunScorecardByRunID(ctx context.Context, runID uuid.UUID) (repository.RunScorecard, error)
	ListRunAgentsByRunID(ctx context.Context, runID uuid.UUID) ([]domain.RunAgent, error)
	ListRunFailureReviewItems(ctx context.Context, runID uuid.UUID, agentID *uuid.UUID) ([]failurereview.Item, error)
	ListRunsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit int32, offset int32) ([]domain.Run, error)
	CountRunsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (int64, error)
}

type RunReadService interface {
	GetRun(ctx context.Context, caller Caller, runID uuid.UUID) (GetRunResult, error)
	GetRunRanking(ctx context.Context, caller Caller, runID uuid.UUID, input GetRunRankingInput) (GetRunRankingResult, error)
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
	Run domain.Run
}

type ListRunAgentsResult struct {
	Run       domain.Run
	RunAgents []domain.RunAgent
}

type RunReadManager struct {
	authorizer WorkspaceAuthorizer
	repo       RunReadRepository
}

func NewRunReadManager(authorizer WorkspaceAuthorizer, repo RunReadRepository) *RunReadManager {
	return &RunReadManager{
		authorizer: authorizer,
		repo:       repo,
	}
}

func (m *RunReadManager) GetRun(ctx context.Context, caller Caller, runID uuid.UUID) (GetRunResult, error) {
	run, err := m.repo.GetRunByID(ctx, runID)
	if err != nil {
		return GetRunResult{}, err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, run.WorkspaceID); err != nil {
		return GetRunResult{}, err
	}

	return GetRunResult{Run: run}, nil
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
	ID                     uuid.UUID        `json:"id"`
	WorkspaceID            uuid.UUID        `json:"workspace_id"`
	ChallengePackVersionID uuid.UUID        `json:"challenge_pack_version_id"`
	ChallengeInputSetID    *uuid.UUID       `json:"challenge_input_set_id,omitempty"`
	Name                   string           `json:"name"`
	Status                 domain.RunStatus `json:"status"`
	ExecutionMode          string           `json:"execution_mode"`
	TemporalWorkflowID     *string          `json:"temporal_workflow_id,omitempty"`
	TemporalRunID          *string          `json:"temporal_run_id,omitempty"`
	QueuedAt               *time.Time       `json:"queued_at,omitempty"`
	StartedAt              *time.Time       `json:"started_at,omitempty"`
	FinishedAt             *time.Time       `json:"finished_at,omitempty"`
	CancelledAt            *time.Time       `json:"cancelled_at,omitempty"`
	FailedAt               *time.Time       `json:"failed_at,omitempty"`
	CreatedAt              time.Time        `json:"created_at"`
	UpdatedAt              time.Time        `json:"updated_at"`
	Links                  runLinksResponse `json:"links"`
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

		writeJSON(w, http.StatusOK, buildGetRunResponse(result.Run))
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
			responseItems = append(responseItems, buildGetRunResponse(run))
		}

		writeJSON(w, http.StatusOK, listRunsResponse{
			Items:  responseItems,
			Total:  result.Total,
			Limit:  limit,
			Offset: offset,
		})
	}
}

func buildGetRunResponse(run domain.Run) getRunResponse {
	return getRunResponse{
		ID:                     run.ID,
		WorkspaceID:            run.WorkspaceID,
		ChallengePackVersionID: run.ChallengePackVersionID,
		ChallengeInputSetID:    run.ChallengeInputSetID,
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
