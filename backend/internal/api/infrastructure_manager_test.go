package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestInfrastructureManagerTestProviderAccountSuccess(t *testing.T) {
	workspaceID := uuid.New()
	accountID := uuid.New()
	client := &provider.FakeClient{Response: provider.Response{ProviderModelID: "gpt-4.1-mini"}}
	repo := &providerAccountTestRepo{
		secrets: map[string]string{"PROVIDER_OPENAI_API_KEY": "sk-test-secret"},
	}
	manager := NewInfrastructureManager(repo).WithProviderClient(client)

	result, err := manager.TestProviderAccount(context.Background(), repository.ProviderAccountRow{
		ID:                  accountID,
		WorkspaceID:         &workspaceID,
		ProviderKey:         "openai",
		CredentialReference: "workspace-secret://PROVIDER_OPENAI_API_KEY",
		Status:              "active",
	}, ProviderAccountTestInput{})
	if err != nil {
		t.Fatalf("TestProviderAccount returned error: %v", err)
	}
	if !result.Passed || result.Status != "passed" {
		t.Fatalf("result = %#v, want passed", result)
	}
	if len(client.Requests) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(client.Requests))
	}
	request := client.Requests[0]
	if request.ProviderKey != "openai" || request.ProviderAccountID != accountID.String() {
		t.Fatalf("request account/provider = %#v", request)
	}
	if request.CredentialReference != "workspace-secret://PROVIDER_OPENAI_API_KEY" {
		t.Fatalf("credential reference = %q", request.CredentialReference)
	}
	if request.Model != "gpt-4.1-mini" {
		t.Fatalf("model = %q, want default openai model", request.Model)
	}
}

func TestInfrastructureManagerTestProviderAccountFailureIsSanitized(t *testing.T) {
	workspaceID := uuid.New()
	secret := "sk-live-leaky-value"
	client := &provider.FakeClient{
		Err: provider.NewFailure("openai", provider.FailureCodeAuth, "bad key "+secret+" from workspace-secret://PROVIDER_OPENAI_API_KEY", false, errors.New("auth")),
	}
	repo := &providerAccountTestRepo{
		secrets: map[string]string{"PROVIDER_OPENAI_API_KEY": secret},
	}
	manager := NewInfrastructureManager(repo).WithProviderClient(client)

	result, err := manager.TestProviderAccount(context.Background(), repository.ProviderAccountRow{
		ID:                  uuid.New(),
		WorkspaceID:         &workspaceID,
		ProviderKey:         "openai",
		CredentialReference: "workspace-secret://PROVIDER_OPENAI_API_KEY",
		Status:              "active",
	}, ProviderAccountTestInput{Model: "custom-model"})
	if err != nil {
		t.Fatalf("TestProviderAccount returned error: %v", err)
	}
	if result.Passed {
		t.Fatalf("result = %#v, want failed", result)
	}
	if result.Code != string(provider.FailureCodeAuth) {
		t.Fatalf("code = %q, want auth", result.Code)
	}
	if strings.Contains(result.Message, secret) || strings.Contains(result.Message, "workspace-secret://") {
		t.Fatalf("message was not sanitized: %q", result.Message)
	}
	if !strings.Contains(result.Message, "[redacted]") || !strings.Contains(result.Message, "[credential-reference]") {
		t.Fatalf("message missing redaction markers: %q", result.Message)
	}
}

func TestInfrastructureManagerTestProviderAccountSanitizesShortWorkspaceSecret(t *testing.T) {
	workspaceID := uuid.New()
	secret := "abc"
	client := &provider.FakeClient{
		Err: provider.NewFailure("openai", provider.FailureCodeAuth, "bad key "+secret, false, errors.New("auth")),
	}
	repo := &providerAccountTestRepo{
		secrets: map[string]string{"PROVIDER_OPENAI_API_KEY": secret},
	}
	manager := NewInfrastructureManager(repo).WithProviderClient(client)

	result, err := manager.TestProviderAccount(context.Background(), repository.ProviderAccountRow{
		ID:                  uuid.New(),
		WorkspaceID:         &workspaceID,
		ProviderKey:         "openai",
		CredentialReference: "workspace-secret://PROVIDER_OPENAI_API_KEY",
		Status:              "active",
	}, ProviderAccountTestInput{})
	if err != nil {
		t.Fatalf("TestProviderAccount returned error: %v", err)
	}
	if strings.Contains(result.Message, secret) || !strings.Contains(result.Message, "[redacted]") {
		t.Fatalf("message was not sanitized: %q", result.Message)
	}
}

func TestInfrastructureManagerTestProviderAccountSanitizesEnvCredential(t *testing.T) {
	workspaceID := uuid.New()
	secret := "env-secret-value"
	t.Setenv("OPENAI_API_KEY", secret)
	client := &provider.FakeClient{
		Err: provider.NewFailure("openai", provider.FailureCodeAuth, "bad key "+secret, false, errors.New("auth")),
	}
	manager := NewInfrastructureManager(&providerAccountTestRepo{}).WithProviderClient(client)

	result, err := manager.TestProviderAccount(context.Background(), repository.ProviderAccountRow{
		ID:                  uuid.New(),
		WorkspaceID:         &workspaceID,
		ProviderKey:         "openai",
		CredentialReference: "env://OPENAI_API_KEY",
		Status:              "active",
	}, ProviderAccountTestInput{})
	if err != nil {
		t.Fatalf("TestProviderAccount returned error: %v", err)
	}
	if strings.Contains(result.Message, secret) || !strings.Contains(result.Message, "[redacted]") {
		t.Fatalf("message was not sanitized: %q", result.Message)
	}
}

