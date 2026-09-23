package vibe

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func TestVibeRetryTimingSurvivesFaultSerialization(t *testing.T) {
	for _, delay := range []time.Duration{0, 7 * time.Second, 90 * time.Second} {
		before := timestamp()
		f := issueFrom(provider.Failure{Code: provider.FailureCodeRateLimit, RetryAfter: delay})
		if delay == 0 {
			delay = 30 * time.Second
		}
		var saved Fault
		if err := json.Unmarshal(raw(f), &saved); err != nil {
			t.Fatal(err)
		}
		if saved.RetryAvailableAt == nil || saved.RetryAvailableAt.Before(before.Add(delay)) || saved.RetryAvailableAt.After(timestamp().Add(delay)) {
			t.Fatalf("lost cooldown: %+v", saved)
		}
		requireFault(t, validateRetryTiming(Operation{Error: &saved}), "retry_cooldown")
	}
}

func TestIntegrationVibeRetryCooldownEnforcedAtBothBoundaries(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	v, original, plan := failedRetrySource(t, s)
	available := timestamp().Add(time.Minute)
	issue := &Fault{Code: "provider_rate_limit", Message: "Busy", RetryAvailableAt: &available}
	if _, err := s.DB.Exec(ctx, "UPDATE vibe_operations SET error=$2 WHERE id=$1", original.ID, raw(issue)); err != nil {
		t.Fatal(err)
	}
	v, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Operations[0].Retryable || v.Operations[0].Error.RetryAvailableAt == nil {
		t.Fatal("countdown unavailable after snapshot")
	}
	service := &Service{Store: s, Config: testConfig(), Gate: testGate(t)}
	request := RetryRequest{ClientID: uuid.New(), Revision: v.Revision}
	_, err = service.Retry(ctx, v.Actor, v.ID, original.ID, request)
	requireFault(t, err, "retry_cooldown")
	// Bypassing the service (or racing its check) must still fail under the DB lock.
	sub := retrySubmission(plan.Submission, original.ID, request)
	retry := retryContext(original, plan)
	plan.Submission, plan.Retry = sub, &retry
	_, err = s.Submit(ctx, v.Actor, v.ID, sub, plan, testConfig())
	requireFault(t, err, "retry_cooldown")
	var count int
	if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_operations WHERE session_id=$1", v.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("cooldown admitted work", count, err)
	}
	available = timestamp().Add(-time.Second)
	if _, err = s.DB.Exec(ctx, "UPDATE vibe_operations SET error=$2 WHERE id=$1", original.ID, raw(issue)); err != nil {
		t.Fatal(err)
	}
	admitted, err := service.Retry(ctx, v.Actor, v.ID, original.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	// A lost acknowledgement remains recoverable even if a later cooldown is recorded.
	available = timestamp().Add(time.Hour)
	if _, err = s.DB.Exec(ctx, "UPDATE vibe_operations SET error=$2 WHERE id=$1", original.ID, raw(issue)); err != nil {
		t.Fatal(err)
	}
	again, err := service.Retry(ctx, v.Actor, v.ID, original.ID, request)
	if err != nil || admitted.ID != again.ID {
		t.Fatal("lost receipt was blocked by cooldown", err)
	}
}

func TestIntegrationVibeDurableProgressAndDiagnostics(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	v, o, _ := outcomeOperation(t, s, 11)
	a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "route", Role: Assistant, Model: o.Models.Assistant, Policy: json.RawMessage(`{}`), InputBound: 10, MaxOutput: 10, MaxCost: 100}
	before, _ := s.GetSession(ctx, v.Actor, v.ID)
	if err := s.BeginAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.EventCursor <= before.EventCursor || current.Operations[0].Progress.Phase != "understanding" {
		t.Fatal("stage transition not streamable", current.Operations)
	}
	start := timestamp().Add(-10 * time.Second)
	if _, err = s.DB.Exec(ctx, "UPDATE vibe_attempts SET created_at=$2 WHERE id=$1", a.ID, start); err != nil {
		t.Fatal(err)
	}
	issue := &Fault{Code: "provider_timeout", Message: "Uncertain outcome"}
	if err = s.EndAttempt(ctx, a, "saved text", json.RawMessage(`{}`), nil, issue); err != nil {
		t.Fatal(err)
	}
	// Placeholder, failed call and completed UNKNOWN grade must remain distinct.
	for _, c := range []CaseResult{
		{CaseKey: "pending", Version: "v", Verdict: Unknown, ExpectedChecks: 1, Error: &Fault{Code: "not_evaluated"}},
		{CaseKey: "failed", Version: "v", Verdict: Unknown, ExpectedChecks: 1, Error: issue},
		{CaseKey: "inconclusive", Version: "v", Verdict: Unknown, ExpectedChecks: 1, Checks: []CheckResult{{Key: "grade", Verdict: Unknown}}},
		{CaseKey: "complete", Version: "v", Verdict: Pass, ExpectedChecks: 1, Checks: []CheckResult{{Key: "grade", Verdict: Pass}}},
	} {
		if err = s.PutResult(ctx, o.ID, c); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Finish(ctx, o.ID, issue); err != nil {
		t.Fatal(err)
	}
	current, err = s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := current.Operations[0]
	if got.Progress.CompletedCases != 2 || got.Progress.TotalCases != 4 || Aggregate(got.Results).Evaluated != 1 {
		t.Fatalf("progress fabricated grades: %+v", got)
	}
	d := got.Diagnostics
	if d.StageMillis["understanding"] < 9000 || d.UnresolvedBillingAgeMillis == nil || *d.UnresolvedBillingAgeMillis < 9000 || current.Diagnostics.FirstUsefulResultMillis == nil {
		t.Fatalf("diagnostics missing: %+v %+v", d, current.Diagnostics)
	}
	if got.Retryable || got.Billing != Reconciling {
		t.Fatal("uncertain timeout allowed replay or released reservation")
	}
	// Reconciliation must preserve the original stage end time.
	if err = s.ReconcileCost(ctx, a.ID, 1, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Operations[0].Diagnostics.StageMillis["understanding"] != d.StageMillis["understanding"] || after.Operations[0].Diagnostics.UnresolvedBillingSince != nil {
		t.Fatal("reconciliation rewrote runtime or retained unresolved cost")
	}
	var milestones int
	if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_events WHERE session_id=$1 AND kind='result.useful'", v.ID).Scan(&milestones); err != nil || milestones != 1 {
		t.Fatal("useful result counted more than once", err)
	}
}
