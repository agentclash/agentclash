package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Sentinel errors for infrastructure resources.
var (
	ErrRuntimeProfileNotFound  = errors.New("runtime profile not found")
	ErrProviderAccountNotFound = errors.New("provider account not found")
	ErrModelAliasNotFound      = errors.New("model alias not found")
	ErrModelCatalogNotFound    = errors.New("model catalog entry not found")
	ErrToolNotFound            = errors.New("tool not found")
	ErrKnowledgeSourceNotFound = errors.New("knowledge source not found")
	ErrRoutingPolicyNotFound   = errors.New("routing policy not found")
	ErrSpendPolicyNotFound     = errors.New("spend policy not found")
)

// --------------------------------------------------------------------------
// Runtime Profiles
// --------------------------------------------------------------------------

type RuntimeProfileRow struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	WorkspaceID        *uuid.UUID
	Name               string
	Slug               string
	ExecutionTarget    string
	TraceMode          string
	MaxIterations      int32
	MaxToolCalls       int32
	StepTimeoutSeconds int32
	RunTimeoutSeconds  int32
	ProfileConfig      json.RawMessage
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateRuntimeProfileParams struct {
	OrganizationID     uuid.UUID
	WorkspaceID        uuid.UUID
	Name               string
	Slug               string
	ExecutionTarget    string
	TraceMode          string
	MaxIterations      int32
	MaxToolCalls       int32
	StepTimeoutSeconds int32
	RunTimeoutSeconds  int32
	ProfileConfig      json.RawMessage
}

func (r *Repository) CreateRuntimeProfile(ctx context.Context, p CreateRuntimeProfileParams) (RuntimeProfileRow, error) {
	var row RuntimeProfileRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		INSERT INTO runtime_profiles (organization_id, workspace_id, name, slug, execution_target, trace_mode,
			max_iterations, max_tool_calls, step_timeout_seconds, run_timeout_seconds, profile_config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, organization_id, workspace_id, name, slug, execution_target, trace_mode,
			max_iterations, max_tool_calls, step_timeout_seconds, run_timeout_seconds, profile_config, created_at, updated_at
	`, p.OrganizationID, p.WorkspaceID, p.Name, p.Slug, p.ExecutionTarget,
		defaultStr(p.TraceMode, "required"),
		defaultInt32(p.MaxIterations, 1),
		defaultInt32(p.MaxToolCalls, 0),
		defaultInt32(p.StepTimeoutSeconds, 60),
		defaultInt32(p.RunTimeoutSeconds, 300),
		defaultRawJSON(p.ProfileConfig),
	).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.Slug, &row.ExecutionTarget,
		&row.TraceMode, &row.MaxIterations, &row.MaxToolCalls, &row.StepTimeoutSeconds, &row.RunTimeoutSeconds,
		&row.ProfileConfig, &createdAt, &updatedAt)
	if err != nil {
		if isDuplicateSlug(err) {
			return RuntimeProfileRow{}, ErrSlugTaken
		}
		return RuntimeProfileRow{}, fmt.Errorf("create runtime profile: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) GetRuntimeProfileByID(ctx context.Context, id uuid.UUID) (RuntimeProfileRow, error) {
	var row RuntimeProfileRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, workspace_id, name, slug, execution_target, trace_mode,
			max_iterations, max_tool_calls, step_timeout_seconds, run_timeout_seconds, profile_config, created_at, updated_at
		FROM runtime_profiles WHERE id = $1 AND archived_at IS NULL
	`, id).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.Slug, &row.ExecutionTarget,
		&row.TraceMode, &row.MaxIterations, &row.MaxToolCalls, &row.StepTimeoutSeconds, &row.RunTimeoutSeconds,
		&row.ProfileConfig, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RuntimeProfileRow{}, ErrRuntimeProfileNotFound
		}
		return RuntimeProfileRow{}, fmt.Errorf("get runtime profile: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) ListRuntimeProfilesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]RuntimeProfileRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, workspace_id, name, slug, execution_target, trace_mode,
			max_iterations, max_tool_calls, step_timeout_seconds, run_timeout_seconds, profile_config, created_at, updated_at
		FROM runtime_profiles
		WHERE workspace_id = $1 AND archived_at IS NULL
		ORDER BY name
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list runtime profiles: %w", err)
	}
	defer rows.Close()
	return scanRuntimeProfiles(rows)
}

