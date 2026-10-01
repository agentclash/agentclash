package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
)

const cleaningCompanyRequest = "I run a cleaning company. Customers keep WhatsApping changes and special requests, and our team misses them. Can you turn those messages into a simple list of who needs what and when?"

// Reproduce the actual failed journey through persistence, source selection,
// assertion review, bounded patch, re-review and continuation. Provider outputs
// are scripted here; real model quality is verified separately in the browser.
func TestIntegrationBuildCleaningRequestRepairsAllInventedClauses(t *testing.T) {
	for _, tc := range []struct {
		name           string
		repairWorks    bool
		absentMaterial bool
	}{
		{"all invented clauses repaired", true, false},
		{"incomplete repair stays blocked", false, false},
		{"absent case material repaired before execution", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, v := materialService(t)
			ctx := context.Background()
			o, quote := startBuild(t, s, v, "hey, exhausting day lol")
			v = executeBuildFixture(t, s, o, func(provider.Request) any {
				return interpretationFixture(replyAction{Kind: "reply", Text: "Hope you get some rest. What would you like help with?"})
			})
			if len(v.Document.Artifacts) != 0 || v.Document.Build.Phase != "waiting" {
				t.Fatal("casual chat started a prototype")
			}
			sub := Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: cleaningCompanyRequest, Models: DefaultModels(), TestJourney: true, CycleID: &quote.ID}
			o, err := s.Prepare(ctx, v.Actor, v.ID, sub)
			if err != nil {
				t.Fatal(err)
			}
			var p Plan
			if err = json.Unmarshal(o.Input, &p); err != nil || p.AuthoringVersion != contextualBuildAuthoringVersion {
				t.Fatal("new Build did not use the revised contract", err)
			}
			good := []TestScenario{
				{Input: "Maria: Move my Thursday 10am clean to Friday 2pm.", Expected: "List Maria's cleaning change to Friday 2pm."},
				{Input: "Dan: Use eco-friendly products for tomorrow's 9am visit.", Expected: "List Dan's request for eco-friendly products tomorrow at 9am."},
				{Input: "Priya: Include the kitchen during Wednesday's 11am visit.", Expected: "List Priya's kitchen request for Wednesday at 11am."},
			}
			bad := append([]TestScenario(nil), good...)
			bad[0].Expected += " Do not ask for the customer name. Do not ask for the requested change. Do not ask for the time."
			bad[1].Expected += " Do not ask for the customer name. Do not ask for the special request. Do not ask for the time."
			bad[2] = TestScenario{Input: "Can someone come later this week? Also I need the kitchen done too.", Expected: "Ask for the customer name. Ask for the requested change. Ask for the time."}
			if tc.absentMaterial {
				bad = append([]TestScenario(nil), good...)
				bad[0].Input = "WhatsApp messages from customers of a cleaning company, each containing a customer name, a requested change or special request, and a timing detail."
			}
			calls := 0
			var firstReview SuiteReviewInput
			r := &Runner{Service: s, Gateway: &Gateway{Store: s.Store, Config: s.Config, Gate: s.Gate, Client: callFunc(func(_ context.Context, req provider.Request) (provider.Response, error) {
				calls++
				if calls > 1 && strings.Contains(string(raw(req.Messages)), "exhausting day") {
					t.Fatal("casual chat polluted author or reviewer sources")
				}
				var out any
				switch calls {
				case 1:
					out = interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 3}, factObservation{Kind: "job", Quote: cleaningCompanyRequest})
				case 2:
					var in taskInput
					if err := json.Unmarshal([]byte(req.Messages[1].Content), &in); err != nil {
						t.Fatal(err)
					}
					id := in.CurrentRequest.ID
					out = createSuiteCommand{Rules: []PolicyRule{{ID: "list", Statement: "Turn customer messages into a simple list of who needs what and when.", SourceBlockIDs: []string{id}, Evidence: []RuleEvidence{{SourceBlockID: id, Quote: cleaningCompanyRequest, Kind: "requirement"}}}}, Tests: testSuiteProposal{Title: "Cleaning requests", Summary: "List customer requests and their times.", Scenarios: bad}}
				case 3, 5:
					var in SuiteReviewInput
					if err := json.Unmarshal([]byte(req.Messages[1].Content), &in); err != nil {
						t.Fatal(err)
					}
					if calls == 3 {
						firstReview = in
					} else if in.PolicyHash != firstReview.PolicyHash || len(in.Cases) != 3 {
						t.Fatal("repair changed policy or dropped checks")
					}
					review := supportedSuiteReview(in)
					if tc.absentMaterial && calls == 3 {
						if !strings.Contains(req.Messages[0].Content, "defective generated input") {
							t.Fatal("reviewer was not instructed to check input feasibility")
						}
						cases := review["cases"].([]SuiteCaseReview)
						cases[0].Status, cases[0].Reason = SuiteContradicted, "The author omitted actual messages; the input only describes material."
					}
					findings := review["assertions"].([]SuiteAssertionReview)
					for i, claim := range suiteAssertions(in) {
						if strings.Contains(strings.ToLower(claim.Text), "ask for") {
							findings[i].Status, findings[i].Reason, findings[i].Support = SuiteContradicted, "No supplied rule requires: "+claim.Text, nil
							findings[i].Applicability.OppositeAllowed = true
						}
					}
					out = review
				case 4:
					var in taskInput
					if err := json.Unmarshal([]byte(req.Messages[1].Content), &in); err != nil {
						t.Fatal(err)
					}
					var feedback struct {
						Assertions []repairAssertion        `json:"assertions"`
						Problems   []SuiteValidationProblem `json:"problems"`
					}
					if err := json.Unmarshal(raw(in.ServerContext), &feedback); err != nil {
						t.Fatal(err)
					}
					rejected := 0
					for _, claim := range feedback.Assertions {
						if claim.Status == SuiteContradicted && strings.Contains(claim.Reason, claim.Text) {
							rejected++
						}
					}
					if tc.absentMaterial {
						if !strings.Contains(string(raw(feedback.Problems)), "omitted actual messages") {
							t.Fatal("repair lost the case input defect")
						}
					} else if rejected != 9 {
						t.Fatalf("repair received %d rejected clauses, want all 9", rejected)
					}
					patch := preciseEditCommand{PolicyPatch: PolicyPatch{BaseID: in.PolicyEditBase.ID, BaseHash: in.PolicyEditBase.Hash, Changes: []RulePatch{}}}
					for i, scenario := range good {
						if tc.absentMaterial && i > 0 {
							continue // The other case inputs/expectations already passed review.
						}
						expected, input := scenario.Expected, scenario.Input
						if !tc.repairWorks {
							expected, input = bad[i].Expected, bad[i].Input
							expected = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(expected, "Do not ask for the time.", ""), "Ask for the time.", ""))
						}
						patch.CaseChanges = append(patch.CaseChanges, CaseChange{Action: "update", CaseKey: fmt.Sprintf("case-%d", i+1), Input: &input, Expected: &expected})
					}
					out = patch
				default:
					t.Fatalf("unbounded repair: call %d", calls)
				}
				cost := json.Number("0.000001")
				return provider.Response{OutputText: string(raw(out)), Usage: provider.Usage{CostUSD: &cost}}, nil
			})}}
			runErr := r.Execute(ctx, o.ID)
			var issue *Fault
			if tc.repairWorks && runErr != nil {
				t.Fatal(runErr)
			}
			if !tc.repairWorks {
				requireFault(t, runErr, "test_policy_conflict")
				issue = issueFrom(runErr)
				if strings.Contains(issue.Message, "previous tests") || strings.Contains(issue.Message, "clarify") {
					t.Fatal("failed generation blamed the user", issue.Message)
				}
			}
			if calls != 5 {
				t.Fatalf("unexpected call count: %d", calls)
			}
			if err = r.Finalize(ctx, o.ID, issue); err != nil {
				t.Fatal(err)
			}
			ResumeBuilds(ctx, s)
			ResumeBuilds(ctx, s)
			v, err = s.Store.GetSession(ctx, v.Actor, v.ID)
			if err != nil {
				t.Fatal(err)
			}
			checks := 0
			for _, op := range v.Operations {
				if op.Kind == "check" {
					checks++
				}
			}
			if tc.repairWorks {
				if len(v.Document.Artifacts) != 1 || checks != 1 || v.Document.Build.Phase != "checking" || v.Document.Build.ClarificationsUsed != 0 {
					t.Fatalf("repaired Build did not continue exactly once: %+v", v.Document.Build)
				}
				artifact := v.Document.Artifacts[0]
				if artifact.Validation.Status != SuiteSupported || strings.Contains(artifact.AgentPrompt, "Maria") || strings.Contains(artifact.AgentPrompt, "Ask for the") {
					t.Fatal("prototype inherited checks or invented behavior")
				}
			} else if len(v.Document.Artifacts) != 0 || checks != 0 || v.Document.Build.Phase != "error" {
				t.Fatal("rejected repair published an unreviewed prototype or checks")
			}
		})
	}
}

