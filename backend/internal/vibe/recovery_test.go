package vibe

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"testing"
)

func TestIntegrationCrashKeepsJournaledEvidenceWithoutReexecution(t *testing.T) {
	s := integrationStore(t)
	v := anonSession(t, s)
	ctx := context.Background()
	cfg := testConfig()
	sub := Submission{ClientID: uuid.New(), Kind: "check", Models: DefaultModels()}
	artifact := Artifact{ID: uuid.New()}
	o, err := s.Submit(ctx, v.Actor, v.ID, sub, Plan{Submission: sub, Anonymous: true, Artifact: &artifact, ChecksPerCase: 1, Cases: []string{"one", "two"}, Calls: 4, MaxCost: 100000000}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Start(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "target:one", Role: Target, Model: o.Models.Target, Policy: json.RawMessage(`{}`), InputBound: 1000, MaxOutput: 100, MaxCost: 10000000}
	if err = s.BeginAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err = s.Generation(ctx, a.ID, "gen-crashed-worker"); err != nil {
		t.Fatal(err)
	}
	if err = s.AppendOutput(ctx, a.ID, "Provider text saved before the worker crashed."); err != nil {
		t.Fatal(err)
	}
	// Simulate provider success followed by worker death before cost/final result
	// persistence. The database-only finalizer must recover evidence, not retry.
	if err = s.Finish(ctx, o.ID, &Fault{Code: "worker_interrupted", Message: "Worker crashed."}); err != nil {
		t.Fatal(err)
	}
	v, err = s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Operations[0].Scorecard.Unknown != 2 || v.Operations[0].Billing != Reconciling {
		t.Fatal("crash was represented as normal completion")
	}
	c, err := s.GetCase(ctx, v.Actor, o.ID, "one")
	if err != nil || c.Output == "" || c.Verdict != Unknown {
		t.Fatal("journaled evidence was lost or judged without execution", err)
	}
	if err = s.BeginAttempt(ctx, a); err == nil {
		t.Fatal("paid call repeated after crash")
	}
	if err = s.ReconcileCost(ctx, a.ID, 1000000, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	if v.Operations[0].Scorecard.Unknown != 2 || v.Operations[0].Billing != Settled {
		t.Fatal("accounting reconciliation reran the evaluation")
	}
}