func (r *Repository) ArchiveRuntimeProfile(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE runtime_profiles SET archived_at = now() WHERE id = $1 AND archived_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("archive runtime profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRuntimeProfileNotFound
	}
	return nil
}

func scanRuntimeProfiles(rows pgx.Rows) ([]RuntimeProfileRow, error) {
	var result []RuntimeProfileRow
	for rows.Next() {
		var row RuntimeProfileRow
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.Slug, &row.ExecutionTarget,
			&row.TraceMode, &row.MaxIterations, &row.MaxToolCalls, &row.StepTimeoutSeconds, &row.RunTimeoutSeconds,
			&row.ProfileConfig, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan runtime profile: %w", err)
		}
		row.CreatedAt = createdAt.Time
		row.UpdatedAt = updatedAt.Time
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runtime profiles: %w", err)
	}
	if result == nil {
		result = []RuntimeProfileRow{}
	}
	return result, nil
}

// --------------------------------------------------------------------------
// Provider Accounts
// --------------------------------------------------------------------------

type ProviderAccountRow struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	WorkspaceID         *uuid.UUID
	ProviderKey         string
	Name                string
	CredentialReference string
	Status              string
	LimitsConfig        json.RawMessage
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CreateProviderAccountParams struct {
	OrganizationID      uuid.UUID
	WorkspaceID         uuid.UUID
	ProviderKey         string
	Name                string
	CredentialReference string
	LimitsConfig        json.RawMessage
}

func (r *Repository) CreateProviderAccount(ctx context.Context, p CreateProviderAccountParams) (ProviderAccountRow, error) {
	var row ProviderAccountRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		INSERT INTO provider_accounts (organization_id, workspace_id, provider_key, name, credential_reference, limits_config)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, organization_id, workspace_id, provider_key, name, credential_reference, status, limits_config, created_at, updated_at
	`, p.OrganizationID, p.WorkspaceID, p.ProviderKey, p.Name, p.CredentialReference, defaultRawJSON(p.LimitsConfig),
	).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.ProviderKey, &row.Name, &row.CredentialReference,
		&row.Status, &row.LimitsConfig, &createdAt, &updatedAt)
	if err != nil {
		return ProviderAccountRow{}, fmt.Errorf("create provider account: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) GetProviderAccountByID(ctx context.Context, id uuid.UUID) (ProviderAccountRow, error) {
	var row ProviderAccountRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, workspace_id, provider_key, name, credential_reference, status, limits_config, created_at, updated_at
		FROM provider_accounts WHERE id = $1 AND archived_at IS NULL
	`, id).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.ProviderKey, &row.Name, &row.CredentialReference,
		&row.Status, &row.LimitsConfig, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProviderAccountRow{}, ErrProviderAccountNotFound
		}
		return ProviderAccountRow{}, fmt.Errorf("get provider account: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) ListProviderAccountsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]ProviderAccountRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, workspace_id, provider_key, name, credential_reference, status, limits_config, created_at, updated_at
		FROM provider_accounts
		WHERE workspace_id = $1 AND archived_at IS NULL
		ORDER BY name
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list provider accounts: %w", err)
	}
	defer rows.Close()

	var result []ProviderAccountRow
	for rows.Next() {
		var row ProviderAccountRow
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.ProviderKey, &row.Name, &row.CredentialReference,
			&row.Status, &row.LimitsConfig, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan provider account: %w", err)
		}
		row.CreatedAt = createdAt.Time
		row.UpdatedAt = updatedAt.Time
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider accounts: %w", err)
	}
	if result == nil {
		result = []ProviderAccountRow{}
	}
	return result, nil
}

func (r *Repository) ArchiveProviderAccount(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE provider_accounts SET status = 'archived', archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("archive provider account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrProviderAccountNotFound
	}
	return nil
}

// --------------------------------------------------------------------------
// Model Catalog Entries (global, read-only)
// --------------------------------------------------------------------------

