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

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --------------------------------------------------------------------------
// Service Interface
// --------------------------------------------------------------------------

type InfrastructureService interface {
	// Runtime Profiles
	CreateRuntimeProfile(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateRuntimeProfileInput) (repository.RuntimeProfileRow, error)
	ListRuntimeProfiles(ctx context.Context, workspaceID uuid.UUID) ([]repository.RuntimeProfileRow, error)
	GetRuntimeProfile(ctx context.Context, id uuid.UUID) (repository.RuntimeProfileRow, error)
	ArchiveRuntimeProfile(ctx context.Context, id uuid.UUID) error

	// Provider Accounts
	CreateProviderAccount(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateProviderAccountInput) (repository.ProviderAccountRow, error)
	ListProviderAccounts(ctx context.Context, workspaceID uuid.UUID) ([]repository.ProviderAccountRow, error)
	GetProviderAccount(ctx context.Context, id uuid.UUID) (repository.ProviderAccountRow, error)

	// Model Catalog (global, read-only)
	ListModelCatalog(ctx context.Context) ([]repository.ModelCatalogEntryRow, error)
	GetModelCatalogEntry(ctx context.Context, id uuid.UUID) (repository.ModelCatalogEntryRow, error)

	// Model Aliases
	CreateModelAlias(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateModelAliasInput) (repository.ModelAliasRow, error)
	ListModelAliases(ctx context.Context, workspaceID uuid.UUID) ([]repository.ModelAliasRow, error)
	GetModelAlias(ctx context.Context, id uuid.UUID) (repository.ModelAliasRow, error)

	// Tools
	CreateTool(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateToolInput) (repository.ToolRow, error)
	ListTools(ctx context.Context, workspaceID uuid.UUID) ([]repository.ToolRow, error)
	GetTool(ctx context.Context, id uuid.UUID) (repository.ToolRow, error)

	// Knowledge Sources
	CreateKnowledgeSource(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateKnowledgeSourceInput) (repository.KnowledgeSourceRow, error)
	ListKnowledgeSources(ctx context.Context, workspaceID uuid.UUID) ([]repository.KnowledgeSourceRow, error)
	GetKnowledgeSource(ctx context.Context, id uuid.UUID) (repository.KnowledgeSourceRow, error)

	// Routing Policies
	CreateRoutingPolicy(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateRoutingPolicyInput) (repository.RoutingPolicyRow, error)
	ListRoutingPolicies(ctx context.Context, workspaceID uuid.UUID) ([]repository.RoutingPolicyRow, error)

	// Spend Policies
	CreateSpendPolicy(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateSpendPolicyInput) (repository.SpendPolicyRow, error)
	ListSpendPolicies(ctx context.Context, workspaceID uuid.UUID) ([]repository.SpendPolicyRow, error)
}

// --------------------------------------------------------------------------
// Input Types
// --------------------------------------------------------------------------

type CreateRuntimeProfileInput struct {
	Name               string          `json:"name"`
	ExecutionTarget    string          `json:"execution_target"`
	TraceMode          string          `json:"trace_mode,omitempty"`
	MaxIterations      int32           `json:"max_iterations,omitempty"`
	MaxToolCalls       int32           `json:"max_tool_calls,omitempty"`
	StepTimeoutSeconds int32           `json:"step_timeout_seconds,omitempty"`
	RunTimeoutSeconds  int32           `json:"run_timeout_seconds,omitempty"`
	ProfileConfig      json.RawMessage `json:"profile_config,omitempty"`
}

func (i *CreateRuntimeProfileInput) Validate() error {
	return requireFields(map[string]string{"name": i.Name, "execution_target": i.ExecutionTarget})
}

type CreateProviderAccountInput struct {
	ProviderKey         string          `json:"provider_key"`
	Name                string          `json:"name"`
	CredentialReference string          `json:"credential_reference"`
	LimitsConfig        json.RawMessage `json:"limits_config,omitempty"`
}

func (i *CreateProviderAccountInput) Validate() error {
	return requireFields(map[string]string{"provider_key": i.ProviderKey, "name": i.Name, "credential_reference": i.CredentialReference})
}

type CreateModelAliasInput struct {
	AliasKey            string  `json:"alias_key"`
	DisplayName         string  `json:"display_name"`
	ModelCatalogEntryID string  `json:"model_catalog_entry_id"`
	ProviderAccountID   *string `json:"provider_account_id,omitempty"`
}

func (i *CreateModelAliasInput) Validate() error {
	return requireFields(map[string]string{"alias_key": i.AliasKey, "display_name": i.DisplayName, "model_catalog_entry_id": i.ModelCatalogEntryID})
}

