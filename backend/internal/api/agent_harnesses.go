package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	AgentHarnessAuthModeChatGPTDevice   = "chatgpt_device"
	AgentHarnessAuthModeAPIKeySecret    = "api_key_secret"
	AgentHarnessAuthModeBringYourOwnEnv = "bring_your_own_env"
	defaultCodexE2BTemplate             = "codex"
)

type AgentHarnessRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	CreateAgentHarness(ctx context.Context, p repository.CreateAgentHarnessParams) (repository.AgentHarness, error)
	GetAgentHarnessByID(ctx context.Context, id uuid.UUID) (repository.AgentHarness, error)
	ListAgentHarnessesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.AgentHarness, error)
}

type AgentHarnessService interface {
	CreateAgentHarness(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateAgentHarnessInput) (repository.AgentHarness, error)
	GetAgentHarness(ctx context.Context, caller Caller, workspaceID uuid.UUID, id uuid.UUID) (repository.AgentHarness, error)
	ListAgentHarnesses(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.AgentHarness, error)
}

type AgentHarnessManager struct {
	authorizer WorkspaceAuthorizer
	repo       AgentHarnessRepository
}

func NewAgentHarnessManager(authorizer WorkspaceAuthorizer, repo AgentHarnessRepository) *AgentHarnessManager {
	return &AgentHarnessManager{authorizer: authorizer, repo: repo}
}

type CreateAgentHarnessInput struct {
	Name                   string          `json:"name"`
	Description            string          `json:"description"`
	TaskPrompt             string          `json:"task_prompt"`
	CodexTemplate          string          `json:"codex_template"`
	CodexModel             string          `json:"codex_model"`
	AuthMode               string          `json:"auth_mode"`
	OpenAIAPIKeySecretName string          `json:"openai_api_key_secret_name"`
	E2BAPIKeySecretName    string          `json:"e2b_api_key_secret_name"`
	RepositoryURL          string          `json:"repository_url"`
	BaseBranch             string          `json:"base_branch"`
	ExecutionConfig        json.RawMessage `json:"execution_config"`
	EvaluationConfig       json.RawMessage `json:"evaluation_config"`
}

type AgentHarnessValidationError struct {
	Code    string
	Message string
}

func (e AgentHarnessValidationError) Error() string {
	return e.Message
}

func (m *AgentHarnessManager) CreateAgentHarness(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateAgentHarnessInput) (repository.AgentHarness, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return repository.AgentHarness{}, err
	}
	if err := validateAgentHarnessInput(input); err != nil {
		return repository.AgentHarness{}, err
	}

	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return repository.AgentHarness{}, err
	}

	codexTemplate := strings.TrimSpace(input.CodexTemplate)
	if codexTemplate == "" {
		codexTemplate = defaultCodexE2BTemplate
	}

	return m.repo.CreateAgentHarness(ctx, repository.CreateAgentHarnessParams{
		OrganizationID:         orgID,
		WorkspaceID:            workspaceID,
		CreatedByUserID:        &caller.UserID,
		Name:                   strings.TrimSpace(input.Name),
		Slug:                   generateSlug(input.Name),
		Description:            strings.TrimSpace(input.Description),
		TaskPrompt:             strings.TrimSpace(input.TaskPrompt),
		CodexTemplate:          codexTemplate,
		CodexModel:             optionalHarnessString(input.CodexModel),
		AuthMode:               strings.TrimSpace(input.AuthMode),
		OpenAIAPIKeySecretName: optionalHarnessString(input.OpenAIAPIKeySecretName),
		E2BAPIKeySecretName:    optionalHarnessString(input.E2BAPIKeySecretName),
		RepositoryURL:          optionalHarnessString(input.RepositoryURL),
		BaseBranch:             optionalHarnessString(input.BaseBranch),
		ExecutionConfig:        defaultJSON(input.ExecutionConfig),
		EvaluationConfig:       defaultJSON(input.EvaluationConfig),
	})
}

func (m *AgentHarnessManager) GetAgentHarness(ctx context.Context, caller Caller, workspaceID uuid.UUID, id uuid.UUID) (repository.AgentHarness, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return repository.AgentHarness{}, err
	}
	harness, err := m.repo.GetAgentHarnessByID(ctx, id)
	if err != nil {
		return repository.AgentHarness{}, err
	}
	if harness.WorkspaceID != workspaceID {
		return repository.AgentHarness{}, repository.ErrAgentHarnessNotFound
	}
	return harness, nil
}

func (m *AgentHarnessManager) ListAgentHarnesses(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.AgentHarness, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	return m.repo.ListAgentHarnessesByWorkspaceID(ctx, workspaceID)
}

func validateAgentHarnessInput(input CreateAgentHarnessInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return AgentHarnessValidationError{Code: "invalid_name", Message: "name is required"}
	}
	if strings.TrimSpace(input.TaskPrompt) == "" {
		return AgentHarnessValidationError{Code: "invalid_task_prompt", Message: "task_prompt is required"}
	}
	switch strings.TrimSpace(input.AuthMode) {
	case AgentHarnessAuthModeChatGPTDevice, AgentHarnessAuthModeAPIKeySecret, AgentHarnessAuthModeBringYourOwnEnv:
	case "":
		return AgentHarnessValidationError{Code: "invalid_auth_mode", Message: "auth_mode is required"}
	default:
		return AgentHarnessValidationError{Code: "invalid_auth_mode", Message: "auth_mode must be one of chatgpt_device, api_key_secret, bring_your_own_env"}
	}
	if strings.TrimSpace(input.AuthMode) == AgentHarnessAuthModeAPIKeySecret && strings.TrimSpace(input.OpenAIAPIKeySecretName) == "" {
		return AgentHarnessValidationError{Code: "missing_openai_secret", Message: "openai_api_key_secret_name is required when auth_mode is api_key_secret"}
	}
	for field, raw := range map[string]json.RawMessage{
		"execution_config":  input.ExecutionConfig,
		"evaluation_config": input.EvaluationConfig,
	} {
		if len(raw) > 0 && !json.Valid(raw) {
			return AgentHarnessValidationError{Code: "invalid_json", Message: fmt.Sprintf("%s must be valid JSON", field)}
		}
	}
	return nil
}

