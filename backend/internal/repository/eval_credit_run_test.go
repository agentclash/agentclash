package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestEvalCredit_GetOpenReservationByRunID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)
	mustGrant(t, ctx, repo, org, "seed", 3_000_000)

	runID := uuid.New()
	res, err := repo.ReserveEvalCredit(ctx, org, "run:"+runID.String(), 1_000_000, repository.CreditRef{RunID: &runID})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	got, found, err := repo.GetOpenReservationByRunID(ctx, runID)
	if err != nil || !found || got.ID != res.ID {
		t.Fatalf("open reservation lookup: found=%v id=%v err=%v", found, got.ID, err)
	}

	// Once resolved it is no longer "open".
	if err := repo.SettleEvalCredit(ctx, res.ID, "settle", 500_000, repository.CreditRef{}); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if _, found, _ := repo.GetOpenReservationByRunID(ctx, runID); found {
		t.Fatal("settled reservation should not be returned as open")
	}
}

func TestEvalCredit_GetRunActualCostMicros_NoScorecard(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)

	// Unknown run → no agents → no scorecards → 0 (the failed/cancelled-before-scoring case).
	micros, err := repo.GetRunActualCostMicros(ctx, uuid.New())
	if err != nil {
		t.Fatalf("cost: %v", err)
	}
	if micros != 0 {
		t.Fatalf("cost = %d, want 0 (no scorecard)", micros)
	}
}

func TestEvalCredit_GetOpenReservationByRunID_DuplicateErrors(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := repository.New(db)
	org := seedEvalCreditOrg(t, ctx, db)
	mustGrant(t, ctx, repo, org, "seed", 5_000_000)

	runID := uuid.New()
	// Two distinct reservations both pointing at the same run (invariant violation).
	if _, err := repo.ReserveEvalCredit(ctx, org, "run:"+runID.String()+":a", 1_000_000, repository.CreditRef{RunID: &runID}); err != nil {
		t.Fatalf("reserve a: %v", err)
	}
	if _, err := repo.ReserveEvalCredit(ctx, org, "run:"+runID.String()+":b", 1_000_000, repository.CreditRef{RunID: &runID}); err != nil {
		t.Fatalf("reserve b: %v", err)
	}
	if _, _, err := repo.GetOpenReservationByRunID(ctx, runID); !errors.Is(err, repository.ErrMultipleOpenEvalCreditReservations) {
		t.Fatalf("err = %v, want ErrMultipleOpenEvalCreditReservations", err)
	}
}