type CreateToolInput struct {
	Name          string          `json:"name"`
	ToolKind      string          `json:"tool_kind"`
	CapabilityKey string          `json:"capability_key"`
	Definition    json.RawMessage `json:"definition,omitempty"`
}

func (i *CreateToolInput) Validate() error {
	return requireFields(map[string]string{"name": i.Name, "tool_kind": i.ToolKind, "capability_key": i.CapabilityKey})
}

type CreateKnowledgeSourceInput struct {
	Name             string          `json:"name"`
	SourceKind       string          `json:"source_kind"`
	ConnectionConfig json.RawMessage `json:"connection_config,omitempty"`
}

func (i *CreateKnowledgeSourceInput) Validate() error {
	return requireFields(map[string]string{"name": i.Name, "source_kind": i.SourceKind})
}

type CreateRoutingPolicyInput struct {
	Name       string          `json:"name"`
	PolicyKind string          `json:"policy_kind"`
	Config     json.RawMessage `json:"config,omitempty"`
}

func (i *CreateRoutingPolicyInput) Validate() error {
	return requireFields(map[string]string{"name": i.Name, "policy_kind": i.PolicyKind})
}

type CreateSpendPolicyInput struct {
	Name         string          `json:"name"`
	CurrencyCode string          `json:"currency_code,omitempty"`
	WindowKind   string          `json:"window_kind"`
	SoftLimit    *float64        `json:"soft_limit,omitempty"`
	HardLimit    *float64        `json:"hard_limit,omitempty"`
	Config       json.RawMessage `json:"config,omitempty"`
}

func (i *CreateSpendPolicyInput) Validate() error {
	return requireFields(map[string]string{"name": i.Name, "window_kind": i.WindowKind})
}

func requireFields(fields map[string]string) error {
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	return nil
}

// --------------------------------------------------------------------------
// Response Types
// --------------------------------------------------------------------------

type runtimeProfileResponse struct {
	ID                 uuid.UUID       `json:"id"`
	WorkspaceID        *uuid.UUID      `json:"workspace_id,omitempty"`
	Name               string          `json:"name"`
	Slug               string          `json:"slug"`
	ExecutionTarget    string          `json:"execution_target"`
	TraceMode          string          `json:"trace_mode"`
	MaxIterations      int32           `json:"max_iterations"`
	MaxToolCalls       int32           `json:"max_tool_calls"`
	StepTimeoutSeconds int32           `json:"step_timeout_seconds"`
	RunTimeoutSeconds  int32           `json:"run_timeout_seconds"`
	ProfileConfig      json.RawMessage `json:"profile_config"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type providerAccountResponse struct {
	ID                  uuid.UUID       `json:"id"`
	WorkspaceID         *uuid.UUID      `json:"workspace_id,omitempty"`
	ProviderKey         string          `json:"provider_key"`
	Name                string          `json:"name"`
	CredentialReference string          `json:"credential_reference"`
	Status              string          `json:"status"`
	LimitsConfig        json.RawMessage `json:"limits_config"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type modelCatalogResponse struct {
	ID              uuid.UUID       `json:"id"`
	ProviderKey     string          `json:"provider_key"`
	ProviderModelID string          `json:"provider_model_id"`
	DisplayName     string          `json:"display_name"`
	ModelFamily     string          `json:"model_family"`
	Modality        string          `json:"modality"`
	LifecycleStatus string          `json:"lifecycle_status"`
	Metadata        json.RawMessage `json:"metadata"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type modelAliasResponse struct {
	ID                  uuid.UUID  `json:"id"`
	WorkspaceID         *uuid.UUID `json:"workspace_id,omitempty"`
	ProviderAccountID   *uuid.UUID `json:"provider_account_id,omitempty"`
	ModelCatalogEntryID uuid.UUID  `json:"model_catalog_entry_id"`
	AliasKey            string     `json:"alias_key"`
	DisplayName         string     `json:"display_name"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type toolResponse struct {
	ID              uuid.UUID       `json:"id"`
	WorkspaceID     *uuid.UUID      `json:"workspace_id,omitempty"`
	Name            string          `json:"name"`
	Slug            string          `json:"slug"`
	ToolKind        string          `json:"tool_kind"`
	CapabilityKey   string          `json:"capability_key"`
	Definition      json.RawMessage `json:"definition"`
	LifecycleStatus string          `json:"lifecycle_status"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type knowledgeSourceResponse struct {
	ID               uuid.UUID       `json:"id"`
	WorkspaceID      *uuid.UUID      `json:"workspace_id,omitempty"`
	Name             string          `json:"name"`
	Slug             string          `json:"slug"`
	SourceKind       string          `json:"source_kind"`
	ConnectionConfig json.RawMessage `json:"connection_config"`
	LifecycleStatus  string          `json:"lifecycle_status"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type routingPolicyResponse struct {
	ID          uuid.UUID       `json:"id"`
	WorkspaceID *uuid.UUID      `json:"workspace_id,omitempty"`
	Name        string          `json:"name"`
	PolicyKind  string          `json:"policy_kind"`
	Config      json.RawMessage `json:"config"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type spendPolicyResponse struct {
	ID           uuid.UUID       `json:"id"`
	WorkspaceID  *uuid.UUID      `json:"workspace_id,omitempty"`
	Name         string          `json:"name"`
	CurrencyCode string          `json:"currency_code"`
	WindowKind   string          `json:"window_kind"`
	SoftLimit    *float64        `json:"soft_limit,omitempty"`
	HardLimit    *float64        `json:"hard_limit,omitempty"`
	Config       json.RawMessage `json:"config"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// --------------------------------------------------------------------------
// Generic Handlers (DRY pattern for workspace-scoped create/list)
// --------------------------------------------------------------------------

// Validatable can be implemented by input types to provide field validation.
type Validatable interface {
	Validate() error
}

func infraCreateHandler[Input any, Row any, Resp any](
	logger *slog.Logger,
	authorizer WorkspaceAuthorizer,
	create func(ctx context.Context, caller Caller, wsID uuid.UUID, input Input) (Row, error),
	toResponse func(Row) Resp,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		wsID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "workspace ID required")
			return
		}
		if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, wsID, ActionManageInfrastructure); err != nil {
			writeAuthzError(w, err)
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		var input Input
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
			return
		}
		if v, ok := any(&input).(Validatable); ok {
			if err := v.Validate(); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", err.Error())
				return
			}
		}
		row, err := create(r.Context(), caller, wsID, input)
		if err != nil {
			if errors.Is(err, repository.ErrSlugTaken) {
				writeError(w, http.StatusConflict, "slug_taken", "a resource with that name already exists")
				return
			}
			logger.Error("create failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create resource")
			return
		}
		writeJSON(w, http.StatusCreated, toResponse(row))
	}
}

