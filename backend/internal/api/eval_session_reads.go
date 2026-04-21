package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ListEvalSessionsInput struct {
	WorkspaceID uuid.UUID
	Limit       int32
	Offset      int32
}

type EvalSessionSummary struct {
	RunCounts EvalSessionRunCounts
}

type EvalSessionRunCounts struct {
	Total        int `json:"total"`
	Draft        int `json:"draft"`
	Queued       int `json:"queued"`
	Provisioning int `json:"provisioning"`
	Running      int `json:"running"`
	Scoring      int `json:"scoring"`
	Completed    int `json:"completed"`
	Failed       int `json:"failed"`
	Cancelled    int `json:"cancelled"`
}

type GetEvalSessionResult struct {
	Session          domain.EvalSession
	Runs             []domain.Run
	Summary          EvalSessionSummary
	AggregateResult  json.RawMessage
	EvidenceWarnings []string
}

type ListEvalSessionsResult struct {
	Items []GetEvalSessionResult
}

type evalSessionChildRunResponse struct {
	ID                     uuid.UUID        `json:"id"`
	WorkspaceID            uuid.UUID        `json:"workspace_id"`
	ChallengePackVersionID uuid.UUID        `json:"challenge_pack_version_id"`
	ChallengeInputSetID    *uuid.UUID       `json:"challenge_input_set_id,omitempty"`
	EvalSessionID          *uuid.UUID       `json:"eval_session_id,omitempty"`
	OfficialPackMode       string           `json:"official_pack_mode"`
	Name                   string           `json:"name"`
	Status                 domain.RunStatus `json:"status"`
	ExecutionMode          string           `json:"execution_mode"`
	QueuedAt               *time.Time       `json:"queued_at,omitempty"`
	StartedAt              *time.Time       `json:"started_at,omitempty"`
	FinishedAt             *time.Time       `json:"finished_at,omitempty"`
	CancelledAt            *time.Time       `json:"cancelled_at,omitempty"`
	FailedAt               *time.Time       `json:"failed_at,omitempty"`
	CreatedAt              time.Time        `json:"created_at"`
	UpdatedAt              time.Time        `json:"updated_at"`
	Links                  runLinksResponse `json:"links"`
}

type evalSessionSummaryResponse struct {
	RunCounts EvalSessionRunCounts `json:"run_counts"`
}

type getEvalSessionResponse struct {
	EvalSession      evalSessionResponse           `json:"eval_session"`
	Runs             []evalSessionChildRunResponse `json:"runs"`
	Summary          evalSessionSummaryResponse    `json:"summary"`
	AggregateResult  json.RawMessage               `json:"aggregate_result"`
	EvidenceWarnings []string                      `json:"evidence_warnings"`
}

type evalSessionListItemResponse struct {
	EvalSession      evalSessionResponse        `json:"eval_session"`
	Summary          evalSessionSummaryResponse `json:"summary"`
	AggregateResult  json.RawMessage            `json:"aggregate_result"`
	EvidenceWarnings []string                   `json:"evidence_warnings"`
}

type listEvalSessionsResponse struct {
	Items  []evalSessionListItemResponse `json:"items"`
	Limit  int32                         `json:"limit"`
	Offset int32                         `json:"offset"`
}

func (m *RunReadManager) GetEvalSession(ctx context.Context, caller Caller, evalSessionID uuid.UUID) (GetEvalSessionResult, error) {
	value, err := m.repo.GetEvalSessionWithRuns(ctx, evalSessionID)
	if err != nil {
		return GetEvalSessionResult{}, err
	}

	if len(value.Runs) > 0 {
		workspaceID, err := evalSessionWorkspaceID(value.Runs)
		if err != nil {
			return GetEvalSessionResult{}, err
		}
		if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
			return GetEvalSessionResult{}, err
		}
	}

	aggregateResult, err := loadEvalSessionAggregateResult(ctx, m.repo, evalSessionID)
	if err != nil {
		return GetEvalSessionResult{}, err
	}

	return buildEvalSessionReadModel(value, aggregateResult)
}

