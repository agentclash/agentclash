package repository_test

import (
	"context"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
)

func TestMultiTurnHumanTurnStoreReentryKeepsSubmittedMessage(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	store := repository.NewMultiTurnHumanTurnStore(db)
	req := repository.HumanTurnWaitParams{RunAgentID: fixture.primaryRunAgentID, TurnIndex: 2, PhaseID: "phase-a"}

	if err := store.MarkAwaitingHuman(ctx, req); err != nil {
		t.Fatalf("MarkAwaitingHuman returned error: %v", err)
	}
	if err := store.SubmitHumanMessage(ctx, fixture.primaryRunAgentID, 2, "hello"); err != nil {
		t.Fatalf("SubmitHumanMessage returned error: %v", err)
	}
	if err := store.MarkAwaitingHuman(ctx, req); err != nil {
		t.Fatalf("re-entering MarkAwaitingHuman returned error: %v", err)
	}

	message, ready, err := store.PollHumanMessage(ctx, fixture.primaryRunAgentID, 2)
	if err != nil {
		t.Fatalf("PollHumanMessage returned error: %v", err)
	}
	if !ready || message != "hello" {
		t.Fatalf("poll = (%q, %v), want (\"hello\", true)", message, ready)
	}
}

func TestMultiTurnHumanTurnStoreReentryRearmsExpiredTurn(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	store := repository.NewMultiTurnHumanTurnStore(db)
	req := repository.HumanTurnWaitParams{RunAgentID: fixture.primaryRunAgentID, TurnIndex: 1, PhaseID: "phase-a"}

	if err := store.MarkAwaitingHuman(ctx, req); err != nil {
		t.Fatalf("MarkAwaitingHuman returned error: %v", err)
	}
	if err := store.MarkHumanTurnExpired(ctx, fixture.primaryRunAgentID, 1); err != nil {
		t.Fatalf("MarkHumanTurnExpired returned error: %v", err)
	}
	if err := store.MarkAwaitingHuman(ctx, req); err != nil {
		t.Fatalf("re-arm MarkAwaitingHuman returned error: %v", err)
	}

	awaiting, err := store.AwaitingTurn(ctx, fixture.primaryRunAgentID)
	if err != nil {
		t.Fatalf("AwaitingTurn returned error: %v", err)
	}
	if awaiting == nil || awaiting.TurnIndex != 1 {
		t.Fatalf("AwaitingTurn = %#v, want turn 1 awaiting", awaiting)
	}
	if err := store.SubmitHumanMessage(ctx, fixture.primaryRunAgentID, 1, "late answer"); err != nil {
		t.Fatalf("SubmitHumanMessage after re-arm returned error: %v", err)
	}
}
