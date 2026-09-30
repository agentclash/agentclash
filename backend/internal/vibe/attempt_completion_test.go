package vibe

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"sync"
	"testing"
	"time"
)

func TestReceiptAndResponseOrderingsPreserveExecution(t *testing.T) {
	for _, first := range []bool{true, false} {
		t.Run(map[bool]string{true: "receipt-first", false: "response-first"}[first], func(t *testing.T) {
			s := integrationStore(t)
			ctx := context.Background()
			v := anonSession(t, s)
			o, _ := submitPlan(t, s, v, testConfig(), 1000)
			if _, _, e := s.Start(ctx, o.ID); e != nil {
				t.Fatal(e)
			}
			a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "target", Role: Target, Model: o.Models.Target, Policy: json.RawMessage(`{}`), RequestHash: "order", InputBound: 100, MaxOutput: 100, MaxCost: 300}
			if e := s.BeginAttempt(ctx, a); e != nil {
				t.Fatal(e)
			}
			receipt := json.RawMessage(`{"receipt":"provider"}`)
			usage := json.RawMessage(`{"output_tokens":17}`)
			cost := int64(250)
			if first {
				if e := s.ReconcileCost(ctx, a.ID, cost, receipt); e != nil {
					t.Fatal(e)
				}
				var completed *time.Time
				if e := s.DB.QueryRow(ctx, `SELECT completed_at FROM vibe_attempts WHERE id=$1`, a.ID).Scan(&completed); e != nil || completed != nil {
					t.Fatal("receipt fabricated execution completion", e)
				}
				if e := s.AppendOutput(ctx, a.ID, "streamed "); e != nil {
					t.Fatal(e)
				}
			}
			if e := s.EndAttempt(ctx, a, "complete response", usage, &cost, nil); e != nil {
				t.Fatal(e)
			}
			if !first {
				if e := s.ReconcileCost(ctx, a.ID, cost, receipt); e != nil {
					t.Fatal(e)
				}
			}
			if e := s.Finish(ctx, o.ID, nil); e != nil {
				t.Fatal(e)
			}
			var wg sync.WaitGroup
			errs := make(chan error, 4)
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); errs <- s.ReconcileCost(ctx, a.ID, cost, receipt) }()
			}
			wg.Wait()
			close(errs)
			for e := range errs {
				if e != nil {
					t.Fatal(e)
				}
			}
			var output string
			var savedUsage, savedReceipt []byte
			if e := s.DB.QueryRow(ctx, `SELECT output,usage,reconciliation_evidence FROM vibe_attempts WHERE id=$1`, a.ID).Scan(&output, &savedUsage, &savedReceipt); e != nil {
				t.Fatal(e)
			}
			if output != "complete response" || !sameJSON(savedUsage, usage) {
				t.Fatal("execution evidence lost")
			}
			var evidence costEvidence
			if e := json.Unmarshal(savedReceipt, &evidence); e != nil || len(evidence.Receipts) != 1 {
				t.Fatal("receipt lost or duplicated", e)
			}
			var charged int64
			var accounts int64
			if e := s.DB.QueryRow(ctx, `SELECT count(*) FROM vibe_reservations WHERE operation_id=$1`, o.ID).Scan(&accounts); e != nil {
				t.Fatal(e)
			}
			if e := s.DB.QueryRow(ctx, `SELECT sum(settled_amount) FROM vibe_reservations WHERE operation_id=$1`, o.ID).Scan(&charged); e != nil || charged != cost*accounts {
				t.Fatal("settlement changed", charged, e)
			}
		})
	}
}
func TestConflictingCostsFreezeFundingAndPreserveBothEvidence(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	v := anonSession(t, s)
	o, _ := submitPlan(t, s, v, testConfig(), 1000)
	if _, _, e := s.Start(ctx, o.ID); e != nil {
		t.Fatal(e)
	}
	a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "target", Role: Target, Model: o.Models.Target, Policy: json.RawMessage(`{}`), RequestHash: "conflict", InputBound: 100, MaxOutput: 100, MaxCost: 300}
	t.Cleanup(func() { s.DB.Exec(ctx, `DELETE FROM vibe_disabled_profiles WHERE model=$1`, a.Model) })
	if e := s.BeginAttempt(ctx, a); e != nil {
		t.Fatal(e)
	}
	if e := s.ReconcileCost(ctx, a.ID, 200, json.RawMessage(`{"receipt":"200"}`)); e != nil {
		t.Fatal(e)
	}
	cost := int64(250)
	requireFault(t, s.EndAttempt(ctx, a, "real response", json.RawMessage(`{"cost":250}`), &cost, nil), "reconciliation_conflict")
	if e := s.Finish(ctx, o.ID, nil); e != nil {
		t.Fatal(e)
	}
	var output string
	var evidence []byte
	if e := s.DB.QueryRow(ctx, `SELECT output,reconciliation_evidence FROM vibe_attempts WHERE id=$1`, a.ID).Scan(&output, &evidence); e != nil {
		t.Fatal(e)
	}
	var saved costEvidence
	json.Unmarshal(evidence, &saved)
	if output != "real response" || !saved.Conflict || saved.ResponseCost == nil || *saved.ResponseCost != 250 || saved.Receipts[0].Cost != 200 {
		t.Fatal("conflicting evidence discarded")
	}
	o, e := s.Operation(ctx, o.ID)
	if e != nil || o.Billing != Reconciling {
		t.Fatal("conflict settled", e)
	}
	var frozen bool
	if e = s.DB.QueryRow(ctx, `SELECT bool_and(a.disabled AND a.held>=r.amount) FROM vibe_accounts a JOIN vibe_reservations r ON r.account_id=a.id WHERE r.operation_id=$1`, o.ID).Scan(&frozen); e != nil || !frozen {
		t.Fatal("funding not frozen", e)
	}
}
