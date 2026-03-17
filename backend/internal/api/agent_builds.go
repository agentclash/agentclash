package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxAgentBuildRequestBytes = 1 << 20

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

type AgentBuildRepository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	CreateAgentBuild(ctx context.Context, params repository.CreateAgentBuildParams) (repository.AgentBuild, error)
	GetAgentBuildByID(ctx context.Context, id uuid.UUID) (repository.AgentBuild, error)
	ListAgentBuildsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.AgentBuild, error)
	CreateAgentBuildVersion(ctx context.Context, params repository.CreateAgentBuildVersionParams) (repository.AgentBuildVersion, error)
	GetAgentBuildVersionByID(ctx context.Context, id uuid.UUID) (repository.AgentBuildVersion, error)
	GetLatestVersionNumberForBuild(ctx context.Context, agentBuildID uuid.UUID) (int32, error)
	ListAgentBuildVersionsByBuildID(ctx context.Context, agentBuildID uuid.UUID) ([]repository.AgentBuildVersion, error)
	UpdateAgentBuildVersionDraft(ctx context.Context, params repository.UpdateAgentBuildVersionDraftParams) error
	MarkAgentBuildVersionReady(ctx context.Context, id uuid.UUID) error
	CreateAgentDeployment(ctx context.Context, params repository.CreateAgentDeploymentParams) (repository.AgentDeploymentRow, error)
}

type AgentBuildService interface {
	CreateBuild(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateAgentBuildInput) (repository.AgentBuild, error)
	GetBuild(ctx context.Context, id uuid.UUID) (repository.AgentBuild, error)
	ListBuilds(ctx context.Context, workspaceID uuid.UUID) ([]repository.AgentBuild, error)
	ListVersions(ctx context.Context, agentBuildID uuid.UUID) ([]repository.AgentBuildVersion, error)
	CreateVersion(ctx context.Context, caller Caller, agentBuildID uuid.UUID, input CreateAgentBuildVersionInput) (repository.AgentBuildVersion, error)
	GetVersion(ctx context.Context, id uuid.UUID) (repository.AgentBuildVersion, error)
	UpdateVersion(ctx context.Context, id uuid.UUID, input UpdateAgentBuildVersionInput) (repository.AgentBuildVersion, error)
	ValidateVersion(ctx context.Context, id uuid.UUID) (ValidateBuildVersionResult, error)
	MarkVersionReady(ctx context.Context, id uuid.UUID) error
	CreateDeployment(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateAgentDeploymentInput) (repository.AgentDeploymentRow, error)
}

type CreateAgentBuildInput struct {
	Name        string
	Description string
}

type CreateAgentBuildVersionInput struct {
	AgentKind        string
	InterfaceSpec    json.RawMessage
	PolicySpec       json.RawMessage
	ReasoningSpec    json.RawMessage
	MemorySpec       json.RawMessage
	WorkflowSpec     json.RawMessage
	GuardrailSpec    json.RawMessage
	ModelSpec        json.RawMessage
	OutputSchema     json.RawMessage
	TraceContract    json.RawMessage
	PublicationSpec  json.RawMessage
	Tools            []repository.AgentBuildVersionToolBinding
	KnowledgeSources []repository.AgentBuildVersionKnowledgeSourceBinding
}

type UpdateAgentBuildVersionInput = CreateAgentBuildVersionInput

type CreateAgentDeploymentInput struct {
	Name              string
	AgentBuildID      uuid.UUID
	BuildVersionID    uuid.UUID
	RuntimeProfileID  uuid.UUID
	ProviderAccountID *uuid.UUID
	ModelAliasID      *uuid.UUID
	DeploymentConfig  json.RawMessage
}

type ValidateBuildVersionResult struct {
	Valid  bool
	Errors []validationErrorDetail
}

type AgentBuildManager struct {
	repo AgentBuildRepository
}

func NewAgentBuildManager(repo AgentBuildRepository) *AgentBuildManager {
	return &AgentBuildManager{repo: repo}
}

func (m *AgentBuildManager) CreateBuild(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateAgentBuildInput) (repository.AgentBuild, error) {
	if strings.TrimSpace(input.Name) == "" {
		return repository.AgentBuild{}, AgentBuildValidationError{Code: "invalid_name", Message: "name is required"}
	}

	orgID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return repository.AgentBuild{}, err
	}

	slug := generateSlug(input.Name)

	return m.repo.CreateAgentBuild(ctx, repository.CreateAgentBuildParams{
		OrganizationID:  orgID,
		WorkspaceID:     workspaceID,
		Name:            strings.TrimSpace(input.Name),
		Slug:            slug,
		Description:     strings.TrimSpace(input.Description),
		CreatedByUserID: &caller.UserID,
	})
}

