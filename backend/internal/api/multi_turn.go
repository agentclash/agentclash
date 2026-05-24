package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

const timeRFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"

type MultiTurnService interface {
	SubmitHumanTurn(ctx context.Context, caller Caller, workspaceID, runID, runAgentID uuid.UUID, message string) error
	GetHumanTurnStatus(ctx context.Context, caller Caller, workspaceID, runID, runAgentID uuid.UUID) (*repository.HumanTurnStatus, error)
	CreateCalibrationReview(ctx context.Context, caller Caller, workspaceID uuid.UUID, req CalibrationReviewRequest) error
	ListCalibrationReviews(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]CalibrationReviewResponse, error)
	ListArenaTasks(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]ArenaTaskResponse, error)
	SubmitArenaVote(ctx context.Context, caller Caller, workspaceID uuid.UUID, req ArenaVoteRequest) error
}

type CalibrationReviewRequest struct {
	RunAgentID uuid.UUID `json:"run_agent_id"`
	TurnIndex  int       `json:"turn_index"`
	Score      float64   `json:"score"`
	RubricKey  string    `json:"rubric_key,omitempty"`
	Notes      string    `json:"notes,omitempty"`
}

type CalibrationReviewResponse struct {
	ID           uuid.UUID `json:"id"`
	RunAgentID   uuid.UUID `json:"run_agent_id"`
	TurnIndex    int       `json:"turn_index"`
	Score        float64   `json:"score"`
	RubricKey    string    `json:"rubric_key,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	ReviewerID   uuid.UUID `json:"reviewer_user_id"`
	CreatedAtRFC string    `json:"created_at"`
}

type ArenaTaskResponse struct {
	ID                uuid.UUID `json:"id"`
	CaseKey           string    `json:"case_key"`
	LeftRunAgentID    uuid.UUID `json:"left_run_agent_id"`
	RightRunAgentID   uuid.UUID `json:"right_run_agent_id"`
	Status            string    `json:"status"`
}

type ArenaVoteRequest struct {
	TaskID            uuid.UUID          `json:"task_id"`
	WinnerRunAgentID  uuid.UUID          `json:"winner_run_agent_id"`
	RubricScores      map[string]float64 `json:"rubric_scores,omitempty"`
}

type submitHumanTurnRequest struct {
	Message string `json:"message"`
}

func submitMultiTurnHumanTurnHandler(logger *slog.Logger, service MultiTurnService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", err.Error())
			return
		}
		runID, err := runIDFromURLParam("runID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_id", err.Error())
			return
		}
		runAgentID, err := uuid.Parse(r.PathValue("runAgentID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_agent_id", err.Error())
			return
		}
		var body submitHumanTurnRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if err := service.SubmitHumanTurn(r.Context(), caller, workspaceID, runID, runAgentID, body.Message); err != nil {
			writeMultiTurnError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "submitted"})
	}
}

func getMultiTurnHumanTurnStatusHandler(logger *slog.Logger, service MultiTurnService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", err.Error())
			return
		}
		runID, err := runIDFromURLParam("runID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_id", err.Error())
			return
		}
		runAgentID, err := uuid.Parse(r.PathValue("runAgentID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_run_agent_id", err.Error())
			return
		}
		status, err := service.GetHumanTurnStatus(r.Context(), caller, workspaceID, runID, runAgentID)
		if err != nil {
			writeMultiTurnError(w, logger, r, err)
			return
		}
		if status == nil {
			writeJSON(w, http.StatusOK, map[string]any{"awaiting_human": false})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"awaiting_human": true,
			"turn_index":     status.TurnIndex,
			"phase_id":       status.PhaseID,
			"prompt_hint":    status.PromptHint,
		})
	}
}

type MultiTurnManager struct {
	authorizer WorkspaceAuthorizer
	repo       *repository.Repository
	humanTurns *repository.MultiTurnHumanTurnStore
}

func NewMultiTurnManager(authorizer WorkspaceAuthorizer, repo *repository.Repository, humanTurns *repository.MultiTurnHumanTurnStore) *MultiTurnManager {
	return &MultiTurnManager{authorizer: authorizer, repo: repo, humanTurns: humanTurns}
}

func (m *MultiTurnManager) SubmitHumanTurn(ctx context.Context, caller Caller, workspaceID, runID, runAgentID uuid.UUID, message string) error {
	if m.humanTurns == nil {
		return errors.New("multi_turn human turns are not configured")
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return err
	}
	run, err := m.repo.GetRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.WorkspaceID != workspaceID {
		return repository.ErrRunNotFound
	}
	runAgent, err := m.repo.GetRunAgentByID(ctx, runAgentID)
	if err != nil {
		return err
	}
	if runAgent.RunID != runID {
		return repository.ErrRunAgentNotFound
	}
	if runAgent.Status != domain.RunAgentStatusExecuting {
		return errors.New("run agent is not executing")
	}
	status, err := m.humanTurns.AwaitingTurn(ctx, runAgentID)
	if err != nil {
		return err
	}
	if status == nil {
		return repository.ErrHumanTurnNotAwaiting
	}
	return m.humanTurns.SubmitHumanMessage(ctx, runAgentID, status.TurnIndex, message)
}

func (m *MultiTurnManager) GetHumanTurnStatus(ctx context.Context, caller Caller, workspaceID, runID, runAgentID uuid.UUID) (*repository.HumanTurnStatus, error) {
	if m.humanTurns == nil {
		return nil, errors.New("multi_turn human turns are not configured")
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	run, err := m.repo.GetRunByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.WorkspaceID != workspaceID {
		return nil, repository.ErrRunNotFound
	}
	runAgent, err := m.repo.GetRunAgentByID(ctx, runAgentID)
	if err != nil {
		return nil, err
	}
	if runAgent.RunID != runID {
		return nil, repository.ErrRunAgentNotFound
	}
	return m.humanTurns.AwaitingTurn(ctx, runAgentID)
}

func (m *MultiTurnManager) CreateCalibrationReview(ctx context.Context, caller Caller, workspaceID uuid.UUID, req CalibrationReviewRequest) error {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return err
	}
	if req.Score < 1 || req.Score > 5 {
		return fmt.Errorf("calibration score must be between 1 and 5")
	}
	return m.repo.CreateCalibrationReview(ctx, repository.CreateCalibrationReviewParams{
		WorkspaceID:    workspaceID,
		RunAgentID:     req.RunAgentID,
		TurnIndex:      req.TurnIndex,
		ReviewerUserID: caller.UserID,
		Score:          req.Score,
		RubricKey:      req.RubricKey,
		Notes:          req.Notes,
	})
}

func (m *MultiTurnManager) ListCalibrationReviews(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]CalibrationReviewResponse, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	rows, err := m.repo.ListCalibrationReviews(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]CalibrationReviewResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, CalibrationReviewResponse{
			ID:           row.ID,
			RunAgentID:   row.RunAgentID,
			TurnIndex:    row.TurnIndex,
			Score:        row.Score,
			RubricKey:    row.RubricKey,
			Notes:        row.Notes,
			ReviewerID:   row.ReviewerUserID,
			CreatedAtRFC: row.CreatedAt.UTC().Format(timeRFC3339Nano),
		})
	}
	return out, nil
}

func (m *MultiTurnManager) ListArenaTasks(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]ArenaTaskResponse, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	rows, err := m.repo.ListPendingArenaTasks(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]ArenaTaskResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ArenaTaskResponse{
			ID:              row.ID,
			CaseKey:         row.CaseKey,
			LeftRunAgentID:  row.LeftRunAgentID,
			RightRunAgentID: row.RightRunAgentID,
			Status:          row.Status,
		})
	}
	return out, nil
}

func (m *MultiTurnManager) SubmitArenaVote(ctx context.Context, caller Caller, workspaceID uuid.UUID, req ArenaVoteRequest) error {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return err
	}
	return m.repo.SubmitArenaVote(ctx, repository.SubmitArenaVoteParams{
		WorkspaceID:      workspaceID,
		TaskID:           req.TaskID,
		VoterUserID:      caller.UserID,
		WinnerRunAgentID: req.WinnerRunAgentID,
		RubricScores:     req.RubricScores,
	})
}

func writeMultiTurnError(w http.ResponseWriter, logger *slog.Logger, r *http.Request, err error) {
	if errors.Is(err, ErrForbidden) {
		writeAuthzError(w, err)
		return
	}
	if errors.Is(err, repository.ErrRunNotFound) || errors.Is(err, repository.ErrRunAgentNotFound) {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if errors.Is(err, repository.ErrHumanTurnNotAwaiting) {
		writeError(w, http.StatusConflict, "not_awaiting_human", err.Error())
		return
	}
	logger.Error("multi_turn request failed",
		"method", r.Method,
		"path", r.URL.Path,
		"error", err,
	)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func createCalibrationReviewHandler(logger *slog.Logger, service MultiTurnService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", err.Error())
			return
		}
		var body CalibrationReviewRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if err := service.CreateCalibrationReview(r.Context(), caller, workspaceID, body); err != nil {
			writeMultiTurnError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
	}
}

func listCalibrationReviewsHandler(logger *slog.Logger, service MultiTurnService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", err.Error())
			return
		}
		reviews, err := service.ListCalibrationReviews(r.Context(), caller, workspaceID)
		if err != nil {
			writeMultiTurnError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": reviews})
	}
}

func listArenaTasksHandler(logger *slog.Logger, service MultiTurnService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", err.Error())
			return
		}
		tasks, err := service.ListArenaTasks(r.Context(), caller, workspaceID)
		if err != nil {
			writeMultiTurnError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": tasks})
	}
}

func submitArenaVoteHandler(logger *slog.Logger, service MultiTurnService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", err.Error())
			return
		}
		var body ArenaVoteRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		if err := service.SubmitArenaVote(r.Context(), caller, workspaceID, body); err != nil {
			writeMultiTurnError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "recorded"})
	}
}

type noopMultiTurnService struct{}

func (noopMultiTurnService) SubmitHumanTurn(context.Context, Caller, uuid.UUID, uuid.UUID, uuid.UUID, string) error {
	return errors.New("multi_turn service is not configured")
}

func (noopMultiTurnService) GetHumanTurnStatus(context.Context, Caller, uuid.UUID, uuid.UUID, uuid.UUID) (*repository.HumanTurnStatus, error) {
	return nil, errors.New("multi_turn service is not configured")
}

func (noopMultiTurnService) CreateCalibrationReview(context.Context, Caller, uuid.UUID, CalibrationReviewRequest) error {
	return errors.New("multi_turn service is not configured")
}

func (noopMultiTurnService) ListCalibrationReviews(context.Context, Caller, uuid.UUID) ([]CalibrationReviewResponse, error) {
	return nil, errors.New("multi_turn service is not configured")
}

func (noopMultiTurnService) ListArenaTasks(context.Context, Caller, uuid.UUID) ([]ArenaTaskResponse, error) {
	return nil, errors.New("multi_turn service is not configured")
}

func (noopMultiTurnService) SubmitArenaVote(context.Context, Caller, uuid.UUID, ArenaVoteRequest) error {
	return errors.New("multi_turn service is not configured")
}
