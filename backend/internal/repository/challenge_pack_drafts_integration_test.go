package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRepositoryPublishChallengePackBundleMarksSourceDraftFinalInSameTransaction(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	draft, err := repo.CreateChallengePackDraft(ctx, repository.CreateChallengePackDraftParams{
		WorkspaceID: fixture.workspaceID,
		Name:        "Draft pack",
		Composition: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("CreateChallengePackDraft returned error: %v", err)
	}
	source := &repository.PackPublicationSource{DraftID: draft.ID, Composition: []byte(`{}`)}

	bundle := publishableChallengePackBundle("draft-source-pack-"+uuid.NewString(), "Draft Source Pack", "draft-source-v1")
	first, err := repo.PublishChallengePackBundle(ctx, repository.PublishChallengePackBundleParams{
		Source:         source,
		OrganizationID: fixture.organizationID,
		WorkspaceID:    fixture.workspaceID,
		Bundle:         bundle,
	})
	if err != nil {
		t.Fatalf("PublishChallengePackBundle returned error: %v", err)
	}
	published, err := repo.GetChallengePackDraftByID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("GetChallengePackDraftByID returned error: %v", err)
	}
	if published.Status != "published" ||
		published.LastPublishedVersionID == nil || *published.LastPublishedVersionID != first.ChallengePackVersionID ||
		published.ChallengePackID == nil || *published.ChallengePackID != first.ChallengePackID {
		t.Fatalf("draft after publish = %#v, want published and linked to %s", published, first.ChallengePackVersionID)
	}

	// A published draft is final. A second publish fails before it writes a
	// version, with the same version number or with a new one, and the draft
	// stays linked to the first version.
	for _, number := range []int32{bundle.Version.Number, bundle.Version.Number + 1} {
		bundle.Version.Number = number
		_, err := repo.PublishChallengePackBundle(ctx, repository.PublishChallengePackBundleParams{
			Source:         source,
			OrganizationID: fixture.organizationID,
			WorkspaceID:    fixture.workspaceID,
			Bundle:         bundle,
		})
		var final repository.ChallengePackDraftFinalError
		if !errors.Is(err, repository.ErrInvalidTransition) || !errors.As(err, &final) || final.Status != repository.ChallengePackDraftStatusPublished {
			t.Fatalf("republish version %d error = %v, want ChallengePackDraftFinalError", number, err)
		}
	}
	var versions int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM challenge_pack_versions WHERE challenge_pack_id = $1`, first.ChallengePackID).Scan(&versions); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if versions != 1 {
		t.Fatalf("pack versions = %d, want 1 after refused republish", versions)
	}
	relinked, err := repo.GetChallengePackDraftByID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("GetChallengePackDraftByID returned error: %v", err)
	}
	if relinked.LastPublishedVersionID == nil || *relinked.LastPublishedVersionID != first.ChallengePackVersionID {
		t.Fatalf("draft version link = %v, want %s", relinked.LastPublishedVersionID, first.ChallengePackVersionID)
	}

	// The status write outside of a publish uses the same guard.
	if _, err := repo.MarkChallengePackDraftPublished(ctx, repository.MarkChallengePackDraftPublishedParams{
		DraftID:                draft.ID,
		WorkspaceID:            fixture.workspaceID,
		ChallengePackID:        first.ChallengePackID,
		ChallengePackVersionID: first.ChallengePackVersionID,
	}); !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("MarkChallengePackDraftPublished on a published draft error = %v, want ErrInvalidTransition", err)
	}
}

func TestRepositoryPatchChallengePackDraftRefusesPublishedDraft(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	draft, err := repo.CreateChallengePackDraft(ctx, repository.CreateChallengePackDraftParams{
		WorkspaceID: fixture.workspaceID,
		Name:        "Draft pack",
		Composition: []byte(`{"pack":{"name":"before"}}`),
	})
	if err != nil {
		t.Fatalf("CreateChallengePackDraft returned error: %v", err)
	}

	rename := "Renamed draft"
	renamed, err := repo.PatchChallengePackDraft(ctx, repository.PatchChallengePackDraftParams{ID: draft.ID, Name: &rename})
	if err != nil {
		t.Fatalf("PatchChallengePackDraft on a draft returned error: %v", err)
	}
	if renamed.Name != rename {
		t.Fatalf("draft name = %q, want %q", renamed.Name, rename)
	}

	bundle := publishableChallengePackBundle("patch-final-pack-"+uuid.NewString(), "Patch Final Pack", "patch-final-v1")
	if _, err := repo.PublishChallengePackBundle(ctx, repository.PublishChallengePackBundleParams{
		Source:         &repository.PackPublicationSource{DraftID: draft.ID, Composition: []byte(`{}`)},
		OrganizationID: fixture.organizationID,
		WorkspaceID:    fixture.workspaceID,
		Bundle:         bundle,
	}); err != nil {
		t.Fatalf("PublishChallengePackBundle returned error: %v", err)
	}

	late := "Changed after publish"
	_, err = repo.PatchChallengePackDraft(ctx, repository.PatchChallengePackDraftParams{
		ID:          draft.ID,
		Name:        &late,
		Composition: []byte(`{"pack":{"name":"after"}}`),
	})
	var final repository.ChallengePackDraftFinalError
	if !errors.Is(err, repository.ErrInvalidTransition) || !errors.As(err, &final) || final.ID != draft.ID {
		t.Fatalf("PatchChallengePackDraft on a published draft error = %v, want ChallengePackDraftFinalError", err)
	}
	stored, err := repo.GetChallengePackDraftByID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("GetChallengePackDraftByID returned error: %v", err)
	}
	var storedComposition struct {
		Pack struct {
			Name string `json:"name"`
		} `json:"pack"`
	}
	if err := json.Unmarshal(stored.Composition, &storedComposition); err != nil {
		t.Fatalf("decode stored composition: %v", err)
	}
	if stored.Name != rename || storedComposition.Pack.Name != "before" {
		t.Fatalf("published draft = name %q composition %s, want the content from before the publish", stored.Name, stored.Composition)
	}

	if _, err := repo.PatchChallengePackDraft(ctx, repository.PatchChallengePackDraftParams{ID: uuid.New(), Name: &late}); !errors.Is(err, repository.ErrChallengePackDraftNotFound) {
		t.Fatalf("PatchChallengePackDraft on a missing draft error = %v, want ErrChallengePackDraftNotFound", err)
	}
}

func TestRepositoryPublishChallengePackBundleRollsBackWhenSourceDraftIsMissing(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	slug := "missing-draft-pack-" + uuid.NewString()
	_, err := repo.PublishChallengePackBundle(ctx, repository.PublishChallengePackBundleParams{
		Source:         &repository.PackPublicationSource{DraftID: uuid.New(), Composition: []byte(`{}`)},
		OrganizationID: fixture.organizationID,
		WorkspaceID:    fixture.workspaceID,
		Bundle:         publishableChallengePackBundle(slug, "Missing Draft Pack", "missing-draft-v1"),
	})
	if !errors.Is(err, repository.ErrChallengePackDraftNotFound) {
		t.Fatalf("publish with missing draft error = %v, want ErrChallengePackDraftNotFound", err)
	}
	var packs int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM challenge_packs WHERE slug = $1`, slug).Scan(&packs); err != nil {
		t.Fatalf("count packs: %v", err)
	}
	if packs != 0 {
		t.Fatalf("packs with slug = %d, want 0 after rollback", packs)
	}
}