func (m *RunReadManager) ListEvalSessions(ctx context.Context, caller Caller, input ListEvalSessionsInput) (ListEvalSessionsResult, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, input.WorkspaceID); err != nil {
		return ListEvalSessionsResult{}, err
	}

	sessions, err := m.repo.ListEvalSessionsByWorkspaceID(ctx, input.WorkspaceID, input.Limit, input.Offset)
	if err != nil {
		return ListEvalSessionsResult{}, fmt.Errorf("list eval sessions: %w", err)
	}

	items := make([]GetEvalSessionResult, 0, len(sessions))
	for _, session := range sessions {
		runs, runErr := m.repo.ListRunsByEvalSessionID(ctx, session.ID)
		if runErr != nil {
			return ListEvalSessionsResult{}, fmt.Errorf("load runs for eval session %s: %w", session.ID, runErr)
		}
		aggregateResult, aggregateErr := loadEvalSessionAggregateResult(ctx, m.repo, session.ID)
		if aggregateErr != nil {
			return ListEvalSessionsResult{}, fmt.Errorf("load aggregate for eval session %s: %w", session.ID, aggregateErr)
		}
		item, buildErr := buildEvalSessionReadModel(repository.EvalSessionWithRuns{
			Session: session,
			Runs:    runs,
		}, aggregateResult)
		if buildErr != nil {
			return ListEvalSessionsResult{}, fmt.Errorf("build eval session read model for %s: %w", session.ID, buildErr)
		}
		items = append(items, item)
	}

	return ListEvalSessionsResult{Items: items}, nil
}

func buildEvalSessionReadModel(value repository.EvalSessionWithRuns, aggregateResult *repository.EvalSessionAggregateRecord) (GetEvalSessionResult, error) {
	aggregateJSON, warnings, err := resolveEvalSessionEvidence(value.Session, value.Runs, aggregateResult)
	if err != nil {
		return GetEvalSessionResult{}, err
	}
	return GetEvalSessionResult{
		Session:          value.Session,
		Runs:             append([]domain.Run(nil), value.Runs...),
		Summary:          EvalSessionSummary{RunCounts: summarizeEvalSessionRuns(value.Runs)},
		AggregateResult:  aggregateJSON,
		EvidenceWarnings: warnings,
	}, nil
}

func summarizeEvalSessionRuns(runs []domain.Run) EvalSessionRunCounts {
	summary := EvalSessionRunCounts{Total: len(runs)}
	for _, run := range runs {
		switch run.Status {
		case domain.RunStatusDraft:
			summary.Draft++
		case domain.RunStatusQueued:
			summary.Queued++
		case domain.RunStatusProvisioning:
			summary.Provisioning++
		case domain.RunStatusRunning:
			summary.Running++
		case domain.RunStatusScoring:
			summary.Scoring++
		case domain.RunStatusCompleted:
			summary.Completed++
		case domain.RunStatusFailed:
			summary.Failed++
		case domain.RunStatusCancelled:
			summary.Cancelled++
		}
	}
	return summary
}

func buildEvalSessionEvidenceWarnings(session domain.EvalSession, runs []domain.Run) []string {
	warnings := make([]string, 0, 3)
	if len(runs) == 0 {
		warnings = append(warnings, "no child runs are attached to this eval session")
	} else if len(runs) != int(session.Repetitions) {
		warnings = append(warnings, fmt.Sprintf("expected %d child runs but found %d", session.Repetitions, len(runs)))
	}

	switch session.Status {
	case domain.EvalSessionStatusQueued, domain.EvalSessionStatusRunning:
		warnings = append(warnings, "aggregate result unavailable: eval session has not reached aggregation yet")
	default:
		warnings = append(warnings, "aggregate result unavailable: session-level aggregation has not been persisted yet")
	}

	if len(runs) > 1 {
		workspaceID := runs[0].WorkspaceID
		for _, run := range runs[1:] {
			if run.WorkspaceID != workspaceID {
				warnings = append(warnings, "child runs span multiple workspaces; investigate persisted eval session attachments")
				break
			}
		}
	}

	sort.Strings(warnings)
	return warnings
}

type evalSessionAggregateEvidence struct {
	Warnings []string `json:"warnings"`
}

