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
	AgentHarnessAuthModeAPIKeySecret = "api_key_secret"
	defaultCodexE2BTemplate          = "codex"
)

type AgentHarnessRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	CreateAgentHarness(ctx context.Context, p repository.CreateAgentHarnessParams) (repository.AgentHarness, error)
	GetAgentHarnessByID(ctx context.Context, id uuid.UUID) (repository.AgentHarness, error)
	ListAgentHarnessesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.AgentHarness, error)
	CreateAgentHarnessExecution(ctx context.Context, p repository.CreateAgentHarnessExecutionParams) (repository.AgentHarnessExecution, error)
	TransitionAgentHarnessExecutionStatus(ctx context.Context, p repository.TransitionAgentHarnessExecutionStatusParams) (repository.AgentHarnessExecution, error)
	GetAgentHarnessExecutionByID(ctx context.Context, id uuid.UUID) (repository.AgentHarnessExecution, error)
	ListAgentHarnessExecutions(ctx context.Context, p repository.ListAgentHarnessExecutionsParams) ([]repository.AgentHarnessExecution, error)
	ListAgentHarnessExecutionEvents(ctx context.Context, executionID uuid.UUID) ([]repository.AgentHarnessExecutionEvent, error)
}

type AgentHarnessService interface {
	CreateAgentHarness(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateAgentHarnessInput) (repository.AgentHarness, error)
	GetAgentHarness(ctx context.Context, caller Caller, workspaceID uuid.UUID, id uuid.UUID) (repository.AgentHarness, error)
	ListAgentHarnesses(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.AgentHarness, error)
	StartAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, harnessID uuid.UUID, input StartAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error)
	GetAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) (repository.AgentHarnessExecution, error)
	ListAgentHarnessExecutions(ctx context.Context, caller Caller, workspaceID uuid.UUID, harnessID *uuid.UUID) ([]repository.AgentHarnessExecution, error)
	ListAgentHarnessExecutionEvents(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) ([]repository.AgentHarnessExecutionEvent, error)
}

type AgentHarnessExecutionWorkflowStarter interface {
	StartAgentHarnessExecutionWorkflow(ctx context.Context, executionID uuid.UUID) error
}

type noopAgentHarnessExecutionWorkflowStarter struct{}

func (noopAgentHarnessExecutionWorkflowStarter) StartAgentHarnessExecutionWorkflow(context.Context, uuid.UUID) error {
	return nil
}

type AgentHarnessManager struct {
	authorizer      WorkspaceAuthorizer
	repo            AgentHarnessRepository
	workflowStarter AgentHarnessExecutionWorkflowStarter
}

func NewAgentHarnessManager(authorizer WorkspaceAuthorizer, repo AgentHarnessRepository, starters ...AgentHarnessExecutionWorkflowStarter) *AgentHarnessManager {
	starter := AgentHarnessExecutionWorkflowStarter(noopAgentHarnessExecutionWorkflowStarter{})
	if len(starters) > 0 && starters[0] != nil {
		starter = starters[0]
	}
	return &AgentHarnessManager{authorizer: authorizer, repo: repo, workflowStarter: starter}
}

type CreateAgentHarnessInput struct {
	Name                   string          `json:"name"`
	Description            string          `json:"description"`
	TaskPrompt             string          `json:"task_prompt"`
	CodexTemplate          string          `json:"codex_template"`
	CodexModel             string          `json:"codex_model"`
	AuthMode               string          `json:"auth_mode"`
	OpenAIAPIKeySecretName string          `json:"openai_api_key_secret_name"`
	RepositoryURL          string          `json:"repository_url"`
	BaseBranch             string          `json:"base_branch"`
	ExecutionConfig        json.RawMessage `json:"execution_config"`
	EvaluationConfig       json.RawMessage `json:"evaluation_config"`
}

type StartAgentHarnessExecutionInput struct {
	Message string `json:"message"`
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

func (m *AgentHarnessManager) StartAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, harnessID uuid.UUID, input StartAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	harness, err := m.repo.GetAgentHarnessByID(ctx, harnessID)
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	if harness.WorkspaceID != workspaceID {
		return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessNotFound
	}
	snapshot, err := marshalAgentHarnessSnapshot(harness, input)
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	execution, err := m.repo.CreateAgentHarnessExecution(ctx, repository.CreateAgentHarnessExecutionParams{
		OrganizationID:           harness.OrganizationID,
		WorkspaceID:              workspaceID,
		AgentHarnessID:           harness.ID,
		CreatedByUserID:          &caller.UserID,
		HarnessSnapshot:          snapshot,
		ExecutionConfigSnapshot:  defaultJSON(harness.ExecutionConfig),
		EvaluationConfigSnapshot: defaultJSON(harness.EvaluationConfig),
	})
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	if err := m.workflowStarter.StartAgentHarnessExecutionWorkflow(ctx, execution.ID); err != nil {
		reason := err.Error()
		_, _ = m.repo.TransitionAgentHarnessExecutionStatus(ctx, repository.TransitionAgentHarnessExecutionStatusParams{
			ExecutionID: execution.ID,
			ToStatus:    repository.AgentHarnessExecutionStatusFailed,
			Reason:      &reason,
		})
		return repository.AgentHarnessExecution{}, err
	}
	return execution, nil
}

