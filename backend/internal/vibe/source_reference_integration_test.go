package vibe

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/jsonschema-go/jsonschema"
)

func TestIntegrationAuthoringSourceRepairPreservesProvenanceAndAccounting(t *testing.T) {
	s, v, _ := memoryService(t)
	calls := 0
	text := "My agent handles returns. Items bought within 30 days qualify. Prepare one test."
	got, op := memoryExecute(t, s, v, text, func(req provider.Request) any {
		calls++
		if calls == 1 {
			return interpretationFixture(prepareAction{Kind: "prepare_tests", Count: 1}, factObservation{Kind: "job", Quote: "My agent handles returns."}, factObservation{Kind: "rule", Quote: "Items bought within 30 days qualify."})
		}
		if calls == 4 {
			var input SuiteReviewInput
			if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil {
				t.Fatal(err)
			}
			return supportedSuiteReview(input)
		}
		if calls != 2 && calls != 3 {
			t.Fatal("unexpected additional provider dispatch")
		}
		var input taskInput
		if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil {
			t.Fatal(err)
		}
		id := input.CurrentRequest.ID
		ref := id
		if calls == 2 {
			ref = "window"
		}
		reply := createSuiteCommand{Rules: []PolicyRule{{ID: "window", Statement: "Items bought within 30 days qualify.", SourceBlockIDs: []string{ref}, Evidence: []RuleEvidence{{SourceBlockID: ref, Kind: "requirement", Quote: "Items bought within 30 days qualify."}}}}, Tests: testSuiteProposal{Title: "Returns", Summary: "Checks eligibility.", SuccessCriteria: "Follow the return window.", Scenarios: []TestScenario{{Input: "Bought 10 days ago.", Expected: "Eligible for return."}}}}
		if calls == 3 {
			// Exercise the response schema actually sent by the repair path, then
			// check that the real server commits only valid provenance below.
			var format struct {
				JSONSchema struct {
					Schema jsonschema.Schema `json:"schema"`
				} `json:"json_schema"`
			}
			if err := json.Unmarshal(req.ResponseFormat, &format); err != nil {
				t.Fatal(err)
			}
			schema, err := format.JSONSchema.Schema.Resolve(nil)
			if err != nil {
				t.Fatal(err)
			}
			var body any
			if err = json.Unmarshal(raw(reply), &body); err != nil {
				t.Fatal(err)
			}
			if err = schema.Validate(body); err != nil {
				t.Fatal("repair rejected supplied source", err)
			}
			reply.Rules[0].SourceBlockIDs = []string{"window"}
			if err = json.Unmarshal(raw(reply), &body); err != nil {
				t.Fatal(err)
			}
			if schema.Validate(body) == nil {
				t.Fatal("repair schema still allowed a rule ID as a source")
			}
			reply.Rules[0].SourceBlockIDs = []string{id}
			feedback := false
			for _, m := range req.Messages {
				feedback = feedback || strings.Contains(m.Content, "server_validation_feedback")
			}
			if !feedback {
				t.Fatal("invalid output skipped bounded repair feedback")
			}
		}
		return reply
	})
	if calls != 4 || op.State != Completed || op.Billing != Settled || op.ModelCalls != 4 {
		t.Fatal("repair did not complete and settle within its bounded call graph")
	}
	if len(got.Document.Artifacts) != 1 || len(got.Document.Policies) != 1 {
		t.Fatal("valid repaired test suite was not committed")
	}
	policy := got.Document.Policies[0]
	if len(policy.Rules) != 1 || policy.Rules[0].SourceBlockIDs[0] == "window" || policy.Rules[0].Evidence[0].SourceBlockID != policy.Rules[0].SourceBlockIDs[0] {
		t.Fatal("invalid source reference persisted")
	}
	var rejected, accepted bool
	if err := s.Store.DB.QueryRow(t.Context(), "SELECT domain_outcome->>'status'='rejected' FROM vibe_attempts WHERE operation_id=$1 AND step_key='handler'", op.ID).Scan(&rejected); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.DB.QueryRow(t.Context(), "SELECT domain_outcome->>'status'='accepted' FROM vibe_attempts WHERE operation_id=$1 AND step_key='handler:repair'", op.ID).Scan(&accepted); err != nil {
		t.Fatal(err)
	}
	if !rejected || !accepted {
		t.Fatal("repair lost the rejected and accepted outcome journal")
	}
}
