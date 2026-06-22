package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/toolspec"
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
	DeleteProviderAccount(ctx context.Context, id uuid.UUID) error
	TestProviderAccount(ctx context.Context, account repository.ProviderAccountRow, input ProviderAccountTestInput) (ProviderAccountTestResult, error)
	ListProviderAccountModels(ctx context.Context, account repository.ProviderAccountRow) ([]provider.ModelInfo, error)

	// Tools
	CreateTool(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateToolInput) (repository.ToolRow, error)
	CreateToolsFromLibrary(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateToolsFromLibraryInput) ([]repository.ToolRow, []LibrarySkip, error)
	ListTools(ctx context.Context, workspaceID uuid.UUID) ([]repository.ToolRow, error)
	GetTool(ctx context.Context, id uuid.UUID) (repository.ToolRow, error)
	UpdateTool(ctx context.Context, caller Caller, id uuid.UUID, input UpdateToolInput) (repository.ToolRow, error)
	DeleteTool(ctx context.Context, id uuid.UUID) error

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
	APIKey              string          `json:"api_key"`
	LimitsConfig        json.RawMessage `json:"limits_config,omitempty"`
}

type ProviderAccountTestInput struct {
	Model              string `json:"model,omitempty"`
	StepTimeoutSeconds int32  `json:"step_timeout_seconds,omitempty"`
}

func (i *CreateProviderAccountInput) Validate() error {
	if err := requireFields(map[string]string{"provider_key": i.ProviderKey, "name": i.Name}); err != nil {
		return err
	}
	if i.CredentialReference == "" && i.APIKey == "" {
		return fmt.Errorf("either api_key or credential_reference is required")
	}
	return nil
}

type CreateToolInput struct {
	Name          string          `json:"name"`
	ToolKind      string          `json:"tool_kind"`
	CapabilityKey string          `json:"capability_key,omitempty"`
	Definition    json.RawMessage `json:"definition,omitempty"`
}

func (i *CreateToolInput) Validate() error {
	if err := requireFields(map[string]string{"name": i.Name, "tool_kind": i.ToolKind}); err != nil {
		return err
	}
	return validateToolKind(i.ToolKind)
}

// FromLibraryEntryInput requests one library tool be added to the workspace.
type FromLibraryEntryInput struct {
	Slug string `json:"slug"`
	// Variant selects "default" (instantly-usable, default) or "live" (the real
	// API call, which needs the tool's secret + a network allowlist).
	Variant string `json:"variant,omitempty"`
	// Conflict is "skip" (default) or "suffix" when a tool with the same slug exists.
	Conflict string `json:"conflict,omitempty"`
}

// CreateToolsFromLibraryInput adds one or more library tools to a workspace.
type CreateToolsFromLibraryInput struct {
	Entries []FromLibraryEntryInput `json:"entries"`
}

const (
	maxToolsFromLibraryEntries      = 100
	maxToolsFromLibraryRequestBytes = 64 << 10
)

func (i *CreateToolsFromLibraryInput) Validate() error {
	if len(i.Entries) == 0 {
		return fmt.Errorf("entries is required")
	}
	if len(i.Entries) > maxToolsFromLibraryEntries {
		return fmt.Errorf("entries must contain at most %d items", maxToolsFromLibraryEntries)
	}
	for _, e := range i.Entries {
		if strings.TrimSpace(e.Slug) == "" {
			return fmt.Errorf("each entry requires a slug")
		}
		switch e.Variant {
		case "", "default", "live":
		default:
			return fmt.Errorf("variant must be \"default\" or \"live\"")
		}
		switch e.Conflict {
		case "", "skip", "suffix":
		default:
			return fmt.Errorf("conflict must be \"skip\" or \"suffix\"")
		}
	}
	return nil
}

// LibrarySkip reports a library entry that was not added and why.
type LibrarySkip struct {
	Slug   string `json:"slug"`
	Reason string `json:"reason"`
}

