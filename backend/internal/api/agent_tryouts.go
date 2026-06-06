package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	maxAgentTryoutRequestBytes = 512 * 1024
	defaultAgentTryoutTTL      = 24 * time.Hour
)

var (
	ErrAgentTryoutTemplateNotFound = errors.New("agent tryout template not found")
	ErrInvalidAgentTryoutInput     = errors.New("invalid agent tryout input")
)

type AgentTryoutRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	CreateAgentTryout(ctx context.Context, params repository.CreateAgentTryoutParams) (repository.AgentTryout, error)
	GetAgentTryoutByID(ctx context.Context, id uuid.UUID) (repository.AgentTryout, error)
	ListAgentTryoutsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error)
	ClaimAgentTryout(ctx context.Context, params repository.ClaimAgentTryoutParams) (repository.AgentTryout, error)
	CreatePublicShareLink(ctx context.Context, params repository.CreatePublicShareLinkParams) (repository.PublicShareLink, error)
}

type AgentTryoutService interface {
	ListTemplates(ctx context.Context) ([]AgentTryoutTemplate, error)
	CreateAnonymousTryout(ctx context.Context, input CreateAnonymousAgentTryoutInput) (repository.AgentTryout, error)
	CreateWorkspaceTryout(ctx context.Context, caller Caller, input CreateWorkspaceAgentTryoutInput) (repository.AgentTryout, error)
	GetPublicTryout(ctx context.Context, id uuid.UUID) (repository.AgentTryout, error)
	GetWorkspaceTryout(ctx context.Context, caller Caller, id uuid.UUID) (repository.AgentTryout, error)
	ListWorkspaceTryouts(ctx context.Context, caller Caller, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error)
	ClaimTryout(ctx context.Context, caller Caller, input ClaimAgentTryoutInput) (repository.AgentTryout, error)
	CreatePrivateShare(ctx context.Context, caller Caller, id uuid.UUID) (CreateAgentTryoutShareResult, error)
}

