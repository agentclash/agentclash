package vibe

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentclash/agentclash/runtime/provider"
)

// Versioned runner preserves exact replay of v11-v14 operation journals.
func (r *Runner) converseInterpreted(ctx context.Context, o Operation, p Plan) error {
	if p.Conversation == nil {
		return fault("invalid_plan", "The saved conversation context is unavailable.")
	}
	if p.stateful() && (p.Conversation.State == nil || p.Conversation.State.Version != conversationStateVersion || len(p.Conversation.StateBaseHash) != 64 || !p.sourceBoundary()) {
		return fault("invalid_plan", "The saved conversation state is unavailable.")
	}
	if action := boundDialogueAction(p); action != nil && action.Kind != "answer_question" {
		next := cloneState(p.Conversation.State)
		current := Message{ID: p.sourceMessageID(), Role: "user", Content: p.Submission.Content}
		reply, e := applyStateInteraction(next, *action, current)
		if e != nil {
			return e
		}
		next.ActionsVersion = 1
		next.ThroughMessageID = deterministicID(o.ID, "completion-message").String()
		p.Conversation.NextState = next
		if e = r.Service.Store.commitConversationDecision(ctx, o, p, "chat"); e != nil {
			return e
		}
		return r.completeReliableDocument(ctx, o, p, reply, nil, nil, AuthoringCompletion{Interaction: action, Outcome: &CompletionReceipt{Action: "chat"}})
	}
	profile, err := r.reliableProfile(o, p)
	if err != nil {
		return err
	}
	fallbackUsed := false
	patched := false
	route := reliableRoute{Intent: "edit_tests"}
	if p.Conversation.Manual == nil {
		if p.precise() && p.Conversation.Confirmed != nil {
			c := p.Conversation.Confirmed
			route = reliableRoute{Intent: c.Intent, Count: c.Count, NewAgent: c.NewAgent, Reply: "I will use those rules.", Memory: &memoryUpdate{}}
			if c.NewAgent {
				route.Memory.NewScopeQuote = p.Submission.Content
			}
			p.Conversation.NextState, err = proposeConversationState(p, route, o)
			if err != nil {
				return err
			}
		} else {
			if err = r.observeUnderstanding(ctx, o, &p); err != nil {
				return err
			}
			_, _, err = r.interpretedStage(ctx, o, p, "route", &fallbackUsed, taskMessages(p, taskRoute, "", nil), func(profile ModelProfile) json.RawMessage { return interpretationFormat(profile) }, func(output []byte) error {
				var e error
				p.Conversation.NextState = nil
				route, e = decodeInterpretation(output, p)
				if e != nil {
					return e
				}
				ids := []string{}
				for _, id := range route.SourceMessageIDs {
					if id != p.Conversation.CurrentRequest.ID {
						ids = append(ids, id)
					}
				}
				route.SourceMessageIDs = ids
				if e = validateReliableRoute(route, p); e != nil {
					return e
				}
				p.Conversation.NextState, e = proposeConversationState(p, route, o)
				if e == nil {
					next := p.Conversation.NextState
					if route.Memory.CancelQuestionQuote != "" {
						next.PendingPreparation = nil
					}
					if route.Intent == "clarify" && route.Memory.CancelQuestionQuote == "" && p.Artifact == nil && next.PendingPreparation == nil && len(route.Memory.Facts) > 0 {
						request := p.Conversation.CurrentRequest
						next.PendingPreparation = &request
					}
					if route.Intent == "prepare_tests" {
						next.PendingPreparation = nil
					}
				}
				return e
			})
			if err != nil {
				return err
			}
		}
		if p.guided() {
			p.Conversation.Example = route.Example
		}
		if p.Submission.AdditionalExamples > 0 {
			// This explicit action adds coverage; it cannot adopt new policy or
			// reinterpret the selected evaluation as a different agent.
			route.Intent, route.NewAgent = "edit_tests", false
			p.Conversation.NextState = cloneState(p.Conversation.State)
		}
		if p.Document.Evaluation != nil && route.NewAgent && len(p.Document.Artifacts) > 0 {
			p.Conversation.NextState = cloneState(p.Conversation.State)
			return r.completeReliableDocument(ctx, o, p, "Use New evaluation beside the message box to start a different agent. These rules and results will stay here.", nil, nil, AuthoringCompletion{Outcome: &CompletionReceipt{Action: "chat"}})
		}
		if p.Cycle != nil {
			if route.Intent == "prepare_tests" {
				route.Count = 3
			}
			if route.Intent == "chat" || route.Intent == "clarify" {
				if p.Cycle.ClarificationsUsed >= 1 || (buildHasJob(p) && buildHasRules(p)) {
					if !buildHasRules(p) {
						return r.completeSamplePrototype(ctx, o, p)
					}
					route.Intent, route.Count = "prepare_tests", 3
					p.Conversation.NextState.PendingQuestion = nil
				} else if route.Intent == "chat" {
					route.Intent, route.Reply = "clarify", "Which repetitive task would you like AI to help with?"
					if route.Memory == nil {
						route.Memory = &memoryUpdate{}
					}
					route.Memory.Question = &memoryQuestion{Purpose: "clarify_job", Text: route.Reply, MaxSelections: 1}
					p.Conversation.NextState, err = proposeConversationState(p, route, o)
					if err != nil {
						return err
					}
				}
			}
		}
		confirmation, e := sourceConfirmationFor(p, o, route)
		if e != nil {
			return e
		}
		if confirmation != nil {
			if p.Cycle != nil && p.Cycle.ClarificationsUsed >= 1 {
				return r.completeSamplePrototype(ctx, o, p)
			}
			if err = r.Service.Store.commitConversationDecision(ctx, o, p, "clarify"); err != nil {
				return err
			}
			return r.completeReliableDocument(ctx, o, p, confirmation.Question, nil, nil, AuthoringCompletion{SourceConfirmation: confirmation, Outcome: &CompletionReceipt{Action: "clarify"}})
		}
		if err = r.Service.Store.commitConversationDecision(ctx, o, p, route.Intent); err != nil {
			return err
		}
		if route.Intent == "chat" || route.Intent == "clarify" || route.Intent == "explain_results" {
			return r.completeReliableDocument(ctx, o, p, route.Reply, nil, nil, AuthoringCompletion{Outcome: &CompletionReceipt{Action: route.Intent}})
		}
		p.Conversation.RequiredCount = route.Count
	} else if err = r.Service.Store.commitConversationDecision(ctx, o, p, route.Intent); err != nil {
		return err
	}
	applySourceScope(&p, route)
	addMemorySources(&p)
	if route.Intent == "edit_tests" {
		if err = r.Service.Store.recordPendingPolicy(ctx, o, p); err != nil {
			return err
		}
	}
	var candidate *Artifact
	var policy PolicySnapshot
	var commandHash string
	changed := 0
	if p.Conversation.Manual != nil {
		if p.Artifact == nil || p.Conversation.Policy == nil {
			return fault("rules_required", "Describe the rules for these tests before editing them here. Your original tests are unchanged.")
		}
		copy := *p.Artifact
		copy.ID = deterministicID(o.ID, "artifact")
		copy.ParentID = &p.Artifact.ID
		copy.CreatedAt = operationTime(o)
		copy.Accepted = false
		copy.Dismissed = false
		copy.Proposal = nil
		copy.Validation = nil
		copy.Blueprint = p.Conversation.Manual.Blueprint
		candidate = &copy
		policy = *p.Conversation.Policy
		commandHash = Hash(raw(p.Conversation.Manual))
		changed = countChangedCases(p.Artifact.Blueprint, copy.Blueprint)
	} else {
		response, _, e := r.interpretedStage(ctx, o, p, "handler", &fallbackUsed, taskMessages(p, taskAuthor, route.Intent, map[string]any{"action": route.Intent, "count": route.Count}), func(profile ModelProfile) json.RawMessage { return reliableCommandFormat(profile, p, route.Intent) }, func(output []byte) error {
			var e error
			candidate, policy, changed, e = r.buildReliableCandidate(output, route.Intent, o, p)
			if e == nil {
				e = validateCoverageExpansion(p, candidate, policy)
			}
			return e
		})
		if e != nil {
			return e
		}
		commandHash = Hash([]byte(response.OutputText))
	}
	if route.Intent == "suggest_fix" {
		// Instruction fixes cannot alter the test contract. Existing verified
		// metadata carries forward; legacy suites are checked before a fresh run.
		return r.completeReliableDocument(ctx, o, p, "", candidate, nil, AuthoringCompletion{Outcome: &CompletionReceipt{Action: route.Intent, CommandHash: commandHash}})
	}
	count := suiteCaseCount(candidate.Blueprint)
	if route.Intent == "prepare_tests" && count != route.Count {
		return fault("suite_count_mismatch", "The generated test count did not match your request. No new tests were saved.")
	}
	for reviewStep := "review"; ; reviewStep = "candidate:review" {
		input, e := BuildSuiteReviewInput(candidate.Blueprint, policy, p.Conversation.Sources, p.Conversation.CurrentRequest, count, p.limits())
		if e != nil {
			return e
		}
		statefulReviewInput(p, &input)
		if p.sourceBoundary() {
			input.Summary = candidate.Summary
		}
		if route.Intent != "prepare_tests" {
			input.PreviousPolicy = p.Conversation.Policy
		}
		if p.reviewVersion() != SuiteValidatorVersion {
			input.ValidatorVersion = p.reviewVersion()
		}
		var validation *SuiteValidation
		validate := func(output []byte) error {
			var e error
			validation, e = ParseSuiteReview(output, input, p.limits())
			if e == nil && validation == nil {
				e = fmt.Errorf("review result is missing")
			}
			if e == nil && ungroundedReview(validation) {
				e = fmt.Errorf("correct the review ledger, not the tests: %s", raw(validation.Consistency.Findings))
			}
			return e
		}
		var resp provider.Response
		actualProfile := profile
		if reviewStep == "review" {
			resp, actualProfile, e = r.interpretedStage(ctx, o, p, reviewStep, &fallbackUsed, reviewTaskMessages(p, input, profile), func(pr ModelProfile) json.RawMessage { return SuiteReviewFormatFor(pr, input) }, validate)
		} else {
			resp, e = r.reliableCall(ctx, o, reviewStep, reviewTaskMessages(p, input, profile), SuiteReviewFormatFor(profile, input))
			if e == nil {
				e = validate([]byte(resp.OutputText))
				if je := r.journalReliable(ctx, o, reviewStep, "review", p, candidate.Blueprint, e); je != nil {
					return je
				}
			}
		}
		if e != nil {
			return e
		}
		validation.Model = actualProfile.ID
		validation.ProfileHash = Hash(raw(actualProfile))
		candidate.Validation = validation
		if validation.Status == SuiteSupported {
			break
		}
		if validation.Status == SuiteUnclear {
			if validation.Consistency != nil && len(validation.Consistency.Findings) > 0 {
				// A failure to ground the review is not a missing business rule.
				// Keep diagnostics in the attempt instead of asking the user to
				// repair an internal ledger or pretending their rule is unclear.
				return fault("validation_unavailable", "I couldn't reliably check these tests. Your request is saved and your previous tests are unchanged. Please retry.")
			}
			if p.Cycle != nil && p.Cycle.ClarificationsUsed >= 1 {
				return r.completeSamplePrototype(ctx, o, p)
			}
			return r.completeReliableDocument(ctx, o, p, validationQuestion(validation), nil, nil, AuthoringCompletion{Outcome: &CompletionReceipt{Action: "clarify"}})
		}
		if patched || p.Conversation.Manual != nil {
			return fault("test_policy_conflict", "The proposed tests conflict with the supplied rules. Your previous tests are unchanged. Please review the expected answers or clarify the rule.")
		}
		patched = true
		// A semantic repair is a patch against the candidate, not a second
		// independent generation. Only rejected case fields may be changed.
		repairPlan := p
		copyContext := *p.Conversation
		repairPlan.Conversation = &copyContext
		repairPlan.Artifact = candidate
		if p.precise() {
			repairPlan.Conversation.Policy = &policy
		}
		repairContext := map[string]any{"action": "edit_tests", "repair_candidate": true, "problems": validation.Problems, "candidate_policy": policy, "instruction": "Patch only rejected cases; do not add/remove cases. Keep supported rules and fields unchanged."}
		if p.precise() {
			delete(repairContext, "candidate_policy")
		}
		messages := taskMessages(repairPlan, taskAuthor, "edit_tests", repairContext)
		resp, e = r.reliableCall(ctx, o, "candidate:patch", messages, reliableCommandFormat(profile, repairPlan, "edit_tests"))
		if e != nil {
			return e
		}
		patch, decodeErr := decodeEditCommand([]byte(resp.OutputText), repairPlan)
		err = decodeErr
		if err == nil {
			err = checkScopedRepair(patch, *validation, policy)
		}
		if err == nil {
			if p.sourceBoundary() {
				criteria := policyGradingCriteria(patch.Rules)
				patch.Criteria = &criteria
			}
			candidate.Blueprint, err = patchTestSuite(candidate.Blueprint, patch.CaseChanges, patch.Criteria, p.limits(), o.ID)
			if err == nil {
				policy, err = reconcilePolicy(patch.Rules, p, o, route.Intent == "prepare_tests")
			}
		}
		if err == nil {
			_, err = r.Service.Compiler.Compile(candidate.Blueprint, o.Models.Evaluator, candidate.ID, p.limits())
		}
		if e = r.journalReliable(ctx, o, "candidate:patch", "repair", p, candidate.Blueprint, err); e != nil {
			return e
		}
		if err != nil {
			return fault("invalid_repair", "I couldn't finish correcting these tests. Your previous tests are unchanged.")
		}
		commandHash = Hash(raw(map[string]any{"original": commandHash, "patch": patch}))
		candidate.Validation = nil
	}
	candidate.PolicyID = &policy.ID
	if err = validateCoverageExpansion(p, candidate, policy); err != nil {
		return fault("invalid_update", "The additional examples changed existing tests or rules. Your baseline is unchanged; retry preparing this batch.")
	}
	if err = r.preparePrototype(ctx, o, p, candidate, policy, &fallbackUsed); err != nil {
		return err
	}
	candidate.Provenance = "ai_generated"
	coverage := []SourceCoverage{}
	referenced := map[string]bool{}
	for _, rule := range policy.Rules {
		for _, id := range rule.SourceBlockIDs {
			referenced[id] = true
		}
	}
	for _, source := range p.Conversation.Sources {
		// Referencing a block does not prove every clause was extracted. Retain
		// the original in subsequent context even after a successful review.
		if referenced[source.ID] {
			coverage = append(coverage, SourceCoverage{MessageID: source.MessageID, State: "referenced"})
		}
	}
	return r.completeReliableDocument(ctx, o, p, "", candidate, nil, AuthoringCompletion{Policy: &policy, Coverage: coverage, Outcome: &CompletionReceipt{Action: route.Intent, CommandHash: commandHash, CaseCount: count, ChangedCaseCount: changed, ValidationStatus: SuiteSupported, ValidationVersion: candidate.Validation.ValidatorVersion}})
}
