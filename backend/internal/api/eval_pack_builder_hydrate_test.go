package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/evalpack"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

var errBuilderHydrateNotImpl = errors.New("not implemented")

// fakeBuilderHydrateRepository is a minimal EvalPackBuilderRepository for
// exercising CreateDraft's hydrate-from-version branch. Only the version lookup,
// public-packs flag, and draft creation are meaningful; the rest are stubs.
type fakeBuilderHydrateRepository struct {
	version     repository.RunnableEvalPackVersion
	versionErr  error
	publicPacks bool
	lastCreate  repository.CreateEvalPackDraftParams
}

func (f *fakeBuilderHydrateRepository) GetRunnableEvalPackVersionByID(_ context.Context, _ uuid.UUID) (repository.RunnableEvalPackVersion, error) {
	if f.versionErr != nil {
		return repository.RunnableEvalPackVersion{}, f.versionErr
	}
	return f.version, nil
}

func (f *fakeBuilderHydrateRepository) WorkspacePublicPacksEnabled(_ context.Context, _ uuid.UUID) (bool, error) {
	return f.publicPacks, nil
}

func (f *fakeBuilderHydrateRepository) CreateEvalPackDraft(_ context.Context, params repository.CreateEvalPackDraftParams) (repository.EvalPackDraft, error) {
	f.lastCreate = params
	return repository.EvalPackDraft{
		ID:              uuid.New(),
		WorkspaceID:     params.WorkspaceID,
		Name:            params.Name,
		ExecutionMode:   params.ExecutionMode,
		EvalPackID: params.EvalPackID,
		Composition:     params.Composition,
		Status:          "draft",
	}, nil
}

func (f *fakeBuilderHydrateRepository) CreateChallengePiece(context.Context, repository.CreateChallengePieceParams) (repository.ChallengePiece, error) {
	return repository.ChallengePiece{}, errBuilderHydrateNotImpl
}
func (f *fakeBuilderHydrateRepository) GetChallengePieceByID(context.Context, uuid.UUID) (repository.ChallengePiece, error) {
	return repository.ChallengePiece{}, errBuilderHydrateNotImpl
}
func (f *fakeBuilderHydrateRepository) ListChallengePieces(context.Context, uuid.UUID, *string) ([]repository.ChallengePiece, error) {
	return nil, nil
}
func (f *fakeBuilderHydrateRepository) ListChallengePiecesByIDs(context.Context, uuid.UUID, []uuid.UUID) ([]repository.ChallengePiece, error) {
	return nil, nil
}
func (f *fakeBuilderHydrateRepository) PatchChallengePiece(context.Context, repository.PatchChallengePieceParams) (repository.ChallengePiece, error) {
	return repository.ChallengePiece{}, errBuilderHydrateNotImpl
}
func (f *fakeBuilderHydrateRepository) ArchiveChallengePiece(context.Context, uuid.UUID, uuid.UUID) error {
	return errBuilderHydrateNotImpl
}
func (f *fakeBuilderHydrateRepository) GetEvalPackDraftByID(context.Context, uuid.UUID) (repository.EvalPackDraft, error) {
	return repository.EvalPackDraft{}, errBuilderHydrateNotImpl
}
func (f *fakeBuilderHydrateRepository) ListEvalPackDrafts(context.Context, uuid.UUID) ([]repository.EvalPackDraft, error) {
	return nil, nil
}
func (f *fakeBuilderHydrateRepository) PatchEvalPackDraft(context.Context, repository.PatchEvalPackDraftParams) (repository.EvalPackDraft, error) {
	return repository.EvalPackDraft{}, errBuilderHydrateNotImpl
}
func (f *fakeBuilderHydrateRepository) DeleteEvalPackDraft(context.Context, uuid.UUID) error {
	return errBuilderHydrateNotImpl
}

func catalogManifest(t *testing.T, slug string) json.RawMessage {
	t.Helper()
	entry, ok, err := evalpack.CatalogBySlug(slug)
	if err != nil || !ok {
		t.Fatalf("CatalogBySlug(%s): ok=%v err=%v", slug, ok, err)
	}
	bundle, err := evalpack.ParseYAML([]byte(entry.YAML))
	if err != nil {
		t.Fatalf("ParseYAML: %v", err)
	}
	manifest, err := evalpack.ManifestJSON(bundle)
	if err != nil {
		t.Fatalf("ManifestJSON: %v", err)
	}
	return manifest
}

func adminCaller(workspaceID uuid.UUID) Caller {
	return Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceAdmin},
		},
	}
}

