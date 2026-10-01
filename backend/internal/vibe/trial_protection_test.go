package vibe

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// Chat and playground limits must leave the advertised first check and retest
// reachable. Exercise real journal/reservation accounting without a provider.
func TestIntegrationTrialPreservesCheckAndRetestAfterExploration(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	cfg := testConfig()
	ctx := context.Background()
	submit := func(kind string, calls int) (Operation, error) {
		current, err := s.GetSession(ctx, v.Actor, v.ID)
		if err != nil {
			t.Fatal(err)
		}
		sub := Submission{ClientID: uuid.New(), Revision: current.Revision, Kind: kind, Models: DefaultModels()}
		return s.Submit(ctx, v.Actor, v.ID, sub, Plan{Submission: sub, Anonymous: true, Calls: calls, MaxCost: int64(calls)}, cfg)
	}
	run := func(kind string, calls int) {
		t.Helper()
		o, err := submit(kind, calls)
		if err != nil {
			t.Fatalf("%s admission: %v", kind, err)
		}
		if _, _, err = s.Start(ctx, o.ID); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < calls; i++ {
			role, model := Target, o.Models.Target
			if kind == "message" {
				role, model = Assistant, o.Models.Assistant
			} else if kind != "playground" && i%2 == 1 {
				role, model = Evaluator, o.Models.Evaluator
			}
			a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: fmt.Sprintf("step:%d", i), Role: role, Model: model, Policy: raw(map[string]any{}), RequestHash: "fixture", InputBound: 1, MaxOutput: 1, MaxCost: 1}
			if err = s.BeginAttempt(ctx, a); err != nil {
				t.Fatal(err)
			}
			cost := int64(1)
			if err = s.EndAttempt(ctx, a, "fixture output", raw(map[string]any{}), &cost, nil); err != nil {
				t.Fatal(err)
			}
		}
		if err = s.Finish(ctx, o.ID, nil); err != nil {
			t.Fatal(err)
		}
	}
	expectLimited := func(kind string) {
		t.Helper()
		_, err := submit(kind, 1)
		var f *Fault
		if !errors.As(err, &f) || f.Code != "trial_limit" {
			t.Fatalf("expected %s limit, got %v", kind, err)
		}
	}
	for i := 0; i < TrialMessages; i++ {
		run("message", 1)
	}
	expectLimited("message")
	for i := TrialMessages; i < TrialExploreCalls; i++ {
		run("playground", 1)
	}
	expectLimited("playground")
	run("check", 6)
	run("retest", 6)
	expectLimited("playground")
	var calls int
	if err := s.DB.QueryRow(ctx, "SELECT sum(model_calls) FROM vibe_operations WHERE session_id=$1", v.ID).Scan(&calls); err != nil || calls != TrialCalls {
		t.Fatalf("expected all 40 calls including protected checks: %d, %v", calls, err)
	}
}
