package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
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

	// Model Catalog
	ListModelCatalogEntries(ctx context.Context) ([]repository.ModelCatalogEntryRow, error)
	GetModelCatalogEntryByID(ctx context.Context, id uuid.UUID) (repository.ModelCatalogEntryRow, error)

	// Model Aliases
	CreateModelAlias(ctx context.Context, p repository.CreateModelAliasParams) (repository.ModelAliasRow, error)
	GetModelAliasByID(ctx context.Context, id uuid.UUID) (repository.ModelAliasRow, error)
	ListModelAliasesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.ModelAliasRow, error)
	ArchiveModelAlias(ctx context.Context, id uuid.UUID) error

	// Tools
	CreateTool(ctx context.Context, p repository.CreateToolParams) (repository.ToolRow, error)
	GetToolByID(ctx context.Context, id uuid.UUID) (repository.ToolRow, error)
	ListToolsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.ToolRow, error)

	// Knowledge Sources
	CreateKnowledgeSource(ctx context.Context, p repository.CreateKnowledgeSourceParams) (repository.KnowledgeSourceRow, error)
	GetKnowledgeSourceByID(ctx context.Context, id uuid.UUID) (repository.KnowledgeSourceRow, error)
	ListKnowledgeSourcesByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.KnowledgeSourceRow, error)

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
}

func NewInfrastructureManager(repo InfrastructureRepository) *InfrastructureManager {
	return &InfrastructureManager{repo: repo}
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
	orgID, err := m.resolveOrgID(ctx, workspaceID)
	if err != nil {
		return repository.ProviderAccountRow{}, fmt.Errorf("resolve org: %w", err)
	}

	credRef := input.CredentialReference

	// When a raw API key is provided, store it as a workspace secret and
	// set the credential reference to point at it automatically.
	if input.APIKey != "" {
		secretKey := fmt.Sprintf("PROVIDER_%s_API_KEY", strings.ToUpper(strings.ReplaceAll(input.ProviderKey, "-", "_")))
		if err := m.repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{
			WorkspaceID: workspaceID,
			Key:         secretKey,
			Value:       input.APIKey,
			ActorUserID: &caller.UserID,
		}); err != nil {
			return repository.ProviderAccountRow{}, fmt.Errorf("store api key as workspace secret: %w", err)
		}
		credRef = "workspace-secret://" + secretKey
	}

	return m.repo.CreateProviderAccount(ctx, repository.CreateProviderAccountParams{
		OrganizationID:      orgID,
		WorkspaceID:         workspaceID,
		ProviderKey:         input.ProviderKey,
		Name:                input.Name,
		CredentialReference: credRef,
		LimitsConfig:        input.LimitsConfig,
	})
}

func (m *InfrastructureManager) ListProviderAccounts(ctx context.Context, workspaceID uuid.UUID) ([]repository.ProviderAccountRow, error) {
	return m.repo.ListProviderAccountsByWorkspaceID(ctx, workspaceID)
}

func (m *InfrastructureManager) GetProviderAccount(ctx context.Context, id uuid.UUID) (repository.ProviderAccountRow, error) {
	return m.repo.GetProviderAccountByID(ctx, id)
}

func (m *InfrastructureManager) DeleteProviderAccount(ctx context.Context, id uuid.UUID) error {
	return m.repo.ArchiveProviderAccount(ctx, id)
}

// --------------------------------------------------------------------------
// Model Catalog
// --------------------------------------------------------------------------

func (m *InfrastructureManager) ListModelCatalog(ctx context.Context) ([]repository.ModelCatalogEntryRow, error) {
	return m.repo.ListModelCatalogEntries(ctx)
}

func (m *InfrastructureManager) GetModelCatalogEntry(ctx context.Context, id uuid.UUID) (repository.ModelCatalogEntryRow, error) {
	return m.repo.GetModelCatalogEntryByID(ctx, id)
}

// --------------------------------------------------------------------------
// Model Aliases
// --------------------------------------------------------------------------

