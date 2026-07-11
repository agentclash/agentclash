package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/connection"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/toolspec"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

type InfrastructureRepository interface {
	// Runtime Profiles
	CreateRuntimeProfile(ctx context.Context, p repository.CreateRuntimeProfileParams) (repository.RuntimeProfileRow, error)
	GetRuntimeProfileByID(ctx context.Context, id uuid.UUID) (repository.RuntimeProfileRow, error)
	ListRuntimeProfilesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.RuntimeProfileRow, error)
	ArchiveRuntimeProfile(ctx context.Context, id uuid.UUID) error

	// Provider Accounts
	CreateProviderAccount(ctx context.Context, p repository.CreateProviderAccountParams) (repository.ProviderAccountRow, error)
	GetProviderAccountByID(ctx context.Context, id uuid.UUID) (repository.ProviderAccountRow, error)
	ListProviderAccountsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.ProviderAccountRow, error)
	ArchiveProviderAccount(ctx context.Context, id uuid.UUID) error

	// Workspace Secrets
	UpsertWorkspaceSecret(ctx context.Context, params repository.UpsertWorkspaceSecretParams) error
	LoadWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error)

	// Tools
	CreateTool(ctx context.Context, p repository.CreateToolParams) (repository.ToolRow, error)
	GetToolByID(ctx context.Context, id uuid.UUID) (repository.ToolRow, error)
	ListToolsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.ToolRow, error)
	UpdateTool(ctx context.Context, p repository.UpdateToolParams) (repository.ToolRow, error)
	ArchiveTool(ctx context.Context, id uuid.UUID) error

	// Routing Policies
	CreateRoutingPolicy(ctx context.Context, p repository.CreateRoutingPolicyParams) (repository.RoutingPolicyRow, error)
	ListRoutingPoliciesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.RoutingPolicyRow, error)

	// Spend Policies
	CreateSpendPolicy(ctx context.Context, p repository.CreateSpendPolicyParams) (repository.SpendPolicyRow, error)
	ListSpendPoliciesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.SpendPolicyRow, error)

	// Workspace lookup for org ID
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
}

type InfrastructureManager struct {
	repo InfrastructureRepository
	conn *connection.Service
}

func NewInfrastructureManager(repo InfrastructureRepository) *InfrastructureManager {
	return &InfrastructureManager{repo: repo, conn: connection.NewService(repo, nil)}
}

// WithConnectionService installs the provider-backed connection service used for
// smoke tests and live model listing. Without it, those paths report the
// provider client as unconfigured.
func (m *InfrastructureManager) WithConnectionService(conn *connection.Service) *InfrastructureManager {
	m.conn = conn
	return m
}

func (m *InfrastructureManager) resolveOrgID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error) {
	return m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
}

// --------------------------------------------------------------------------
// Runtime Profiles
// --------------------------------------------------------------------------

func (m *InfrastructureManager) CreateRuntimeProfile(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateRuntimeProfileInput) (repository.RuntimeProfileRow, error) {
	orgID, err := m.resolveOrgID(ctx, workspaceID)
	if err != nil {
		return repository.RuntimeProfileRow{}, fmt.Errorf("resolve org: %w", err)
	}
	slug := generateSlug(input.Name)
	return m.repo.CreateRuntimeProfile(ctx, repository.CreateRuntimeProfileParams{
		OrganizationID:     orgID,
		WorkspaceID:        workspaceID,
		Name:               input.Name,
		Slug:               slug,
		ExecutionTarget:    input.ExecutionTarget,
		TraceMode:          input.TraceMode,
		MaxIterations:      input.MaxIterations,
		MaxToolCalls:       input.MaxToolCalls,
		StepTimeoutSeconds: input.StepTimeoutSeconds,
		RunTimeoutSeconds:  input.RunTimeoutSeconds,
		ProfileConfig:      input.ProfileConfig,
	})
}

func (m *InfrastructureManager) ListRuntimeProfiles(ctx context.Context, workspaceID uuid.UUID) ([]repository.RuntimeProfileRow, error) {
	return m.repo.ListRuntimeProfilesByWorkspaceID(ctx, workspaceID)
}