type ModelCatalogEntryRow struct {
	ID              uuid.UUID
	ProviderKey     string
	ProviderModelID string
	DisplayName     string
	ModelFamily     string
	Modality        string
	LifecycleStatus string
	Metadata        json.RawMessage
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (r *Repository) GetModelCatalogEntryByID(ctx context.Context, id uuid.UUID) (ModelCatalogEntryRow, error) {
	var row ModelCatalogEntryRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		SELECT id, provider_key, provider_model_id, display_name, model_family, modality, lifecycle_status, metadata, created_at, updated_at
		FROM model_catalog_entries WHERE id = $1
	`, id).Scan(&row.ID, &row.ProviderKey, &row.ProviderModelID, &row.DisplayName, &row.ModelFamily,
		&row.Modality, &row.LifecycleStatus, &row.Metadata, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ModelCatalogEntryRow{}, ErrModelCatalogNotFound
		}
		return ModelCatalogEntryRow{}, fmt.Errorf("get model catalog entry: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) UpsertModelCatalogEntry(ctx context.Context, providerKey, providerModelID string) (ModelCatalogEntryRow, error) {
	var row ModelCatalogEntryRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		INSERT INTO model_catalog_entries (provider_key, provider_model_id, display_name, model_family, modality)
		VALUES ($1, $2, $2, $2, 'text')
		ON CONFLICT (provider_key, provider_model_id) DO UPDATE SET updated_at = now()
		RETURNING id, provider_key, provider_model_id, display_name, model_family, modality, lifecycle_status, metadata, created_at, updated_at
	`, providerKey, providerModelID).Scan(&row.ID, &row.ProviderKey, &row.ProviderModelID, &row.DisplayName, &row.ModelFamily,
		&row.Modality, &row.LifecycleStatus, &row.Metadata, &createdAt, &updatedAt)
	if err != nil {
		return ModelCatalogEntryRow{}, fmt.Errorf("upsert model catalog entry: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) GetModelCatalogEntryByProviderModel(ctx context.Context, providerKey, providerModelID string) (ModelCatalogEntryRow, error) {
	var row ModelCatalogEntryRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		SELECT id, provider_key, provider_model_id, display_name, model_family, modality, lifecycle_status, metadata, created_at, updated_at
		FROM model_catalog_entries WHERE provider_key = $1 AND provider_model_id = $2
	`, providerKey, providerModelID).Scan(&row.ID, &row.ProviderKey, &row.ProviderModelID, &row.DisplayName, &row.ModelFamily,
		&row.Modality, &row.LifecycleStatus, &row.Metadata, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ModelCatalogEntryRow{}, ErrModelCatalogNotFound
		}
		return ModelCatalogEntryRow{}, fmt.Errorf("get model catalog entry by provider model: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) ListModelCatalogEntries(ctx context.Context) ([]ModelCatalogEntryRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, provider_key, provider_model_id, display_name, model_family, modality, lifecycle_status, metadata, created_at, updated_at
		FROM model_catalog_entries
		WHERE lifecycle_status != 'archived'
		ORDER BY provider_key, display_name
	`)
	if err != nil {
		return nil, fmt.Errorf("list model catalog entries: %w", err)
	}
	defer rows.Close()

	var result []ModelCatalogEntryRow
	for rows.Next() {
		var row ModelCatalogEntryRow
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&row.ID, &row.ProviderKey, &row.ProviderModelID, &row.DisplayName, &row.ModelFamily,
			&row.Modality, &row.LifecycleStatus, &row.Metadata, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan model catalog entry: %w", err)
		}
		row.CreatedAt = createdAt.Time
		row.UpdatedAt = updatedAt.Time
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model catalog entries: %w", err)
	}
	if result == nil {
		result = []ModelCatalogEntryRow{}
	}
	return result, nil
}

// --------------------------------------------------------------------------
// Model Aliases
// --------------------------------------------------------------------------

type ModelAliasRow struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	WorkspaceID         *uuid.UUID
	ProviderAccountID   *uuid.UUID
	ModelCatalogEntryID uuid.UUID
	AliasKey            string
	DisplayName         string
	Status              string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CreateModelAliasParams struct {
	OrganizationID      uuid.UUID
	WorkspaceID         uuid.UUID
	ProviderAccountID   *uuid.UUID
	ModelCatalogEntryID uuid.UUID
	AliasKey            string
	DisplayName         string
}

