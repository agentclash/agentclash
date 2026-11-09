package vibe

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"testing"
)

// Prepare once, then change persisted evidence/accounting before transactional
// admission. Service-only checks cannot close these races.
func TestIntegrationResultSubsetRechecksAdmission(t *testing.T) {
	ctx := context.Background()
	store := integrationStore(t)
	cfg := freeConfig()
	cfg.LocalTesting = true // Deterministic fixture admits a second check without consuming a guest trial.
	store = NewStore(store.DB, cfg)
	v, err := store.CreateAgent(ctx, "anon:"+uuid.NewString(), nil, uuid.New(), "test", cfg.DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	cleanupSession(t, store, v.ID)
	svc := &Service{Store: store, Config: cfg, Gate: testGate(t)}
	client := uuid.New()
	if err = svc.AddEvidence(ctx, v.Actor, v.ID, EvidenceInput{ClientID: &client, Revision: v.Revision, Content: `{"conversations":[{"messages":[{"role":"user","content":"First"},{"role":"assistant","content":"First reply"}]},{"messages":[{"role":"user","content":"Second"},{"role":"assistant","content":"Second reply"}]}]}`, Expectations: []Expectation{{ID: "assessed", Statement: "Remember known information."}, {ID: "missing", Statement: "Answer the question."}}}); err != nil {
		t.Fatal(err)
	}
	v, _ = store.GetSession(ctx, v.Actor, v.ID)
	e := v.Document.EvidenceSets[0]
	a := v.Document.Artifacts[0]
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "check", ArtifactID: &a.ID, ApproveArtifact: true, Models: cfg.DefaultModels()}
	initialQuote, err := svc.QuoteRun(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	sub.RunQuoteID = &initialQuote.ID
	source, err := svc.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Stop(ctx, v.Actor, source.ID); err != nil {
		t.Fatal(err)
	}
	v, _ = store.GetSession(ctx, v.Actor, v.ID)
	sub.ClientID = uuid.New()
	sub.Revision = v.Revision
	sub.RunQuoteID = nil
	svc.Gate = testGate(t)
	sub.CaseKeys = []string{e.Conversations[0].Key}
	sub.RetrySourceResultRef = &ResultRef{Kind: ResultVibeOperation, ID: source.ID}
	mixed, err := store.GetCase(ctx, v.Actor, source.ID, sub.CaseKeys[0])
	if err != nil {
		t.Fatal(err)
	}
	mixed.Error = nil
	mixed.Verdict = Fail
	mixed.ExpectedChecks = 2
	mixed.Checks = []CheckResult{{Key: "assessed", Verdict: Fail}, {Key: "missing", Verdict: Unknown, Error: &Fault{Code: "judge_unavailable", Message: "Missing assessment"}}}
	if Aggregate([]CaseResult{mixed}).IncompleteCases != 1 {
		t.Fatal("fixture must remain incomplete")
	}
	if err = store.PutResult(ctx, source.ID, mixed); err != nil {
		t.Fatal(err)
	}
	quote, err := svc.QuoteRun(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	other := sub
	other.CaseKeys = []string{e.Conversations[1].Key}
	otherQuote, err := svc.QuoteRun(ctx, v.Actor, v.ID, other)
	if err != nil {
		t.Fatal(err)
	}
	if quote.Fingerprint == otherQuote.Fingerprint || quote.Cases != otherQuote.Cases {
		t.Fatal("same-size subsets lost their identity")
	}
	for _, keys := range [][]string{{}, {sub.CaseKeys[0], sub.CaseKeys[0]}, {"foreign-case"}} {
		invalid := sub
		invalid.CaseKeys = keys
		_, err = svc.QuoteRun(ctx, v.Actor, v.ID, invalid)
		requireFault(t, err, "invalid_subset")
	}
	foreign := sub
	foreign.RetrySourceResultRef = &ResultRef{Kind: ResultVibeOperation, ID: uuid.New()}
	_, err = svc.QuoteRun(ctx, v.Actor, v.ID, foreign)
	requireFault(t, err, "invalid_subset")
	sub.RunQuoteID = &quote.ID
	sub.estimateOnly = true
	estimate, err := svc.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	var plan Plan
	if err = json.Unmarshal(estimate.Input, &plan); err != nil {
		t.Fatal(err)
	}
	sub.estimateOnly = false
	plan.Submission = sub
	c, err := store.GetCase(ctx, v.Actor, source.ID, sub.CaseKeys[0])
	if err != nil {
		t.Fatal(err)
	}
	c.Output = "late evidence"
	if err = store.PutResult(ctx, source.ID, c); err != nil {
		t.Fatal(err)
	}
	_, err = store.Submit(ctx, v.Actor, v.ID, sub, plan, cfg)
	requireFault(t, err, "quote_expired")
	if _, err = store.DB.Exec(ctx, "UPDATE vibe_operations SET billing='RECONCILING' WHERE id=$1", source.ID); err != nil {
		t.Fatal(err)
	}
	_, err = store.Submit(ctx, v.Actor, v.ID, sub, plan, cfg)
	requireFault(t, err, "retry_uncertain")
	var n int
	if err = store.DB.QueryRow(ctx, "SELECT count(*) FROM vibe_operations WHERE session_id=$1", v.ID).Scan(&n); err != nil || n != 1 {
		t.Fatal("stale recovery admitted work", n, err)
	}
	// The same mixed failed/unassessed case remains admissible once reconciled.
	svc.Gate = testGate(t)
	if _, err = store.DB.Exec(ctx, "UPDATE vibe_operations SET billing='RELEASED' WHERE id=$1", source.ID); err != nil {
		t.Fatal(err)
	}
	quote, err = svc.QuoteRun(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	sub.RunQuoteID = &quote.ID
	child, err := svc.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Stop(ctx, v.Actor, child.ID); err != nil {
		t.Fatal(err)
	}
	mixed.Checks[1].Verdict = Pass
	mixed.Checks[1].Error = nil
	if err = store.PutResult(ctx, source.ID, mixed); err != nil {
		t.Fatal(err)
	}
	v, _ = store.GetSession(ctx, v.Actor, v.ID)
	sub.Revision = v.Revision
	sub.ClientID = uuid.New()
	_, err = svc.QuoteRun(ctx, v.Actor, v.ID, sub)
	requireFault(t, err, "case_resolved")
}
