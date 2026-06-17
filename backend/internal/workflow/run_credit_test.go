package workflow

import (
	"context"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type fakeRunCreditStore struct {
	reservation repository.CreditReservation
	found       bool
	cost        int64
	settled     []int64
	released    int
}

func (f *fakeRunCreditStore) GetOpenReservationByRunID(context.Context, uuid.UUID) (repository.CreditReservation, bool, error) {
	return f.reservation, f.found, nil
}
func (f *fakeRunCreditStore) GetRunActualCostMicros(context.Context, uuid.UUID) (int64, error) {
	return f.cost, nil
}
func (f *fakeRunCreditStore) SettleEvalCredit(_ context.Context, _ uuid.UUID, _ string, micros int64, _ repository.CreditRef) error {
	f.settled = append(f.settled, micros)
	return nil
}
func (f *fakeRunCreditStore) ReleaseEvalCredit(context.Context, uuid.UUID, string, repository.CreditRef) error {
	f.released++
	return nil
}

func TestFinalizeRunCredit_SettlesObservedSpend(t *testing.T) {
	f := &fakeRunCreditStore{found: true, reservation: repository.CreditReservation{ID: uuid.New(), AmountMicros: 1_000_000}, cost: 600_000}
	if err := finalizeRunCredit(context.Background(), f, FinalizeRunCreditInput{RunID: uuid.New(), TerminalStatus: "completed"}); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if len(f.settled) != 1 || f.settled[0] != 600_000 || f.released != 0 {
		t.Fatalf("settled=%v released=%d, want one settle of 600k, no release", f.settled, f.released)
	}
}

func TestFinalizeRunCredit_ReleasesWhenZeroCost(t *testing.T) {
	f := &fakeRunCreditStore{found: true, reservation: repository.CreditReservation{ID: uuid.New(), AmountMicros: 1_000_000}, cost: 0}
	if err := finalizeRunCredit(context.Background(), f, FinalizeRunCreditInput{RunID: uuid.New(), TerminalStatus: "failed"}); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if f.released != 1 || len(f.settled) != 0 {
		t.Fatalf("released=%d settled=%v, want one release, no settle", f.released, f.settled)
	}
}

func TestFinalizeRunCredit_NoReservationIsNoop(t *testing.T) {
	f := &fakeRunCreditStore{found: false} // BYOK / unmanaged run
	if err := finalizeRunCredit(context.Background(), f, FinalizeRunCreditInput{RunID: uuid.New(), TerminalStatus: "completed"}); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if len(f.settled) != 0 || f.released != 0 {
		t.Fatalf("settled=%v released=%d, want no-op", f.settled, f.released)
	}
}