func (m *AgentBuildManager) GetBuild(ctx context.Context, id uuid.UUID) (repository.AgentBuild, error) {
	return m.repo.GetAgentBuildByID(ctx, id)
}

func (m *AgentBuildManager) ListBuilds(ctx context.Context, workspaceID uuid.UUID) ([]repository.AgentBuild, error) {
	return m.repo.ListAgentBuildsByWorkspaceID(ctx, workspaceID)
}

func (m *AgentBuildManager) ListVersions(ctx context.Context, agentBuildID uuid.UUID) ([]repository.AgentBuildVersion, error) {
	return m.repo.ListAgentBuildVersionsByBuildID(ctx, agentBuildID)
}

func (m *AgentBuildManager) CreateVersion(ctx context.Context, caller Caller, agentBuildID uuid.UUID, input CreateAgentBuildVersionInput) (repository.AgentBuildVersion, error) {
	_, err := m.repo.GetAgentBuildByID(ctx, agentBuildID)
	if err != nil {
		return repository.AgentBuildVersion{}, err
	}

	latestVersion, err := m.repo.GetLatestVersionNumberForBuild(ctx, agentBuildID)
	if err != nil {
		return repository.AgentBuildVersion{}, err
	}

	return m.repo.CreateAgentBuildVersion(ctx, repository.CreateAgentBuildVersionParams{
		AgentBuildID:     agentBuildID,
		VersionNumber:    latestVersion + 1,
		AgentKind:        defaultString(input.AgentKind, "llm_agent"),
		InterfaceSpec:    defaultJSON(input.InterfaceSpec),
		PolicySpec:       defaultJSON(input.PolicySpec),
		ReasoningSpec:    defaultJSON(input.ReasoningSpec),
		MemorySpec:       defaultJSON(input.MemorySpec),
		WorkflowSpec:     defaultJSON(input.WorkflowSpec),
		GuardrailSpec:    defaultJSON(input.GuardrailSpec),
		ModelSpec:        defaultJSON(input.ModelSpec),
		OutputSchema:     defaultJSON(input.OutputSchema),
		TraceContract:    defaultJSON(input.TraceContract),
		PublicationSpec:  defaultJSON(input.PublicationSpec),
		Tools:            input.Tools,
		KnowledgeSources: input.KnowledgeSources,
		CreatedByUserID:  &caller.UserID,
	})
}

func (m *AgentBuildManager) GetVersion(ctx context.Context, id uuid.UUID) (repository.AgentBuildVersion, error) {
	return m.repo.GetAgentBuildVersionByID(ctx, id)
}

func (m *AgentBuildManager) UpdateVersion(ctx context.Context, id uuid.UUID, input UpdateAgentBuildVersionInput) (repository.AgentBuildVersion, error) {
	version, err := m.repo.GetAgentBuildVersionByID(ctx, id)
	if err != nil {
		return repository.AgentBuildVersion{}, err
	}

	if version.VersionStatus != "draft" {
		return repository.AgentBuildVersion{}, AgentBuildValidationError{
			Code:    "version_not_draft",
			Message: "only draft versions can be updated",
		}
	}

	err = m.repo.UpdateAgentBuildVersionDraft(ctx, repository.UpdateAgentBuildVersionDraftParams{
		ID:               id,
		AgentKind:        defaultString(input.AgentKind, version.AgentKind),
		InterfaceSpec:    defaultJSONOrExisting(input.InterfaceSpec, version.InterfaceSpec),
		PolicySpec:       defaultJSONOrExisting(input.PolicySpec, version.PolicySpec),
		ReasoningSpec:    defaultJSONOrExisting(input.ReasoningSpec, version.ReasoningSpec),
		MemorySpec:       defaultJSONOrExisting(input.MemorySpec, version.MemorySpec),
		WorkflowSpec:     defaultJSONOrExisting(input.WorkflowSpec, version.WorkflowSpec),
		GuardrailSpec:    defaultJSONOrExisting(input.GuardrailSpec, version.GuardrailSpec),
		ModelSpec:        defaultJSONOrExisting(input.ModelSpec, version.ModelSpec),
		OutputSchema:     defaultJSONOrExisting(input.OutputSchema, version.OutputSchema),
		TraceContract:    defaultJSONOrExisting(input.TraceContract, version.TraceContract),
		PublicationSpec:  defaultJSONOrExisting(input.PublicationSpec, version.PublicationSpec),
		Tools:            defaultToolBindingsOrExisting(input.Tools, version.Tools),
		KnowledgeSources: defaultKnowledgeSourceBindingsOrExisting(input.KnowledgeSources, version.KnowledgeSources),
	})
	if err != nil {
		return repository.AgentBuildVersion{}, err
	}

	return m.repo.GetAgentBuildVersionByID(ctx, id)
}