type providerAccountTestRepo struct {
	secrets map[string]string
}

func (r *providerAccountTestRepo) CreateRuntimeProfile(context.Context, repository.CreateRuntimeProfileParams) (repository.RuntimeProfileRow, error) {
	return repository.RuntimeProfileRow{}, nil
}
func (r *providerAccountTestRepo) GetRuntimeProfileByID(context.Context, uuid.UUID) (repository.RuntimeProfileRow, error) {
	return repository.RuntimeProfileRow{}, repository.ErrRuntimeProfileNotFound
}
func (r *providerAccountTestRepo) ListRuntimeProfilesByWorkspaceID(context.Context, uuid.UUID) ([]repository.RuntimeProfileRow, error) {
	return nil, nil
}
func (r *providerAccountTestRepo) ArchiveRuntimeProfile(context.Context, uuid.UUID) error { return nil }
func (r *providerAccountTestRepo) CreateProviderAccount(context.Context, repository.CreateProviderAccountParams) (repository.ProviderAccountRow, error) {
	return repository.ProviderAccountRow{}, nil
}
func (r *providerAccountTestRepo) GetProviderAccountByID(context.Context, uuid.UUID) (repository.ProviderAccountRow, error) {
	return repository.ProviderAccountRow{}, repository.ErrProviderAccountNotFound
}
func (r *providerAccountTestRepo) ListProviderAccountsByWorkspaceID(context.Context, uuid.UUID) ([]repository.ProviderAccountRow, error) {
	return nil, nil
}
func (r *providerAccountTestRepo) ArchiveProviderAccount(context.Context, uuid.UUID) error {
	return nil
}
func (r *providerAccountTestRepo) UpsertWorkspaceSecret(context.Context, repository.UpsertWorkspaceSecretParams) error {
	return nil
}
func (r *providerAccountTestRepo) LoadWorkspaceSecrets(context.Context, uuid.UUID) (map[string]string, error) {
	return r.secrets, nil
}
func (r *providerAccountTestRepo) ListModelCatalogEntries(context.Context) ([]repository.ModelCatalogEntryRow, error) {
	return nil, nil
}
func (r *providerAccountTestRepo) GetModelCatalogEntryByID(context.Context, uuid.UUID) (repository.ModelCatalogEntryRow, error) {
	return repository.ModelCatalogEntryRow{}, repository.ErrModelCatalogNotFound
}
func (r *providerAccountTestRepo) CreateModelAlias(context.Context, repository.CreateModelAliasParams) (repository.ModelAliasRow, error) {
	return repository.ModelAliasRow{}, nil
}
func (r *providerAccountTestRepo) GetModelAliasByID(context.Context, uuid.UUID) (repository.ModelAliasRow, error) {
	return repository.ModelAliasRow{}, repository.ErrModelAliasNotFound
}
func (r *providerAccountTestRepo) ListModelAliasesByWorkspaceID(context.Context, uuid.UUID) ([]repository.ModelAliasRow, error) {
	return nil, nil
}
func (r *providerAccountTestRepo) ArchiveModelAlias(context.Context, uuid.UUID) error { return nil }
func (r *providerAccountTestRepo) CreateTool(context.Context, repository.CreateToolParams) (repository.ToolRow, error) {
	return repository.ToolRow{}, nil
}
func (r *providerAccountTestRepo) GetToolByID(context.Context, uuid.UUID) (repository.ToolRow, error) {
	return repository.ToolRow{}, repository.ErrToolNotFound
}
func (r *providerAccountTestRepo) ListToolsByWorkspaceID(context.Context, uuid.UUID) ([]repository.ToolRow, error) {
	return nil, nil
}
func (r *providerAccountTestRepo) UpdateTool(context.Context, repository.UpdateToolParams) (repository.ToolRow, error) {
	return repository.ToolRow{}, nil
}
func (r *providerAccountTestRepo) ArchiveTool(context.Context, uuid.UUID) error {
	return nil
}
func (r *providerAccountTestRepo) CreateKnowledgeSource(context.Context, repository.CreateKnowledgeSourceParams) (repository.KnowledgeSourceRow, error) {
	return repository.KnowledgeSourceRow{}, nil
}
func (r *providerAccountTestRepo) GetKnowledgeSourceByID(context.Context, uuid.UUID) (repository.KnowledgeSourceRow, error) {
	return repository.KnowledgeSourceRow{}, repository.ErrKnowledgeSourceNotFound
}
func (r *providerAccountTestRepo) ListKnowledgeSourcesByWorkspaceID(context.Context, uuid.UUID) ([]repository.KnowledgeSourceRow, error) {
	return nil, nil
}
func (r *providerAccountTestRepo) CreateRoutingPolicy(context.Context, repository.CreateRoutingPolicyParams) (repository.RoutingPolicyRow, error) {
	return repository.RoutingPolicyRow{}, nil
}
func (r *providerAccountTestRepo) ListRoutingPoliciesByWorkspaceID(context.Context, uuid.UUID) ([]repository.RoutingPolicyRow, error) {
	return nil, nil
}
func (r *providerAccountTestRepo) CreateSpendPolicy(context.Context, repository.CreateSpendPolicyParams) (repository.SpendPolicyRow, error) {
	return repository.SpendPolicyRow{}, nil
}
func (r *providerAccountTestRepo) ListSpendPoliciesByWorkspaceID(context.Context, uuid.UUID) ([]repository.SpendPolicyRow, error) {
	return nil, nil
}
func (r *providerAccountTestRepo) GetOrganizationIDByWorkspaceID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}
