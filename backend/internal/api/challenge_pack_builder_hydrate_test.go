package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/google/uuid"
)

var errBuilderHydrateNotImpl = errors.New("not implemented")

// fakeBuilderHydrateRepository is a minimal ChallengePackBuilderRepository for
// exercising CreateDraft's hydrate-from-version branch. Only the version lookup,
// public-packs flag, and draft creation are meaningful; the rest are stubs.
type fakeBuilderHydrateRepository struct {
	version     repository.RunnableChallengePackVersion
	versionErr  error
	publicPacks bool
	lastCreate  repository.CreateChallengePackDraftParams
}

func (f *fakeBuilderHydrateRepository) GetRunnableChallengePackVersionByID(_ context.Context, _ uuid.UUID) (repository.RunnableChallengePackVersion, error) {
	if f.versionErr != nil {
		return repository.RunnableChallengePackVersion{}, f.versionErr
	}
	return f.version, nil
}

func (f *fakeBuilderHydrateRepository) WorkspacePublicPacksEnabled(_ context.Context, _ uuid.UUID) (bool, error) {
	return f.publicPacks, nil
}

func (f *fakeBuilderHydrateRepository) CreateChallengePackDraft(_ context.Context, params repository.CreateChallengePackDraftParams) (repository.ChallengePackDraft, error) {
	f.lastCreate = params
	return repository.ChallengePackDraft{
		ID:              uuid.New(),
		WorkspaceID:     params.WorkspaceID,
		Name:            params.Name,
		ExecutionMode:   params.ExecutionMode,
		ChallengePackID: params.ChallengePackID,
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
func (f *fakeBuilderHydrateRepository) GetChallengePackDraftByID(context.Context, uuid.UUID) (repository.ChallengePackDraft, error) {
	return repository.ChallengePackDraft{}, errBuilderHydrateNotImpl
}
func (f *fakeBuilderHydrateRepository) ListChallengePackDrafts(context.Context, uuid.UUID) ([]repository.ChallengePackDraft, error) {
	return nil, nil
}
func (f *fakeBuilderHydrateRepository) PatchChallengePackDraft(context.Context, repository.PatchChallengePackDraftParams) (repository.ChallengePackDraft, error) {
	return repository.ChallengePackDraft{}, errBuilderHydrateNotImpl
}
func (f *fakeBuilderHydrateRepository) DeleteChallengePackDraft(context.Context, uuid.UUID) error {
	return errBuilderHydrateNotImpl
}

func catalogManifest(t *testing.T, slug string) json.RawMessage {
	t.Helper()
	entry, ok, err := challengepack.CatalogBySlug(slug)
	if err != nil || !ok {
		t.Fatalf("CatalogBySlug(%s): ok=%v err=%v", slug, ok, err)
	}
	bundle, err := challengepack.ParseYAML([]byte(entry.YAML))
	if err != nil {
		t.Fatalf("ParseYAML: %v", err)
	}
	manifest, err := challengepack.ManifestJSON(bundle)
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
		version: repository.RunnableChallengePackVersion{
			ID:              versionID,
			ChallengePackID: packID,
			WorkspaceID:     &workspaceID,
			Manifest:        manifest,
		},
	}
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, nil)

	draft, err := manager.CreateDraft(context.Background(), adminCaller(workspaceID), CreateDraftInput{
		WorkspaceID:                workspaceID,
		FromChallengePackVersionID: &versionID,
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	if draft.ChallengePackID == nil || *draft.ChallengePackID != packID {
		t.Fatalf("challenge_pack_id = %v, want %s", draft.ChallengePackID, packID)
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
	var comp challengepack.Composition
	if err := json.Unmarshal(draft.Composition, &comp); err != nil {
		t.Fatalf("unmarshal composition: %v", err)
	}
	recomposed, err := challengepack.ComposeBundle(comp, nil)
	if err != nil {
		t.Fatalf("ComposeBundle: %v", err)
	}
	got, err := challengepack.ManifestJSON(recomposed)
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
		version: repository.RunnableChallengePackVersion{
			ID:          versionID,
			WorkspaceID: &otherWorkspaceID,
			Manifest:    catalogManifest(t, "json-output-conformance"),
		},
	}
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, nil)

	_, err := manager.CreateDraft(context.Background(), adminCaller(workspaceID), CreateDraftInput{
		WorkspaceID:                workspaceID,
		FromChallengePackVersionID: &versionID,
	})
	if !errors.Is(err, repository.ErrChallengePackVersionNotFound) {
		t.Fatalf("error = %v, want ErrChallengePackVersionNotFound", err)
	}
}

func TestCreateDraftGlobalVersionRequiresPublicPacks(t *testing.T) {
	workspaceID := uuid.New()
	versionID := uuid.New()
	manifest := catalogManifest(t, "json-output-conformance")

	// Global pack (workspace_id == nil), public packs disabled → hidden.
	repo := &fakeBuilderHydrateRepository{
		version:     repository.RunnableChallengePackVersion{ID: versionID, WorkspaceID: nil, Manifest: manifest},
		publicPacks: false,
	}
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, nil)
	if _, err := manager.CreateDraft(context.Background(), adminCaller(workspaceID), CreateDraftInput{
		WorkspaceID:                workspaceID,
		FromChallengePackVersionID: &versionID,
	}); !errors.Is(err, repository.ErrChallengePackVersionNotFound) {
		t.Fatalf("public packs disabled: error = %v, want ErrChallengePackVersionNotFound", err)
	}

	// Public packs enabled → allowed.
	repo.publicPacks = true
	if _, err := manager.CreateDraft(context.Background(), adminCaller(workspaceID), CreateDraftInput{
		WorkspaceID:                workspaceID,
		FromChallengePackVersionID: &versionID,
	}); err != nil {
		t.Fatalf("public packs enabled: unexpected error %v", err)
	}
}

func TestCreateDraftRejectsCompositionWithFromVersion(t *testing.T) {
	workspaceID := uuid.New()
	versionID := uuid.New()
	repo := &fakeBuilderHydrateRepository{}
	manager := NewChallengePackBuilderManager(NewCallerWorkspaceAuthorizer(), repo, nil)

	_, err := manager.CreateDraft(context.Background(), adminCaller(workspaceID), CreateDraftInput{
		WorkspaceID:                workspaceID,
		FromChallengePackVersionID: &versionID,
		Composition:                json.RawMessage(`{"pack":{}}`),
	})
	var validationErr ChallengePackBuilderValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want ChallengePackBuilderValidationError", err)
	}
}
