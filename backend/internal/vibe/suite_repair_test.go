package vibe

import (
	"strings"
	"testing"
)

func TestSuiteRepairIncludesEveryClauseDespiteLossyCaseSummary(t *testing.T) {
	_, input := suiteReviewFixture(t)
	input.ValidatorVersion = EntailmentSuiteValidatorVersion
	input.Cases[0].Expected = "Explain eligibility. Ask for the customer name. Ask for the time."
	reply := supportedSuiteReview(input)
	assertions := reply["assertions"].([]SuiteAssertionReview)
	for i, claim := range suiteAssertions(input) {
		if strings.HasPrefix(claim.Text, "Ask for") {
			assertions[i].Status = SuiteContradicted
			assertions[i].Reason = "Unsupported: " + claim.Text
			assertions[i].Support = nil
			assertions[i].Applicability.OppositeAllowed = true
		}
	}
	review, err := ParseSuiteReview(raw(reply), input, LimitsFor(false))
	if err != nil || review.Status != SuiteContradicted {
		t.Fatal("fixture must reject both invented clauses", err)
	}
	// This is the production defect: the case summary retains only the last
	// assertion's reason. Both exact clauses must reach the repair anyway.
	if review.Cases[0].Reason != "Unsupported: Ask for the time." {
		t.Fatal("fixture no longer reproduces the lossy case summary")
	}
	p := Plan{AuthoringVersion: groundedBuildAuthoringVersion}
	feedback := suiteRepairContext(p, input, *review, input.Policy)
	clauses := feedback["assertions"].([]repairAssertion)
	if len(clauses) != len(suiteAssertions(input)) || clauses[0].Status != SuiteSupported {
		t.Fatal("repair must also know which clauses to preserve")
	}
	for i, text := range []string{"Ask for the customer name.", "Ask for the time."} {
		if clauses[i+1].Text != text || clauses[i+1].Reason != "Unsupported: "+text || clauses[i+1].Status != SuiteContradicted {
			t.Fatalf("repair lost rejected clause: %+v", clauses[i+1])
		}
	}
	p.AuthoringVersion = materialBuildAuthoringVersion
	legacy := suiteRepairContext(p, input, *review, input.Policy)
	if _, exists := legacy["assertions"]; exists || len(legacy) != 4 {
		t.Fatal("historical repair request changed")
	}
}

func TestSuitePreparationFailureDistinguishesGeneratedAndManualEdits(t *testing.T) {
	for _, tc := range []struct {
		name string
		plan Plan
		want string
	}{
		{"first build", Plan{}, "Your request is saved."},
		{"existing version", Plan{Artifact: &Artifact{}}, "Your saved version and results are unchanged."},
		{"existing document", Plan{Document: Document{Artifacts: []Artifact{{}}}}, "Your saved version and results are unchanged."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issue := issueFrom(suitePreparationFailure(tc.plan, "test_policy_conflict"))
			if issue.Code != "test_policy_conflict" || !strings.Contains(issue.Message, tc.want) || strings.Contains(issue.Message, "clarify") || strings.Contains(issue.Message, "Review") {
				t.Fatalf("generated error blames user or invents history: %+v", issue)
			}
		})
	}
	manual := Plan{Conversation: &ConversationContext{Manual: &ManualSuiteEdit{}}}
	if !strings.Contains(suitePreparationFailure(manual, "test_policy_conflict").Error(), "These edits don't match") {
		t.Fatal("user-edited checks lost actionable feedback")
	}
}

func TestGroundedBuildKeepsLegacyAuthoringPrompt(t *testing.T) {
	p := Plan{AuthoringVersion: materialBuildAuthoringVersion, Conversation: &ConversationContext{ValidatorVersion: EntailmentSuiteValidatorVersion}}
	legacy := reliableHandlerPrompt(p)
	if !strings.Contains(legacy, legacyPreparationPrompt) || !strings.Contains(legacy, "For newly written expected answers under an ask-only-for-missing-information rule") {
		t.Fatal("historical authoring prompt changed")
	}
	p.AuthoringVersion = groundedBuildAuthoringVersion
	current := reliableHandlerPrompt(p)
	if strings.Contains(current, legacyPreparationPrompt) || !strings.Contains(current, groundedPreparationPrompt) || strings.Contains(current, "For already supplied facts, omit a request or say") {
		t.Fatal("current Build still pushes generic question-asking checks")
	}
}
