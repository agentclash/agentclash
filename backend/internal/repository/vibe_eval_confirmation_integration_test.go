package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

// Confirmation/audit repo round-trips for Step 3a (#875 §5.3/§6).
//
// WARNING — DESTRUCTIVE: calls seedFixture, which TRUNCATEs core tables CASCADE. Point
// DATABASE_URL at a DISPOSABLE database (see vibe_eval_concurrency_integration_test.go for the
// throwaway-container recipe), never a dev/shared DB. Skips when DATABASE_URL is unset.

func seedVibeEvalConversation(t *testing.T, ctx context.Context, repo *repository.Repository, f testFixture) repository.VibeEvalConversation {
	t.Helper()
	conv, err := repo.CreateVibeEvalConversation(ctx, repository.CreateVibeEvalConversationParams{
		OrganizationID:  f.organizationID,
		WorkspaceID:     f.workspaceID,
		CreatedByUserID: f.userID,
		Title:           "confirm",
		Phase:           "author",
		Status:          "active",
	})
	if err != nil {
		t.Fatalf("CreateVibeEvalConversation: %v", err)
	}
	return conv
}

func newPendingParams(f testFixture, convID uuid.UUID, hash string, expires time.Time) repository.CreateVibeEvalPendingConfirmationParams {
	return repository.CreateVibeEvalPendingConfirmationParams{
		OrganizationID:   f.organizationID,
		WorkspaceID:      f.workspaceID,
		ConversationID:   convID,
		ProposedByUserID: f.userID,
		ToolName:         "publish_draft",
		ToolCallID:       "toolu_pub_1",
		Action:           "publish_challenge_pack",
		RiskTier:         "workspace_write",
		PayloadHash:      hash,
		Summary:          "Publish draft as challenge pack",
		ExpiresAt:        expires,
	}
}

func TestRepositoryVibeEvalToolInvocationRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	conv := seedVibeEvalConversation(t, ctx, repo, fixture)

	// Nullable confirmation_id / message_id and JSON payloads must round-trip.
	got, err := repo.AppendVibeEvalToolInvocation(ctx, repository.AppendVibeEvalToolInvocationParams{
		OrganizationID: fixture.organizationID,
		WorkspaceID:    fixture.workspaceID,
		ConversationID: conv.ID,
		ActorUserID:    fixture.userID,
		ToolName:       "get_run_status",
		Action:         "read_workspace",
		RiskTier:       "read",
		PayloadHash:    "hash-abc",
		RequestPayload: []byte(`{"run_id":"r1"}`),
		ResultPayload:  []byte(`{"status":"draft"}`),
		Outcome:        "ok",
	})
	if err != nil {
		t.Fatalf("AppendVibeEvalToolInvocation: %v", err)
	}
	if got.MessageID != nil || got.ConfirmationID != nil || got.CreditReservationID != nil {
		t.Fatalf("expected nil optional refs, got message=%v confirmation=%v credit=%v", got.MessageID, got.ConfirmationID, got.CreditReservationID)
	}
	if got.Outcome != "ok" || got.RiskTier != "read" {
		t.Fatalf("unexpected row: %+v", got)
	}

	list, err := repo.ListVibeEvalToolInvocationsByConversationID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("ListVibeEvalToolInvocationsByConversationID: %v", err)
	}
	if len(list) != 1 || list[0].ID != got.ID {
		t.Fatalf("list = %d rows, want the one appended row", len(list))
	}
}

func TestRepositoryVibeEvalConfirmationApproveIsSingleUse(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	conv := seedVibeEvalConversation(t, ctx, repo, fixture)

	pc, err := repo.CreateVibeEvalPendingConfirmation(ctx, newPendingParams(fixture, conv.ID, "hash-1", time.Now().Add(time.Hour)))
	if err != nil {
		t.Fatalf("CreateVibeEvalPendingConfirmation: %v", err)
	}
	if pc.Status != "pending" {
		t.Fatalf("status = %q, want pending", pc.Status)
	}

	approved, err := repo.ApproveVibeEvalPendingConfirmation(ctx, pc.ID, "hash-1", fixture.userID)
	if err != nil {
		t.Fatalf("first approve: %v", err)
	}
	if approved.Status != "executing" {
		t.Fatalf("status = %q, want executing", approved.Status)
	}
	if approved.ResolvedByUserID == nil || *approved.ResolvedByUserID != fixture.userID {
		t.Fatalf("resolved_by = %v, want %s", approved.ResolvedByUserID, fixture.userID)
	}

	// Second approve (e.g. a concurrent/retried POST) must not re-claim.
	if _, err := repo.ApproveVibeEvalPendingConfirmation(ctx, pc.ID, "hash-1", fixture.userID); !errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
		t.Fatalf("second approve err = %v, want ErrVibeEvalConfirmationNotResolvable", err)
	}
}