func TestCreateDraftHydratesFromOwnWorkspaceVersion(t *testing.T) {
	workspaceID := uuid.New()
	packID := uuid.New()
	versionID := uuid.New()
	manifest := catalogManifest(t, "json-output-conformance")

	repo := &fakeBuilderHydrateRepository{
		version: repository.RunnableEvalPackVersion{
			ID:              versionID,
			EvalPackID: packID,
			WorkspaceID:     &workspaceID,
			Manifest:        manifest,
		},
	}
	manager := NewEvalPackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, nil)

	draft, err := manager.CreateDraft(context.Background(), adminCaller(workspaceID), CreateDraftInput{
		WorkspaceID:                workspaceID,
		FromEvalPackVersionID: &versionID,
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	if draft.EvalPackID == nil || *draft.EvalPackID != packID {
		t.Fatalf("eval_pack_id = %v, want %s", draft.EvalPackID, packID)
	}
	if draft.ExecutionMode != "prompt_eval" {
		t.Errorf("execution_mode = %q, want prompt_eval", draft.ExecutionMode)
	}
	if draft.Name == "" {
		t.Error("name should default to the pack name")
	}
	if len(draft.Composition) == 0 {
		t.Fatal("composition was not hydrated")
	}

	// The hydrated composition must recompose to the original manifest.
	var comp evalpack.Composition
	if err := json.Unmarshal(draft.Composition, &comp); err != nil {
		t.Fatalf("unmarshal composition: %v", err)
	}
	recomposed, err := evalpack.ComposeBundle(comp, nil)
	if err != nil {
		t.Fatalf("ComposeBundle: %v", err)
	}
	got, err := evalpack.ManifestJSON(recomposed)
	if err != nil {
		t.Fatalf("ManifestJSON: %v", err)
	}
	if !bytes.Equal(got, manifest) {
		t.Errorf("hydrated composition does not recompose to the original manifest")
	}
}

func TestCreateDraftRejectsForeignWorkspaceVersion(t *testing.T) {
	workspaceID := uuid.New()
	otherWorkspaceID := uuid.New()
	versionID := uuid.New()

	repo := &fakeBuilderHydrateRepository{
		version: repository.RunnableEvalPackVersion{
			ID:          versionID,
			WorkspaceID: &otherWorkspaceID,
			Manifest:    catalogManifest(t, "json-output-conformance"),
		},
	}
	manager := NewEvalPackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, nil)

	_, err := manager.CreateDraft(context.Background(), adminCaller(workspaceID), CreateDraftInput{
		WorkspaceID:                workspaceID,
		FromEvalPackVersionID: &versionID,
	})
	if !errors.Is(err, repository.ErrEvalPackVersionNotFound) {
		t.Fatalf("error = %v, want ErrEvalPackVersionNotFound", err)
	}
}

func TestCreateDraftGlobalVersionRequiresPublicPacks(t *testing.T) {
	workspaceID := uuid.New()
	versionID := uuid.New()
	manifest := catalogManifest(t, "json-output-conformance")

	// Global pack (workspace_id == nil), public packs disabled → hidden.
	repo := &fakeBuilderHydrateRepository{
		version:     repository.RunnableEvalPackVersion{ID: versionID, WorkspaceID: nil, Manifest: manifest},
		publicPacks: false,
	}
	manager := NewEvalPackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, nil)
	if _, err := manager.CreateDraft(context.Background(), adminCaller(workspaceID), CreateDraftInput{
		WorkspaceID:                workspaceID,
		FromEvalPackVersionID: &versionID,
	}); !errors.Is(err, repository.ErrEvalPackVersionNotFound) {
		t.Fatalf("public packs disabled: error = %v, want ErrEvalPackVersionNotFound", err)
	}

	// Public packs enabled → allowed.
	repo.publicPacks = true
	if _, err := manager.CreateDraft(context.Background(), adminCaller(workspaceID), CreateDraftInput{
		WorkspaceID:                workspaceID,
		FromEvalPackVersionID: &versionID,
	}); err != nil {
		t.Fatalf("public packs enabled: unexpected error %v", err)
	}
}

func TestCreateDraftRejectsCompositionWithFromVersion(t *testing.T) {
	workspaceID := uuid.New()
	versionID := uuid.New()
	repo := &fakeBuilderHydrateRepository{}
	manager := NewEvalPackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, nil)

	_, err := manager.CreateDraft(context.Background(), adminCaller(workspaceID), CreateDraftInput{
		WorkspaceID:                workspaceID,
		FromEvalPackVersionID: &versionID,
		Composition:                json.RawMessage(`{"pack":{}}`),
	})
	var validationErr EvalPackBuilderValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want EvalPackBuilderValidationError", err)
	}
}