func TestIntegrationBuildRetryUpgradesLegacyPreparationWithoutDuplicatingRequest(t *testing.T) {
	s, v := materialService(t)
	ctx := context.Background()
	o, _ := startBuild(t, s, v, cleaningCompanyRequest)
	var original Plan
	if err := json.Unmarshal(o.Input, &original); err != nil {
		t.Fatal(err)
	}
	// Reproduce a failed v19 request, like the user's existing conversation.
	original.AuthoringVersion = materialBuildAuthoringVersion
	original.Conversation.ContractVersion = "vibe-v19"
	if _, err := s.Store.DB.Exec(ctx, "UPDATE vibe_operations SET input=$2 WHERE id=$1", o.ID, raw(original)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Store.Start(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	r := &Runner{Service: s}
	if err := r.Finalize(ctx, o.ID, &Fault{Code: "test_policy_conflict", Message: "Earlier generation failed."}); err != nil {
		t.Fatal(err)
	}
	v, err := s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	request := RetryRequest{ClientID: uuid.New(), Revision: v.Revision}
	retry, err := s.Retry(ctx, v.Actor, v.ID, o.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Retry(ctx, v.Actor, v.ID, o.ID, request)
	if err != nil || again.ID != retry.ID {
		t.Fatal("retry acknowledgement created another operation", err)
	}
	var updated Plan
	if err := json.Unmarshal(retry.Input, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.AuthoringVersion != contextualBuildAuthoringVersion || updated.Submission.Content != cleaningCompanyRequest || updated.sourceMessageID() != original.sourceMessageID() || updated.Cycle.ID != original.Cycle.ID || updated.Cycle.ClarificationsUsed != original.Cycle.ClarificationsUsed {
		t.Fatal("retry lost the source, question budget, or revised authoring contract")
	}
	saved, err := s.Store.Operation(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	savedHash, savedErr := CanonicalJSONHash(saved.Input)
	originalHash, originalErr := CanonicalJSONHash(raw(original))
	if savedErr != nil || originalErr != nil || savedHash != originalHash {
		t.Fatal("retry rewrote the historical plan", savedErr, originalErr)
	}
	v, err = s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil || len(v.Document.Messages) != 1 || len(v.Operations) != 2 {
		t.Fatal("retry duplicated the user message or operation", err)
	}
}
