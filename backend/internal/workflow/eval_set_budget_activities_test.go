package workflow

import (
	"context"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
)

func TestRefreshEvalSetSpendReconcilesCaseCosts(t *testing.T) {
	setID := uuid.New()
	budget := 10.0
	repo := &fakeEvalSetRepo{
		set: repository.EvalSet{
			ID:        setID,
			Status:    domain.EvalSetStatusRunning,
			BudgetUSD: &budget,
			SpentUSD:  0,
		},
		caseCosts: 3.5,
	}
	acts := (&Activities{}).WithEvalSetBudgetRepository(repo)
	result, err := acts.RefreshEvalSetSpend(context.Background(), RefreshEvalSetSpendInput{EvalSetID: setID})
	if err != nil {
		t.Fatal(err)
	}
	if result.SpentUSD != 3.5 {
		t.Fatalf("spent=%v", result.SpentUSD)
	}
	if !result.Allowed {
		t.Fatal("should allow under budget")
	}
	for range 3 {
		replayed, err := acts.RefreshEvalSetSpend(context.Background(), RefreshEvalSetSpendInput{EvalSetID: setID})
		if err != nil || replayed.SpentUSD != 3.5 {
			t.Fatalf("replayed reconciliation: spent=%v err=%v", replayed.SpentUSD, err)
		}
	}
	check, err := acts.CheckEvalSetBudget(context.Background(), CheckEvalSetBudgetInput{EvalSetID: setID})
	if err != nil {
		t.Fatal(err)
	}
	if check.SpentUSD != 3.5 {
		t.Fatalf("check spent=%v", check.SpentUSD)
	}
}

func TestWaitWorkspaceRunCapacityQueuesWhenFull(t *testing.T) {
	repo := &fakeEvalSetRepo{activeRuns: 2}
	acts := (&Activities{}).WithWorkspaceRunCounter(repo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // force immediate stop after first check fails capacity
	err := acts.WaitWorkspaceRunCapacity(ctx, WaitWorkspaceRunCapacityInput{
		WorkspaceID:   uuid.New(),
		MaxConcurrent: 1,
	})
	if err == nil {
		t.Fatal("expected context cancel while waiting for capacity")
	}
}