func (r *Repository) CreateModelAlias(ctx context.Context, p CreateModelAliasParams) (ModelAliasRow, error) {
	var row ModelAliasRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		INSERT INTO model_aliases (organization_id, workspace_id, provider_account_id, model_catalog_entry_id, alias_key, display_name)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, organization_id, workspace_id, provider_account_id, model_catalog_entry_id, alias_key, display_name, status, created_at, updated_at
	`, p.OrganizationID, p.WorkspaceID, p.ProviderAccountID, p.ModelCatalogEntryID, p.AliasKey, p.DisplayName,
	).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.ProviderAccountID, &row.ModelCatalogEntryID,
		&row.AliasKey, &row.DisplayName, &row.Status, &createdAt, &updatedAt)
	if err != nil {
		return ModelAliasRow{}, fmt.Errorf("create model alias: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) GetModelAliasByID(ctx context.Context, id uuid.UUID) (ModelAliasRow, error) {
	var row ModelAliasRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, workspace_id, provider_account_id, model_catalog_entry_id, alias_key, display_name, status, created_at, updated_at
		FROM model_aliases WHERE id = $1 AND archived_at IS NULL
	`, id).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.ProviderAccountID, &row.ModelCatalogEntryID,
		&row.AliasKey, &row.DisplayName, &row.Status, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ModelAliasRow{}, ErrModelAliasNotFound
		}
		return ModelAliasRow{}, fmt.Errorf("get model alias: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) ListModelAliasesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]ModelAliasRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, workspace_id, provider_account_id, model_catalog_entry_id, alias_key, display_name, status, created_at, updated_at
		FROM model_aliases
		WHERE workspace_id = $1 AND archived_at IS NULL
		ORDER BY display_name
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list model aliases: %w", err)
	}
	defer rows.Close()

	var result []ModelAliasRow
	for rows.Next() {
		var row ModelAliasRow
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.ProviderAccountID, &row.ModelCatalogEntryID,
			&row.AliasKey, &row.DisplayName, &row.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan model alias: %w", err)
		}
		row.CreatedAt = createdAt.Time
		row.UpdatedAt = updatedAt.Time
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model aliases: %w", err)
	}
	if result == nil {
		result = []ModelAliasRow{}
	}
	return result, nil
}

func (r *Repository) ArchiveModelAlias(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE model_aliases SET status = 'archived', archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("archive model alias: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrModelAliasNotFound
	}
	return nil
}

// --------------------------------------------------------------------------
// Tools
// --------------------------------------------------------------------------

type ToolRow struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	WorkspaceID     *uuid.UUID
	Name            string
	Slug            string
	ToolKind        string
	CapabilityKey   string
	Definition      json.RawMessage
	LifecycleStatus string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateToolParams struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	Name           string
	Slug           string
	ToolKind       string
	CapabilityKey  string
	Definition     json.RawMessage
}

func (r *Repository) CreateTool(ctx context.Context, p CreateToolParams) (ToolRow, error) {
	var row ToolRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		INSERT INTO tools (organization_id, workspace_id, name, slug, tool_kind, capability_key, definition)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, organization_id, workspace_id, name, slug, tool_kind, capability_key, definition, lifecycle_status, created_at, updated_at
	`, p.OrganizationID, p.WorkspaceID, p.Name, p.Slug, p.ToolKind, p.CapabilityKey, defaultRawJSON(p.Definition),
	).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.Slug, &row.ToolKind,
		&row.CapabilityKey, &row.Definition, &row.LifecycleStatus, &createdAt, &updatedAt)
	if err != nil {
		if isDuplicateSlug(err) {
			return ToolRow{}, ErrSlugTaken
		}
		return ToolRow{}, fmt.Errorf("create tool: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) GetToolByID(ctx context.Context, id uuid.UUID) (ToolRow, error) {
	var row ToolRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, workspace_id, name, slug, tool_kind, capability_key, definition, lifecycle_status, created_at, updated_at
		FROM tools WHERE id = $1 AND archived_at IS NULL
	`, id).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.Slug, &row.ToolKind,
		&row.CapabilityKey, &row.Definition, &row.LifecycleStatus, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ToolRow{}, ErrToolNotFound
		}
		return ToolRow{}, fmt.Errorf("get tool: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) ListToolsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]ToolRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, workspace_id, name, slug, tool_kind, capability_key, definition, lifecycle_status, created_at, updated_at
		FROM tools
		WHERE workspace_id = $1 AND archived_at IS NULL
		ORDER BY name
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}
	defer rows.Close()

	var result []ToolRow
	for rows.Next() {
		var row ToolRow
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.Slug, &row.ToolKind,
			&row.CapabilityKey, &row.Definition, &row.LifecycleStatus, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan tool: %w", err)
		}
		row.CreatedAt = createdAt.Time
		row.UpdatedAt = updatedAt.Time
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tools: %w", err)
	}
	if result == nil {
		result = []ToolRow{}
	}
	return result, nil
}

