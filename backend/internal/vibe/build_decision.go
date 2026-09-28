package vibe

// applyBuildDecision enforces the initial question budget without treating the
// budget itself as user intent. A casual reply leaves the pending task alone.
// True selects an explicitly labelled starter, never invented business policy.
func applyBuildDecision(p Plan, route *reliableRoute) bool {
	if p.Cycle == nil || p.Submission.AdditionalExamples > 0 {
		return false
	}
	if route.Intent == "prepare_tests" {
		if !buildHasRules(p) && buildSampleKind(p) == "email_sorting" {
			return true
		}
		route.Count = 3
		return false
	}
	if route.Intent != "chat" && route.Intent != "clarify" {
		return false
	}
	if route.Memory != nil && route.Memory.CancelQuestionQuote != "" {
		return false
	}
	// An answer is bound to an active question and an exact current quotation
	// by interpretation validation. Saved job/rules alone cannot authorize work.
	answer := route.Memory != nil && route.Memory.Answer != nil
	if route.Intent == "chat" && !answer {
		return false
	}
	ready := buildHasJob(p) && buildHasRules(p)
	starter := !buildHasRules(p) && buildSampleKind(p) == "email_sorting"
	if !ready && !starter && p.Cycle.ClarificationsUsed == 0 {
		return false
	}
	if !ready {
		return true
	}
	route.Intent, route.Count = "prepare_tests", 3
	if q := p.Conversation.NextState.PendingQuestion; q != nil && q.Status == "active" {
		q.Status = "superseded"
	}
	return false
}
