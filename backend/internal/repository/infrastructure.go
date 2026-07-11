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
	ErrToolNotFound            = errors.New("tool not found")
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
	// A soft-deleted tool still owns its unique slug. Recreating the same tool
	// restores that row so callers do not see a phantom slug conflict for a tool
	// that list/get endpoints intentionally hide.
	err := r.db.QueryRow(ctx, `
		UPDATE tools
		SET name = $3,
		    tool_kind = $5,
		    capability_key = $6,
		    definition = $7,
		    lifecycle_status = 'active',
		    archived_at = NULL,
		    updated_at = now()
		WHERE organization_id = $1 AND workspace_id = $2 AND slug = $4 AND archived_at IS NOT NULL
		RETURNING id, organization_id, workspace_id, name, slug, tool_kind, capability_key, definition, lifecycle_status, created_at, updated_at
	`, p.OrganizationID, p.WorkspaceID, p.Name, p.Slug, p.ToolKind, p.CapabilityKey, defaultRawJSON(p.Definition),
	).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.Slug, &row.ToolKind,
		&row.CapabilityKey, &row.Definition, &row.LifecycleStatus, &createdAt, &updatedAt)
	if err == nil {
		row.CreatedAt = createdAt.Time
		row.UpdatedAt = updatedAt.Time
		return row, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ToolRow{}, fmt.Errorf("restore archived tool: %w", err)
	}

	row = ToolRow{}
	createdAt, updatedAt = pgtype.Timestamptz{}, pgtype.Timestamptz{}
	err = r.db.QueryRow(ctx, `
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

type UpdateToolParams struct {
	ID              uuid.UUID
	Name            string
	CapabilityKey   string
	Definition      json.RawMessage
	LifecycleStatus string
}

// UpdateTool updates the mutable fields of a tool. The slug and tool_kind are
// intentionally immutable so that composed tools referencing this tool by slug
// keep resolving. An empty Definition or LifecycleStatus leaves the existing
// value unchanged (COALESCE), so callers can perform partial updates.
func (r *Repository) UpdateTool(ctx context.Context, p UpdateToolParams) (ToolRow, error) {
	var row ToolRow
	var createdAt, updatedAt pgtype.Timestamptz
	var definitionArg any
	if len(p.Definition) > 0 {
		definitionArg = string(p.Definition)
	}
	err := r.db.QueryRow(ctx, `
		UPDATE tools
		SET name = $2,
		    capability_key = $3,
		    definition = COALESCE($4::jsonb, definition),
		    lifecycle_status = COALESCE(NULLIF($5, ''), lifecycle_status),
		    updated_at = now()
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, organization_id, workspace_id, name, slug, tool_kind, capability_key, definition, lifecycle_status, created_at, updated_at
	`, p.ID, p.Name, p.CapabilityKey, definitionArg, p.LifecycleStatus,
	).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.Slug, &row.ToolKind,
		&row.CapabilityKey, &row.Definition, &row.LifecycleStatus, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ToolRow{}, ErrToolNotFound
		}
		return ToolRow{}, fmt.Errorf("update tool: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
}

// ArchiveTool soft-deletes a tool by setting archived_at. Subsequent Get/List
// calls (which filter archived_at IS NULL) will no longer return it.
func (r *Repository) ArchiveTool(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE tools SET lifecycle_status = 'archived', archived_at = now(), updated_at = now() WHERE id = $1 AND archived_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("archive tool: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrToolNotFound
	}
	return nil
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

func (r *Repository) GetRoutingPolicyByID(ctx context.Context, id uuid.UUID) (RoutingPolicyRow, error) {
	var row RoutingPolicyRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, workspace_id, name, policy_kind, config, created_at, updated_at
		FROM routing_policies WHERE id = $1 AND archived_at IS NULL
	`, id).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.PolicyKind, &row.Config, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RoutingPolicyRow{}, ErrRoutingPolicyNotFound
		}
		return RoutingPolicyRow{}, fmt.Errorf("get routing policy: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
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

func (r *Repository) GetSpendPolicyByID(ctx context.Context, id uuid.UUID) (SpendPolicyRow, error) {
	var row SpendPolicyRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, workspace_id, name, currency_code, window_kind, soft_limit, hard_limit, config, created_at, updated_at
		FROM spend_policies WHERE id = $1 AND archived_at IS NULL
	`, id).Scan(&row.ID, &row.OrganizationID, &row.WorkspaceID, &row.Name, &row.CurrencyCode, &row.WindowKind,
		&row.SoftLimit, &row.HardLimit, &row.Config, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SpendPolicyRow{}, ErrSpendPolicyNotFound
		}
		return SpendPolicyRow{}, fmt.Errorf("get spend policy: %w", err)
	}
	row.CreatedAt = createdAt.Time
	row.UpdatedAt = updatedAt.Time
	return row, nil
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
func (r ProviderAccountRow) GetWorkspaceID() *uuid.UUID { return r.WorkspaceID }
func (r ToolRow) GetWorkspaceID() *uuid.UUID            { return r.WorkspaceID }

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