func (m *InfrastructureManager) GetRuntimeProfile(ctx context.Context, id uuid.UUID) (repository.RuntimeProfileRow, error) {
	return m.repo.GetRuntimeProfileByID(ctx, id)
}

func (m *InfrastructureManager) ArchiveRuntimeProfile(ctx context.Context, id uuid.UUID) error {
	return m.repo.ArchiveRuntimeProfile(ctx, id)
}

// --------------------------------------------------------------------------
// Provider Accounts
// --------------------------------------------------------------------------

func (m *InfrastructureManager) CreateProviderAccount(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateProviderAccountInput) (repository.ProviderAccountRow, error) {
	return m.conn.Create(ctx, workspaceID, connection.CreateConnectionInput{
		ProviderKey:         input.ProviderKey,
		Name:                input.Name,
		CredentialReference: input.CredentialReference,
		APIKey:              input.APIKey,
		LimitsConfig:        input.LimitsConfig,
		ActorUserID:         &caller.UserID,
	})
}

func (m *InfrastructureManager) ListProviderAccounts(ctx context.Context, workspaceID uuid.UUID) ([]repository.ProviderAccountRow, error) {
	return m.conn.List(ctx, workspaceID)
}

func (m *InfrastructureManager) GetProviderAccount(ctx context.Context, id uuid.UUID) (repository.ProviderAccountRow, error) {
	return m.conn.Get(ctx, id)
}

func (m *InfrastructureManager) DeleteProviderAccount(ctx context.Context, id uuid.UUID) error {
	return m.conn.Delete(ctx, id)
}

func (m *InfrastructureManager) TestProviderAccount(ctx context.Context, account repository.ProviderAccountRow, input ProviderAccountTestInput) (ProviderAccountTestResult, error) {
	result, err := m.conn.Test(ctx, account, connection.TestInput{
		Model:              input.Model,
		StepTimeoutSeconds: input.StepTimeoutSeconds,
	})
	if err != nil {
		return ProviderAccountTestResult{}, err
	}
	return providerAccountTestResponse{
		AccountID:       result.AccountID,
		ProviderKey:     result.ProviderKey,
		Model:           result.Model,
		ProviderModelID: result.ProviderModelID,
		Passed:          result.Passed,
		Status:          result.Status,
		Code:            result.Code,
		Message:         result.Message,
		Retryable:       result.Retryable,
		DurationMS:      result.DurationMS,
	}, nil
}

// ListProviderAccountModels returns the live model list reachable with a
// connection's credential (cached per account).
func (m *InfrastructureManager) ListProviderAccountModels(ctx context.Context, account repository.ProviderAccountRow) ([]provider.ModelInfo, error) {
	return m.conn.ListModels(ctx, account)
}

// --------------------------------------------------------------------------
// Tools
// --------------------------------------------------------------------------

func (m *InfrastructureManager) CreateTool(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateToolInput) (repository.ToolRow, error) {
	orgID, err := m.resolveOrgID(ctx, workspaceID)
	if err != nil {
		return repository.ToolRow{}, fmt.Errorf("resolve org: %w", err)
	}
	slug := generateSlug(input.Name)
	capabilityKey := strings.TrimSpace(input.CapabilityKey)
	if capabilityKey == "" {
		capabilityKey = slug
	}
	if err := m.validateToolDefinition(ctx, &workspaceID, input.ToolKind, slug, input.Definition); err != nil {
		return repository.ToolRow{}, err
	}
	return m.repo.CreateTool(ctx, repository.CreateToolParams{
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		Name:           input.Name,
		Slug:           slug,
		ToolKind:       input.ToolKind,
		CapabilityKey:  capabilityKey,
		Definition:     input.Definition,
	})
}

