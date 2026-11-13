package repository_test

import (
	"context"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRepositoryGetNextWorkspaceChallengePackVersionNumber(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	slug := "next-version-pack-" + uuid.NewString()

	next := func(workspaceID uuid.UUID) int32 {
		t.Helper()
		got, err := repo.GetNextWorkspaceChallengePackVersionNumber(ctx, workspaceID, slug)
		if err != nil {
			t.Fatalf("GetNextWorkspaceChallengePackVersionNumber returned error: %v", err)
		}
		return got
	}

	if got := next(fixture.workspaceID); got != 1 {
		t.Fatalf("next version with no pack = %d, want 1", got)
	}

	bundle := publishableChallengePackBundle(slug, "Next Version Pack", "next-version-spec")
	publish := func(number int32) repository.PublishedChallengePack {
		t.Helper()
		bundle.Version.Number = number
		published, err := repo.PublishChallengePackBundle(ctx, repository.PublishChallengePackBundleParams{
			OrganizationID: fixture.organizationID,
			WorkspaceID:    fixture.workspaceID,
			Bundle:         bundle,
		})
		if err != nil {
			t.Fatalf("PublishChallengePackBundle version %d returned error: %v", number, err)
		}
		return published
	}

	first := publish(1)
	if got := next(fixture.workspaceID); got != 2 {
		t.Fatalf("next version after version 1 = %d, want 2", got)
	}
	publish(5)
	if got := next(fixture.workspaceID); got != 6 {
		t.Fatalf("next version after versions 1 and 5 = %d, want 6", got)
	}

	// The unique key on (challenge_pack_id, version_number) counts archived
	// versions, and the publish finds an archived pack by its slug.
	if _, err := db.Exec(ctx, `UPDATE challenge_pack_versions SET archived_at = now() WHERE challenge_pack_id = $1`, first.ChallengePackID); err != nil {
		t.Fatalf("archive versions: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE challenge_packs SET archived_at = now() WHERE id = $1`, first.ChallengePackID); err != nil {
		t.Fatalf("archive pack: %v", err)
	}
	if got := next(fixture.workspaceID); got != 6 {
		t.Fatalf("next version after archive = %d, want 6", got)
	}

	// A global pack and a pack of a different workspace with the same slug do
	// not count.
	var globalPackID uuid.UUID
	if err := db.QueryRow(ctx, `
		INSERT INTO challenge_packs (slug, name, family, lifecycle_status)
		VALUES ($1, 'Global Next Version Pack', 'support', 'active')
		RETURNING id
	`, slug).Scan(&globalPackID); err != nil {
		t.Fatalf("insert global pack: %v", err)
	}
	otherWorkspaceID := uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO workspaces (id, organization_id, name, slug) VALUES ($1, $2, 'Other', $3)`, otherWorkspaceID, fixture.organizationID, "other-"+uuid.NewString()); err != nil {
		t.Fatalf("insert other workspace: %v", err)
	}
	var otherPackID uuid.UUID
	if err := db.QueryRow(ctx, `
		INSERT INTO challenge_packs (workspace_id, slug, name, family, lifecycle_status)
		VALUES ($1, $2, 'Other Next Version Pack', 'support', 'active')
		RETURNING id
	`, otherWorkspaceID, slug).Scan(&otherPackID); err != nil {
		t.Fatalf("insert other workspace pack: %v", err)
	}
	for packID, number := range map[uuid.UUID]int{globalPackID: 9, otherPackID: 20} {
		if _, err := db.Exec(ctx, `
			INSERT INTO challenge_pack_versions (challenge_pack_id, version_number, lifecycle_status, manifest_checksum, manifest)
			VALUES ($1, $2, 'runnable', 'next-version-test', '{}'::jsonb)
		`, packID, number); err != nil {
			t.Fatalf("insert version %d: %v", number, err)
		}
	}
	if got := next(fixture.workspaceID); got != 6 {
		t.Fatalf("next version with a global pack and a pack of a different workspace = %d, want 6", got)
	}
	if got := next(otherWorkspaceID); got != 21 {
		t.Fatalf("next version of the other workspace = %d, want 21", got)
	}
}