func optionalHarnessString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

type agentHarnessResponse struct {
	ID                     uuid.UUID       `json:"id"`
	OrganizationID         uuid.UUID       `json:"organization_id"`
	WorkspaceID            uuid.UUID       `json:"workspace_id"`
	CreatedByUserID        *uuid.UUID      `json:"created_by_user_id,omitempty"`
	Name                   string          `json:"name"`
	Slug                   string          `json:"slug"`
	Description            string          `json:"description"`
	Status                 string          `json:"status"`
	HarnessKind            string          `json:"harness_kind"`
	TaskPrompt             string          `json:"task_prompt"`
	CodexTemplate          string          `json:"codex_template"`
	CodexModel             *string         `json:"codex_model,omitempty"`
	AuthMode               string          `json:"auth_mode"`
	OpenAIAPIKeySecretName *string         `json:"openai_api_key_secret_name,omitempty"`
	E2BAPIKeySecretName    *string         `json:"e2b_api_key_secret_name,omitempty"`
	RepositoryURL          *string         `json:"repository_url,omitempty"`
	BaseBranch             *string         `json:"base_branch,omitempty"`
	ExecutionConfig        json.RawMessage `json:"execution_config"`
	EvaluationConfig       json.RawMessage `json:"evaluation_config"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

type listAgentHarnessesResponse struct {
	Items []agentHarnessResponse `json:"items"`
}

func createAgentHarnessHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		var input CreateAgentHarnessInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		harness, err := service.CreateAgentHarness(r.Context(), caller, workspaceID, input)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapAgentHarnessResponse(harness))
	}
}

func listAgentHarnessesHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		harnesses, err := service.ListAgentHarnesses(r.Context(), caller, workspaceID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		items := make([]agentHarnessResponse, 0, len(harnesses))
		for _, harness := range harnesses {
			items = append(items, mapAgentHarnessResponse(harness))
		}
		writeJSON(w, http.StatusOK, listAgentHarnessesResponse{Items: items})
	}
}

func getAgentHarnessHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		harnessID, err := uuid.Parse(chi.URLParam(r, "harnessID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_harness_id", "harnessID must be a UUID")
			return
		}
		harness, err := service.GetAgentHarness(r.Context(), caller, workspaceID, harnessID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAgentHarnessResponse(harness))
	}
}

func mapAgentHarnessResponse(h repository.AgentHarness) agentHarnessResponse {
	return agentHarnessResponse{
		ID:                     h.ID,
		OrganizationID:         h.OrganizationID,
		WorkspaceID:            h.WorkspaceID,
		CreatedByUserID:        h.CreatedByUserID,
		Name:                   h.Name,
		Slug:                   h.Slug,
		Description:            h.Description,
		Status:                 h.Status,
		HarnessKind:            h.HarnessKind,
		TaskPrompt:             h.TaskPrompt,
		CodexTemplate:          h.CodexTemplate,
		CodexModel:             h.CodexModel,
		AuthMode:               h.AuthMode,
		OpenAIAPIKeySecretName: h.OpenAIAPIKeySecretName,
		E2BAPIKeySecretName:    h.E2BAPIKeySecretName,
		RepositoryURL:          h.RepositoryURL,
		BaseBranch:             h.BaseBranch,
		ExecutionConfig:        h.ExecutionConfig,
		EvaluationConfig:       h.EvaluationConfig,
		CreatedAt:              h.CreatedAt,
		UpdatedAt:              h.UpdatedAt,
	}
}

func writeAgentHarnessError(w http.ResponseWriter, logger *slog.Logger, r *http.Request, err error) {
	var validationErr AgentHarnessValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
	case errors.Is(err, repository.ErrAgentHarnessSlugConflict):
		writeError(w, http.StatusConflict, "agent_harness_slug_conflict", "an agent harness with this name already exists in the workspace")
	case errors.Is(err, repository.ErrAgentHarnessNotFound):
		writeError(w, http.StatusNotFound, "not_found", "agent harness not found")
	default:
		if errors.Is(err, ErrUnauthenticated) || errors.Is(err, ErrCallerMissing) || errors.Is(err, ErrForbidden) {
			writeAuthzError(w, err)
			return
		}
		logger.Error("agent harness request failed",
			"method", r.Method,
			"path", r.URL.Path,
			"error", err,
		)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

type noopAgentHarnessService struct{}

func (noopAgentHarnessService) CreateAgentHarness(context.Context, Caller, uuid.UUID, CreateAgentHarnessInput) (repository.AgentHarness, error) {
	return repository.AgentHarness{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) GetAgentHarness(context.Context, Caller, uuid.UUID, uuid.UUID) (repository.AgentHarness, error) {
	return repository.AgentHarness{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) ListAgentHarnesses(context.Context, Caller, uuid.UUID) ([]repository.AgentHarness, error) {
	return nil, errors.New("agent harness service is not configured")
}
