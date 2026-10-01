package vibe

// A case-level finding is a summary and may describe only the last rejected
// clause. Repair needs every clause, including the supported ones to preserve.
type repairAssertion struct {
	SuiteAssertion
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func suiteRepairContext(p Plan, input SuiteReviewInput, validation SuiteValidation, policy PolicySnapshot) map[string]any {
	context := map[string]any{
		"action": "edit_tests", "repair_candidate": true,
		"problems": validation.Problems, "candidate_policy": policy,
		"instruction": "Patch only rejected cases; do not add/remove cases. Keep supported rules and fields unchanged.",
	}
	if p.precise() {
		delete(context, "candidate_policy")
	}
	if p.AuthoringVersion < groundedBuildAuthoringVersion {
		return context
	}
	byID := make(map[string]SuiteAssertionReview, len(validation.Assertions))
	for _, finding := range validation.Assertions {
		byID[finding.ID] = finding
	}
	assertions := make([]repairAssertion, 0, len(validation.Assertions))
	for _, assertion := range suiteAssertions(input) {
		finding := byID[assertion.ID]
		assertions = append(assertions, repairAssertion{SuiteAssertion: assertion, Status: finding.Status, Reason: finding.Reason})
	}
	context["assertions"] = assertions
	context["instruction"] = "Repair ALL rejected assertions and case problems in this single patch, not just the case summary's last issue. Preserve supported assertions and rules. Remove unsupported obligations; do not invent a policy to justify them. If a generated input omitted necessary task content, supply concrete fictional text/data in that input rather than asking the user for it. If no supported expectation remains for a rejected case, replace that case's input and expectation with a distinct situation decided by the existing job/rules. Only update rejected case keys; do not add/remove cases. Keep the original request and accepted count. The independent reviewer will check every clause again."
	return context
}

func suitePreparationFailure(p Plan, code string) error {
	if code == "test_policy_conflict" && p.Conversation != nil && p.Conversation.Manual != nil {
		return fault("test_policy_conflict", "These edits don't match the saved rules. Your saved checks are unchanged. Review the edited expectations or describe the rule you want to change.")
	}
	message := "I couldn't finish preparing reliable checks for this task. Your request is saved. Please retry."
	if p.Artifact != nil || len(p.Document.Artifacts) > 0 {
		message = "I couldn't finish preparing reliable checks for this update. Your saved version and results are unchanged. Please retry."
	}
	return fault(code, message)
}
