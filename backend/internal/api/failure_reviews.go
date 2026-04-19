package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/failurereview"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
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
	Run        domain.Run           `json:"-"`
	Items      []failurereview.Item `json:"items"`
	NextCursor *string              `json:"next_cursor,omitempty"`
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
		value, parseErr := parseFailureClass(raw)
		if parseErr != nil {
			return ListRunFailuresInput{}, parseErr
		}
		input.FailureClass = &value
	}
	if raw := strings.TrimSpace(query.Get("evidence_tier")); raw != "" {
		value, parseErr := parseEvidenceTier(raw)
		if parseErr != nil {
			return ListRunFailuresInput{}, parseErr
		}
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

func parseFailureClass(raw string) (failurereview.FailureClass, error) {
	trimmed := strings.TrimSpace(raw)
	switch failurereview.FailureClass(trimmed) {
	case failurereview.FailureClassIncorrectFinalOutput,
		failurereview.FailureClassToolSelectionError,
		failurereview.FailureClassToolArgumentError,
		failurereview.FailureClassRetrievalGrounding,
		failurereview.FailureClassPolicyViolation,
		failurereview.FailureClassTimeoutOrBudget,
		failurereview.FailureClassSandboxFailure,
		failurereview.FailureClassMalformedOutput,
		failurereview.FailureClassFlakyNonDeterministic,
		failurereview.FailureClassInsufficientEvidence,
		failurereview.FailureClassOther:
		return failurereview.FailureClass(trimmed), nil
	default:
		return "", errors.New("failure_class must be a valid failure review class")
	}
}

func parseEvidenceTier(raw string) (failurereview.EvidenceTier, error) {
	trimmed := strings.TrimSpace(raw)
	switch failurereview.EvidenceTier(trimmed) {
	case failurereview.EvidenceTierNone,
		failurereview.EvidenceTierNativeStructured,
		failurereview.EvidenceTierHostedStructured,
		failurereview.EvidenceTierHostedBlackBox,
		failurereview.EvidenceTierDerivedSummary:
		return failurereview.EvidenceTier(trimmed), nil
	default:
		return "", errors.New("evidence_tier must be one of none, native_structured, hosted_structured, hosted_black_box, derived_summary")
	}
}

func promoteFailureHandler(logger *slog.Logger, service RegressionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		input, err := promoteFailureInputFromRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		result, err := service.PromoteFailure(r.Context(), caller, input)
		if err != nil {
			handleRegressionError(w, logger, err)
			return
		}

		status := http.StatusCreated
		if !result.Created {
			status = http.StatusOK
		}
		writeJSON(w, status, buildRegressionCaseResponse(result.Case))
	}
}

func promoteFailureInputFromRequest(r *http.Request) (PromoteFailureInput, error) {
	workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
	if err != nil {
		return PromoteFailureInput{}, err
	}
	runID, err := runIDFromURLParam("runID")(r)
	if err != nil {
		return PromoteFailureInput{}, err
	}
	challengeIdentityID, err := challengeIdentityIDFromURLParam("challengeIdentityID")(r)
	if err != nil {
		return PromoteFailureInput{}, err
	}

	var req struct {
		RunAgentID         string          `json:"run_agent_id,omitempty"`
		SuiteID            uuid.UUID       `json:"suite_id"`
		PromotionMode      string          `json:"promotion_mode"`
		Title              string          `json:"title"`
		FailureSummary     string          `json:"failure_summary,omitempty"`
		Severity           *string         `json:"severity,omitempty"`
		ValidatorOverrides json.RawMessage `json:"validator_overrides,omitempty"`
		Metadata           json.RawMessage `json:"metadata,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		return PromoteFailureInput{}, errors.New("request body must be valid JSON")
	}
	if req.SuiteID == uuid.Nil {
		return PromoteFailureInput{}, errors.New("suite_id is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return PromoteFailureInput{}, errors.New("title is required")
	}

	var runAgentID *uuid.UUID
	if raw := strings.TrimSpace(req.RunAgentID); raw != "" {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return PromoteFailureInput{}, errors.New("run_agent_id must be a valid UUID")
		}
		runAgentID = &parsed
	}

	promotionMode, err := domain.ParseRegressionPromotionMode(req.PromotionMode)
	if err != nil {
		return PromoteFailureInput{}, errors.New("promotion_mode must be full_executable or output_only")
	}

	var severity *domain.RegressionSeverity
	if req.Severity != nil {
		parsed, parseErr := domain.ParseRegressionSeverity(strings.TrimSpace(*req.Severity))
		if parseErr != nil {
			return PromoteFailureInput{}, errors.New("severity must be info, warning, or blocking")
		}
		severity = &parsed
	}

	validatorOverrides, err := domain.ValidatePromotionOverrides(req.ValidatorOverrides)
	if err != nil {
		return PromoteFailureInput{}, err
	}
	metadata, err := normalizeOptionalJSONObject(req.Metadata, "metadata")
	if err != nil {
		return PromoteFailureInput{}, err
	}

	return PromoteFailureInput{
		WorkspaceID:         workspaceID,
		RunID:               runID,
		ChallengeIdentityID: challengeIdentityID,
		RunAgentID:          runAgentID,
		Request: domain.PromotionRequest{
			SuiteID:            req.SuiteID,
			PromotionMode:      promotionMode,
			Title:              strings.TrimSpace(req.Title),
			FailureSummary:     strings.TrimSpace(req.FailureSummary),
			Severity:           severity,
			ValidatorOverrides: validatorOverrides,
			Metadata:           metadata,
		},
	}, nil
}

func challengeIdentityIDFromURLParam(name string) func(*http.Request) (uuid.UUID, error) {
	return func(r *http.Request) (uuid.UUID, error) {
		raw := chi.URLParam(r, name)
		if raw == "" {
			return uuid.Nil, errors.New("challenge identity id is required")
		}
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, errors.New("challenge identity id is malformed")
		}
		return parsed, nil
	}
}

func normalizeOptionalJSONObject(raw json.RawMessage, fieldName string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, errors.New(fieldName + " must be a JSON object or null")
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}
