package vibe

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// sweptAttempt is the row that the sweep and the cost writers leave. The
// evidence keys are part of the accounting record, so the test reads them by
// their JSON names.
type sweptAttempt struct {
	State     string
	Cost      *int64
	Completed *time.Time
	Error     Fault
	Output    string
	Evidence  struct {
		Abandoned    bool   `json:"abandoned"`
		AssumedCost  *int64 `json:"assumed_cost"`
		ResponseCost *int64 `json:"response_cost"`
	}
}

func readAttempt(t *testing.T, s *Store, id uuid.UUID) sweptAttempt {
	t.Helper()
	var a sweptAttempt
	var issue, evidence []byte
	if err := s.DB.QueryRow(context.Background(), `SELECT state,actual_cost,completed_at,error,output,reconciliation_evidence FROM vibe_attempts WHERE id=$1`, id).Scan(&a.State, &a.Cost, &a.Completed, &issue, &a.Output, &evidence); err != nil {
		t.Fatal(err)
	}
	if len(issue) > 0 {
		if err := json.Unmarshal(issue, &a.Error); err != nil {
			t.Fatal(err)
		}
	}
	if len(evidence) > 0 {
		if err := json.Unmarshal(evidence, &a.Evidence); err != nil {
			t.Fatal(err)
		}
	}
	return a
}

func backdateAttempt(t *testing.T, s *Store, id uuid.UUID, age time.Duration) {
	t.Helper()
	if _, err := s.DB.Exec(context.Background(), `UPDATE vibe_attempts SET created_at=now()-make_interval(secs => $2) WHERE id=$1`, id, age.Seconds()); err != nil {
		t.Fatal(err)
	}
}