func (m *AgentHarnessManager) GetAgentHarnessExecution(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) (repository.AgentHarnessExecution, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	execution, err := m.repo.GetAgentHarnessExecutionByID(ctx, executionID)
	if err != nil {
		return repository.AgentHarnessExecution{}, err
	}
	if execution.WorkspaceID != workspaceID {
		return repository.AgentHarnessExecution{}, repository.ErrAgentHarnessExecutionNotFound
	}
	return execution, nil
}

func (m *AgentHarnessManager) ListAgentHarnessExecutionEvents(ctx context.Context, caller Caller, workspaceID uuid.UUID, executionID uuid.UUID) ([]repository.AgentHarnessExecutionEvent, error) {
	if _, err := m.GetAgentHarnessExecution(ctx, caller, workspaceID, executionID); err != nil {
		return nil, err
	}
	return m.repo.ListAgentHarnessExecutionEvents(ctx, executionID)
}

func (m *AgentHarnessManager) ListAgentHarnessExecutions(ctx context.Context, caller Caller, workspaceID uuid.UUID, harnessID *uuid.UUID) ([]repository.AgentHarnessExecution, error) {
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	if harnessID != nil {
		harness, err := m.repo.GetAgentHarnessByID(ctx, *harnessID)
		if err != nil {
			return nil, err
		}
		if harness.WorkspaceID != workspaceID {
			return nil, repository.ErrAgentHarnessNotFound
		}
	}
	return m.repo.ListAgentHarnessExecutions(ctx, repository.ListAgentHarnessExecutionsParams{
		WorkspaceID:    workspaceID,
		AgentHarnessID: harnessID,
	})
}

func validateAgentHarnessInput(input CreateAgentHarnessInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return AgentHarnessValidationError{Code: "invalid_name", Message: "name is required"}
	}
	if strings.TrimSpace(input.TaskPrompt) == "" {
		return AgentHarnessValidationError{Code: "invalid_task_prompt", Message: "task_prompt is required"}
	}
	switch strings.TrimSpace(input.AuthMode) {
	case AgentHarnessAuthModeAPIKeySecret:
	case "":
		return AgentHarnessValidationError{Code: "invalid_auth_mode", Message: "auth_mode is required"}
	default:
		return AgentHarnessValidationError{Code: "invalid_auth_mode", Message: "auth_mode must be api_key_secret for hosted agent harness execution"}
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

type agentHarnessExecutionResponse struct {
	ID                       uuid.UUID                            `json:"id"`
	OrganizationID           uuid.UUID                            `json:"organization_id"`
	WorkspaceID              uuid.UUID                            `json:"workspace_id"`
	AgentHarnessID           uuid.UUID                            `json:"agent_harness_id"`
	CreatedByUserID          *uuid.UUID                           `json:"created_by_user_id,omitempty"`
	Status                   string                               `json:"status"`
	HarnessSnapshot          json.RawMessage                      `json:"harness_snapshot"`
	ExecutionConfigSnapshot  json.RawMessage                      `json:"execution_config_snapshot"`
	EvaluationConfigSnapshot json.RawMessage                      `json:"evaluation_config_snapshot"`
	ErrorMessage             *string                              `json:"error_message,omitempty"`
	StartedAt                *time.Time                           `json:"started_at,omitempty"`
	CompletedAt              *time.Time                           `json:"completed_at,omitempty"`
	CancelledAt              *time.Time                           `json:"cancelled_at,omitempty"`
	CreatedAt                time.Time                            `json:"created_at"`
	UpdatedAt                time.Time                            `json:"updated_at"`
	Events                   []agentHarnessExecutionEventResponse `json:"events,omitempty"`
}

type agentHarnessExecutionEventResponse struct {
	ID                      int64           `json:"id"`
	AgentHarnessExecutionID uuid.UUID       `json:"agent_harness_execution_id"`
	SequenceNumber          int64           `json:"sequence_number"`
	EventType               string          `json:"event_type"`
	ActorType               string          `json:"actor_type"`
	OccurredAt              time.Time       `json:"occurred_at"`
	ArtifactID              *uuid.UUID      `json:"artifact_id,omitempty"`
	Payload                 json.RawMessage `json:"payload"`
}

type listAgentHarnessExecutionsResponse struct {
	Items []agentHarnessExecutionResponse `json:"items"`
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

func startAgentHarnessExecutionHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		var input StartAgentHarnessExecutionInput
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
				return
			}
		}
		execution, err := service.StartAgentHarnessExecution(r.Context(), caller, workspaceID, harnessID, input)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, mapAgentHarnessExecutionResponse(execution))
	}
}

func listAgentHarnessExecutionsHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		var harnessID *uuid.UUID
		if rawHarnessID := strings.TrimSpace(r.URL.Query().Get("harness_id")); rawHarnessID != "" {
			parsedHarnessID, err := uuid.Parse(rawHarnessID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_harness_id", "harness_id must be a UUID")
				return
			}
			harnessID = &parsedHarnessID
		}
		executions, err := service.ListAgentHarnessExecutions(r.Context(), caller, workspaceID, harnessID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		items := make([]agentHarnessExecutionResponse, 0, len(executions))
		for _, execution := range executions {
			items = append(items, mapAgentHarnessExecutionResponse(execution))
		}
		writeJSON(w, http.StatusOK, listAgentHarnessExecutionsResponse{Items: items})
	}
}

func getAgentHarnessExecutionHandler(logger *slog.Logger, service AgentHarnessService) http.HandlerFunc {
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
		executionID, err := uuid.Parse(chi.URLParam(r, "executionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_execution_id", "executionID must be a UUID")
			return
		}
		execution, err := service.GetAgentHarnessExecution(r.Context(), caller, workspaceID, executionID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		response := mapAgentHarnessExecutionResponse(execution)
		events, err := service.ListAgentHarnessExecutionEvents(r.Context(), caller, workspaceID, executionID)
		if err != nil {
			writeAgentHarnessError(w, logger, r, err)
			return
		}
		response.Events = mapAgentHarnessExecutionEventResponses(events)
		writeJSON(w, http.StatusOK, response)
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
		RepositoryURL:          h.RepositoryURL,
		BaseBranch:             h.BaseBranch,
		ExecutionConfig:        h.ExecutionConfig,
		EvaluationConfig:       h.EvaluationConfig,
		CreatedAt:              h.CreatedAt,
		UpdatedAt:              h.UpdatedAt,
	}
}

func mapAgentHarnessExecutionResponse(e repository.AgentHarnessExecution) agentHarnessExecutionResponse {
	return agentHarnessExecutionResponse{
		ID:                       e.ID,
		OrganizationID:           e.OrganizationID,
		WorkspaceID:              e.WorkspaceID,
		AgentHarnessID:           e.AgentHarnessID,
		CreatedByUserID:          e.CreatedByUserID,
		Status:                   e.Status,
		HarnessSnapshot:          e.HarnessSnapshot,
		ExecutionConfigSnapshot:  e.ExecutionConfigSnapshot,
		EvaluationConfigSnapshot: e.EvaluationConfigSnapshot,
		ErrorMessage:             e.ErrorMessage,
		StartedAt:                e.StartedAt,
		CompletedAt:              e.CompletedAt,
		CancelledAt:              e.CancelledAt,
		CreatedAt:                e.CreatedAt,
		UpdatedAt:                e.UpdatedAt,
	}
}

func mapAgentHarnessExecutionEventResponses(events []repository.AgentHarnessExecutionEvent) []agentHarnessExecutionEventResponse {
	if len(events) == 0 {
		return nil
	}
	items := make([]agentHarnessExecutionEventResponse, 0, len(events))
	for _, event := range events {
		items = append(items, agentHarnessExecutionEventResponse{
			ID:                      event.ID,
			AgentHarnessExecutionID: event.AgentHarnessExecutionID,
			SequenceNumber:          event.SequenceNumber,
			EventType:               event.EventType,
			ActorType:               event.ActorType,
			OccurredAt:              event.OccurredAt,
			ArtifactID:              event.ArtifactID,
			Payload:                 event.Payload,
		})
	}
	return items
}

func marshalAgentHarnessSnapshot(h repository.AgentHarness, input StartAgentHarnessExecutionInput) (json.RawMessage, error) {
	response := mapAgentHarnessResponse(h)
	if message := strings.TrimSpace(input.Message); message != "" {
		response.TaskPrompt = message
	}
	snapshot, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
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
	case errors.Is(err, repository.ErrAgentHarnessExecutionNotFound):
		writeError(w, http.StatusNotFound, "not_found", "agent harness execution not found")
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

func (noopAgentHarnessService) StartAgentHarnessExecution(context.Context, Caller, uuid.UUID, uuid.UUID, StartAgentHarnessExecutionInput) (repository.AgentHarnessExecution, error) {
	return repository.AgentHarnessExecution{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) GetAgentHarnessExecution(context.Context, Caller, uuid.UUID, uuid.UUID) (repository.AgentHarnessExecution, error) {
	return repository.AgentHarnessExecution{}, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) ListAgentHarnessExecutionEvents(context.Context, Caller, uuid.UUID, uuid.UUID) ([]repository.AgentHarnessExecutionEvent, error) {
	return nil, errors.New("agent harness service is not configured")
}

func (noopAgentHarnessService) ListAgentHarnessExecutions(context.Context, Caller, uuid.UUID, *uuid.UUID) ([]repository.AgentHarnessExecution, error) {
	return nil, errors.New("agent harness service is not configured")
}
