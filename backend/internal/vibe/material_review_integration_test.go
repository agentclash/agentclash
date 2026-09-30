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
	s, v := materialService(t)
	ctx := context.Background()
	ref := addText(t, s, v, "The project codename is KESTREL.")
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
	executeBuildFixture(t, s, o, func(req provider.Request) any {
		reviews++
		if !strings.Contains(string(raw(req.Messages)), "KESTREL") {
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
}