func loadEvalSessionAggregateResult(ctx context.Context, repo RunReadRepository, evalSessionID uuid.UUID) (*repository.EvalSessionAggregateRecord, error) {
	aggregateResult, err := repo.GetEvalSessionResultBySessionID(ctx, evalSessionID)
	if err != nil {
		if errors.Is(err, repository.ErrEvalSessionResultNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get eval session result by session id: %w", err)
	}
	return &aggregateResult, nil
}

func resolveEvalSessionEvidence(session domain.EvalSession, runs []domain.Run, aggregateResult *repository.EvalSessionAggregateRecord) (json.RawMessage, []string, error) {
	if aggregateResult == nil {
		return nil, buildEvalSessionEvidenceWarnings(session, runs), nil
	}

	var evidence evalSessionAggregateEvidence
	if len(aggregateResult.Evidence) > 0 {
		if err := json.Unmarshal(aggregateResult.Evidence, &evidence); err != nil {
			return nil, nil, fmt.Errorf("decode eval session aggregate evidence: %w", err)
		}
	}

	warnings := append([]string(nil), evidence.Warnings...)
	sort.Strings(warnings)
	return append(json.RawMessage(nil), aggregateResult.Aggregate...), warnings, nil
}

func evalSessionWorkspaceID(runs []domain.Run) (uuid.UUID, error) {
	if len(runs) == 0 {
		return uuid.Nil, fmt.Errorf("eval session has no child runs")
	}

	workspaceID := runs[0].WorkspaceID
	for _, run := range runs[1:] {
		if run.WorkspaceID != workspaceID {
			return uuid.Nil, fmt.Errorf("eval session child runs span multiple workspaces")
		}
	}
	return workspaceID, nil
}

func getEvalSessionHandler(logger *slog.Logger, service RunReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		evalSessionID, err := uuid.Parse(chi.URLParam(r, "evalSessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_eval_session_id", "evalSessionID must be a valid UUID")
			return
		}

		result, err := service.GetEvalSession(r.Context(), caller, evalSessionID)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrEvalSessionNotFound):
				writeError(w, http.StatusNotFound, "eval_session_not_found", "eval session not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("get eval session request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"eval_session_id", evalSessionID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		writeJSON(w, http.StatusOK, buildGetEvalSessionResponse(result))
	}
}

func listEvalSessionsHandler(logger *slog.Logger, service RunReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		workspaceID, err := parseRequiredUUIDQueryParam(r, "workspace_id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", err.Error())
			return
		}

		limit := int32(20)
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
				limit = int32(parsed)
			}
		}
		if limit > 100 {
			limit = 100
		}

		offset := int32(0)
		if raw := r.URL.Query().Get("offset"); raw != "" {
			if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed >= 0 {
				offset = int32(parsed)
			}
		}

		result, err := service.ListEvalSessions(r.Context(), caller, ListEvalSessionsInput{
			WorkspaceID: workspaceID,
			Limit:       limit,
			Offset:      offset,
		})
		if err != nil {
			switch {
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("list eval sessions request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"workspace_id", workspaceID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		items := make([]evalSessionListItemResponse, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, buildEvalSessionListItemResponse(item))
		}

		writeJSON(w, http.StatusOK, listEvalSessionsResponse{
			Items:  items,
			Limit:  limit,
			Offset: offset,
		})
	}
}

func buildGetEvalSessionResponse(result GetEvalSessionResult) getEvalSessionResponse {
	runs := make([]evalSessionChildRunResponse, 0, len(result.Runs))
	for _, run := range result.Runs {
		runs = append(runs, buildEvalSessionChildRunResponse(run))
	}

	return getEvalSessionResponse{
		EvalSession:      buildEvalSessionResponse(result.Session),
		Runs:             runs,
		Summary:          evalSessionSummaryResponse{RunCounts: result.Summary.RunCounts},
		AggregateResult:  result.AggregateResult,
		EvidenceWarnings: append([]string(nil), result.EvidenceWarnings...),
	}
}

func buildEvalSessionListItemResponse(result GetEvalSessionResult) evalSessionListItemResponse {
	return evalSessionListItemResponse{
		EvalSession:      buildEvalSessionResponse(result.Session),
		Summary:          evalSessionSummaryResponse{RunCounts: result.Summary.RunCounts},
		AggregateResult:  result.AggregateResult,
		EvidenceWarnings: append([]string(nil), result.EvidenceWarnings...),
	}
}

func buildEvalSessionChildRunResponse(run domain.Run) evalSessionChildRunResponse {
	return evalSessionChildRunResponse{
		ID:                     run.ID,
		WorkspaceID:            run.WorkspaceID,
		ChallengePackVersionID: run.ChallengePackVersionID,
		ChallengeInputSetID:    run.ChallengeInputSetID,
		EvalSessionID:          run.EvalSessionID,
		OfficialPackMode:       string(run.OfficialPackMode),
		Name:                   run.Name,
		Status:                 run.Status,
		ExecutionMode:          run.ExecutionMode,
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