func sweepOnce(t *testing.T, s *Store) {
	t.Helper()
	if _, err := s.sweepAttempts(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// operationHeld is the amount that the funding accounts of one operation
// still hold. Each test config has its own campaign accounts.
func operationHeld(t *testing.T, s *Store, id uuid.UUID) int64 {
	t.Helper()
	var held int64
	if err := s.DB.QueryRow(context.Background(), `SELECT COALESCE(sum(held),0) FROM vibe_accounts WHERE id IN (SELECT account_id FROM vibe_reservations WHERE operation_id=$1)`, id).Scan(&held); err != nil {
		t.Fatal(err)
	}
	return held
}

func operationEvents(t *testing.T, s *Store, id uuid.UUID, kind string) int {
	t.Helper()
	var n int
	if err := s.DB.QueryRow(context.Background(), `SELECT count(*) FROM vibe_events WHERE operation_id=$1 AND kind=$2`, id, kind).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// crashedDispatch commits a paid attempt and stops before EndAttempt, as a
// worker crash does. The workflow then finalizes the operation with a fault.
func crashedDispatch(t *testing.T, s *Store, generation bool, age time.Duration) (Session, Operation, Attempt) {
	t.Helper()
	ctx := context.Background()
	v, o, _ := outcomeOperation(t, s, 11)
	a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "route", Role: Assistant, Model: o.Models.Assistant, Policy: json.RawMessage(`{}`), RequestHash: "crashed", InputBound: 10, MaxOutput: 10, MaxCost: 4_000_000}
	if err := s.BeginAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if generation {
		if err := s.Generation(ctx, a.ID, "gen-"+a.ID.String()); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Finish(ctx, o.ID, &Fault{Code: "worker_interrupted", Message: "Execution was interrupted. Saved evidence remains available; uncertain provider calls will not be repeated."}); err != nil {
		t.Fatal(err)
	}
	backdateAttempt(t, s, a.ID, age)
	current, err := s.Operation(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.State != Failed || current.Billing != Reconciling || operationHeld(t, s, o.ID) == 0 {
		t.Fatalf("the crash must first keep the hold: state=%s billing=%s", current.State, current.Billing)
	}
	return v, current, a
}

func requireRetryAllowed(t *testing.T, s *Store, v Session, o Operation) {
	t.Helper()
	ctx := context.Background()
	current, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range current.Operations {
		if summary.ID == o.ID && !summary.Retryable {
			t.Fatalf("the operation is not retryable: billing=%s", summary.Billing)
		}
	}
	service := &Service{Store: s, Config: testConfig(), Gate: testGate(t)}
	retried, err := service.Retry(ctx, v.Actor, v.ID, o.ID, RetryRequest{ClientID: uuid.New(), Revision: current.Revision})
	if err != nil {
		t.Fatalf("retry refused: %v", err)
	}
	if retried.RetryOfOperationID == nil || *retried.RetryOfOperationID != o.ID {
		t.Fatalf("retry is not linked to its source: %+v", retried)
	}
}

func requireRetryRefused(t *testing.T, s *Store, v Session, o Operation) {
	t.Helper()
	ctx := context.Background()
	current, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{Store: s, Config: testConfig(), Gate: testGate(t)}
	_, err = service.Retry(ctx, v.Actor, v.ID, o.ID, RetryRequest{ClientID: uuid.New(), Revision: current.Revision})
	requireFault(t, err, "retry_uncertain")
}

func requireBilling(t *testing.T, s *Store, id uuid.UUID, billing BillingState, cost int64) {
	t.Helper()
	o, err := s.Operation(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if o.Billing != billing || o.ActualCost == nil || *o.ActualCost != cost {
		t.Fatalf("billing=%s actual=%v, want %s %d", o.Billing, o.ActualCost, billing, cost)
	}
	if held := operationHeld(t, s, id); held != 0 {
		t.Fatalf("the operation still holds %d", held)
	}
	var open int
	if err = s.DB.QueryRow(context.Background(), `SELECT count(*) FROM vibe_reservations WHERE operation_id=$1 AND (settled_amount IS NULL OR settled_amount<>$2)`, id, cost).Scan(&open); err != nil || open != 0 {
		t.Fatalf("reservations are not settled at %d: open=%d err=%v", cost, open, err)
	}
}

// With no generation ID, the provider never accepted the request. After the
// receipt window, the sweep charges nothing and releases the hold.
func TestMoneyCrashedDispatchEndsAtReceiptHorizon(t *testing.T) {
	s := integrationStore(t)
	v, o, a := crashedDispatch(t, s, false, 26*time.Hour)
	sweepOnce(t, s)
	got := readAttempt(t, s, a.ID)
	if got.State != "RECONCILED" || got.Cost == nil || *got.Cost != 0 || got.Completed == nil {
		t.Fatalf("crashed attempt did not end: state=%s cost=%v completed=%v", got.State, got.Cost, got.Completed)
	}
	if got.Error.Code != "attempt_abandoned" || !got.Evidence.Abandoned || got.Evidence.AssumedCost == nil || *got.Evidence.AssumedCost != 0 {
		t.Fatalf("evidence does not record the assumed cost: %+v %+v", got.Error, got.Evidence)
	}
	requireBilling(t, s, o.ID, Released, 0)
	if operationEvents(t, s, o.ID, "attempt.abandoned") != 1 || operationEvents(t, s, o.ID, "billing.reconciled") != 1 {
		t.Fatal("the sweep did not write one event for each move")
	}
	requireRetryAllowed(t, s, v, o)
}

// A generation ID proves that the provider accepted the work. With no receipt
// after the receipt window, the sweep charges the approved ceiling.
func TestMoneyCrashedDispatchChargesCeilingWithGenerationID(t *testing.T) {
	s := integrationStore(t)
	v, o, a := crashedDispatch(t, s, true, 26*time.Hour)
	sweepOnce(t, s)
	got := readAttempt(t, s, a.ID)
	if got.State != "RECONCILED" || got.Cost == nil || *got.Cost != a.MaxCost || got.Evidence.AssumedCost == nil || *got.Evidence.AssumedCost != a.MaxCost || !got.Evidence.Abandoned {
		t.Fatalf("accepted work was not charged at its ceiling: state=%s cost=%v evidence=%+v", got.State, got.Cost, got.Evidence)
	}
	requireBilling(t, s, o.ID, Settled, a.MaxCost)
	requireRetryAllowed(t, s, v, o)
}

// After 20 minutes, the sweep ends the attempt but keeps the hold. A late
// receipt then settles it, and the 25-hour rule never applies.
func TestMoneyCrashedDispatchSettlesFromLateReceipt(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	v, o, a := crashedDispatch(t, s, true, 21*time.Minute)
	sweepOnce(t, s)
	got := readAttempt(t, s, a.ID)
	if got.State != "UNCERTAIN" || got.Cost != nil || got.Completed == nil || got.Error.Code != "attempt_abandoned" || !got.Evidence.Abandoned || got.Evidence.AssumedCost != nil {
		t.Fatalf("abandoned attempt: state=%s cost=%v completed=%v error=%+v evidence=%+v", got.State, got.Cost, got.Completed, got.Error, got.Evidence)
	}
	current, err := s.Operation(ctx, o.ID)
	if err != nil || current.Billing != Reconciling || operationHeld(t, s, o.ID) == 0 {
		t.Fatalf("an abandoned attempt with no cost released its hold: %+v %v", current, err)
	}
	requireRetryRefused(t, s, v, o)
	session, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range session.Operations {
		for phase, millis := range summary.Diagnostics.StageMillis {
			if millis >= (20 * time.Minute).Milliseconds() {
				t.Fatalf("the sweep wait counted as stage time: %s=%d", phase, millis)
			}
		}
	}
	cost := int64(1_500_000)
	if err = s.ReconcileCost(ctx, a.ID, cost, json.RawMessage(`{"receipt":"late"}`)); err != nil {
		t.Fatal(err)
	}
	got = readAttempt(t, s, a.ID)
	if got.State != "RECONCILED" || got.Cost == nil || *got.Cost != cost || got.Evidence.AssumedCost != nil {
		t.Fatalf("late receipt did not reconcile: state=%s cost=%v evidence=%+v", got.State, got.Cost, got.Evidence)
	}
	requireBilling(t, s, o.ID, Settled, cost)
	sweepOnce(t, s)
	if again := readAttempt(t, s, a.ID); again.State != "RECONCILED" || *again.Cost != cost {
		t.Fatal("a second sweep changed a final attempt")
	}
	requireRetryAllowed(t, s, v, o)
}

// A live call is younger than the abandon age. The sweep does not touch it, nor
// an attempt that EndAttempt finished, and concurrent sweeps move a row once.
func TestMoneyAttemptSweepKeepsLiveDispatch(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	v := anonSession(t, s)
	o, _ := submitPlan(t, s, v, testConfig(), 100_000_000)
	if _, _, err := s.Start(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	live := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "target:1", Role: Target, Model: o.Models.Target, Policy: json.RawMessage(`{}`), RequestHash: "live", InputBound: 1000, MaxOutput: 100, MaxCost: 10_000_000}
	if err := s.BeginAttempt(ctx, live); err != nil {
		t.Fatal(err)
	}
	backdateAttempt(t, s, live.ID, 19*time.Minute)
	sweepOnce(t, s)
	if got := readAttempt(t, s, live.ID); got.State != "DISPATCHING" || got.Completed != nil || got.Evidence.Abandoned {
		t.Fatalf("the sweep ended a live call: %+v", got)
	}
	cost := int64(2_000_000)
	if err := s.EndAttempt(ctx, live, "reply", json.RawMessage(`{}`), &cost, nil); err != nil {
		t.Fatal(err)
	}
	backdateAttempt(t, s, live.ID, 26*time.Hour)
	sweepOnce(t, s)
	if got := readAttempt(t, s, live.ID); got.State != "SUCCEEDED" || got.Cost == nil || *got.Cost != cost || got.Evidence.Abandoned || got.Evidence.AssumedCost != nil {
		t.Fatalf("the sweep changed a finished attempt: %+v", got)
	}
	if operationEvents(t, s, o.ID, "attempt.abandoned") != 0 {
		t.Fatal("the sweep wrote an event for a live or finished attempt")
	}

	stale := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "target:2", Role: Target, Model: o.Models.Target, Policy: json.RawMessage(`{}`), RequestHash: "stale", InputBound: 1000, MaxOutput: 100, MaxCost: 10_000_000}
	if err := s.BeginAttempt(ctx, stale); err != nil {
		t.Fatal(err)
	}
	backdateAttempt(t, s, stale.ID, 21*time.Minute)
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.sweepAttempts(ctx)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := readAttempt(t, s, stale.ID); got.State != "UNCERTAIN" || got.Completed == nil || !got.Evidence.Abandoned {
		t.Fatalf("concurrent sweeps did not end the stale attempt: %+v", got)
	}
	if n := operationEvents(t, s, o.ID, "attempt.abandoned"); n != 1 {
		t.Fatalf("concurrent sweeps wrote %d events, want 1", n)
	}
}

// A response that arrives after the sweep is the real cost. EndAttempt records
// it as cost evidence and settles the operation.
func TestMoneyLateResponseAfterAbandonIsCostEvidence(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	v, o, a := crashedDispatch(t, s, true, 21*time.Minute)
	sweepOnce(t, s)
	abandoned := readAttempt(t, s, a.ID)
	if abandoned.State != "UNCERTAIN" || abandoned.Completed == nil {
		t.Fatalf("the sweep did not abandon the attempt: %+v", abandoned)
	}
	cost := int64(1_200_000)
	if err := s.EndAttempt(ctx, a, "late reply", json.RawMessage(`{"late":true}`), &cost, nil); err != nil {
		t.Fatal(err)
	}
	got := readAttempt(t, s, a.ID)
	if got.State != "RECONCILED" || got.Cost == nil || *got.Cost != cost || got.Evidence.ResponseCost == nil || *got.Evidence.ResponseCost != cost || !got.Evidence.Abandoned {
		t.Fatalf("the late cost was lost: state=%s cost=%v evidence=%+v", got.State, got.Cost, got.Evidence)
	}
	if got.Error.Code != "attempt_abandoned" || got.Output != "" || !got.Completed.Equal(*abandoned.Completed) {
		t.Fatalf("a late response rewrote the abandoned attempt: %+v", got)
	}
	requireBilling(t, s, o.ID, Settled, cost)
	requireRetryAllowed(t, s, v, o)
}