// UpdateToolInput carries the mutable fields of a tool. ToolKind and slug are
// immutable; name, capability_key and lifecycle_status default to their existing
// values when left empty.
type UpdateToolInput struct {
	Name            string          `json:"name,omitempty"`
	CapabilityKey   string          `json:"capability_key,omitempty"`
	Definition      json.RawMessage `json:"definition,omitempty"`
	LifecycleStatus string          `json:"lifecycle_status,omitempty"`
}

func validateToolKind(kind string) error {
	switch kind {
	case toolspec.ToolTypePrimitive, toolspec.ToolTypeComposed:
		return nil
	default:
		return fmt.Errorf("tool_kind must be %q or %q", toolspec.ToolTypePrimitive, toolspec.ToolTypeComposed)
	}
}

// ToolDefinitionError wraps validation problems with a tool definition so handlers
// can return a 400 with field-level detail instead of a 500.
type ToolDefinitionError struct {
	Errors toolspec.ValidationErrors
}

func (e *ToolDefinitionError) Error() string { return e.Errors.Error() }

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

type providerAccountTestResponse struct {
	AccountID       uuid.UUID `json:"account_id"`
	ProviderKey     string    `json:"provider_key"`
	Model           string    `json:"model"`
	ProviderModelID string    `json:"provider_model_id,omitempty"`
	Passed          bool      `json:"passed"`
	Status          string    `json:"status"`
	Code            string    `json:"code,omitempty"`
	Message         string    `json:"message,omitempty"`
	Retryable       bool      `json:"retryable,omitempty"`
	DurationMS      int64     `json:"duration_ms"`
}

