package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRepositoryPublishChallengePackBundleMarksSourceDraftInSameTransaction(t *testing.T) {
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

	// Publishing the same version again fails and leaves the draft linked to
	// the version that exists.
	if _, err := repo.PublishChallengePackBundle(ctx, repository.PublishChallengePackBundleParams{
		Source:         source,
		OrganizationID: fixture.organizationID,
		WorkspaceID:    fixture.workspaceID,
		Bundle:         bundle,
	}); !errors.Is(err, repository.ErrChallengePackVersionExists) {
		t.Fatalf("republish same version error = %v, want ErrChallengePackVersionExists", err)
	}

	// A draft may publish a new version; the link moves to it.
	bundle.Version.Number = 2
	second, err := repo.PublishChallengePackBundle(ctx, repository.PublishChallengePackBundleParams{
		Source:         source,
		OrganizationID: fixture.organizationID,
		WorkspaceID:    fixture.workspaceID,
		Bundle:         bundle,
	})
	if err != nil {
		t.Fatalf("publish version 2 returned error: %v", err)
	}
	relinked, err := repo.GetChallengePackDraftByID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("GetChallengePackDraftByID returned error: %v", err)
	}
	if relinked.LastPublishedVersionID == nil || *relinked.LastPublishedVersionID != second.ChallengePackVersionID {
		t.Fatalf("draft version link = %v, want %s", relinked.LastPublishedVersionID, second.ChallengePackVersionID)
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