// --------------------------------------------------------------------------
// Knowledge Sources
// --------------------------------------------------------------------------

type KnowledgeSourceRow struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	WorkspaceID      *uuid.UUID
	Name             string
	Slug             string
	SourceKind       string
	ConnectionConfig json.RawMessage
	LifecycleStatus  string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateKnowledgeSourceParams struct {
	OrganizationID   uuid.UUID
	WorkspaceID      uuid.UUID
	Name             string
	Slug             string
	SourceKind       string
	ConnectionConfig json.RawMessage
}

func (r *Repository) CreateKnowledgeSource(ctx context.Context, p CreateKnowledgeSourceParams) (KnowledgeSourceRow, error) {
	var row KnowledgeSourceRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		INSERT INTO knowledge_sources (organization_id, workspace_id, name, slug, source_kind, connection_config)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, organization_id, workspace_id, name, slug, source_kind, connection_config, lifecycle_status, created_at, updated_at
	`, p.OrganizationID, p.WorkspaceID, p.Name, p.Slug, p.SourceKind, defaultRawJSON(p.ConnectionConfig),
	).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.Slug, &row.SourceKind,
		&row.ConnectionConfig, &row.LifecycleStatus, &createdAt, &updatedAt)
	if err != nil {
		if isDuplicateSlug(err) {
			return KnowledgeSourceRow{}, ErrSlugTaken
		}
		return KnowledgeSourceRow{}, fmt.Errorf("create knowledge source: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) GetKnowledgeSourceByID(ctx context.Context, id uuid.UUID) (KnowledgeSourceRow, error) {
	var row KnowledgeSourceRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, workspace_id, name, slug, source_kind, connection_config, lifecycle_status, created_at, updated_at
		FROM knowledge_sources WHERE id = $1 AND archived_at IS NULL
	`, id).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.Slug, &row.SourceKind,
		&row.ConnectionConfig, &row.LifecycleStatus, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return KnowledgeSourceRow{}, ErrKnowledgeSourceNotFound
		}
		return KnowledgeSourceRow{}, fmt.Errorf("get knowledge source: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) ListKnowledgeSourcesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]KnowledgeSourceRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, workspace_id, name, slug, source_kind, connection_config, lifecycle_status, created_at, updated_at
		FROM knowledge_sources
		WHERE workspace_id = $1 AND archived_at IS NULL
		ORDER BY name
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list knowledge sources: %w", err)
	}
	defer rows.Close()

	var result []KnowledgeSourceRow
	for rows.Next() {
		var row KnowledgeSourceRow
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.Slug, &row.SourceKind,
			&row.ConnectionConfig, &row.LifecycleStatus, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge source: %w", err)
		}
		row.CreatedAt = createdAt.Time
		row.UpdatedAt = updatedAt.Time
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge sources: %w", err)
	}
	if result == nil {
		result = []KnowledgeSourceRow{}
	}
	return result, nil
}

// --------------------------------------------------------------------------
// Routing Policies
// --------------------------------------------------------------------------

type RoutingPolicyRow struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    *uuid.UUID
	Name           string
	PolicyKind     string
	Config         json.RawMessage
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateRoutingPolicyParams struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	Name           string
	PolicyKind     string
	Config         json.RawMessage
}