// CreateToolsFromLibrary instantiates catalog entries (by slug) into the
// workspace as real tools. The server owns the definitions — the client only
// sends slugs/variants, never a definition. Each entry is created through the
// same validation path as CreateTool. Conflicts skip by default (or suffix the
// slug); unknown slugs and missing live variants are reported as skips rather
// than failing the whole batch.
func (m *InfrastructureManager) CreateToolsFromLibrary(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateToolsFromLibraryInput) ([]repository.ToolRow, []LibrarySkip, error) {
	orgID, err := m.resolveOrgID(ctx, workspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve org: %w", err)
	}
	existing, err := m.repo.ListToolsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("load workspace tools: %w", err)
	}
	used := make(map[string]struct{}, len(existing))
	for _, t := range existing {
		used[t.Slug] = struct{}{}
	}

	var created []repository.ToolRow
	var skipped []LibrarySkip
	skip := func(slug, reason string) { skipped = append(skipped, LibrarySkip{Slug: slug, Reason: reason}) }

	for _, e := range input.Entries {
		key := strings.TrimSpace(e.Slug)
		entry, ok := toolspec.LibraryBySlug(key)
		if !ok {
			skip(key, "unknown library tool")
			continue
		}
		definition := entry.Definition
		if e.Variant == "live" {
			if !entry.HasLive() {
				skip(key, "no live variant available")
				continue
			}
			definition = entry.Live
		}

		baseSlug := generateSlug(entry.Name)
		slug := baseSlug
		if _, clash := used[slug]; clash {
			if e.Conflict != "suffix" {
				skip(key, "a tool with this name already exists")
				continue
			}
			slug = nextAvailableToolSlug(baseSlug, used)
		}

		var row repository.ToolRow
		creationFailed := false
		const sequentialSlugAttempts = 3
		for attempt := 0; attempt <= sequentialSlugAttempts; attempt++ {
			if err := m.validateToolDefinition(ctx, &workspaceID, entry.ToolKind, slug, definition); err != nil {
				// Catalog entries are validated in toolspec tests, so this is defensive.
				skip(key, "definition is not valid")
				creationFailed = true
				break
			}

			row, err = m.repo.CreateTool(ctx, repository.CreateToolParams{
				OrganizationID: orgID,
				WorkspaceID:    workspaceID,
				Name:           entry.Name,
				Slug:           slug,
				ToolKind:       entry.ToolKind,
				CapabilityKey:  slug,
				Definition:     definition,
			})
			if err == nil {
				break
			}
			if !errors.Is(err, repository.ErrSlugTaken) {
				return nil, nil, fmt.Errorf("create tool %q: %w", key, err)
			}
			used[slug] = struct{}{}
			if e.Conflict != "suffix" {
				break
			}
			if attempt == sequentialSlugAttempts-1 {
				slug = collisionResistantToolSlug(baseSlug)
			} else {
				slug = nextAvailableToolSlug(baseSlug, used)
			}
		}
		if creationFailed {
			continue
		}
		if err != nil {
			skip(key, "a tool with this name already exists")
			continue
		}
		used[slug] = struct{}{}
		created = append(created, row)
	}

	return created, skipped, nil
}

func collisionResistantToolSlug(base string) string {
	suffix := uuid.NewString()[:8]
	const maxSlugLength = 60
	maxBaseLength := maxSlugLength - len(suffix) - 1
	if len(base) > maxBaseLength {
		base = strings.TrimRight(base[:maxBaseLength], "-")
	}
	return base + "-" + suffix
}

func nextAvailableToolSlug(base string, used map[string]struct{}) string {
	if _, clash := used[base]; !clash {
		return base
	}
	// With len(used) occupied slugs, one of these len(used)+1 candidates must
	// be free. The bound makes this deterministic even for degenerate maps.
	for n := 2; n <= len(used)+2; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if _, clash := used[candidate]; !clash {
			return candidate
		}
	}
	panic("unreachable: suffix candidate set exceeds occupied slug count")
}

func (m *InfrastructureManager) ListTools(ctx context.Context, workspaceID uuid.UUID) ([]repository.ToolRow, error) {
	return m.repo.ListToolsByWorkspaceID(ctx, workspaceID)
}

func (m *InfrastructureManager) GetTool(ctx context.Context, id uuid.UUID) (repository.ToolRow, error) {
	return m.repo.GetToolByID(ctx, id)
}