func (m *AgentBuildManager) ValidateVersion(ctx context.Context, id uuid.UUID) (ValidateBuildVersionResult, error) {
	version, err := m.repo.GetAgentBuildVersionByID(ctx, id)
	if err != nil {
		return ValidateBuildVersionResult{}, err
	}

	var validationErrors []validationErrorDetail

	validKinds := map[string]bool{
		"llm_agent": true, "workflow_agent": true, "programmatic_agent": true,
		"multi_agent_system": true, "hosted_external": true,
	}
	if !validKinds[version.AgentKind] {
		validationErrors = append(validationErrors, validationErrorDetail{
			Field:   "agent_kind",
			Message: fmt.Sprintf("agent_kind must be one of: llm_agent, workflow_agent, programmatic_agent, multi_agent_system, hosted_external; got %q", version.AgentKind),
		})
	}

	if hasInstructions := jsonHasKey(version.PolicySpec, "instructions"); !hasInstructions {
		validationErrors = append(validationErrors, validationErrorDetail{
			Field:   "policy_spec",
			Message: "policy_spec must contain an 'instructions' field",
		})
	}

	return ValidateBuildVersionResult{
		Valid:  len(validationErrors) == 0,
		Errors: validationErrors,
	}, nil
}

func (m *AgentBuildManager) MarkVersionReady(ctx context.Context, id uuid.UUID) error {
	result, err := m.ValidateVersion(ctx, id)
	if err != nil {
		return err
	}
	if !result.Valid {
		return AgentBuildValidationError{
			Code:    "validation_failed",
			Message: "version has validation errors and cannot be marked ready",
		}
	}

	return m.repo.MarkAgentBuildVersionReady(ctx, id)
}

func (m *AgentBuildManager) CreateDeployment(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateAgentDeploymentInput) (repository.AgentDeploymentRow, error) {
	version, err := m.repo.GetAgentBuildVersionByID(ctx, input.BuildVersionID)
	if err != nil {
		return repository.AgentDeploymentRow{}, err
	}

	if version.VersionStatus != "ready" {
		return repository.AgentDeploymentRow{}, AgentBuildValidationError{
			Code:    "version_not_ready",
			Message: "only ready versions can be deployed",
		}
	}

	build, err := m.repo.GetAgentBuildByID(ctx, input.AgentBuildID)
	if err != nil {
		return repository.AgentDeploymentRow{}, err
	}

	slug := generateSlug(input.Name)

	return m.repo.CreateAgentDeployment(ctx, repository.CreateAgentDeploymentParams{
		OrganizationID:        build.OrganizationID,
		WorkspaceID:           workspaceID,
		AgentBuildID:          input.AgentBuildID,
		CurrentBuildVersionID: input.BuildVersionID,
		RuntimeProfileID:      input.RuntimeProfileID,
		ProviderAccountID:     input.ProviderAccountID,
		ModelAliasID:          input.ModelAliasID,
		Name:                  strings.TrimSpace(input.Name),
		Slug:                  slug,
		DeploymentConfig:      defaultJSON(input.DeploymentConfig),
	})
}

// --- Request types ---

type createAgentBuildRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type createAgentBuildVersionRequest struct {
	AgentKind        string                          `json:"agent_kind"`
	InterfaceSpec    json.RawMessage                 `json:"interface_spec"`
	PolicySpec       json.RawMessage                 `json:"policy_spec"`
	ReasoningSpec    json.RawMessage                 `json:"reasoning_spec"`
	MemorySpec       json.RawMessage                 `json:"memory_spec"`
	WorkflowSpec     json.RawMessage                 `json:"workflow_spec"`
	GuardrailSpec    json.RawMessage                 `json:"guardrail_spec"`
	ModelSpec        json.RawMessage                 `json:"model_spec"`
	OutputSchema     json.RawMessage                 `json:"output_schema"`
	TraceContract    json.RawMessage                 `json:"trace_contract"`
	PublicationSpec  json.RawMessage                 `json:"publication_spec"`
	Tools            []toolBindingRequest            `json:"tools"`
	KnowledgeSources []knowledgeSourceBindingRequest `json:"knowledge_sources"`
}

type createAgentDeploymentRequest struct {
	Name              string          `json:"name"`
	AgentBuildID      string          `json:"agent_build_id"`
	BuildVersionID    string          `json:"build_version_id"`
	RuntimeProfileID  string          `json:"runtime_profile_id"`
	ProviderAccountID *string         `json:"provider_account_id,omitempty"`
	ModelAliasID      *string         `json:"model_alias_id,omitempty"`
	DeploymentConfig  json.RawMessage `json:"deployment_config,omitempty"`
}