type ProviderAccountTestResult = providerAccountTestResponse

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
			var toolDefErr *ToolDefinitionError
			if errors.As(err, &toolDefErr) {
				writeError(w, http.StatusBadRequest, "validation_error", toolDefErr.Error())
				return
			}
			if errors.Is(err, repository.ErrSlugTaken) {
				writeError(w, http.StatusConflict, "slug_taken", "a resource with that name already exists")
				return
			}
			if errors.Is(err, repository.ErrModelCatalogNotFound) {
				writeError(w, http.StatusBadRequest, "validation_error", "model_catalog_entry_id must reference an existing model catalog entry")
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

func infraDeleteHandler[Row WorkspaceOwned](
	logger *slog.Logger,
	authorizer WorkspaceAuthorizer,
	paramName string,
	get func(ctx context.Context, id uuid.UUID) (Row, error),
	del func(ctx context.Context, id uuid.UUID) error,
	resourceName string,
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

		// Fetch first to get workspace ID for authorization.
		row, err := get(r.Context(), id)
		if err != nil {
			if isInfraNotFoundErr(err) {
				writeError(w, http.StatusNotFound, "not_found", resourceName+" not found")
				return
			}
			logger.Error("get for delete failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to get resource")
			return
		}

		// A nil workspace means this row cannot be authorized via workspace
		// membership. Rather than silently skipping authorization (which would
		// let any authenticated caller delete global resources), deny it.
		wsID := row.GetWorkspaceID()
		if wsID == nil {
			writeError(w, http.StatusNotFound, "not_found", resourceName+" not found")
			return
		}
		if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, *wsID, ActionManageInfrastructure); err != nil {
			writeAuthzError(w, err)
			return
		}

		if err := del(r.Context(), id); err != nil {
			if isInfraNotFoundErr(err) {
				writeError(w, http.StatusNotFound, "not_found", resourceName+" not found")
				return
			}
			logger.Error("delete failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete resource")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func testProviderAccountHandler(logger *slog.Logger, authorizer WorkspaceAuthorizer, svc InfrastructureService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		accountID, err := uuid.Parse(chi.URLParam(r, "accountID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid ID")
			return
		}
		account, err := svc.GetProviderAccount(r.Context(), accountID)
		if err != nil {
			if isInfraNotFoundErr(err) {
				writeError(w, http.StatusNotFound, "not_found", "provider account not found")
				return
			}
			logger.Error("get provider account for smoke test failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to get provider account")
			return
		}
		if account.WorkspaceID != nil {
			if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, *account.WorkspaceID, ActionManageInfrastructure); err != nil {
				writeAuthzError(w, err)
				return
			}
		} else {
			writeError(w, http.StatusNotFound, "not_found", "provider account not found")
			return
		}

		var input ProviderAccountTestInput
		if r.Body != nil && r.ContentLength != 0 {
			if err := requireJSONContentType(r); err != nil {
				writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
				writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
				return
			}
		}

		result, err := svc.TestProviderAccount(r.Context(), account, input)
		if err != nil {
			logger.Error("provider account smoke test failed internally", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to test provider account")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

type providerConnectionModelsResponse struct {
	Items []providerConnectionModel `json:"items"`
}

type providerConnectionModel struct {
	ID                string  `json:"id"`
	DisplayName       string  `json:"display_name"`
	InputCostPerMTok  float64 `json:"input_cost_per_mtok"`
	OutputCostPerMTok float64 `json:"output_cost_per_mtok"`
	PricingSource     string  `json:"pricing_source"`
}

// listProviderAccountModelsHandler returns the live model list reachable with a
// provider connection's credential, for use in model pickers.
func listProviderAccountModelsHandler(logger *slog.Logger, authorizer WorkspaceAuthorizer, svc InfrastructureService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		accountID, err := uuid.Parse(chi.URLParam(r, "accountID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid ID")
			return
		}
		account, err := svc.GetProviderAccount(r.Context(), accountID)
		if err != nil {
			if isInfraNotFoundErr(err) {
				writeError(w, http.StatusNotFound, "not_found", "provider account not found")
				return
			}
			logger.Error("get provider account for model listing failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to get provider account")
			return
		}
		if account.WorkspaceID == nil {
			writeError(w, http.StatusNotFound, "not_found", "provider account not found")
			return
		}
		if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, *account.WorkspaceID, ActionManageInfrastructure); err != nil {
			writeAuthzError(w, err)
			return
		}

		models, err := svc.ListProviderAccountModels(r.Context(), account)
		if err != nil {
			// Provider-side failures (bad key, provider down, capability not
			// supported) map to 502; the failure code is surfaced without the raw
			// provider message to avoid leaking credentials.
			if failure, ok := provider.AsFailure(err); ok {
				writeError(w, http.StatusBadGateway, string(failure.Code), "provider returned an error while listing models")
				return
			}
			logger.Error("list provider account models failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list models")
			return
		}

		items := make([]providerConnectionModel, len(models))
		for i, m := range models {
			items[i] = providerConnectionModel{
				ID:                m.ID,
				DisplayName:       m.DisplayName,
				InputCostPerMTok:  m.InputCostPerMTok,
				OutputCostPerMTok: m.OutputCostPerMTok,
				PricingSource:     m.PricingSource,
			}
		}
		writeJSON(w, http.StatusOK, providerConnectionModelsResponse{Items: items})
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

// --------------------------------------------------------------------------
// Tools: update + primitive catalog
// --------------------------------------------------------------------------

func updateToolHandler(logger *slog.Logger, authorizer WorkspaceAuthorizer, svc InfrastructureService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "toolID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid ID")
			return
		}
		// Fetch first to authorize against the tool's workspace.
		existing, err := svc.GetTool(r.Context(), id)
		if err != nil {
			if isInfraNotFoundErr(err) {
				writeError(w, http.StatusNotFound, "not_found", "tool not found")
				return
			}
			logger.Error("get tool for update failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to get tool")
			return
		}
		if existing.WorkspaceID != nil {
			if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, *existing.WorkspaceID, ActionManageInfrastructure); err != nil {
				writeAuthzError(w, err)
				return
			}
		} else {
			writeError(w, http.StatusNotFound, "not_found", "tool not found")
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		var input UpdateToolInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
			return
		}
		// Only active/disabled are settable here; archiving goes through DELETE.
		// Reject unknown values up front so an invalid status is a 400, not a 500
		// from the DB CHECK constraint, and so "archived" can't create a row whose
		// lifecycle_status disagrees with archived_at.
		switch strings.TrimSpace(input.LifecycleStatus) {
		case "", "active", "disabled":
		default:
			writeError(w, http.StatusBadRequest, "validation_error", `lifecycle_status must be "active" or "disabled"`)
			return
		}
		row, err := svc.UpdateTool(r.Context(), caller, id, input)
		if err != nil {
			var toolDefErr *ToolDefinitionError
			if errors.As(err, &toolDefErr) {
				writeError(w, http.StatusBadRequest, "validation_error", toolDefErr.Error())
				return
			}
			if isInfraNotFoundErr(err) {
				writeError(w, http.StatusNotFound, "not_found", "tool not found")
				return
			}
			logger.Error("update tool failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to update tool")
			return
		}
		writeJSON(w, http.StatusOK, mapTool(row))
	}
}

