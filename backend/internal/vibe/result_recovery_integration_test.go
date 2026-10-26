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
	v := anonSession(t, store)
	cfg := freeConfig()
	svc := &Service{Store: store, Config: cfg, Gate: testGate(t)}
	if err := svc.AddEvidence(ctx, v.Actor, v.ID, EvidenceInput{Revision: v.Revision, Content: `{"conversations":[{"messages":[{"role":"user","content":"First"},{"role":"assistant","content":"First reply"}]},{"messages":[{"role":"user","content":"Second"},{"role":"assistant","content":"Second reply"}]}]}`}); err != nil {
		t.Fatal(err)
	}
	v, _ = store.GetSession(ctx, v.Actor, v.ID)
	e := v.Document.EvidenceSets[0]
	a := Artifact{ID: uuid.New(), Kind: "conversation_evaluation", ConversationEvaluation: &ConversationEvaluation{EvidenceSetID: e.ID, Expectations: []Expectation{{ID: "memory", Statement: "Remember known information."}}}}
	if err := store.Edit(ctx, v.Actor, v.ID, v.Revision, func(v *Session) error { v.Document.Artifacts = append(v.Document.Artifacts, a); return nil }); err != nil {
		t.Fatal(err)
	}
	v, _ = store.GetSession(ctx, v.Actor, v.ID)
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "check", ArtifactID: &a.ID, ApproveArtifact: true, Models: cfg.DefaultModels()}
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
	sub.CaseKeys = []string{e.Conversations[0].Key}
	sub.RetrySourceResultRef = &ResultRef{Kind: ResultVibeOperation, ID: source.ID}
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
}
