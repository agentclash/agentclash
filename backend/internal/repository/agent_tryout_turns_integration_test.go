package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
)

func TestRepositoryAgentTryoutTurnLeaseReclaim(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	tryout := createQueuedAnonymousAgentTryout(t, ctx, repo)

	first, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: *tryout.AnonymousOwnerProofHash, Message: "first"})
	if err != nil {
		t.Fatalf("AppendAgentTryoutTurn returned error: %v", err)
	}
	second, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: *tryout.AnonymousOwnerProofHash, Message: "second"})
	if err != nil {
		t.Fatalf("AppendAgentTryoutTurn returned error: %v", err)
	}

	if _, _, err := repo.ClaimNextPendingAgentTryoutTurn(ctx, tryout.ID, 0); err == nil {
		t.Fatal("claim with a zero lease returned nil error")
	}

	claimed, ok, err := repo.ClaimNextPendingAgentTryoutTurn(ctx, tryout.ID, time.Hour)
	if err != nil || !ok {
		t.Fatalf("claim = ok %v err %v, want the first turn", ok, err)
	}
	if claimed.ID != first.ID || claimed.Status != repository.AgentTryoutTurnStatusProcessing || claimed.LeaseUntil == nil {
		t.Fatalf("claimed turn = %#v, want first turn processing with a lease", claimed)
	}

	// The first turn's session "crashes". While its lease runs, the next
	// claim takes the second turn and leaves the first alone.
	next, ok, err := repo.ClaimNextPendingAgentTryoutTurn(ctx, tryout.ID, time.Hour)
	if err != nil || !ok || next.ID != second.ID {
		t.Fatalf("second claim = %#v ok %v err %v, want the second turn", next, ok, err)
	}
	if _, ok, err := repo.ClaimNextPendingAgentTryoutTurn(ctx, tryout.ID, time.Hour); err != nil || ok {
		t.Fatalf("claim with live leases = ok %v err %v, want nothing", ok, err)
	}

	// Once the lease runs out, the stuck turn is claimed again.
	if _, err := db.Exec(ctx, `UPDATE agent_tryout_turns SET lease_until = now() - interval '1 minute' WHERE id = $1`, first.ID); err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	reclaimed, ok, err := repo.ClaimNextPendingAgentTryoutTurn(ctx, tryout.ID, time.Hour)
	if err != nil || !ok || reclaimed.ID != first.ID {
		t.Fatalf("reclaim = %#v ok %v err %v, want the first turn", reclaimed, ok, err)
	}
	if reclaimed.LeaseUntil == nil || !reclaimed.LeaseUntil.After(time.Now().Add(30*time.Minute)) {
		t.Fatalf("reclaimed lease = %v, want a fresh lease", reclaimed.LeaseUntil)
	}

	// A processing turn from before leases has no lease and counts as expired.
	if _, err := db.Exec(ctx, `UPDATE agent_tryout_turns SET lease_until = NULL WHERE id = $1`, second.ID); err != nil {
		t.Fatalf("clear lease: %v", err)
	}
	legacy, ok, err := repo.ClaimNextPendingAgentTryoutTurn(ctx, tryout.ID, time.Hour)
	if err != nil || !ok || legacy.ID != second.ID {
		t.Fatalf("legacy reclaim = %#v ok %v err %v, want the second turn", legacy, ok, err)
	}

	if err := repo.MarkAgentTryoutTurnProcessed(ctx, first.ID); err != nil {
		t.Fatalf("MarkAgentTryoutTurnProcessed returned error: %v", err)
	}
	// Marking a done turn again is an idempotent no-op.
	if err := repo.MarkAgentTryoutTurnProcessed(ctx, first.ID); err != nil {
		t.Fatalf("second MarkAgentTryoutTurnProcessed returned error: %v", err)
	}
	var status string
	var leaseCleared bool
	if err := db.QueryRow(ctx, `SELECT status, lease_until IS NULL FROM agent_tryout_turns WHERE id = $1`, first.ID).Scan(&status, &leaseCleared); err != nil {
		t.Fatalf("load turn: %v", err)
	}
	if status != "done" || !leaseCleared {
		t.Fatalf("done turn = %q lease cleared %v, want done without a lease", status, leaseCleared)
	}
	// A done turn is never claimed again.
	if _, err := db.Exec(ctx, `UPDATE agent_tryout_turns SET lease_until = now() - interval '1 minute' WHERE id = $1`, first.ID); err != nil {
		t.Fatalf("set stale lease on done turn: %v", err)
	}
	if _, ok, err := repo.ClaimNextPendingAgentTryoutTurn(ctx, tryout.ID, time.Hour); err != nil || ok {
		t.Fatalf("claim after done = ok %v err %v, want nothing", ok, err)
	}

	pending, err := repo.AppendAgentTryoutTurn(ctx, repository.AppendAgentTryoutTurnParams{AgentTryoutID: tryout.ID, OwnerProofHash: *tryout.AnonymousOwnerProofHash, Message: "third"})
	if err != nil {
		t.Fatalf("AppendAgentTryoutTurn returned error: %v", err)
	}
	if err := repo.MarkAgentTryoutTurnProcessed(ctx, pending.ID); !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("mark pending turn error = %v, want ErrInvalidTransition", err)
	}
	if err := repo.MarkAgentTryoutTurnProcessed(ctx, -1); !errors.Is(err, repository.ErrAgentTryoutTurnNotFound) {
		t.Fatalf("mark missing turn error = %v, want ErrAgentTryoutTurnNotFound", err)
	}
}