func infraListHandler[Row any, Resp any](
	logger *slog.Logger,
	list func(ctx context.Context, wsID uuid.UUID) ([]Row, error),
	toResponse func(Row) Resp,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wsID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "workspace ID required")
			return
		}
		rows, err := list(r.Context(), wsID)
		if err != nil {
			logger.Error("list failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list resources")
			return
		}
		items := make([]Resp, len(rows))
		for i, row := range rows {
			items[i] = toResponse(row)
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

// WorkspaceOwned is implemented by row types that carry a workspace ID for authorization.
type WorkspaceOwned interface {
	GetWorkspaceID() *uuid.UUID
}

func infraGetHandler[Row WorkspaceOwned, Resp any](
	logger *slog.Logger,
	authorizer WorkspaceAuthorizer,
	paramName string,
	get func(ctx context.Context, id uuid.UUID) (Row, error),
	toResponse func(Row) Resp,
	notFoundCode string,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		raw := chi.URLParam(r, paramName)
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid ID")
			return
		}
		row, err := get(r.Context(), id)
		if err != nil {
			if isInfraNotFoundErr(err) {
				writeError(w, http.StatusNotFound, "not_found", notFoundCode+" not found")
				return
			}
			logger.Error("get failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to get resource")
			return
		}

		// Authorize: caller must have access to the resource's workspace
		if wsID := row.GetWorkspaceID(); wsID != nil {
			if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, *wsID, ActionReadWorkspace); err != nil {
				writeAuthzError(w, err)
				return
			}
		}

		writeJSON(w, http.StatusOK, toResponse(row))
	}
}

func isInfraNotFoundErr(err error) bool {
	return errors.Is(err, repository.ErrRuntimeProfileNotFound) ||
		errors.Is(err, repository.ErrProviderAccountNotFound) ||
		errors.Is(err, repository.ErrModelAliasNotFound) ||
		errors.Is(err, repository.ErrModelCatalogNotFound) ||
		errors.Is(err, repository.ErrToolNotFound) ||
		errors.Is(err, repository.ErrKnowledgeSourceNotFound) ||
		errors.Is(err, repository.ErrRoutingPolicyNotFound) ||
		errors.Is(err, repository.ErrSpendPolicyNotFound)
}

