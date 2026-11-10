package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestMultiTurnHumanTurnStoreLocalReentryKeepsSubmittedMessage(t *testing.T) {
	ctx := context.Background()
	store := NewMultiTurnHumanTurnStore(nil)
	runAgentID := uuid.New()
	req := HumanTurnWaitParams{RunAgentID: runAgentID, TurnIndex: 2, PhaseID: "phase-a"}

	if err := store.MarkAwaitingHuman(ctx, req); err != nil {
		t.Fatalf("MarkAwaitingHuman returned error: %v", err)
	}
	if err := store.SubmitHumanMessage(ctx, runAgentID, 2, "hello"); err != nil {
		t.Fatalf("SubmitHumanMessage returned error: %v", err)
	}
	if err := store.MarkAwaitingHuman(ctx, req); err != nil {
		t.Fatalf("re-entering MarkAwaitingHuman returned error: %v", err)
	}

	message, ready, err := store.PollHumanMessage(ctx, runAgentID, 2)
	if err != nil {
		t.Fatalf("PollHumanMessage returned error: %v", err)
	}
	if !ready || message != "hello" {
		t.Fatalf("poll = (%q, %v), want (\"hello\", true)", message, ready)
	}
	awaiting, err := store.AwaitingTurn(ctx, runAgentID)
	if err != nil {
		t.Fatalf("AwaitingTurn returned error: %v", err)
	}
	if awaiting != nil {
		t.Fatalf("AwaitingTurn = %#v, want nil after submit", awaiting)
	}
}

func TestMultiTurnHumanTurnStoreLocalReentryRearmsExpiredTurn(t *testing.T) {
	ctx := context.Background()
	store := NewMultiTurnHumanTurnStore(nil)
	runAgentID := uuid.New()
	req := HumanTurnWaitParams{RunAgentID: runAgentID, TurnIndex: 1, PhaseID: "phase-a"}

	if err := store.MarkAwaitingHuman(ctx, req); err != nil {
		t.Fatalf("MarkAwaitingHuman returned error: %v", err)
	}
	if err := store.MarkHumanTurnExpired(ctx, runAgentID, 1); err != nil {
		t.Fatalf("MarkHumanTurnExpired returned error: %v", err)
	}
	if err := store.MarkAwaitingHuman(ctx, req); err != nil {
		t.Fatalf("re-arm MarkAwaitingHuman returned error: %v", err)
	}

	awaiting, err := store.AwaitingTurn(ctx, runAgentID)
	if err != nil {
		t.Fatalf("AwaitingTurn returned error: %v", err)
	}
	if awaiting == nil || awaiting.TurnIndex != 1 {
		t.Fatalf("AwaitingTurn = %#v, want turn 1 awaiting", awaiting)
	}
	if err := store.SubmitHumanMessage(ctx, runAgentID, 1, "late answer"); err != nil {
		t.Fatalf("SubmitHumanMessage after re-arm returned error: %v", err)
	}
}
