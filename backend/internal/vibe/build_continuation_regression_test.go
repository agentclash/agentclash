package vibe

import (
	"context"
	"encoding/json"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"testing"
	"time"
)

func claimTestProject(t *testing.T, s *Store, v Session) string {
	t.Helper()
	ctx := context.Background()
	user := uuid.New()
	actor := "user:" + user.String()
	if _, e := s.DB.Exec(ctx, `INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)`, user, user.String(), user.String()+"@example.invalid"); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.DB.Exec(ctx, `DELETE FROM users WHERE id=$1`, user) })
	if e := s.Claim(ctx, v.Actor, actor, v.ID); e != nil {
		t.Fatal(e)
	}
	_, e := s.GetSession(ctx, v.Actor, v.ID)
	requireFault(t, e, "not_found")
	return actor
}

func TestDeletedBuildFinalizationPreservesUncertainHold(t *testing.T) {
	svc, v := buildService(t)
	s := svc.Store
	ctx := context.Background()
	o, _ := startBuild(t, svc, v, "Summarize meeting notes.")
	if _, _, e := s.Start(ctx, o.ID); e != nil {
		t.Fatal(e)
	}
	a := Attempt{ID: uuid.New(), OperationID: o.ID, Step: "route", Role: Assistant, Model: o.Models.Assistant, InputBound: 100, MaxOutput: 100, MaxCost: 300, Policy: raw(map[string]any{"profile": func() ModelProfile { p, _ := svc.Config.Profile(o.Models.Assistant); return p }()})}
	if e := s.BeginAttempt(ctx, a); e != nil {
		t.Fatal(e)
	}
	v, e := s.GetSession(ctx, v.Actor, v.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DeleteProject(ctx, v.Actor, v.ID, v.Revision); e != nil {
		t.Fatal(e)
	}
	r := Runner{Service: svc}
	for i := 0; i < 2; i++ {
		if e = r.Finalize(ctx, o.ID, nil); e != nil {
			t.Fatal(e)
		}
	}
	o, e = s.Operation(ctx, o.ID)
	if e != nil || o.Billing != Reconciling {
		t.Fatal("uncertain hold released", o.Billing, e)
	}
}

// Keep a real persisted artifact/quote, and pause only the admission boundary.
func TestBuildTemporaryAdmissionResumesOnceAfterClaim(t *testing.T) {
	s, v := materialService(t)
	ctx := context.Background()
	job := "Summarize notes without inventing details."
	material := "Mira ships Friday."
	o, _ := startBuild(t, s, v, job+"\nNotes:\n"+material)
	gate := s.Gate
	actor := ""
	calls := 0
	v = executeBuildFixture(t, s, o, func(req provider.Request) any {
		calls++
		switch calls {
		case 1:
			actor = claimTestProject(t, s.Store, v)
			value := interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 3}, factObservation{Kind: "job", Quote: job})
			value.MaterialQuote = material
			return value
		case 2:
			var in taskInput
			json.Unmarshal([]byte(req.Messages[1].Content), &in)
			id := in.CurrentRequest.ID
			return createSuiteCommand{Rules: []PolicyRule{{ID: "summary", Statement: job, SourceBlockIDs: []string{id}, Evidence: []RuleEvidence{{SourceBlockID: id, Quote: job, Kind: "requirement"}}}}, Tests: testSuiteProposal{Title: "Notes", Summary: job, SuccessCriteria: job, Scenarios: []TestScenario{{Input: "Ana ships Tuesday.", Expected: "Ana ships Tuesday."}, {Input: "Owner unknown.", Expected: "Do not invent an owner."}, {Input: "Deadline unknown.", Expected: "Do not invent a deadline."}}}}
		default:
			var in SuiteReviewInput
			json.Unmarshal([]byte(req.Messages[1].Content), &in)
			s.Gate = Gate{}
			return supportedSuiteReview(in)
		}
	})
	if v.Document.Build.Phase != "ready" || v.Document.Build.Error == nil || v.Document.Build.Error.RetryAvailableAt == nil {
		t.Fatal("temporary outage stranded Build", string(raw(v.Document.Build)))
	}
	if v.Document.Build.Error.RetryAvailableAt.Sub(timestamp()) < 4*time.Second {
		t.Fatal("missing backoff")
	}
	s.Gate = gate
	if e := s.Store.Edit(ctx, actor, v.ID, v.Revision, func(v *Session) error {
		past := timestamp().Add(-time.Second)
		v.Document.Build.Error.RetryAvailableAt = &past
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		ResumeBuilds(ctx, s)
	}
	v, e := s.Store.GetSession(ctx, actor, v.ID)
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, child := range v.Operations {
		if child.Kind == "playground" {
			count++
		}
	}
	if count != 1 || v.Document.Build.InlineInput == nil {
		t.Fatalf("expected one material trial; got %d, progress=%s", count, raw(v.Document.Build))
	}
	original, e := s.Store.Operation(ctx, o.ID)
	if e != nil || original.Actor == actor {
		t.Fatal("historical attribution changed", e)
	}
}