func TestRepositoryVibeEvalConfirmationHashMismatchAndDeny(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	conv := seedVibeEvalConversation(t, ctx, repo, fixture)

	pc, err := repo.CreateVibeEvalPendingConfirmation(ctx, newPendingParams(fixture, conv.ID, "hash-2", time.Now().Add(time.Hour)))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Presented hash mismatch (bait-and-switch) is rejected, row stays pending.
	if _, err := repo.ApproveVibeEvalPendingConfirmation(ctx, pc.ID, "wrong-hash", fixture.userID); !errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
		t.Fatalf("mismatch approve err = %v, want ErrVibeEvalConfirmationNotResolvable", err)
	}

	denied, err := repo.DenyVibeEvalPendingConfirmation(ctx, pc.ID, "hash-2", fixture.userID)
	if err != nil {
		t.Fatalf("deny: %v", err)
	}
	if denied.Status != "denied" {
		t.Fatalf("status = %q, want denied", denied.Status)
	}
	// Second resolve (approve or deny) after denied must fail.
	if _, err := repo.ApproveVibeEvalPendingConfirmation(ctx, pc.ID, "hash-2", fixture.userID); !errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
		t.Fatalf("approve-after-deny err = %v, want ErrVibeEvalConfirmationNotResolvable", err)
	}
}

func TestRepositoryVibeEvalConfirmationExpiryUnblocksReproposal(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	conv := seedVibeEvalConversation(t, ctx, repo, fixture)

	// A lapsed pending row (already past expiry) must not block a fresh proposal with the same
	// (conversation, tool, payload_hash): the create path expires it in-tx first.
	stale, err := repo.CreateVibeEvalPendingConfirmation(ctx, newPendingParams(fixture, conv.ID, "hash-3", time.Now().Add(-time.Minute)))
	if err != nil {
		t.Fatalf("create stale: %v", err)
	}

	fresh, err := repo.CreateVibeEvalPendingConfirmation(ctx, newPendingParams(fixture, conv.ID, "hash-3", time.Now().Add(time.Hour)))
	if err != nil {
		t.Fatalf("re-propose after expiry should succeed, got: %v", err)
	}
	if fresh.ID == stale.ID {
		t.Fatal("expected a new confirmation row")
	}

	staleReloaded, err := repo.GetVibeEvalPendingConfirmationByID(ctx, stale.ID)
	if err != nil {
		t.Fatalf("get stale: %v", err)
	}
	if staleReloaded.Status != "expired" {
		t.Fatalf("stale status = %q, want expired", staleReloaded.Status)
	}
	if fresh.Status != "pending" {
		t.Fatalf("fresh status = %q, want pending", fresh.Status)
	}
}

func TestRepositoryVibeEvalConfirmationMarkResultAndResume(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	conv := seedVibeEvalConversation(t, ctx, repo, fixture)

	pc, err := repo.CreateVibeEvalPendingConfirmation(ctx, newPendingParams(fixture, conv.ID, "hash-4", time.Now().Add(time.Hour)))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.ApproveVibeEvalPendingConfirmation(ctx, pc.ID, "hash-4", fixture.userID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	// While executing, the crash-safe resume primitive returns the row for matching hash only.
	if _, err := repo.GetVibeEvalPendingConfirmationForResume(ctx, pc.ID, "hash-4"); err != nil {
		t.Fatalf("resume while executing: %v", err)
	}
	if _, err := repo.GetVibeEvalPendingConfirmationForResume(ctx, pc.ID, "wrong"); !errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
		t.Fatalf("resume wrong-hash err = %v, want ErrVibeEvalConfirmationNotResolvable", err)
	}

	done, err := repo.MarkVibeEvalPendingConfirmationSucceeded(ctx, pc.ID)
	if err != nil {
		t.Fatalf("mark succeeded: %v", err)
	}
	if done.Status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", done.Status)
	}

	// Finalize exactly once: a second mark (or fail) must not re-transition a terminal row.
	if _, err := repo.MarkVibeEvalPendingConfirmationSucceeded(ctx, pc.ID); !errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
		t.Fatalf("double mark err = %v, want ErrVibeEvalConfirmationNotResolvable", err)
	}
	if _, err := repo.MarkVibeEvalPendingConfirmationFailed(ctx, pc.ID); !errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
		t.Fatalf("mark-failed-after-succeeded err = %v, want ErrVibeEvalConfirmationNotResolvable", err)
	}
	// Resume after terminal must also reject.
	if _, err := repo.GetVibeEvalPendingConfirmationForResume(ctx, pc.ID, "hash-4"); !errors.Is(err, repository.ErrVibeEvalConfirmationNotResolvable) {
		t.Fatalf("resume after terminal err = %v, want ErrVibeEvalConfirmationNotResolvable", err)
	}
}
