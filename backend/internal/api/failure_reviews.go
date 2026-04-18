package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/failurereview"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

const (
	defaultRunFailurePageLimit = 50
	maxRunFailurePageLimit     = 200
)

type ListRunFailuresInput struct {
	WorkspaceID  uuid.UUID
	RunID        uuid.UUID
	AgentID      *uuid.UUID
	Severity     *failurereview.Severity
	FailureClass *failurereview.FailureClass
	EvidenceTier *failurereview.EvidenceTier
	ChallengeKey *string
	CaseKey      *string
	Cursor       *failurereview.CursorKey
	Limit        int
}

type ListRunFailuresResult struct {
	Run        domain.Run                `json:"-"`
	Items      []failurereview.Item      `json:"items"`
	NextCursor *string                   `json:"next_cursor,omitempty"`
}

func (m *RunReadManager) ListRunFailures(ctx context.Context, caller Caller, input ListRunFailuresInput) (ListRunFailuresResult, error) {
	run, err := m.repo.GetRunByID(ctx, input.RunID)
	if err != nil {
		return ListRunFailuresResult{}, err
	}
	if run.WorkspaceID != input.WorkspaceID {
		return ListRunFailuresResult{}, repository.ErrRunNotFound
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, input.WorkspaceID); err != nil {
		return ListRunFailuresResult{}, err
	}

	items, err := m.repo.ListRunFailureReviewItems(ctx, input.RunID, input.AgentID)
	if err != nil {
		return ListRunFailuresResult{}, err
	}
	filtered := failurereview.FilterItems(items, input.AgentID, input.Severity, input.FailureClass, input.EvidenceTier, input.ChallengeKey, input.CaseKey)
	page, next := failurereview.PaginateItems(filtered, input.Cursor, input.Limit)

	var nextCursor *string
	if next != nil {
		encoded, encodeErr := failurereview.EncodeCursor(*next)
		if encodeErr != nil {
			return ListRunFailuresResult{}, encodeErr
		}
		nextCursor = &encoded
	}

	return ListRunFailuresResult{
		Run:        run,
		Items:      page,
		NextCursor: nextCursor,
	}, nil
}

type listRunFailuresResponse struct {
	Items      []failurereview.Item `json:"items"`
	NextCursor *string              `json:"next_cursor,omitempty"`
}

func listRunFailuresHandler(logger *slog.Logger, service RunReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		input, err := listRunFailuresInputFromRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_failure_review_request", err.Error())
			return
		}

		result, err := service.ListRunFailures(r.Context(), caller, input)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrRunNotFound):
				writeError(w, http.StatusNotFound, "run_not_found", "run not found")
			case errors.Is(err, ErrForbidden):
				writeAuthzError(w, err)
			default:
				logger.Error("list run failures request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"run_id", input.RunID,
					"workspace_id", input.WorkspaceID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		writeJSON(w, http.StatusOK, listRunFailuresResponse{
			Items:      result.Items,
			NextCursor: result.NextCursor,
		})
	}
}

func listRunFailuresInputFromRequest(r *http.Request) (ListRunFailuresInput, error) {
	workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
	if err != nil {
		return ListRunFailuresInput{}, err
	}
	runID, err := runIDFromURLParam("runID")(r)
	if err != nil {
		return ListRunFailuresInput{}, err
	}

	query := r.URL.Query()
	input := ListRunFailuresInput{
		WorkspaceID: workspaceID,
		RunID:       runID,
		Limit:       defaultRunFailurePageLimit,
	}

	if raw := strings.TrimSpace(query.Get("agent_id")); raw != "" {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return ListRunFailuresInput{}, parseErr
		}
		input.AgentID = &parsed
	}
	if raw := strings.TrimSpace(query.Get("severity")); raw != "" {
		value, parseErr := parseFailureSeverity(raw)
		if parseErr != nil {
			return ListRunFailuresInput{}, parseErr
		}
		input.Severity = &value
	}
	if raw := strings.TrimSpace(query.Get("failure_class")); raw != "" {
		value := failurereview.FailureClass(raw)
		input.FailureClass = &value
	}
	if raw := strings.TrimSpace(query.Get("evidence_tier")); raw != "" {
		value := failurereview.EvidenceTier(raw)
		input.EvidenceTier = &value
	}
	if raw := strings.TrimSpace(query.Get("challenge_key")); raw != "" {
		input.ChallengeKey = &raw
	}
	if raw := strings.TrimSpace(query.Get("case_key")); raw != "" {
		input.CaseKey = &raw
	}
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			return ListRunFailuresInput{}, parseErr
		}
		if parsed <= 0 {
			return ListRunFailuresInput{}, errors.New("limit must be greater than zero")
		}
		if parsed > maxRunFailurePageLimit {
			parsed = maxRunFailurePageLimit
		}
		input.Limit = parsed
	}
	if raw := strings.TrimSpace(query.Get("cursor")); raw != "" {
		cursor, parseErr := failurereview.DecodeCursor(raw)
		if parseErr != nil {
			return ListRunFailuresInput{}, parseErr
		}
		input.Cursor = &cursor
	}

	return input, nil
}

func parseFailureSeverity(raw string) (failurereview.Severity, error) {
	switch failurereview.Severity(strings.TrimSpace(raw)) {
	case failurereview.SeverityInfo, failurereview.SeverityWarning, failurereview.SeverityBlocking:
		return failurereview.Severity(strings.TrimSpace(raw)), nil
	default:
		return "", errors.New("severity must be one of info, warning, blocking")
	}
}