type AgentTryoutTemplate struct {
	Slug               string          `json:"slug"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	InputSchema        json.RawMessage `json:"input_schema"`
	ToolPolicy         json.RawMessage `json:"tool_policy"`
	EvaluationSpec     json.RawMessage `json:"evaluation_spec"`
	DefaultModelPolicy json.RawMessage `json:"default_model_policy"`
	AnonymousEnabled   bool            `json:"anonymous_enabled"`
	MaxInputBytes      int64           `json:"max_input_bytes"`
	MaxDurationSeconds int32           `json:"max_duration_seconds"`
	MaxCostUSD         float64         `json:"max_cost_usd"`
}

type CreateAnonymousAgentTryoutInput struct {
	TemplateSlug         string
	Input                json.RawMessage
	AnonymousFingerprint string
	Now                  time.Time
}

type CreateWorkspaceAgentTryoutInput struct {
	WorkspaceID  uuid.UUID
	TemplateSlug string
	Input        json.RawMessage
	Now          time.Time
}

type ClaimAgentTryoutInput struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Now         time.Time
}

type CreateAgentTryoutShareResult struct {
	Share repository.PublicShareLink
	Token string
}

type AgentTryoutManager struct {
	authorizer WorkspaceAuthorizer
	repo       AgentTryoutRepository
	now        func() time.Time
	templates  map[string]AgentTryoutTemplate
}

func NewAgentTryoutManager(authorizer WorkspaceAuthorizer, repo AgentTryoutRepository) *AgentTryoutManager {
	templates := builtinAgentTryoutTemplates()
	bySlug := make(map[string]AgentTryoutTemplate, len(templates))
	for _, template := range templates {
		bySlug[template.Slug] = template
	}
	return &AgentTryoutManager{
		authorizer: authorizer,
		repo:       repo,
		now:        time.Now,
		templates:  bySlug,
	}
}

func (m *AgentTryoutManager) ListTemplates(context.Context) ([]AgentTryoutTemplate, error) {
	templates := builtinAgentTryoutTemplates()
	return templates, nil
}

func (m *AgentTryoutManager) CreateAnonymousTryout(ctx context.Context, input CreateAnonymousAgentTryoutInput) (repository.AgentTryout, error) {
	template, err := m.lookupTemplate(input.TemplateSlug)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if !template.AnonymousEnabled {
		return repository.AgentTryout{}, fmt.Errorf("%w: template does not allow anonymous tryouts", ErrInvalidAgentTryoutInput)
	}
	if err := validateAgentTryoutInput(template, input.Input); err != nil {
		return repository.AgentTryout{}, err
	}
	now := input.Now
	if now.IsZero() {
		now = m.now()
	}
	expiresAt := now.UTC().Add(defaultAgentTryoutTTL)
	fingerprintHash := hashAnonymousFingerprint(input.AnonymousFingerprint)
	return m.repo.CreateAgentTryout(ctx, repository.CreateAgentTryoutParams{
		TemplateSlug:             template.Slug,
		Status:                   repository.AgentTryoutStatusQueued,
		InputSnapshot:            input.Input,
		TemplateSnapshot:         templateSnapshot(template),
		ToolPolicySnapshot:       template.ToolPolicy,
		EvaluationSpecSnapshot:   template.EvaluationSpec,
		SelectedModelPolicy:      template.DefaultModelPolicy,
		Summary:                  json.RawMessage(`{}`),
		RedactionStatus:          repository.AgentTryoutRedactionPending,
		CostLimitUSD:             template.MaxCostUSD,
		MaxDurationSeconds:       template.MaxDurationSeconds,
		AnonymousFingerprintHash: &fingerprintHash,
		ExpiresAt:                &expiresAt,
	})
}

func (m *AgentTryoutManager) CreateWorkspaceTryout(ctx context.Context, caller Caller, input CreateWorkspaceAgentTryoutInput) (repository.AgentTryout, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return repository.AgentTryout{}, err
	}
	template, err := m.lookupTemplate(input.TemplateSlug)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if err := validateAgentTryoutInput(template, input.Input); err != nil {
		return repository.AgentTryout{}, err
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	callerID := caller.UserID
	return m.repo.CreateAgentTryout(ctx, repository.CreateAgentTryoutParams{
		OrganizationID:         &orgID,
		WorkspaceID:            &input.WorkspaceID,
		TemplateSlug:           template.Slug,
		Status:                 repository.AgentTryoutStatusQueued,
		InputSnapshot:          input.Input,
		TemplateSnapshot:       templateSnapshot(template),
		ToolPolicySnapshot:     template.ToolPolicy,
		EvaluationSpecSnapshot: template.EvaluationSpec,
		SelectedModelPolicy:    template.DefaultModelPolicy,
		Summary:                json.RawMessage(`{}`),
		RedactionStatus:        repository.AgentTryoutRedactionPending,
		CostLimitUSD:           template.MaxCostUSD,
		MaxDurationSeconds:     template.MaxDurationSeconds,
		CreatedByUserID:        &callerID,
	})
}

func (m *AgentTryoutManager) GetPublicTryout(ctx context.Context, id uuid.UUID) (repository.AgentTryout, error) {
	tryout, err := m.repo.GetAgentTryoutByID(ctx, id)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if tryout.WorkspaceID != nil {
		return repository.AgentTryout{}, repository.ErrAgentTryoutNotFound
	}
	return tryout, nil
}

func (m *AgentTryoutManager) GetWorkspaceTryout(ctx context.Context, caller Caller, id uuid.UUID) (repository.AgentTryout, error) {
	tryout, err := m.repo.GetAgentTryoutByID(ctx, id)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	if tryout.WorkspaceID == nil {
		return repository.AgentTryout{}, repository.ErrAgentTryoutNotFound
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, *tryout.WorkspaceID); err != nil {
		return repository.AgentTryout{}, err
	}
	return tryout, nil
}

func (m *AgentTryoutManager) ListWorkspaceTryouts(ctx context.Context, caller Caller, workspaceID uuid.UUID, limit, offset int32) ([]repository.AgentTryout, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionReadWorkspace); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return m.repo.ListAgentTryoutsByWorkspaceID(ctx, workspaceID, limit, offset)
}

func (m *AgentTryoutManager) ClaimTryout(ctx context.Context, caller Caller, input ClaimAgentTryoutInput) (repository.AgentTryout, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManagePlaygrounds); err != nil {
		return repository.AgentTryout{}, err
	}
	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return repository.AgentTryout{}, err
	}
	now := input.Now
	if now.IsZero() {
		now = m.now()
	}
	return m.repo.ClaimAgentTryout(ctx, repository.ClaimAgentTryoutParams{
		ID:              input.ID,
		OrganizationID:  orgID,
		WorkspaceID:     input.WorkspaceID,
		ClaimedByUserID: caller.UserID,
		ClaimedAt:       now,
	})
}

func (m *AgentTryoutManager) CreatePrivateShare(ctx context.Context, caller Caller, id uuid.UUID) (CreateAgentTryoutShareResult, error) {
	tryout, err := m.GetWorkspaceTryout(ctx, caller, id)
	if err != nil {
		return CreateAgentTryoutShareResult{}, err
	}
	if tryout.OrganizationID == nil || tryout.WorkspaceID == nil {
		return CreateAgentTryoutShareResult{}, repository.ErrAgentTryoutNotFound
	}
	key, err := newShareKey()
	if err != nil {
		return CreateAgentTryoutShareResult{}, err
	}
	callerID := caller.UserID
	share, err := m.repo.CreatePublicShareLink(ctx, repository.CreatePublicShareLinkParams{
		Key:             key,
		OrganizationID:  *tryout.OrganizationID,
		WorkspaceID:     *tryout.WorkspaceID,
		ResourceType:    repository.PublicShareResourceAgentTryout,
		ResourceID:      tryout.ID,
		CreatedByUserID: &callerID,
		SearchIndexing:  false,
	})
	if err != nil {
		return CreateAgentTryoutShareResult{}, err
	}
	return CreateAgentTryoutShareResult{Share: share, Token: share.Key}, nil
}

func (m *AgentTryoutManager) lookupTemplate(slug string) (AgentTryoutTemplate, error) {
	template, ok := m.templates[strings.TrimSpace(slug)]
	if !ok {
		return AgentTryoutTemplate{}, ErrAgentTryoutTemplateNotFound
	}
	return template, nil
}

type createAgentTryoutRequest struct {
	TemplateSlug string          `json:"template_slug"`
	Input        json.RawMessage `json:"input"`
}

type claimAgentTryoutRequest struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
}

type listAgentTryoutTemplatesResponse struct {
	Items []AgentTryoutTemplate `json:"items"`
}

type listAgentTryoutsResponse struct {
	Items []agentTryoutResponse `json:"items"`
}

type agentTryoutResponse struct {
	ID                     uuid.UUID                             `json:"id"`
	OrganizationID         *uuid.UUID                            `json:"organization_id,omitempty"`
	WorkspaceID            *uuid.UUID                            `json:"workspace_id,omitempty"`
	TemplateSlug           string                                `json:"template_slug"`
	Status                 repository.AgentTryoutStatus          `json:"status"`
	InputSnapshot          json.RawMessage                       `json:"input_snapshot"`
	TemplateSnapshot       json.RawMessage                       `json:"template_snapshot"`
	ToolPolicySnapshot     json.RawMessage                       `json:"tool_policy_snapshot"`
	EvaluationSpecSnapshot json.RawMessage                       `json:"evaluation_spec_snapshot"`
	SelectedModelPolicy    json.RawMessage                       `json:"selected_model_policy"`
	Summary                json.RawMessage                       `json:"summary"`
	RedactionStatus        repository.AgentTryoutRedactionStatus `json:"redaction_status"`
	RunID                  *uuid.UUID                            `json:"run_id,omitempty"`
	CostLimitUSD           float64                               `json:"cost_limit_usd"`
	ActualCostUSD          *float64                              `json:"actual_cost_usd,omitempty"`
	LatencyMS              *int64                                `json:"latency_ms,omitempty"`
	MaxDurationSeconds     int32                                 `json:"max_duration_seconds"`
	CreatedByUserID        *uuid.UUID                            `json:"created_by_user_id,omitempty"`
	ClaimedByUserID        *uuid.UUID                            `json:"claimed_by_user_id,omitempty"`
	ClaimedAt              *time.Time                            `json:"claimed_at,omitempty"`
	ExpiresAt              *time.Time                            `json:"expires_at,omitempty"`
	CreatedAt              time.Time                             `json:"created_at"`
	UpdatedAt              time.Time                             `json:"updated_at"`
}

type publicAgentTryoutResponse struct {
	ID                     uuid.UUID                             `json:"id"`
	TemplateSlug           string                                `json:"template_slug"`
	Status                 repository.AgentTryoutStatus          `json:"status"`
	InputSnapshot          json.RawMessage                       `json:"input_snapshot"`
	TemplateSnapshot       json.RawMessage                       `json:"template_snapshot"`
	ToolPolicySnapshot     json.RawMessage                       `json:"tool_policy_snapshot"`
	EvaluationSpecSnapshot json.RawMessage                       `json:"evaluation_spec_snapshot"`
	SelectedModelPolicy    json.RawMessage                       `json:"selected_model_policy"`
	Summary                json.RawMessage                       `json:"summary"`
	RedactionStatus        repository.AgentTryoutRedactionStatus `json:"redaction_status"`
	RunID                  *uuid.UUID                            `json:"run_id,omitempty"`
	CostLimitUSD           float64                               `json:"cost_limit_usd"`
	ActualCostUSD          *float64                              `json:"actual_cost_usd,omitempty"`
	LatencyMS              *int64                                `json:"latency_ms,omitempty"`
	MaxDurationSeconds     int32                                 `json:"max_duration_seconds"`
	CreatedAt              time.Time                             `json:"created_at"`
	UpdatedAt              time.Time                             `json:"updated_at"`
}

type agentTryoutShareResponse struct {
	Share publicShareLinkResponse `json:"share"`
	Token string                  `json:"token"`
}

func registerPublicAgentTryoutRoutes(router chi.Router, logger *slog.Logger, service AgentTryoutService) {
	router.Get("/agent-tryout-templates", listAgentTryoutTemplatesHandler(logger, service))
	router.Post("/agent-tryouts", createAnonymousAgentTryoutHandler(logger, service))
	router.Get("/agent-tryouts/{tryoutID}", getPublicAgentTryoutHandler(logger, service))
}

func registerProtectedAgentTryoutRoutes(router chi.Router, logger *slog.Logger, service AgentTryoutService) {
	router.Post("/workspaces/{workspaceID}/agent-tryouts", createWorkspaceAgentTryoutHandler(logger, service))
	router.Get("/workspaces/{workspaceID}/agent-tryouts", listWorkspaceAgentTryoutsHandler(logger, service))
	router.Get("/workspaces/{workspaceID}/agent-tryouts/{tryoutID}", getWorkspaceAgentTryoutHandler(logger, service))
	router.Post("/agent-tryouts/{tryoutID}/claim", claimAgentTryoutHandler(logger, service))
	router.Post("/agent-tryouts/{tryoutID}/share", createAgentTryoutShareHandler(logger, service))
}

func listAgentTryoutTemplatesHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := service.ListTemplates(r.Context())
		if err != nil {
			logger.Error("list agent tryout templates failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, listAgentTryoutTemplatesResponse{Items: items})
	}
}

func createAnonymousAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createAgentTryoutRequest
		if err := decodeAgentTryoutJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		tryout, err := service.CreateAnonymousTryout(r.Context(), CreateAnonymousAgentTryoutInput{
			TemplateSlug:         req.TemplateSlug,
			Input:                req.Input,
			AnonymousFingerprint: anonymousFingerprintFromRequest(r),
		})
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapPublicAgentTryoutResponse(tryout))
	}
}

func getPublicAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		tryout, err := service.GetPublicTryout(r.Context(), id)
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, mapPublicAgentTryoutResponse(tryout))
	}
}

func createWorkspaceAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace_id must be a UUID")
			return
		}
		var req createAgentTryoutRequest
		if err := decodeAgentTryoutJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		tryout, err := service.CreateWorkspaceTryout(r.Context(), caller, CreateWorkspaceAgentTryoutInput{
			WorkspaceID:  workspaceID,
			TemplateSlug: req.TemplateSlug,
			Input:        req.Input,
		})
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapAgentTryoutResponse(tryout))
	}
}

func listWorkspaceAgentTryoutsHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace_id must be a UUID")
			return
		}
		limit, offset := parseAgentTryoutPagination(r)
		items, err := service.ListWorkspaceTryouts(r.Context(), caller, workspaceID, limit, offset)
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		response := listAgentTryoutsResponse{Items: make([]agentTryoutResponse, 0, len(items))}
		for _, item := range items {
			response.Items = append(response.Items, mapAgentTryoutResponse(item))
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func getWorkspaceAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace_id must be a UUID")
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		tryout, err := service.GetWorkspaceTryout(r.Context(), caller, id)
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		if tryout.WorkspaceID == nil || *tryout.WorkspaceID != workspaceID {
			writeError(w, http.StatusNotFound, "agent_tryout_not_found", "agent tryout not found")
			return
		}
		writeJSON(w, http.StatusOK, mapAgentTryoutResponse(tryout))
	}
}

func parseAgentTryoutPagination(r *http.Request) (int32, int32) {
	limit := int32(50)
	offset := int32(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 32); err == nil {
			limit = int32(value)
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 32); err == nil {
			offset = int32(value)
		}
	}
	return limit, offset
}

func claimAgentTryoutHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		var req claimAgentTryoutRequest
		if err := decodeAgentTryoutJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		tryout, err := service.ClaimTryout(r.Context(), caller, ClaimAgentTryoutInput{ID: id, WorkspaceID: req.WorkspaceID})
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAgentTryoutResponse(tryout))
	}
}

func createAgentTryoutShareHandler(logger *slog.Logger, service AgentTryoutService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "tryoutID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tryout_id", "tryout_id must be a UUID")
			return
		}
		result, err := service.CreatePrivateShare(r.Context(), caller, id)
		if err != nil {
			writeAgentTryoutError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, agentTryoutShareResponse{
			Share: mapPublicShareLink(result.Share, ""),
			Token: result.Token,
		})
	}
}

func validateAgentTryoutInput(template AgentTryoutTemplate, raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("%w: input is required", ErrInvalidAgentTryoutInput)
	}
	if int64(len(raw)) > template.MaxInputBytes {
		return fmt.Errorf("%w: input exceeds %d bytes", ErrInvalidAgentTryoutInput, template.MaxInputBytes)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return fmt.Errorf("%w: input must be a JSON object", ErrInvalidAgentTryoutInput)
	}
	if object == nil {
		return fmt.Errorf("%w: input must be a JSON object", ErrInvalidAgentTryoutInput)
	}
	return nil
}

func decodeAgentTryoutJSON(r *http.Request, dest any) error {
	defer r.Body.Close()
	reader := http.MaxBytesReader(nil, r.Body, maxAgentTryoutRequestBytes)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		if errors.As(err, new(*http.MaxBytesError)) {
			return fmt.Errorf("request body exceeds %d bytes", maxAgentTryoutRequestBytes)
		}
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain a single JSON object")
		}
		return err
	}
	return nil
}

func writeAgentTryoutError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, ErrAgentTryoutTemplateNotFound):
		writeError(w, http.StatusNotFound, "template_not_found", "agent tryout template not found")
	case errors.Is(err, repository.ErrAgentTryoutNotFound):
		writeError(w, http.StatusNotFound, "agent_tryout_not_found", "agent tryout not found")
	case errors.Is(err, repository.ErrAgentTryoutAlreadyClaimed):
		writeError(w, http.StatusConflict, "agent_tryout_already_claimed", "agent tryout is already claimed")
	case errors.Is(err, ErrInvalidAgentTryoutInput):
		writeError(w, http.StatusBadRequest, "invalid_agent_tryout_input", err.Error())
	case errors.Is(err, ErrForbidden):
		writeAuthzError(w, err)
	default:
		logger.Error("agent tryout request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func mapAgentTryoutResponse(tryout repository.AgentTryout) agentTryoutResponse {
	return agentTryoutResponse{
		ID:                     tryout.ID,
		OrganizationID:         tryout.OrganizationID,
		WorkspaceID:            tryout.WorkspaceID,
		TemplateSlug:           tryout.TemplateSlug,
		Status:                 tryout.Status,
		InputSnapshot:          tryout.InputSnapshot,
		TemplateSnapshot:       tryout.TemplateSnapshot,
		ToolPolicySnapshot:     tryout.ToolPolicySnapshot,
		EvaluationSpecSnapshot: tryout.EvaluationSpecSnapshot,
		SelectedModelPolicy:    tryout.SelectedModelPolicy,
		Summary:                tryout.Summary,
		RedactionStatus:        tryout.RedactionStatus,
		RunID:                  tryout.RunID,
		CostLimitUSD:           tryout.CostLimitUSD,
		ActualCostUSD:          tryout.ActualCostUSD,
		LatencyMS:              tryout.LatencyMS,
		MaxDurationSeconds:     tryout.MaxDurationSeconds,
		CreatedByUserID:        tryout.CreatedByUserID,
		ClaimedByUserID:        tryout.ClaimedByUserID,
		ClaimedAt:              tryout.ClaimedAt,
		ExpiresAt:              tryout.ExpiresAt,
		CreatedAt:              tryout.CreatedAt,
		UpdatedAt:              tryout.UpdatedAt,
	}
}

func mapPublicAgentTryoutResponse(tryout repository.AgentTryout) publicAgentTryoutResponse {
	return publicAgentTryoutResponse{
		ID:                     tryout.ID,
		TemplateSlug:           tryout.TemplateSlug,
		Status:                 tryout.Status,
		InputSnapshot:          tryout.InputSnapshot,
		TemplateSnapshot:       tryout.TemplateSnapshot,
		ToolPolicySnapshot:     tryout.ToolPolicySnapshot,
		EvaluationSpecSnapshot: tryout.EvaluationSpecSnapshot,
		SelectedModelPolicy:    tryout.SelectedModelPolicy,
		Summary:                tryout.Summary,
		RedactionStatus:        tryout.RedactionStatus,
		RunID:                  tryout.RunID,
		CostLimitUSD:           tryout.CostLimitUSD,
		ActualCostUSD:          tryout.ActualCostUSD,
		LatencyMS:              tryout.LatencyMS,
		MaxDurationSeconds:     tryout.MaxDurationSeconds,
		CreatedAt:              tryout.CreatedAt,
		UpdatedAt:              tryout.UpdatedAt,
	}
}

func templateSnapshot(template AgentTryoutTemplate) json.RawMessage {
	payload, _ := json.Marshal(map[string]any{
		"slug":                 template.Slug,
		"name":                 template.Name,
		"description":          template.Description,
		"anonymous_enabled":    template.AnonymousEnabled,
		"max_input_bytes":      template.MaxInputBytes,
		"max_duration_seconds": template.MaxDurationSeconds,
		"max_cost_usd":         template.MaxCostUSD,
	})
	return payload
}

func anonymousFingerprintFromRequest(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			value = strings.TrimSpace(strings.Split(value, ",")[0])
		}
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func hashAnonymousFingerprint(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func builtinAgentTryoutTemplates() []AgentTryoutTemplate {
	return []AgentTryoutTemplate{
		{
			Slug:               "meeting-minutes",
			Name:               "Meeting Minutes to Action Plan",
			Description:        "Turn notes or a transcript into minutes, decisions, risks, and action items.",
			InputSchema:        json.RawMessage(`{"type":"object","required":["notes"],"properties":{"notes":{"type":"string"},"audience":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["file_writer"],"network":"disabled","external_side_effects":false}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"has_action_items","type":"jsonpath","path":"$.action_items"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      64 * 1024,
			MaxDurationSeconds: 120,
			MaxCostUSD:         0.25,
		},
		{
			Slug:               "structured-data",
			Name:               "Extract Structured Data",
			Description:        "Extract rows from messy text into JSON or CSV and validate the shape.",
			InputSchema:        json.RawMessage(`{"type":"object","required":["text"],"properties":{"text":{"type":"string"},"schema":{"type":"object"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["schema_validator","file_writer"],"network":"disabled","external_side_effects":false}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"valid_json","type":"json_schema"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   true,
			MaxInputBytes:      64 * 1024,
			MaxDurationSeconds: 120,
			MaxCostUSD:         0.25,
		},
		{
			Slug:               "tiny-bugfix",
			Name:               "Fix a Tiny Bug",
			Description:        "Run an agent against a small fixture, inspect the diff, and see whether tests pass.",
			InputSchema:        json.RawMessage(`{"type":"object","required":["task"],"properties":{"task":{"type":"string"},"fixture":{"type":"string"}}}`),
			ToolPolicy:         json.RawMessage(`{"tools":["sandbox_shell","file_editor"],"network":"disabled","external_side_effects":false}`),
			EvaluationSpec:     json.RawMessage(`{"validators":[{"key":"tests_pass","type":"command_exit_code"}],"scorecard":{"dimensions":["correctness","reliability","latency","cost"]}}`),
			DefaultModelPolicy: json.RawMessage(`{"mode":"hosted_default","max_models":1}`),
			AnonymousEnabled:   false,
			MaxInputBytes:      32 * 1024,
			MaxDurationSeconds: 300,
			MaxCostUSD:         0.75,
		},
	}
}