// --- Response types ---

type agentBuildResponse struct {
	ID              uuid.UUID `json:"id"`
	WorkspaceID     uuid.UUID `json:"workspace_id"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	Description     string    `json:"description,omitempty"`
	LifecycleStatus string    `json:"lifecycle_status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type agentBuildDetailResponse struct {
	agentBuildResponse
	Versions []agentBuildVersionResponse `json:"versions"`
}

type agentBuildVersionResponse struct {
	ID               uuid.UUID                        `json:"id"`
	AgentBuildID     uuid.UUID                        `json:"agent_build_id"`
	VersionNumber    int32                            `json:"version_number"`
	VersionStatus    string                           `json:"version_status"`
	AgentKind        string                           `json:"agent_kind"`
	InterfaceSpec    json.RawMessage                  `json:"interface_spec"`
	PolicySpec       json.RawMessage                  `json:"policy_spec"`
	ReasoningSpec    json.RawMessage                  `json:"reasoning_spec"`
	MemorySpec       json.RawMessage                  `json:"memory_spec"`
	WorkflowSpec     json.RawMessage                  `json:"workflow_spec"`
	GuardrailSpec    json.RawMessage                  `json:"guardrail_spec"`
	ModelSpec        json.RawMessage                  `json:"model_spec"`
	OutputSchema     json.RawMessage                  `json:"output_schema"`
	TraceContract    json.RawMessage                  `json:"trace_contract"`
	PublicationSpec  json.RawMessage                  `json:"publication_spec"`
	Tools            []toolBindingResponse            `json:"tools"`
	KnowledgeSources []knowledgeSourceBindingResponse `json:"knowledge_sources"`
	CreatedAt        time.Time                        `json:"created_at"`
}

type toolBindingRequest struct {
	ToolID        string          `json:"tool_id"`
	BindingRole   string          `json:"binding_role"`
	BindingConfig json.RawMessage `json:"binding_config"`
}

type knowledgeSourceBindingRequest struct {
	KnowledgeSourceID string          `json:"knowledge_source_id"`
	BindingRole       string          `json:"binding_role"`
	BindingConfig     json.RawMessage `json:"binding_config"`
}

type toolBindingResponse struct {
	ToolID        uuid.UUID       `json:"tool_id"`
	BindingRole   string          `json:"binding_role"`
	BindingConfig json.RawMessage `json:"binding_config"`
}

type knowledgeSourceBindingResponse struct {
	KnowledgeSourceID uuid.UUID       `json:"knowledge_source_id"`
	BindingRole       string          `json:"binding_role"`
	BindingConfig     json.RawMessage `json:"binding_config"`
}

type listAgentBuildsResponse struct {
	Items []agentBuildResponse `json:"items"`
}

type validationErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type validateBuildVersionResponse struct {
	Valid  bool                    `json:"valid"`
	Errors []validationErrorDetail `json:"errors"`
}