func (m *InfrastructureManager) CreateModelAlias(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateModelAliasInput) (repository.ModelAliasRow, error) {
	orgID, err := m.resolveOrgID(ctx, workspaceID)
	if err != nil {
		return repository.ModelAliasRow{}, fmt.Errorf("resolve org: %w", err)
	}
	catalogID, err := uuid.Parse(input.ModelCatalogEntryID)
	if err != nil {
		return repository.ModelAliasRow{}, fmt.Errorf("invalid model_catalog_entry_id: %w", err)
	}
	var providerAccountID *uuid.UUID
	if input.ProviderAccountID != nil {
		parsed, err := uuid.Parse(*input.ProviderAccountID)
		if err != nil {
			return repository.ModelAliasRow{}, fmt.Errorf("invalid provider_account_id: %w", err)
		}
		providerAccountID = &parsed
	}
	return m.repo.CreateModelAlias(ctx, repository.CreateModelAliasParams{
		OrganizationID:      orgID,
		WorkspaceID:         workspaceID,
		ProviderAccountID:   providerAccountID,
		ModelCatalogEntryID: catalogID,
		AliasKey:            input.AliasKey,
		DisplayName:         input.DisplayName,
	})
}

func (m *InfrastructureManager) ListModelAliases(ctx context.Context, workspaceID uuid.UUID) ([]repository.ModelAliasRow, error) {
	return m.repo.ListModelAliasesByWorkspaceID(ctx, workspaceID)
}

func (m *InfrastructureManager) GetModelAlias(ctx context.Context, id uuid.UUID) (repository.ModelAliasRow, error) {
	return m.repo.GetModelAliasByID(ctx, id)
}

func (m *InfrastructureManager) DeleteModelAlias(ctx context.Context, id uuid.UUID) error {
	return m.repo.ArchiveModelAlias(ctx, id)
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
	return m.repo.CreateTool(ctx, repository.CreateToolParams{
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		Name:           input.Name,
		Slug:           slug,
		ToolKind:       input.ToolKind,
		CapabilityKey:  input.CapabilityKey,
		Definition:     input.Definition,
	})
}

func (m *InfrastructureManager) ListTools(ctx context.Context, workspaceID uuid.UUID) ([]repository.ToolRow, error) {
	return m.repo.ListToolsByWorkspaceID(ctx, workspaceID)
}

func (m *InfrastructureManager) GetTool(ctx context.Context, id uuid.UUID) (repository.ToolRow, error) {
	return m.repo.GetToolByID(ctx, id)
}

// --------------------------------------------------------------------------
// Knowledge Sources
// --------------------------------------------------------------------------

func (m *InfrastructureManager) CreateKnowledgeSource(ctx context.Context, caller Caller, workspaceID uuid.UUID, input CreateKnowledgeSourceInput) (repository.KnowledgeSourceRow, error) {
	orgID, err := m.resolveOrgID(ctx, workspaceID)
	if err != nil {
		return repository.KnowledgeSourceRow{}, fmt.Errorf("resolve org: %w", err)
	}
	slug := generateSlug(input.Name)
	return m.repo.CreateKnowledgeSource(ctx, repository.CreateKnowledgeSourceParams{
		OrganizationID:   orgID,
		WorkspaceID:      workspaceID,
		Name:             input.Name,
		Slug:             slug,
		SourceKind:       input.SourceKind,
		ConnectionConfig: input.ConnectionConfig,
	})
}

func (m *InfrastructureManager) ListKnowledgeSources(ctx context.Context, workspaceID uuid.UUID) ([]repository.KnowledgeSourceRow, error) {
	return m.repo.ListKnowledgeSourcesByWorkspaceID(ctx, workspaceID)
}

func (m *InfrastructureManager) GetKnowledgeSource(ctx context.Context, id uuid.UUID) (repository.KnowledgeSourceRow, error) {
	return m.repo.GetKnowledgeSourceByID(ctx, id)
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
