package vibe

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestReceiptAndResponseOrderingsPreserveExecution(t *testing.T) {
	for _, first := range []bool{true, false} {
		t.Run(map[bool]string{true: "receipt-first", false: "response-first"}[first], func(t *testing.T) {
			svc, v := buildService(t)
			s := svc.Store
			ctx := context.Background()
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
	svc, v := buildService(t)
	s := svc.Store
	ctx := context.Background()
	o, _ := submitPlan(t, s, v, testConfig(), 1000)
	if _, _, e := s.Start(ctx, o.ID); e != nil {
		t.Fatal(e)
	}
	a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "target", Role: Target, Model: o.Models.Target, Policy: json.RawMessage(`{}`), RequestHash: "conflict", InputBound: 100, MaxOutput: 100, MaxCost: 300}
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
	var modelDisabled bool
	if e = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_disabled_profiles WHERE model=$1)`, a.Model).Scan(&modelDisabled); e != nil || modelDisabled {
		t.Fatal("one conflicting receipt disabled the shared model", e)
	}
	v, e = s.GetSession(ctx, v.Actor, v.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DeleteProject(ctx, v.Actor, v.ID, v.Revision); e != nil {
		t.Fatal(e)
	}
	requireFault(t, s.ReconcileCost(ctx, a.ID, 300, json.RawMessage(`{"private":"late receipt"}`)), "reconciliation_conflict")
	if e = s.DB.QueryRow(ctx, `SELECT reconciliation_evidence FROM vibe_attempts WHERE id=$1`, a.ID).Scan(&evidence); e != nil {
		t.Fatal(e)
	}
	saved = costEvidence{}
	if e = json.Unmarshal(evidence, &saved); e != nil || !saved.Conflict || saved.ResponseCost == nil || *saved.ResponseCost != 250 || len(saved.Receipts) != 2 || saved.Receipts[0].Cost != 200 || saved.Receipts[1].Cost != 300 {
		t.Fatal("deletion lost the conflicting financial amounts", string(evidence), e)
	}
	for _, receipt := range saved.Receipts {
		if len(receipt.Receipt) != 0 {
			t.Fatal("late reconciliation retained deleted receipt content")
		}
	}
}

func TestReceiptFirstInterruptionDoesNotFabricateSuccess(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "deleted"}[deleted], func(t *testing.T) {
			svc, v := buildService(t)
			s := svc.Store
			ctx := context.Background()
			o, _ := submitPlan(t, s, v, testConfig(), 1000)
			if _, _, e := s.Start(ctx, o.ID); e != nil {
				t.Fatal(e)
			}
			a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "target", Role: Target, Model: o.Models.Target, Policy: json.RawMessage(`{}`), RequestHash: "interrupted", InputBound: 100, MaxOutput: 100, MaxCost: 300}
			if e := s.BeginAttempt(ctx, a); e != nil {
				t.Fatal(e)
			}
			if e := s.ReconcileCost(ctx, a.ID, 200, json.RawMessage(`{"private":"receipt content"}`)); e != nil {
				t.Fatal(e)
			}
			if deleted {
				latest, e := s.GetSession(ctx, v.Actor, v.ID)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = s.DeleteProject(ctx, v.Actor, v.ID, latest.Revision); e != nil {
					t.Fatal(e)
				}
			}
			issue := &Fault{Code: "worker_interrupted", Message: "Reply interrupted."}
			if e := s.EndAttempt(ctx, a, "partial reply", json.RawMessage(`{"partial":true}`), nil, issue); e != nil {
				t.Fatal(e)
			}
			if e := s.Finish(ctx, o.ID, issue); e != nil {
				t.Fatal(e)
			}
			if deleted {
				if e := s.CleanupProjects(ctx); e != nil {
					t.Fatal(e)
				}
			}
			var state, output string
			var saved []byte
			if e := s.DB.QueryRow(ctx, `SELECT state,output,reconciliation_evidence FROM vibe_attempts WHERE id=$1`, a.ID).Scan(&state, &output, &saved); e != nil || state == "SUCCEEDED" {
				t.Fatal("receipt converted an interrupted reply into success", state, e)
			}
			var evidence costEvidence
			if e := json.Unmarshal(saved, &evidence); e != nil || len(evidence.Receipts) != 1 || evidence.Receipts[0].Cost != 200 {
				t.Fatal("financial amount was lost", string(saved), e)
			}
			if deleted && (output != "" || len(evidence.Receipts[0].Receipt) != 0) {
				t.Fatal("deleted execution retained content")
			}
		})
	}
}
