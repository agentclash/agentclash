package api

import (
	"context"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

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

	// Sustained contention falls back to a collision-resistant suffix instead
	// of dropping a valid create after the sequential candidates are claimed.
	repo = &providerAccountTestRepo{
		orgID:         uuid.New(),
		existingTools: []repository.ToolRow{{Slug: "search-the-web"}},
		slugConflicts: map[string]int{
			"search-the-web-2": 1,
			"search-the-web-3": 1,
			"search-the-web-4": 1,
		},
	}
	mgr = NewInfrastructureManager(repo)
	created, _, err = mgr.CreateToolsFromLibrary(ctx, Caller{}, wsID, CreateToolsFromLibraryInput{
		Entries: []FromLibraryEntryInput{{Slug: "web-search", Conflict: "suffix"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 || !strings.HasPrefix(created[0].Slug, "search-the-web-") || len(created[0].Slug) != len("search-the-web-")+8 {
		t.Fatalf("expected collision-resistant suffix after sustained conflicts; got %#v", created)
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
