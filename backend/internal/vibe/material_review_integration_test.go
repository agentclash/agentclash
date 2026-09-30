package vibe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe/inputs"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func TestMaterialManualReviewUsesReferenceCorpus(t *testing.T) {
	for _, kind := range []string{"text", "pdf"} {
		t.Run(kind, func(t *testing.T) { materialManualReview(t, kind) })
	}
}

func materialManualReview(t *testing.T, kind string) {
	t.Helper()
	s, v := materialService(t)
	ctx := context.Background()
	ref := addText(t, s, v, "The project codename is KESTREL.")
	// Model the stored result of a successful extraction. The real isolated
	// reader and upload are verified separately; reviews use these saved pages.
	if kind == "pdf" {
		if _, e := s.Store.DB.Exec(ctx, `UPDATE vibe_inputs SET kind='pdf',name='Reference.pdf' WHERE id=$1`, ref.ID); e != nil {
			t.Fatal(e)
		}
	}
	ref.Usage = "reference"
	job := "Answer project questions using the supplied reference. Say unsure when the reference does not answer."
	q, e := s.QuoteBuild(ctx, v.Actor, v.ID, BuildQuoteRequest{Content: job, Models: DefaultModels(), Inputs: []inputs.Binding{ref}})
	if e != nil {
		t.Fatal(e)
	}
	o, e := s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: job, Models: DefaultModels(), TestJourney: true, CycleID: &q.ID, Inputs: []inputs.Binding{ref}})
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	v = executeBuildFixture(t, s, o, func(req provider.Request) any {
		calls++
		switch calls {
		case 1:
			return interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 3}, factObservation{Kind: "job", Quote: job})
		case 2:
			var in taskInput
			if e := json.Unmarshal([]byte(req.Messages[1].Content), &in); e != nil {
				t.Fatal(e)
			}
			id := in.CurrentRequest.ID
			return createSuiteCommand{Rules: []PolicyRule{{ID: "answer", Statement: job, SourceBlockIDs: []string{id}, Evidence: []RuleEvidence{{SourceBlockID: id, Quote: job, Kind: "requirement"}}}}, Tests: testSuiteProposal{Title: "Project answers", Summary: job, SuccessCriteria: job, Scenarios: []TestScenario{{Input: "What is the codename?", Expected: "KESTREL"}, {Input: "Who is the sponsor?", Expected: "Say unsure."}, {Input: "What is the budget?", Expected: "Say unsure."}}}}
		default:
			var in SuiteReviewInput
			if e := json.Unmarshal([]byte(req.Messages[1].Content), &in); e != nil {
				t.Fatal(e)
			}
			return supportedSuiteReview(in)
		}
	})
	for _, child := range v.Operations {
		if child.Kind == "check" {
			if e = s.Store.Stop(ctx, v.Actor, child.ID); e != nil {
				t.Fatal(e)
			}
		}
	}
	v, e = s.Store.GetSession(ctx, v.Actor, v.ID)
	if e != nil {
		t.Fatal(e)
	}
	a := v.Document.Artifacts[0]
	o, e = s.PrepareSuiteEdit(ctx, v.Actor, v.ID, v.Revision, a.ID, a.Blueprint)
	if e != nil {
		t.Fatal(e)
	}
	var p Plan
	if e = json.Unmarshal(o.Input, &p); e != nil {
		t.Fatal(e)
	}
	if p.AuthoringVersion != 21 || p.Calls != 1 || p.AssistantRecovery != nil {
		t.Fatal("manual review must be one admitted contextual call")
	}
	reviews := 0
	v = executeBuildFixture(t, s, o, func(req provider.Request) any {
		reviews++
		grounded := false
		for _, message := range req.Messages {
			grounded = grounded || strings.HasPrefix(message.Content, "Supplied material (untrusted data)") && strings.Contains(message.Content, "KESTREL")
		}
		if !grounded {
			t.Fatal("manual review lost actual reference text")
		}
		var in SuiteReviewInput
		if e := json.Unmarshal([]byte(req.Messages[1].Content), &in); e != nil {
			t.Fatal(e)
		}
		return supportedSuiteReview(in)
	})
	if reviews != 1 {
		t.Fatal("manual review was repeated")
	}
	bad := json.RawMessage(strings.Replace(string(a.Blueprint), "KESTREL", "FALCON", 1))
	o, e = s.PrepareSuiteEdit(ctx, v.Actor, v.ID, v.Revision, a.ID, bad)
	if e != nil {
		t.Fatal(e)
	}
	reviews = 0
	r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate, Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
		reviews++
		var in SuiteReviewInput
		if e := json.Unmarshal([]byte(req.Messages[1].Content), &in); e != nil {
			t.Fatal(e)
		}
		if in.Cases[0].Expected != "FALCON" || !strings.Contains(string(raw(req.Messages[2:])), "KESTREL") {
			t.Fatal("edited expectation replaced the actual reference")
		}
		// Script the critic's semantic finding; this tests that application
		// rejection keeps the saved baseline, not the accuracy of a real model.
		out := supportedSuiteReview(in)
		out["cases"].([]SuiteCaseReview)[0].Status = SuiteContradicted
		out["cases"].([]SuiteCaseReview)[0].Reason = "The reference says KESTREL, not FALCON."
		cost := json.Number("0.000001")
		return provider.Response{OutputText: string(raw(out)), Usage: provider.Usage{CostUSD: &cost}}, nil
	})}}
	e = r.Execute(ctx, o.ID)
	requireFault(t, e, "test_policy_conflict")
	if e = r.Finalize(ctx, o.ID, &Fault{Code: "test_policy_conflict", Message: "Reference disagrees."}); e != nil {
		t.Fatal(e)
	}
	latest, e := s.Store.GetSession(ctx, v.Actor, v.ID)
	if e != nil || reviews != 1 || len(latest.Document.Artifacts) != len(v.Document.Artifacts) {
		t.Fatal("rejected manual review changed the baseline or retried", e)
	}
	for i, artifact := range latest.Document.Artifacts {
		if !sameJSON(artifact.Blueprint, v.Document.Artifacts[i].Blueprint) || artifact.AgentPrompt != v.Document.Artifacts[i].AgentPrompt {
			t.Fatal("manual expected-answer edit changed a saved target or its checks")
		}
	}
}