// --------------------------------------------------------------------------
// Response Mappers
// --------------------------------------------------------------------------

func mapRuntimeProfile(r repository.RuntimeProfileRow) runtimeProfileResponse {
	return runtimeProfileResponse{
		ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Slug: r.Slug,
		ExecutionTarget: r.ExecutionTarget, TraceMode: r.TraceMode,
		MaxIterations: r.MaxIterations, MaxToolCalls: r.MaxToolCalls,
		StepTimeoutSeconds: r.StepTimeoutSeconds, RunTimeoutSeconds: r.RunTimeoutSeconds,
		ProfileConfig: r.ProfileConfig, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapProviderAccount(r repository.ProviderAccountRow) providerAccountResponse {
	return providerAccountResponse{
		ID: r.ID, WorkspaceID: r.WorkspaceID, ProviderKey: r.ProviderKey, Name: r.Name,
		CredentialReference: r.CredentialReference, Status: r.Status, LimitsConfig: r.LimitsConfig,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapModelCatalog(r repository.ModelCatalogEntryRow) modelCatalogResponse {
	return modelCatalogResponse{
		ID: r.ID, ProviderKey: r.ProviderKey, ProviderModelID: r.ProviderModelID,
		DisplayName: r.DisplayName, ModelFamily: r.ModelFamily, Modality: r.Modality,
		LifecycleStatus: r.LifecycleStatus, Metadata: r.Metadata,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapModelAlias(r repository.ModelAliasRow) modelAliasResponse {
	return modelAliasResponse{
		ID: r.ID, WorkspaceID: r.WorkspaceID, ProviderAccountID: r.ProviderAccountID,
		ModelCatalogEntryID: r.ModelCatalogEntryID, AliasKey: r.AliasKey, DisplayName: r.DisplayName,
		Status: r.Status, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapTool(r repository.ToolRow) toolResponse {
	return toolResponse{
		ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Slug: r.Slug,
		ToolKind: r.ToolKind, CapabilityKey: r.CapabilityKey, Definition: r.Definition,
		LifecycleStatus: r.LifecycleStatus, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapKnowledgeSource(r repository.KnowledgeSourceRow) knowledgeSourceResponse {
	return knowledgeSourceResponse{
		ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Slug: r.Slug,
		SourceKind: r.SourceKind, ConnectionConfig: r.ConnectionConfig,
		LifecycleStatus: r.LifecycleStatus, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapRoutingPolicy(r repository.RoutingPolicyRow) routingPolicyResponse {
	return routingPolicyResponse{
		ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, PolicyKind: r.PolicyKind,
		Config: r.Config, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapSpendPolicy(r repository.SpendPolicyRow) spendPolicyResponse {
	return spendPolicyResponse{
		ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, CurrencyCode: r.CurrencyCode,
		WindowKind: r.WindowKind, SoftLimit: r.SoftLimit, HardLimit: r.HardLimit,
		Config: r.Config, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// --------------------------------------------------------------------------
// Model Catalog Handlers (global, no workspace scope)
// --------------------------------------------------------------------------

func listModelCatalogHandler(logger *slog.Logger, svc InfrastructureService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := svc.ListModelCatalog(r.Context())
		if err != nil {
			logger.Error("list model catalog failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list model catalog")
			return
		}
		items := make([]modelCatalogResponse, len(entries))
		for i, e := range entries {
			items[i] = mapModelCatalog(e)
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func getModelCatalogEntryHandler(logger *slog.Logger, svc InfrastructureService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := chi.URLParam(r, "entryID")
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid ID")
			return
		}
		entry, err := svc.GetModelCatalogEntry(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "model catalog entry not found")
			return
		}
		writeJSON(w, http.StatusOK, mapModelCatalog(entry))
	}
}

// --------------------------------------------------------------------------
// Archive handler for runtime profiles
// --------------------------------------------------------------------------

func archiveRuntimeProfileHandler(logger *slog.Logger, svc InfrastructureService, authorizer WorkspaceAuthorizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		raw := chi.URLParam(r, "profileID")
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid ID")
			return
		}

		// Fetch first to get workspace ID for authorization
		profile, err := svc.GetRuntimeProfile(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "runtime profile not found")
			return
		}
		if profile.WorkspaceID != nil {
			if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, *profile.WorkspaceID, ActionManageInfrastructure); err != nil {
				writeAuthzError(w, err)
				return
			}
		}

		if err := svc.ArchiveRuntimeProfile(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, "not_found", "runtime profile not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
