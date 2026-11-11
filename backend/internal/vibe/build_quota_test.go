package vibe

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// partialOperation makes a real PARTIAL check or retest. The plan reserves
// calls, one paid attempt runs, and a provider fault ends the operation.
func partialOperation(t *testing.T, s *Store, v Session, kind string, calls int) Operation {
	t.Helper()
	ctx := context.Background()
	current, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	sub := Submission{ClientID: uuid.New(), Revision: current.Revision, Kind: kind, Models: DefaultModels()}
	p := Plan{Submission: sub, Anonymous: v.Anonymous, Calls: calls, MaxCost: NanoUSD / 10}
	o, err := s.Submit(ctx, v.Actor, v.ID, sub, p, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Start(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "target:1", Role: Target, Model: o.Models.Target, Policy: json.RawMessage(`{}`), InputBound: 1000, MaxOutput: 100, MaxCost: NanoUSD / 100}
	if err = s.BeginAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	cost := NanoUSD / 200
	if err = s.EndAttempt(ctx, a, "reply", json.RawMessage(`{}`), &cost, nil); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, o.ID, &Fault{Code: "provider_unavailable", Message: "The provider stopped."}); err != nil {
		t.Fatal(err)
	}
	o, err = s.Operation(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	var saved Plan
	if err = json.Unmarshal(o.Input, &saved); err != nil {
		t.Fatal(err)
	}
	if o.State != Partial || o.ModelCalls != 1 || saved.Calls != calls {
		t.Fatalf("fixture is not a PARTIAL operation with 1 model call and %d reserved calls: state=%s model_calls=%d calls=%d", calls, o.State, o.ModelCalls, saved.Calls)
	}
	return o
}

func TestMoneyPartialStepCountsModelCallsInBuildCycle(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	ctx := context.Background()
	partial := partialOperation(t, s, v, "check", 5)
	cycle := uuid.New()
	quote := BuildQuote{ID: cycle, MaxCalls: 6, ExpiresAt: timestamp().Add(time.Hour)}
	if _, err := s.DB.Exec(ctx, `INSERT INTO vibe_cycle_quotes(id,session_id,request_hash,specification,max_cost,expires_at,authorized_at) VALUES($1,$2,'build-quota-test',$3,$4,now()+interval '1 hour',now())`, cycle, v.ID, raw(quote), MaxOperationCost); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO vibe_cycle_steps(cycle_id,step,operation_id) VALUES($1,'check',$2)`, cycle, partial.ID); err != nil {
		t.Fatal(err)
	}
	// admit runs one admission in a transaction and then rolls it back.
	admit := func(calls int) error {
		tx, err := s.DB.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		sub := Submission{ClientID: uuid.New(), Kind: "message", Models: DefaultModels()}
		p := Plan{Submission: sub, Anonymous: true, Calls: calls, MaxCost: NanoUSD / 100, Cycle: &BuildCyclePlan{ID: cycle, Step: "answer"}}
		o := Operation{ID: uuid.New(), SessionID: v.ID, Actor: v.Actor, Kind: sub.Kind, Models: sub.Models, Input: raw(p), MaxCost: p.MaxCost, Deadline: timestamp().Add(time.Minute)}
		// Admission inserts the operation in CREATED before it admits the cycle step.
		if _, err = tx.Exec(ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,deadline) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, o.ID, v.ID, v.Actor, sub.ClientID, Hash(raw(sub)), sub.Kind, Created, Unreserved, raw(o.Models), o.Input, o.MaxCost, o.Deadline); err != nil {
			t.Fatal(err)
		}
		return admitBuildCycle(ctx, tx, v, sub, p, o)
	}
	// The PARTIAL check made 1 model call. 1+5 calls fit the 6-call cycle.
	if err := admit(5); err != nil {
		t.Fatalf("the cycle must count the PARTIAL check at its 1 model call, not at its 5 reserved calls: %v", err)
	}
	// 1+6 calls do not fit the 6-call cycle.
	requireFault(t, admit(6), "budget_limit")
}

func TestMoneyPartialOperationCountsModelCallsInGuestBuildQuota(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	ctx := context.Background()
	// A used check stops a new Build before the call count, so use a retest.
	partialOperation(t, s, v, "retest", 12)
	// With 6 cases, the exploration limit allows both quotes below.
	quote := BuildQuote{MaxCalls: TrialCalls - 1, Cases: 6}
	if err := checkInitialBuildQuota(ctx, s.DB, v.ID, uuid.Nil, quote); err != nil {
		t.Fatalf("the guest quota must count the PARTIAL retest at its 1 model call, not at its 12 reserved calls: %v", err)
	}
	quote.MaxCalls = TrialCalls
	requireFault(t, checkInitialBuildQuota(ctx, s.DB, v.ID, uuid.Nil, quote), "trial_limit")
}

func TestStateMachineTerminalStates(t *testing.T) {
	listed := map[string]bool{}
	for _, state := range terminalStates() {
		if listed[state] {
			t.Fatalf("terminalStates() has %s two times", state)
		}
		listed[state] = true
	}
	all := []Execution{Created, Validating, AwaitingInput, AwaitingApproval, Reserved, Queued, Running, Finalizing, Cancelling, Completed, Partial, Failed, Cancelled, Expired}
	known := 0
	for _, state := range all {
		if state.Terminal() != listed[string(state)] {
			t.Fatalf("%s: Terminal()=%v, but terminalStates() has it=%v", state, state.Terminal(), listed[string(state)])
		}
		if listed[string(state)] {
			known++
		}
	}
	if known != len(listed) {
		t.Fatalf("terminalStates() has a state that is not an Execution: %v", terminalStates())
	}
	if !listed[string(Partial)] {
		t.Fatal("terminalStates() must have PARTIAL")
	}
}
