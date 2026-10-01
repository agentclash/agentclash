package vibe

import (
	"context"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

func TestIntegrationExpansionAuthorizationBindsBatchAndPreventsDuplicates(t *testing.T) {
	s, v := buildService(t)
	s.Config.LocalTesting = true
	s.Config.LocalBudget = 10 * NanoUSD
	s.Config.Campaign = "v1-expansion-" + uuid.NewString()
	s.Store.localTesting = true
	ctx := context.Background()
	job := "Sort spam from customer emails."
	o, _ := startBuild(t, s, v, job)
	v = executeBuildFixture(t, s, o, func(provider.Request) any {
		return interpretationFixture(buildAskAction{askAction: askAction{Kind: "ask", Text: "Which messages are spam?", Purpose: "clarify_rule", Options: []string{}}, MissingFactType: "correctness_rule", WhyNeeded: "Spam criteria"}, factObservation{Kind: "job", Quote: job})
	})
	// Stop the queued initial check; already prepared, validated sample stays usable.
	if err := s.Store.Stop(ctx, v.Actor, v.Operations[len(v.Operations)-1].ID); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Store.GetSession(ctx, v.Actor, v.ID)
	a := v.Document.Artifacts[0]
	request := BuildQuoteRequest{Content: "Add two tougher sample situations.", Models: DefaultModels(), ArtifactID: &a.ID, AdditionalExamples: 2}
	q, err := s.QuoteBuild(ctx, v.Actor, v.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	if q.Cases != 5 || q.BaselineHash == "" || q.MaxCalls < 15 {
		t.Fatal("quote omitted the complete batch")
	}
	sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: request.Content, Models: request.Models, ArtifactID: &a.ID, AdditionalExamples: 1, CycleID: &q.ID}
	_, err = s.Prepare(ctx, v.Actor, v.ID, sub)
	requireFault(t, err, "quote_expired")
	missingCycle := sub
	missingCycle.CycleID = nil
	_, err = s.Prepare(ctx, v.Actor, v.ID, missingCycle)
	requireFault(t, err, "invalid_request")
	sub.AdditionalExamples = 2
	admitted, err := s.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Prepare(ctx, v.Actor, v.ID, sub)
	if err != nil || again.ID != admitted.ID {
		t.Fatal("duplicated expansion work", err)
	}
	if err = s.Store.Stop(ctx, v.Actor, admitted.ID); err != nil {
		t.Fatal(err)
	}
	ResumeBuilds(ctx, s)
	after, _ := s.Store.GetSession(ctx, v.Actor, v.ID)
	if len(after.Operations) != 3 || !sameJSON(after.Document.Artifacts[0].Blueprint, a.Blueprint) {
		t.Fatal("stopped expansion started a run or changed baseline")
	}
}
