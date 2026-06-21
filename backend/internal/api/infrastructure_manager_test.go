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

func TestCreateToolsFromLibrary(t *testing.T) {
	ctx := context.Background()
	repo := &providerAccountTestRepo{orgID: uuid.New()}
	mgr := NewInfrastructureManager(repo)

	created, skipped, err := mgr.CreateToolsFromLibrary(ctx, Caller{}, uuid.New(), CreateToolsFromLibraryInput{
		Entries: []FromLibraryEntryInput{
			{Slug: "web-search"},                     // default (live delegate)
			{Slug: "slack-message", Variant: "live"}, // live HTTP variant
			{Slug: "not-a-real-tool"},                // unknown -> skipped
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("created = %d, want 2", len(created))
	}
	if len(skipped) != 1 || skipped[0].Slug != "not-a-real-tool" {
		t.Fatalf("skipped = %#v, want the unknown slug", skipped)
	}
	slugs := map[string]bool{}
	for _, row := range created {
		slugs[row.Slug] = true
	}
	if !slugs["search-the-web"] || !slugs["send-a-slack-message"] {
		t.Fatalf("created slugs = %v, want search-the-web and send-a-slack-message", slugs)
	}
}

func TestCreateToolsFromLibraryConflicts(t *testing.T) {
	ctx := context.Background()
	wsID := uuid.New()

	// Default conflict behaviour skips a slug that already exists.
	repo := &providerAccountTestRepo{orgID: uuid.New(), existingTools: []repository.ToolRow{{Slug: "search-the-web"}}}
	mgr := NewInfrastructureManager(repo)
	created, skipped, err := mgr.CreateToolsFromLibrary(ctx, Caller{}, wsID, CreateToolsFromLibraryInput{
		Entries: []FromLibraryEntryInput{{Slug: "web-search"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 0 || len(skipped) != 1 {
		t.Fatalf("expected a skip on conflict; created=%d skipped=%d", len(created), len(skipped))
	}

	// conflict=suffix appends a numeric suffix instead of skipping.
	repo = &providerAccountTestRepo{orgID: uuid.New(), existingTools: []repository.ToolRow{{Slug: "search-the-web"}}}
	mgr = NewInfrastructureManager(repo)
	created, _, err = mgr.CreateToolsFromLibrary(ctx, Caller{}, wsID, CreateToolsFromLibraryInput{
		Entries: []FromLibraryEntryInput{{Slug: "web-search", Conflict: "suffix"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 || created[0].Slug != "search-the-web-2" {
		t.Fatalf("expected created slug search-the-web-2; got %#v", created)
	}

	// A concurrent writer can claim the selected suffix after the initial list.
	// Retry against the repository uniqueness constraint and choose the next one.
	repo = &providerAccountTestRepo{
		orgID:         uuid.New(),
		existingTools: []repository.ToolRow{{Slug: "search-the-web"}},
		slugConflicts: map[string]int{"search-the-web-2": 1},
	}
	mgr = NewInfrastructureManager(repo)
	created, _, err = mgr.CreateToolsFromLibrary(ctx, Caller{}, wsID, CreateToolsFromLibraryInput{
		Entries: []FromLibraryEntryInput{{Slug: "web-search", Conflict: "suffix"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 || created[0].Slug != "search-the-web-3" {
		t.Fatalf("expected concurrent conflict retry to create search-the-web-3; got %#v", created)
	}
}

func TestCreateToolsFromLibraryInputValidate(t *testing.T) {
	if err := (&CreateToolsFromLibraryInput{}).Validate(); err == nil {
		t.Fatal("empty entries should fail validation")
	}
	tooMany := make([]FromLibraryEntryInput, maxToolsFromLibraryEntries+1)
	for i := range tooMany {
		tooMany[i].Slug = "web-search"
	}
	if err := (&CreateToolsFromLibraryInput{Entries: tooMany}).Validate(); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("oversized entries error = %v, want maximum validation error", err)
	}
}

type providerAccountTestRepo struct {
	secrets map[string]string
	// Optional tool fixtures (used by the library tests; zero values keep the
	// original behaviour for the provider-account tests).
	orgID         uuid.UUID
	existingTools []repository.ToolRow
	createdTools  []repository.ToolRow
	slugConflicts map[string]int
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
func (r *providerAccountTestRepo) CreateTool(_ context.Context, p repository.CreateToolParams) (repository.ToolRow, error) {
	if r.slugConflicts[p.Slug] > 0 {
		r.slugConflicts[p.Slug]--
		return repository.ToolRow{}, repository.ErrSlugTaken
	}
	ws := p.WorkspaceID
	row := repository.ToolRow{
		ID:            uuid.New(),
		WorkspaceID:   &ws,
		Name:          p.Name,
		Slug:          p.Slug,
		ToolKind:      p.ToolKind,
		CapabilityKey: p.CapabilityKey,
		Definition:    p.Definition,
	}
	r.createdTools = append(r.createdTools, row)
	return row, nil
}
func (r *providerAccountTestRepo) GetToolByID(context.Context, uuid.UUID) (repository.ToolRow, error) {
	return repository.ToolRow{}, repository.ErrToolNotFound
}
func (r *providerAccountTestRepo) ListToolsByWorkspaceID(context.Context, uuid.UUID) ([]repository.ToolRow, error) {
	return r.existingTools, nil
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
	if r.orgID != uuid.Nil {
		return r.orgID, nil
	}
	return uuid.New(), nil
}