type toolPrimitiveResponse struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Kind        string          `json:"kind"`
	Parameters  json.RawMessage `json:"parameters"`
	Delegatable bool            `json:"delegatable"`
}

// listToolPrimitivesHandler returns the static catalog of base primitives a
// custom tool can delegate to or compose. Global and read-only.
func listToolPrimitivesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prims := toolspec.Primitives()
		items := make([]toolPrimitiveResponse, len(prims))
		for i, p := range prims {
			items[i] = toolPrimitiveResponse{
				Name:        p.Name,
				Description: p.Description,
				Kind:        string(p.Kind),
				Parameters:  p.Parameters,
				Delegatable: p.Delegatable,
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

type toolLibraryEntryResponse struct {
	Slug           string          `json:"slug"`
	Name           string          `json:"name"`
	Category       string          `json:"category"`
	Description    string          `json:"description"`
	Tags           []string        `json:"tags"`
	ToolKind       string          `json:"tool_kind"`
	Delivery       string          `json:"delivery"`
	RequiresSecret string          `json:"requires_secret,omitempty"`
	HasLive        bool            `json:"has_live"`
	Definition     json.RawMessage `json:"definition"`
}

func mapLibraryEntry(e toolspec.LibraryEntry) toolLibraryEntryResponse {
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	return toolLibraryEntryResponse{
		Slug:           e.Slug,
		Name:           e.Name,
		Category:       e.Category,
		Description:    e.Description,
		Tags:           tags,
		ToolKind:       e.ToolKind,
		Delivery:       e.Delivery,
		RequiresSecret: e.RequiresSecret,
		HasLive:        e.HasLive(),
		Definition:     e.Definition,
	}
}

// listToolLibraryHandler returns the global, read-only catalog of prebuilt tools
// (optionally filtered by ?category=). Instantiate entries with the from-library
// endpoint. Global and read-only.
func listToolLibraryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		category := strings.TrimSpace(r.URL.Query().Get("category"))
		entries := toolspec.Library()
		items := make([]toolLibraryEntryResponse, 0, len(entries))
		for _, e := range entries {
			if category != "" && e.Category != category {
				continue
			}
			items = append(items, mapLibraryEntry(e))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items":      items,
			"categories": toolspec.LibraryCategories(),
		})
	}
}

// createToolsFromLibraryHandler instantiates one or more library tools into the
// workspace as real, editable tools. Returns the created tools plus any skipped
// (e.g. already added). Bulk, so it doesn't use the single-row infraCreateHandler.
func createToolsFromLibraryHandler(logger *slog.Logger, authorizer WorkspaceAuthorizer, svc InfrastructureService) http.HandlerFunc {
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
		r.Body = http.MaxBytesReader(w, r.Body, maxToolsFromLibraryRequestBytes)
		var input CreateToolsFromLibraryInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
			return
		}
		if err := input.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		created, skipped, err := svc.CreateToolsFromLibrary(r.Context(), caller, wsID, input)
		if err != nil {
			var toolDefErr *ToolDefinitionError
			if errors.As(err, &toolDefErr) {
				writeError(w, http.StatusBadRequest, "validation_error", toolDefErr.Error())
				return
			}
			logger.Error("create tools from library failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to add tools from library")
			return
		}
		items := make([]toolResponse, len(created))
		for i, row := range created {
			items[i] = mapTool(row)
		}
		if skipped == nil {
			skipped = []LibrarySkip{}
		}
		writeJSON(w, http.StatusCreated, map[string]any{"items": items, "skipped": skipped})
	}
}