type agentDeploymentCreateResponse struct {
	ID                    uuid.UUID `json:"id"`
	WorkspaceID           uuid.UUID `json:"workspace_id"`
	AgentBuildID          uuid.UUID `json:"agent_build_id"`
	CurrentBuildVersionID uuid.UUID `json:"current_build_version_id"`
	Name                  string    `json:"name"`
	Slug                  string    `json:"slug"`
	DeploymentType        string    `json:"deployment_type"`
	Status                string    `json:"status"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// --- Error type ---

type AgentBuildValidationError struct {
	Code    string
	Message string
}

func (e AgentBuildValidationError) Error() string {
	return e.Message
}

// --- Handlers ---

func createAgentBuildHandler(logger *slog.Logger, service AgentBuildService) http.HandlerFunc {
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

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxAgentBuildRequestBytes)

		var body createAgentBuildRequest
		if err := decodeJSON(r, &body); err != nil {
			handleDecodeError(w, logger, r, err)
			return
		}

		build, err := service.CreateBuild(r.Context(), caller, workspaceID, CreateAgentBuildInput{
			Name:        body.Name,
			Description: body.Description,
		})
		if err != nil {
			handleServiceError(w, logger, r, err)
			return
		}

		writeJSON(w, http.StatusCreated, buildAgentBuildResponse(build))
	}
}

func listAgentBuildsHandler(logger *slog.Logger, service AgentBuildService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		builds, err := service.ListBuilds(r.Context(), workspaceID)
		if err != nil {
			logger.Error("list agent builds request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"workspace_id", workspaceID,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		items := make([]agentBuildResponse, 0, len(builds))
		for _, b := range builds {
			items = append(items, buildAgentBuildResponse(b))
		}

		writeJSON(w, http.StatusOK, listAgentBuildsResponse{Items: items})
	}
}

func getAgentBuildHandler(logger *slog.Logger, service AgentBuildService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		id, err := uuid.Parse(chi.URLParam(r, "agentBuildID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_agent_build_id", "agent_build_id must be a valid UUID")
			return
		}

		build, err := service.GetBuild(r.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrAgentBuildNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent build not found")
				return
			}
			logger.Error("get agent build request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if err := ensureCallerCanAccessWorkspace(caller, build.WorkspaceID); err != nil {
			writeAuthzError(w, err)
			return
		}

		versions, err := service.ListVersions(r.Context(), id)
		if err != nil {
			logger.Error("list agent build versions request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		versionItems := make([]agentBuildVersionResponse, 0, len(versions))
		for _, v := range versions {
			versionItems = append(versionItems, buildAgentBuildVersionResponse(v))
		}

		writeJSON(w, http.StatusOK, agentBuildDetailResponse{
			agentBuildResponse: buildAgentBuildResponse(build),
			Versions:           versionItems,
		})
	}
}

func createAgentBuildVersionHandler(logger *slog.Logger, service AgentBuildService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		agentBuildID, err := uuid.Parse(chi.URLParam(r, "agentBuildID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_agent_build_id", "agent_build_id must be a valid UUID")
			return
		}
		build, err := service.GetBuild(r.Context(), agentBuildID)
		if err != nil {
			if errors.Is(err, repository.ErrAgentBuildNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent build not found")
				return
			}
			handleServiceError(w, logger, r, err)
			return
		}
		if err := ensureCallerCanAccessWorkspace(caller, build.WorkspaceID); err != nil {
			writeAuthzError(w, err)
			return
		}

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxAgentBuildRequestBytes)

		var body createAgentBuildVersionRequest
		if err := decodeJSON(r, &body); err != nil {
			handleDecodeError(w, logger, r, err)
			return
		}
		input, err := decodeCreateAgentBuildVersionInput(body)
		if err != nil {
			handleServiceError(w, logger, r, err)
			return
		}

		version, err := service.CreateVersion(r.Context(), caller, agentBuildID, input)
		if err != nil {
			if errors.Is(err, repository.ErrAgentBuildNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent build not found")
				return
			}
			handleServiceError(w, logger, r, err)
			return
		}

		writeJSON(w, http.StatusCreated, buildAgentBuildVersionResponse(version))
	}
}

func getAgentBuildVersionHandler(logger *slog.Logger, service AgentBuildService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		id, err := uuid.Parse(chi.URLParam(r, "versionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_version_id", "version_id must be a valid UUID")
			return
		}

		version, err := service.GetVersion(r.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrAgentBuildVersionNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent build version not found")
				return
			}
			logger.Error("get agent build version request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if err := ensureCallerCanAccessVersionWorkspace(r.Context(), caller, service, version); err != nil {
			writeAuthzError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, buildAgentBuildVersionResponse(version))
	}
}

func updateAgentBuildVersionHandler(logger *slog.Logger, service AgentBuildService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		id, err := uuid.Parse(chi.URLParam(r, "versionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_version_id", "version_id must be a valid UUID")
			return
		}
		currentVersion, err := service.GetVersion(r.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrAgentBuildVersionNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent build version not found")
				return
			}
			handleServiceError(w, logger, r, err)
			return
		}
		if err := ensureCallerCanAccessVersionWorkspace(r.Context(), caller, service, currentVersion); err != nil {
			writeAuthzError(w, err)
			return
		}

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxAgentBuildRequestBytes)

		var body createAgentBuildVersionRequest
		if err := decodeJSON(r, &body); err != nil {
			handleDecodeError(w, logger, r, err)
			return
		}
		input, err := decodeCreateAgentBuildVersionInput(body)
		if err != nil {
			handleServiceError(w, logger, r, err)
			return
		}

		version, err := service.UpdateVersion(r.Context(), id, input)
		if err != nil {
			if errors.Is(err, repository.ErrAgentBuildVersionNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent build version not found")
				return
			}
			handleServiceError(w, logger, r, err)
			return
		}

		writeJSON(w, http.StatusOK, buildAgentBuildVersionResponse(version))
	}
}

func validateAgentBuildVersionHandler(logger *slog.Logger, service AgentBuildService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		id, err := uuid.Parse(chi.URLParam(r, "versionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_version_id", "version_id must be a valid UUID")
			return
		}
		version, err := service.GetVersion(r.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrAgentBuildVersionNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent build version not found")
				return
			}
			handleServiceError(w, logger, r, err)
			return
		}
		if err := ensureCallerCanAccessVersionWorkspace(r.Context(), caller, service, version); err != nil {
			writeAuthzError(w, err)
			return
		}

		result, err := service.ValidateVersion(r.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrAgentBuildVersionNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent build version not found")
				return
			}
			logger.Error("validate agent build version request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		errs := result.Errors
		if errs == nil {
			errs = []validationErrorDetail{}
		}

		writeJSON(w, http.StatusOK, validateBuildVersionResponse{
			Valid:  result.Valid,
			Errors: errs,
		})
	}
}

func markAgentBuildVersionReadyHandler(logger *slog.Logger, service AgentBuildService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		id, err := uuid.Parse(chi.URLParam(r, "versionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_version_id", "version_id must be a valid UUID")
			return
		}
		version, err := service.GetVersion(r.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrAgentBuildVersionNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent build version not found")
				return
			}
			handleServiceError(w, logger, r, err)
			return
		}
		if err := ensureCallerCanAccessVersionWorkspace(r.Context(), caller, service, version); err != nil {
			writeAuthzError(w, err)
			return
		}

		if err := service.MarkVersionReady(r.Context(), id); err != nil {
			if errors.Is(err, repository.ErrAgentBuildVersionNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent build version not found")
				return
			}
			handleServiceError(w, logger, r, err)
			return
		}

		version, err = service.GetVersion(r.Context(), id)
		if err != nil {
			logger.Error("get version after mark ready failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		writeJSON(w, http.StatusOK, buildAgentBuildVersionResponse(version))
	}
}

func createAgentDeploymentHandler(logger *slog.Logger, service AgentBuildService) http.HandlerFunc {
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

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxAgentBuildRequestBytes)

		var body createAgentDeploymentRequest
		if err := decodeJSON(r, &body); err != nil {
			handleDecodeError(w, logger, r, err)
			return
		}

		input, err := decodeCreateAgentDeploymentInput(body)
		if err != nil {
			handleServiceError(w, logger, r, err)
			return
		}

		dep, err := service.CreateDeployment(r.Context(), caller, workspaceID, input)
		if err != nil {
			if errors.Is(err, repository.ErrAgentBuildNotFound) || errors.Is(err, repository.ErrAgentBuildVersionNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent build or version not found")
				return
			}
			handleServiceError(w, logger, r, err)
			return
		}

		writeJSON(w, http.StatusCreated, agentDeploymentCreateResponse{
			ID:                    dep.ID,
			WorkspaceID:           dep.WorkspaceID,
			AgentBuildID:          dep.AgentBuildID,
			CurrentBuildVersionID: dep.CurrentBuildVersionID,
			Name:                  dep.Name,
			Slug:                  dep.Slug,
			DeploymentType:        dep.DeploymentType,
			Status:                dep.Status,
			CreatedAt:             dep.CreatedAt,
			UpdatedAt:             dep.UpdatedAt,
		})
	}
}

// --- Response builders ---

func buildAgentBuildResponse(b repository.AgentBuild) agentBuildResponse {
	return agentBuildResponse{
		ID:              b.ID,
		WorkspaceID:     b.WorkspaceID,
		Name:            b.Name,
		Slug:            b.Slug,
		Description:     b.Description,
		LifecycleStatus: b.LifecycleStatus,
		CreatedAt:       b.CreatedAt,
		UpdatedAt:       b.UpdatedAt,
	}
}

func buildAgentBuildVersionResponse(v repository.AgentBuildVersion) agentBuildVersionResponse {
	return agentBuildVersionResponse{
		ID:               v.ID,
		AgentBuildID:     v.AgentBuildID,
		VersionNumber:    v.VersionNumber,
		VersionStatus:    v.VersionStatus,
		AgentKind:        v.AgentKind,
		InterfaceSpec:    defaultJSON(v.InterfaceSpec),
		PolicySpec:       defaultJSON(v.PolicySpec),
		ReasoningSpec:    defaultJSON(v.ReasoningSpec),
		MemorySpec:       defaultJSON(v.MemorySpec),
		WorkflowSpec:     defaultJSON(v.WorkflowSpec),
		GuardrailSpec:    defaultJSON(v.GuardrailSpec),
		ModelSpec:        defaultJSON(v.ModelSpec),
		OutputSchema:     defaultJSON(v.OutputSchema),
		TraceContract:    defaultJSON(v.TraceContract),
		PublicationSpec:  defaultJSON(v.PublicationSpec),
		Tools:            buildToolBindingResponses(v.Tools),
		KnowledgeSources: buildKnowledgeSourceBindingResponses(v.KnowledgeSources),
		CreatedAt:        v.CreatedAt,
	}
}

// --- Decode helpers ---

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return err
		}
		if errors.Is(err, io.EOF) {
			return AgentBuildValidationError{
				Code:    "invalid_request",
				Message: "request body is required",
			}
		}
		return AgentBuildValidationError{
			Code:    "invalid_request",
			Message: "request body must be valid JSON",
		}
	}
	return nil
}

func decodeCreateAgentDeploymentInput(body createAgentDeploymentRequest) (CreateAgentDeploymentInput, error) {
	if strings.TrimSpace(body.Name) == "" {
		return CreateAgentDeploymentInput{}, AgentBuildValidationError{
			Code:    "invalid_name",
			Message: "name is required",
		}
	}

	agentBuildID, err := uuid.Parse(body.AgentBuildID)
	if err != nil {
		return CreateAgentDeploymentInput{}, AgentBuildValidationError{
			Code:    "invalid_agent_build_id",
			Message: "agent_build_id must be a valid UUID",
		}
	}

	buildVersionID, err := uuid.Parse(body.BuildVersionID)
	if err != nil {
		return CreateAgentDeploymentInput{}, AgentBuildValidationError{
			Code:    "invalid_build_version_id",
			Message: "build_version_id must be a valid UUID",
		}
	}

	runtimeProfileID, err := uuid.Parse(body.RuntimeProfileID)
	if err != nil {
		return CreateAgentDeploymentInput{}, AgentBuildValidationError{
			Code:    "invalid_runtime_profile_id",
			Message: "runtime_profile_id must be a valid UUID",
		}
	}

	var providerAccountID *uuid.UUID
	if body.ProviderAccountID != nil && strings.TrimSpace(*body.ProviderAccountID) != "" {
		parsed, parseErr := uuid.Parse(*body.ProviderAccountID)
		if parseErr != nil {
			return CreateAgentDeploymentInput{}, AgentBuildValidationError{
				Code:    "invalid_provider_account_id",
				Message: "provider_account_id must be a valid UUID",
			}
		}
		providerAccountID = &parsed
	}

	var modelAliasID *uuid.UUID
	if body.ModelAliasID != nil && strings.TrimSpace(*body.ModelAliasID) != "" {
		parsed, parseErr := uuid.Parse(*body.ModelAliasID)
		if parseErr != nil {
			return CreateAgentDeploymentInput{}, AgentBuildValidationError{
				Code:    "invalid_model_alias_id",
				Message: "model_alias_id must be a valid UUID",
			}
		}
		modelAliasID = &parsed
	}

	return CreateAgentDeploymentInput{
		Name:              body.Name,
		AgentBuildID:      agentBuildID,
		BuildVersionID:    buildVersionID,
		RuntimeProfileID:  runtimeProfileID,
		ProviderAccountID: providerAccountID,
		ModelAliasID:      modelAliasID,
		DeploymentConfig:  body.DeploymentConfig,
	}, nil
}

func decodeCreateAgentBuildVersionInput(body createAgentBuildVersionRequest) (CreateAgentBuildVersionInput, error) {
	tools, err := decodeToolBindings(body.Tools)
	if err != nil {
		return CreateAgentBuildVersionInput{}, err
	}
	knowledgeSources, err := decodeKnowledgeSourceBindings(body.KnowledgeSources)
	if err != nil {
		return CreateAgentBuildVersionInput{}, err
	}

	return CreateAgentBuildVersionInput{
		AgentKind:        body.AgentKind,
		InterfaceSpec:    body.InterfaceSpec,
		PolicySpec:       body.PolicySpec,
		ReasoningSpec:    body.ReasoningSpec,
		MemorySpec:       body.MemorySpec,
		WorkflowSpec:     body.WorkflowSpec,
		GuardrailSpec:    body.GuardrailSpec,
		ModelSpec:        body.ModelSpec,
		OutputSchema:     body.OutputSchema,
		TraceContract:    body.TraceContract,
		PublicationSpec:  body.PublicationSpec,
		Tools:            tools,
		KnowledgeSources: knowledgeSources,
	}, nil
}

func decodeToolBindings(bindings []toolBindingRequest) ([]repository.AgentBuildVersionToolBinding, error) {
	if bindings == nil {
		return []repository.AgentBuildVersionToolBinding{}, nil
	}
	out := make([]repository.AgentBuildVersionToolBinding, 0, len(bindings))
	for _, binding := range bindings {
		toolID, err := uuid.Parse(strings.TrimSpace(binding.ToolID))
		if err != nil {
			return nil, AgentBuildValidationError{
				Code:    "invalid_tool_id",
				Message: "each tools entry must include a valid tool_id",
			}
		}
		out = append(out, repository.AgentBuildVersionToolBinding{
			ToolID:        toolID,
			BindingRole:   binding.BindingRole,
			BindingConfig: binding.BindingConfig,
		})
	}
	return out, nil
}

func decodeKnowledgeSourceBindings(bindings []knowledgeSourceBindingRequest) ([]repository.AgentBuildVersionKnowledgeSourceBinding, error) {
	if bindings == nil {
		return []repository.AgentBuildVersionKnowledgeSourceBinding{}, nil
	}
	out := make([]repository.AgentBuildVersionKnowledgeSourceBinding, 0, len(bindings))
	for _, binding := range bindings {
		knowledgeSourceID, err := uuid.Parse(strings.TrimSpace(binding.KnowledgeSourceID))
		if err != nil {
			return nil, AgentBuildValidationError{
				Code:    "invalid_knowledge_source_id",
				Message: "each knowledge_sources entry must include a valid knowledge_source_id",
			}
		}
		out = append(out, repository.AgentBuildVersionKnowledgeSourceBinding{
			KnowledgeSourceID: knowledgeSourceID,
			BindingRole:       binding.BindingRole,
			BindingConfig:     binding.BindingConfig,
		})
	}
	return out, nil
}

func buildToolBindingResponses(bindings []repository.AgentBuildVersionToolBinding) []toolBindingResponse {
	if bindings == nil {
		return []toolBindingResponse{}
	}
	items := make([]toolBindingResponse, 0, len(bindings))
	for _, binding := range bindings {
		items = append(items, toolBindingResponse{
			ToolID:        binding.ToolID,
			BindingRole:   defaultString(binding.BindingRole, "default"),
			BindingConfig: defaultJSON(binding.BindingConfig),
		})
	}
	return items
}

func buildKnowledgeSourceBindingResponses(bindings []repository.AgentBuildVersionKnowledgeSourceBinding) []knowledgeSourceBindingResponse {
	if bindings == nil {
		return []knowledgeSourceBindingResponse{}
	}
	items := make([]knowledgeSourceBindingResponse, 0, len(bindings))
	for _, binding := range bindings {
		items = append(items, knowledgeSourceBindingResponse{
			KnowledgeSourceID: binding.KnowledgeSourceID,
			BindingRole:       defaultString(binding.BindingRole, "default"),
			BindingConfig:     defaultJSON(binding.BindingConfig),
		})
	}
	return items
}

func ensureCallerCanAccessWorkspace(caller Caller, workspaceID uuid.UUID) error {
	if _, ok := caller.WorkspaceMemberships[workspaceID]; !ok {
		return fmt.Errorf("%w: caller %s does not belong to workspace %s", ErrForbidden, caller.UserID, workspaceID)
	}
	return nil
}

func ensureCallerCanAccessVersionWorkspace(ctx context.Context, caller Caller, service AgentBuildService, version repository.AgentBuildVersion) error {
	build, err := service.GetBuild(ctx, version.AgentBuildID)
	if err != nil {
		return err
	}
	return ensureCallerCanAccessWorkspace(caller, build.WorkspaceID)
}

func handleDecodeError(w http.ResponseWriter, logger *slog.Logger, r *http.Request, err error) {
	var validationErr AgentBuildValidationError
	if errors.As(err, &validationErr) {
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
		return
	}
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body must be 1 MiB or smaller")
		return
	}
	logger.Error("failed to decode request",
		"method", r.Method,
		"path", r.URL.Path,
		"error", err,
	)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func handleServiceError(w http.ResponseWriter, logger *slog.Logger, r *http.Request, err error) {
	var validationErr AgentBuildValidationError
	if errors.As(err, &validationErr) {
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
		return
	}
	if errors.Is(err, ErrForbidden) {
		writeAuthzError(w, err)
		return
	}
	logger.Error("request failed",
		"method", r.Method,
		"path", r.URL.Path,
		"error", err,
	)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

// --- Utility functions ---

func generateSlug(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = slugRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 60 {
		slug = slug[:60]
		slug = strings.TrimRight(slug, "-")
	}
	return slug
}

func defaultJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func defaultJSONOrExisting(input json.RawMessage, existing json.RawMessage) json.RawMessage {
	if len(input) == 0 {
		return existing
	}
	return input
}

func defaultToolBindingsOrExisting(
	input []repository.AgentBuildVersionToolBinding,
	existing []repository.AgentBuildVersionToolBinding,
) []repository.AgentBuildVersionToolBinding {
	if input == nil {
		return existing
	}
	return input
}

func defaultKnowledgeSourceBindingsOrExisting(
	input []repository.AgentBuildVersionKnowledgeSourceBinding,
	existing []repository.AgentBuildVersionKnowledgeSourceBinding,
) []repository.AgentBuildVersionKnowledgeSourceBinding {
	if input == nil {
		return existing
	}
	return input
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func jsonHasKey(raw json.RawMessage, key string) bool {
	if len(raw) == 0 {
		return false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}