func (m *InfrastructureManager) UpdateTool(ctx context.Context, caller Caller, id uuid.UUID, input UpdateToolInput) (repository.ToolRow, error) {
	existing, err := m.repo.GetToolByID(ctx, id)
	if err != nil {
		return repository.ToolRow{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = existing.Name
	}
	capabilityKey := strings.TrimSpace(input.CapabilityKey)
	if capabilityKey == "" {
		capabilityKey = existing.CapabilityKey
	}
	// tool_kind and slug are immutable; validate the new definition against the
	// existing tool_kind, excluding this tool's own slug to reject self-reference.
	// A nil/empty definition is a partial update (rename, lifecycle toggle) and is
	// left to the repository's COALESCE to preserve the stored value.
	if err := m.validateToolDefinition(ctx, existing.WorkspaceID, existing.ToolKind, existing.Slug, input.Definition); err != nil {
		return repository.ToolRow{}, err
	}
	return m.repo.UpdateTool(ctx, repository.UpdateToolParams{
		ID:              id,
		Name:            name,
		CapabilityKey:   capabilityKey,
		Definition:      input.Definition,
		LifecycleStatus: strings.TrimSpace(input.LifecycleStatus),
	})
}

func (m *InfrastructureManager) DeleteTool(ctx context.Context, id uuid.UUID) error {
	return m.repo.ArchiveTool(ctx, id)
}

// validateToolDefinition runs the canonical toolspec validator. An empty
// definition is treated as "no change" (partial update) and skips validation.
// Validation otherwise runs for every caller, regardless of whether the tool is
// workspace-scoped. For composed tools with a workspace, it loads the other
// tools so step refs of type "tool" can be checked for existence (and
// self-reference rejected via selfSlug); without a workspace, tool-ref existence
// is not checked but every other rule still applies.
func (m *InfrastructureManager) validateToolDefinition(ctx context.Context, workspaceID *uuid.UUID, toolKind, selfSlug string, definition json.RawMessage) error {
	if len(strings.TrimSpace(string(definition))) == 0 {
		return nil
	}
	opts := toolspec.ValidateOptions{SelfSlug: selfSlug}
	if toolKind == toolspec.ToolTypeComposed && workspaceID != nil {
		tools, err := m.repo.ListToolsByWorkspaceID(ctx, *workspaceID)
		if err != nil {
			return fmt.Errorf("load workspace tools: %w", err)
		}
		known := make(map[string]struct{}, len(tools))
		for _, t := range tools {
			known[t.Slug] = struct{}{}
		}
		opts.KnownToolSlugs = known
	}
	if errs := toolspec.ValidateDefinition(toolKind, definition, opts); len(errs) > 0 {
		return &ToolDefinitionError{Errors: errs}
	}
	return nil
}

// --------------------------------------------------------------------------
// Routing Policies
// --------------------------------------------------------------------------

func (m *InfrastructureManager) CreateRoutingPolicy(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateRoutingPolicyInput) (repository.RoutingPolicyRow, error) {
	orgID, err := m.resolveOrgID(ctx, workspaceID)
	if err != nil {
		return repository.RoutingPolicyRow{}, fmt.Errorf("resolve org: %w", err)
	}
	return m.repo.CreateRoutingPolicy(ctx, repository.CreateRoutingPolicyParams{
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		Name:           input.Name,
		PolicyKind:     input.PolicyKind,
		Config:         input.Config,
	})
}

func (m *InfrastructureManager) ListRoutingPolicies(ctx context.Context, workspaceID uuid.UUID) ([]repository.RoutingPolicyRow, error) {
	return m.repo.ListRoutingPoliciesByWorkspaceID(ctx, workspaceID)
}

// --------------------------------------------------------------------------
// Spend Policies
// --------------------------------------------------------------------------

func (m *InfrastructureManager) CreateSpendPolicy(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateSpendPolicyInput) (repository.SpendPolicyRow, error) {
	orgID, err := m.resolveOrgID(ctx, workspaceID)
	if err != nil {
		return repository.SpendPolicyRow{}, fmt.Errorf("resolve org: %w", err)
	}
	return m.repo.CreateSpendPolicy(ctx, repository.CreateSpendPolicyParams{
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		Name:           input.Name,
		CurrencyCode:   input.CurrencyCode,
		WindowKind:     input.WindowKind,
		SoftLimit:      input.SoftLimit,
		HardLimit:      input.HardLimit,
		Config:         input.Config,
	})
}

func (m *InfrastructureManager) ListSpendPolicies(ctx context.Context, workspaceID uuid.UUID) ([]repository.SpendPolicyRow, error) {
	return m.repo.ListSpendPoliciesByWorkspaceID(ctx, workspaceID)
}
