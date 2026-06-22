package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func guideEntitlements(limit *int) billing.EffectiveEntitlements {
	return billing.EffectiveEntitlements{
		PlanKey:                          "free",
		BillingPeriod:                    "monthly",
		Status:                           "active",
		SeatQuantity:                     1,
		GuideAgentTurnsPerWorkspaceMonth: limit,
	}
}

func guideTurnCount(t *testing.T, ctx context.Context, db *pgxpool.Pool, workspaceID uuid.UUID, windowStart time.Time) int {
	t.Helper()
	var count int
	if err := db.QueryRow(ctx, `SELECT guide_agent_turn_count FROM workspace_usage_windows WHERE workspace_id=$1 AND window_start=$2`, workspaceID, windowStart).Scan(&count); err != nil {
		t.Fatalf("read guide_agent_turn_count: %v", err)
	}
	return count
}

func intPtr(n int) *int { return &n }

func TestConsumeGuideAgentTurn_IncrementsAndBlocksAtLimit(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	windowStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	ent := guideEntitlements(intPtr(2)) // allowance of 2 turns

	// Two consumes succeed and increment.
	for i := 1; i <= 2; i++ {
		if err := repo.ConsumeGuideAgentTurn(ctx, fixture.workspaceID, ent, windowStart, windowEnd); err != nil {
			t.Fatalf("consume %d: %v", i, err)
		}
		if got := guideTurnCount(t, ctx, db, fixture.workspaceID, windowStart); got != i {
			t.Fatalf("count after consume %d = %d, want %d", i, got, i)
		}
	}

	// Third is blocked with a billing GateError, and does NOT increment.
	err := repo.ConsumeGuideAgentTurn(ctx, fixture.workspaceID, ent, windowStart, windowEnd)
	var gateErr billing.GateError
	if !errors.As(err, &gateErr) || gateErr.Decision.Code != billing.GateCodeQuotaExceeded {
		t.Fatalf("third consume err = %v, want quota-exceeded GateError", err)
	}
	if got := guideTurnCount(t, ctx, db, fixture.workspaceID, windowStart); got != 2 {
		t.Fatalf("count after blocked consume = %d, want 2 (no increment)", got)
	}
}

func TestConsumeGuideAgentTurn_UnlimitedAllowed(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	windowStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	ent := guideEntitlements(nil) // unlimited

	for i := 1; i <= 5; i++ {
		if err := repo.ConsumeGuideAgentTurn(ctx, fixture.workspaceID, ent, windowStart, windowEnd); err != nil {
			t.Fatalf("unlimited consume %d: %v", i, err)
		}
	}
	if got := guideTurnCount(t, ctx, db, fixture.workspaceID, windowStart); got != 5 {
		t.Fatalf("count = %d, want 5 (unlimited never blocks)", got)
	}
}

func TestConsumeGuideAgentTurn_ConcurrentLastTurnRace(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	windowStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	ent := guideEntitlements(intPtr(1)) // exactly ONE turn left

	const workers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	var success, blocked int
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			err := repo.ConsumeGuideAgentTurn(ctx, fixture.workspaceID, ent, windowStart, windowEnd)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
				return
			}
			var gateErr billing.GateError
			if errors.As(err, &gateErr) {
				blocked++
			} else {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if success != 1 || blocked != workers-1 {
		t.Fatalf("race outcome = {success:%d, blocked:%d}, want {1, %d}", success, blocked, workers-1)
	}
	if got := guideTurnCount(t, ctx, db, fixture.workspaceID, windowStart); got != 1 {
		t.Fatalf("count = %d, want exactly 1 (the last turn consumed once)", got)
	}
}
