package vibe

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

// A cost receipt or a response that arrives after settlement, with a cost
// that conflicts with the settled cost, must not reopen the settlement
// (#1382). The operation stays SETTLED with its settled cost. The attempt
// keeps the conflict in its evidence, and the funding accounts freeze.
func TestVibeLateCostConflictKeepsSettledBillingFinal(t *testing.T) {
	const cost, late int64 = 5000000, 6000000
	usage := json.RawMessage(`{"output_tokens":17}`)
	t.Run("late receipt", func(t *testing.T) {
		s, _, o, a := lateCostOperation(t)
		ctx := context.Background()
		saved := cost
		if err := s.EndAttempt(ctx, a, "reply", usage, &saved, nil); err != nil {
			t.Fatal(err)
		}
		if err := s.Finish(ctx, o.ID, nil); err != nil {
			t.Fatal(err)
		}
		before := settledFunding(t, s, o.ID, cost)
		requireFault(t, s.ReconcileCost(ctx, a.ID, late, json.RawMessage(`{"cost":0.006}`)), "reconciliation_conflict")
		if err := s.Finish(ctx, o.ID, nil); err != nil {
			t.Fatal(err)
		}
		evidence := requireSettlementKept(t, s, o.ID, a.ID, cost, before)
		if len(evidence.Receipts) != 1 || evidence.Receipts[0].Cost != late {
			t.Fatalf("the late receipt is not in the evidence: %+v", evidence)
		}
	})
	t.Run("late response", func(t *testing.T) {
		s, v, o, a := lateCostOperation(t)
		ctx := context.Background()
		if err := s.ReconcileCost(ctx, a.ID, cost, json.RawMessage(`{"cost":0.005}`)); err != nil {
			t.Fatal(err)
		}
		if err := s.Stop(ctx, v.Actor, o.ID); err != nil {
			t.Fatal(err)
		}
		before := settledFunding(t, s, o.ID, cost)
		response := late
		requireFault(t, s.EndAttempt(ctx, a, "late reply", usage, &response, nil), "reconciliation_conflict")
		if err := s.Finish(ctx, o.ID, nil); err != nil {
			t.Fatal(err)
		}
		evidence := requireSettlementKept(t, s, o.ID, a.ID, cost, before)
		if evidence.ResponseCost == nil || *evidence.ResponseCost != late {
			t.Fatalf("the late response cost is not in the evidence: %+v", evidence)
		}
	})
}

// lateCostOperation starts a funded guest operation with one dispatched attempt.
func lateCostOperation(t *testing.T) (*Store, Session, Operation, Attempt) {
	t.Helper()
	s := integrationStore(t)
	v := anonSession(t, s)
	o, _ := submitPlan(t, s, v, testConfig(), 100000000)
	ctx := context.Background()
	if _, _, err := s.Start(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "target:1", Role: Target, Model: o.Models.Target, Policy: json.RawMessage(`{}`), RequestHash: "late-cost", InputBound: 1000, MaxOutput: 100, MaxCost: 10000000}
	if err := s.BeginAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	return s, v, o, a
}

type reservationFunding struct {
	Account string
	Amount  int64
	Settled *int64
	Balance int64
	Held    int64
}

func operationFundingRows(t *testing.T, s *Store, id uuid.UUID) ([]reservationFunding, int) {
	t.Helper()
	rows, err := s.DB.Query(context.Background(), `SELECT r.account_id,r.amount,r.settled_amount,a.balance,a.held,a.disabled FROM vibe_reservations r JOIN vibe_accounts a ON a.id=r.account_id WHERE r.operation_id=$1 ORDER BY r.account_id`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	funding, disabled := []reservationFunding{}, 0
	for rows.Next() {
		var f reservationFunding
		var off bool
		if err = rows.Scan(&f.Account, &f.Amount, &f.Settled, &f.Balance, &f.Held, &off); err != nil {
			t.Fatal(err)
		}
		if off {
			disabled++
		}
		funding = append(funding, f)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(funding) == 0 {
		t.Fatal("the operation has no reservations")
	}
	return funding, disabled
}

// settledFunding checks the settlement before the late conflict and returns
// the money state of each reservation.
func settledFunding(t *testing.T, s *Store, id uuid.UUID, cost int64) []reservationFunding {
	t.Helper()
	o, err := s.Operation(context.Background(), id)
	if err != nil || o.Billing != Settled || o.ActualCost == nil || *o.ActualCost != cost {
		t.Fatalf("the operation did not settle at %d: billing=%s actual_cost=%v err=%v", cost, o.Billing, o.ActualCost, err)
	}
	funding, disabled := operationFundingRows(t, s, id)
	for _, f := range funding {
		if f.Settled == nil || *f.Settled != cost || f.Held != 0 {
			t.Fatalf("the reservation did not settle at %d: %+v", cost, f)
		}
	}
	if disabled != 0 {
		t.Fatalf("a funding account froze before the conflict: disabled=%d", disabled)
	}
	return funding
}

// requireSettlementKept checks that the late conflict did not reopen the
// settlement, and returns the attempt evidence.
func requireSettlementKept(t *testing.T, s *Store, id, attempt uuid.UUID, cost int64, before []reservationFunding) costEvidence {
	t.Helper()
	o, err := s.Operation(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if o.Billing != Settled || o.ActualCost == nil || *o.ActualCost != cost {
		t.Fatalf("the late conflict reopened a settled operation: billing=%s actual_cost=%v, want SETTLED and %d", o.Billing, o.ActualCost, cost)
	}
	after, disabled := operationFundingRows(t, s, id)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("the late conflict moved money: before=%+v after=%+v", before, after)
	}
	if disabled != len(after) {
		t.Fatalf("the late conflict did not freeze the funding: disabled=%d accounts=%d", disabled, len(after))
	}
	var known *int64
	var saved []byte
	if err = s.DB.QueryRow(context.Background(), `SELECT actual_cost,reconciliation_evidence FROM vibe_attempts WHERE id=$1`, attempt).Scan(&known, &saved); err != nil {
		t.Fatal(err)
	}
	if known == nil || *known != cost {
		t.Fatalf("the late conflict changed the attempt cost: %v", known)
	}
	var evidence costEvidence
	if err = json.Unmarshal(saved, &evidence); err != nil || !evidence.Conflict {
		t.Fatalf("the attempt evidence has no conflict: %s err=%v", saved, err)
	}
	return evidence
}