func (r *Repository) CreateRoutingPolicy(ctx context.Context, p CreateRoutingPolicyParams) (RoutingPolicyRow, error) {
	var row RoutingPolicyRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		INSERT INTO routing_policies (organization_id, workspace_id, name, policy_kind, config)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, organization_id, workspace_id, name, policy_kind, config, created_at, updated_at
	`, p.OrganizationID, p.WorkspaceID, p.Name, p.PolicyKind, defaultRawJSON(p.Config),
	).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.PolicyKind, &row.Config, &createdAt, &updatedAt)
	if err != nil {
		return RoutingPolicyRow{}, fmt.Errorf("create routing policy: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) ListRoutingPoliciesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]RoutingPolicyRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, workspace_id, name, policy_kind, config, created_at, updated_at
		FROM routing_policies
		WHERE workspace_id = $1 AND archived_at IS NULL
		ORDER BY name
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list routing policies: %w", err)
	}
	defer rows.Close()

	var result []RoutingPolicyRow
	for rows.Next() {
		var row RoutingPolicyRow
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.PolicyKind, &row.Config, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan routing policy: %w", err)
		}
		row.CreatedAt = createdAt.Time
		row.UpdatedAt = updatedAt.Time
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate routing policies: %w", err)
	}
	if result == nil {
		result = []RoutingPolicyRow{}
	}
	return result, nil
}

// --------------------------------------------------------------------------
// Spend Policies
// --------------------------------------------------------------------------

type SpendPolicyRow struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    *uuid.UUID
	Name           string
	CurrencyCode   string
	WindowKind     string
	SoftLimit      *float64
	HardLimit      *float64
	Config         json.RawMessage
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateSpendPolicyParams struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	Name           string
	CurrencyCode   string
	WindowKind     string
	SoftLimit      *float64
	HardLimit      *float64
	Config         json.RawMessage
}

func (r *Repository) CreateSpendPolicy(ctx context.Context, p CreateSpendPolicyParams) (SpendPolicyRow, error) {
	var row SpendPolicyRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		INSERT INTO spend_policies (organization_id, workspace_id, name, currency_code, window_kind, soft_limit, hard_limit, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, organization_id, workspace_id, name, currency_code, window_kind, soft_limit, hard_limit, config, created_at, updated_at
	`, p.OrganizationID, p.WorkspaceID, p.Name,
		defaultStr(p.CurrencyCode, "USD"), p.WindowKind, p.SoftLimit, p.HardLimit, defaultRawJSON(p.Config),
	).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.CurrencyCode, &row.WindowKind,
		&row.SoftLimit, &row.HardLimit, &row.Config, &createdAt, &updatedAt)
	if err != nil {
		return SpendPolicyRow{}, fmt.Errorf("create spend policy: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

func (r *Repository) ListSpendPoliciesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]SpendPolicyRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, workspace_id, name, currency_code, window_kind, soft_limit, hard_limit, config, created_at, updated_at
		FROM spend_policies
		WHERE workspace_id = $1 AND archived_at IS NULL
		ORDER BY name
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list spend policies: %w", err)
	}
	defer rows.Close()

	var result []SpendPolicyRow
	for rows.Next() {
		var row SpendPolicyRow
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.CurrencyCode, &row.WindowKind,
			&row.SoftLimit, &row.HardLimit, &row.Config, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan spend policy: %w", err)
		}
		row.CreatedAt = createdAt.Time
		row.UpdatedAt = updatedAt.Time
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spend policies: %w", err)
	}
	if result == nil {
		result = []SpendPolicyRow{}
	}
	return result, nil
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func isDuplicateSlug(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "slug") {
		return true
	}
	return false
}

// GetWorkspaceID methods implement WorkspaceOwned for authorization checks.
func (r RuntimeProfileRow) GetWorkspaceID() *uuid.UUID  { return r.WorkspaceID }
func (r ProviderAccountRow) GetWorkspaceID() *uuid.UUID  { return r.WorkspaceID }
func (r ModelAliasRow) GetWorkspaceID() *uuid.UUID       { return r.WorkspaceID }
func (r ToolRow) GetWorkspaceID() *uuid.UUID             { return r.WorkspaceID }
func (r KnowledgeSourceRow) GetWorkspaceID() *uuid.UUID  { return r.WorkspaceID }

func defaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func defaultInt32(v, fallback int32) int32 {
	if v == 0 {
		return fallback
	}
	return v
}

func defaultRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
