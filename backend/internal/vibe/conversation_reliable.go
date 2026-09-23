package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/runtime/provider"
)

type reliableReplayKey struct{}

func reliableHandlerPrompt(p Plan) string {
	prompt := reliableAuthoringPrompt
	if p.precise() {
		prompt = strings.ReplaceAll(prompt, "Keep the complete effective rule list.", "Preserve untouched rules through patches.") + preciseAuthorPrompt
	}
	if p.sourceBoundary() {
		prompt = strings.Replace(prompt, "Never reproduce quotations or invent source IDs.", "Never invent source IDs.", 1) + sourceAuthorPrompt
	}
	if p.reviewVersion() != LatestSuiteValidatorVersion {
		return prompt
	}
	return prompt + `
For newly written expected answers under an ask-only-for-missing-information rule, put each required question in a simple separate sentence: "Ask for <field name>." Use the exact field names from the user's rule. For already supplied facts, omit a request or say "Do not ask for <field name>." Put other expected behavior in separate sentences. Preserve the user's meaning and scenario facts; do not rewrite unrelated existing cases merely to change their style.`
}

func (r *Runner) reliableCall(ctx context.Context, o Operation, step string, messages []provider.Message, format json.RawMessage) (provider.Response, error) {
	if replay, _ := ctx.Value(reliableReplayKey{}).(bool); replay {
		return r.Service.Store.RecordedResponse(ctx, o.ID, step, Hash(raw(messages)), format)
	}
	return r.Gateway.Call(ctx, o, step, Assistant, messages, format)
}
func (r *Runner) journalReliable(ctx context.Context, o Operation, step, stage string, p Plan, blueprint json.RawMessage, err error) error {
	status := "accepted"
	problems := []DomainProblem{}
	if err != nil {
		status = "rejected"
		problems = append(problems, DomainProblem{Code: "invalid_" + stage, Message: boundedDiagnostic(err.Error())})
	}
	hash := ""
	if len(blueprint) > 0 {
		hash, _ = CanonicalJSONHash(blueprint)
	}
	version := "v11"
	if p.stateful() {
		version = "v12"
	}
	if p.guided() {
		version = "v14"
	}
	if p.sourceBoundary() {
		version += "/" + SourcePolicyVersion
	}
	return r.Service.Store.RecordDomainOutcome(ctx, o.ID, step, DomainOutcome{Stage: stage, Status: status, PromptVersion: version, SchemaVersion: version, ValidatorVersion: p.reviewVersion(), InputHash: Hash(o.Input), CandidateHash: hash, Problems: problems})
}
func boundedDiagnostic(s string) string {
	if len(s) > 900 {
		return strings.ToValidUTF8(s[:900], "")
	}
	return s
}

func (r *Runner) converseReliable(ctx context.Context, o Operation, p Plan) error {
	if p.Conversation == nil {
		return fault("invalid_plan", "The saved conversation context is unavailable.")
	}
	if p.stateful() && (p.Conversation.State == nil || p.Conversation.State.Version != conversationStateVersion || len(p.Conversation.StateBaseHash) != 64 || !p.sourceBoundary()) {
		return fault("invalid_plan", "The saved conversation state is unavailable.")
	}
	if action := boundDialogueAction(p); action != nil {
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
	repaired := false
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
			step := "route"
			messages := taskMessages(p, taskRoute, "", nil)
			for {
				resp, e := r.reliableCall(ctx, o, step, messages, reliableRouteFormat(profile, p))
				if e != nil {
					return e
				}
				route = reliableRoute{}
				err = Decode([]byte(resp.OutputText), p.limits(), &route)
				// Only preparation uses this field to authorize a case count. An
				// edit's real additions/removals are validated from its case patches.
				if err == nil && p.sourceBoundary() && route.Intent != "prepare_tests" {
					route.Count = 0
				}
				if err == nil && p.sourceBoundary() && p.Conversation.Confirmed != nil {
					confirmed := p.Conversation.Confirmed
					route = reliableRoute{Intent: confirmed.Intent, Count: confirmed.Count, NewAgent: confirmed.NewAgent, Reply: "I will use those rules.", Memory: route.Memory}
					if p.stateful() && confirmed.NewAgent && route.Memory != nil {
						// The existing confirmation gate verified this exact affirmative
						// against the displayed source question, including its new scope.
						route.Memory.NewScopeQuote = p.Submission.Content
					}
				}
				if err == nil {
					err = validateReliableRoute(route, p)
				}
				if err == nil && p.stateful() {
					p.Conversation.NextState, err = proposeConversationState(p, route, o)
				}
				if e = r.journalReliable(ctx, o, step, "route", p, nil, err); e != nil {
					return e
				}
				if err == nil {
					break
				}
				var f *Fault
				if errors.As(err, &f) && f.Code == "case_limit" {
					return err
				}
				if repaired {
					return fault("invalid_response", "I couldn't understand that request reliably. Your request is saved; please try again.")
				}
				repaired = true
				step = "repair"
				messages = taskMessages(p, taskRoute, "", map[string]any{"problems": boundedDiagnostic(err.Error()), "invalid_route": resp.OutputText, "instruction": "Correct the route or ask one necessary question; no action has been accepted."})
			}
		}
		if p.guided() {
			p.Conversation.Example = route.Example
		}
		confirmation, e := sourceConfirmationFor(p, o, route)
		if e != nil {
			return e
		}
		if confirmation != nil {
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
		step := "handler"
		messages := taskMessages(p, taskAuthor, route.Intent, map[string]any{"action": route.Intent, "count": route.Count})
		for {
			resp, e := r.reliableCall(ctx, o, step, messages, reliableCommandFormat(profile, p, route.Intent))
			if e != nil {
				return e
			}
			candidate, policy, changed, err = r.buildReliableCandidate([]byte(resp.OutputText), route.Intent, o, p)
			var bp json.RawMessage
			if candidate != nil {
				bp = candidate.Blueprint
			}
			if e = r.journalReliable(ctx, o, step, "command", p, bp, err); e != nil {
				return e
			}
			if err == nil {
				commandHash = Hash([]byte(resp.OutputText))
				break
			}
			if repaired {
				return fault("invalid_response", "I couldn't prepare a valid update. Your request is saved and your previous tests are unchanged.")
			}
			repaired = true
			step = "repair"
			messages = taskMessages(p, taskAuthor, route.Intent, map[string]any{"action": route.Intent, "count": route.Count, "problems": boundedDiagnostic(err.Error()), "invalid_command": resp.OutputText})
		}
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
	for reviewStep := "review"; ; reviewStep = "review:repair" {
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
		resp, e := r.reliableCall(ctx, o, reviewStep, reviewTaskMessages(p, input, profile), SuiteReviewFormatFor(profile, input))
		if e != nil {
			return e
		}
		validation, e := ParseSuiteReview([]byte(resp.OutputText), input, p.limits())
		reviewErr := e
		if e == nil && validation.Status != SuiteSupported {
			reviewErr = fault(validation.Status, validation.ProblemSummary())
		}
		if je := r.journalReliable(ctx, o, reviewStep, "review", p, candidate.Blueprint, reviewErr); je != nil {
			return je
		}
		if p.reviewVersion() == LatestSuiteValidatorVersion && (e != nil || ungroundedReview(validation)) {
			// An invalid review cannot authorize rewriting a valid test. Spend
			// the existing shared repair allowance on the review itself, keeping
			// the candidate and rule snapshot identical. Manual edits remain a
			// single-call review, and uncertain provider calls are never retried.
			if repaired || p.Conversation.Manual != nil {
				return fault("validation_unavailable", "I couldn't reliably check these tests. Your request is saved and your previous tests are unchanged. Please retry.")
			}
			repaired = true
			feedback := map[string]any{"task": "Correct only the review. The candidate tests and original rules are unchanged. Scenario facts and their evidence must come only from that case's input, never from a policy statement, expected answer, or another case. Field aliases must name the requested fields, not possible field values. Return the complete review using the same schema; do not rewrite the tests."}
			if e != nil {
				feedback["error"] = boundedDiagnostic(e.Error())
			} else {
				feedback["problems"] = validation.Consistency.Findings
			}
			messages := append(reviewTaskMessages(p, input, profile), provider.Message{Role: "user", Content: string(raw(map[string]any{"server_validation_feedback": feedback}))})
			resp, e = r.reliableCall(ctx, o, "repair", messages, SuiteReviewFormatFor(profile, input))
			if e != nil {
				return e
			}
			validation, e = ParseSuiteReview([]byte(resp.OutputText), input, p.limits())
			reviewErr = e
			if e == nil && validation.Status != SuiteSupported {
				reviewErr = fault(validation.Status, validation.ProblemSummary())
			}
			if je := r.journalReliable(ctx, o, "repair", "review", p, candidate.Blueprint, reviewErr); je != nil {
				return je
			}
			if e != nil || ungroundedReview(validation) {
				return fault("validation_unavailable", "I couldn't reliably check these tests. Your request is saved and your previous tests are unchanged. Please retry.")
			}
		}
		if e != nil {
			return fault("validation_unavailable", "I couldn't check whether these tests match your rules. Your request is saved and your previous tests are unchanged.")
		}
		validation.Model = o.Models.Assistant
		validation.ProfileHash = Hash(raw(profile))
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
			return r.completeReliableDocument(ctx, o, p, validationQuestion(validation), nil, nil, AuthoringCompletion{Outcome: &CompletionReceipt{Action: "clarify"}})
		}
		if repaired || p.Conversation.Manual != nil {
			return fault("test_policy_conflict", "The proposed tests conflict with the supplied rules. Your previous tests are unchanged. Please review the expected answers or clarify the rule.")
		}
		repaired = true
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
		resp, e = r.reliableCall(ctx, o, "repair", messages, reliableCommandFormat(profile, repairPlan, "edit_tests"))
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
		if e = r.journalReliable(ctx, o, "repair", "repair", p, candidate.Blueprint, err); e != nil {
			return e
		}
		if err != nil {
			return fault("invalid_repair", "I couldn't finish correcting these tests. Your previous tests are unchanged.")
		}
		commandHash = Hash(raw(map[string]any{"original": commandHash, "patch": patch}))
		candidate.Validation = nil
	}
	candidate.PolicyID = &policy.ID
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

func (r *Runner) reliableProfile(o Operation, p Plan) (ModelProfile, error) {
	if p.Conversation != nil && p.Conversation.Profile != nil {
		profile := *p.Conversation.Profile
		if profile.ID != o.Models.Assistant {
			return ModelProfile{}, fault("invalid_plan", "The saved model profile does not match this request.")
		}
		return profile, nil
	}
	return r.Gateway.Config.Profile(o.Models.Assistant)
}

func ungroundedReview(v *SuiteValidation) bool {
	if v == nil {
		return true
	}
	if v.Consistency != nil {
		for _, finding := range v.Consistency.Findings {
			if finding.Code != "asks_present_field" {
				return true
			}
		}
	}
	return false
}

func (r *Runner) buildReliableCandidate(output []byte, intent string, o Operation, p Plan) (*Artifact, PolicySnapshot, int, error) {
	var policy PolicySnapshot
	var candidate Artifact
	var rules []PolicyRule
	changed := 0
	switch intent {
	case "prepare_tests":
		var cmd createSuiteCommand
		if err := Decode(output, p.limits(), &cmd); err != nil {
			return nil, policy, 0, err
		}
		if len(cmd.Tests.Scenarios) != p.Conversation.RequiredCount || strings.TrimSpace(cmd.Tests.Summary) == "" || len(cmd.Tests.Summary) > 360 {
			return nil, policy, 0, fmt.Errorf("provide the requested number of cases and a short summary")
		}
		if p.precise() && p.Conversation.Policy != nil && effectiveConversationState(p).Brief.ScopeID == p.Conversation.Policy.ScopeID.String() {
			seen := map[string]bool{}
			for _, rule := range cmd.Rules {
				seen[rule.ID] = true
			}
			for _, rule := range p.Conversation.Policy.Rules {
				if !seen[rule.ID] {
					cmd.Rules = append(cmd.Rules, rule)
				}
			}
		}
		criteria := cmd.Tests.SuccessCriteria
		if p.sourceBoundary() {
			criteria = policyGradingCriteria(cmd.Rules)
		}
		proposal := DraftProposal{TestsOnly: true, Title: cmd.Tests.Title, Summary: cmd.Tests.Summary, Scenarios: cmd.Tests.Scenarios, SuccessCriteria: criteria}
		bp, err := r.Service.Compiler.Draft(proposal, p.limits())
		if err != nil {
			return nil, policy, 0, err
		}
		candidate = Artifact{Kind: "test_suite", Title: cmd.Tests.Title, Summary: cmd.Tests.Summary, Blueprint: bp, Proposal: &proposal}
		rules = cmd.Rules
		if p.Artifact != nil {
			candidate.AgentPrompt = p.Artifact.AgentPrompt
			candidate.ParentID = &p.Artifact.ID
		}
	case "edit_tests":
		cmd, err := decodeEditCommand(output, p)
		if err != nil {
			return nil, policy, 0, err
		}
		if p.Artifact == nil || len(cmd.CaseChanges) > p.limits().Cases {
			return nil, policy, 0, fmt.Errorf("select existing tests and bounded case changes")
		}
		if p.sourceBoundary() {
			for i := range cmd.CaseChanges {
				if cmd.CaseChanges[i].Action == "add" {
					// IDs are assigned deterministically by the server. A model's
					// guessed key must neither overwrite a case nor waste a repair.
					cmd.CaseChanges[i].CaseKey = ""
					if !hasCurrentExampleEvidence(cmd.Rules, p) {
						return nil, policy, 0, fmt.Errorf("include the requested addition as a case-specific rule with kind=example evidence from the current request; preserve existing business rules")
					}
				}
			}
			criteria := policyGradingCriteria(cmd.Rules)
			cmd.Criteria = &criteria
		}
		bp, err := patchTestSuite(p.Artifact.Blueprint, cmd.CaseChanges, cmd.Criteria, p.limits(), o.ID)
		if err != nil {
			return nil, policy, 0, err
		}
		candidate = *p.Artifact
		candidate.ParentID = &p.Artifact.ID
		candidate.Blueprint = bp
		candidate.Proposal = nil
		candidate.Validation = nil
		if p.sourceBoundary() {
			// Edits do not rewrite the generated summary. Its old counts and
			// policy claims may now be wrong; the UI already summarizes the
			// actual case count and exposes the current reviewed rules.
			candidate.Summary = ""
		}
		rules = cmd.Rules
		changed = countChangedCases(p.Artifact.Blueprint, bp)
	case "suggest_fix":
		var cmd fixInstructionsCommand
		if err := Decode(output, p.limits(), &cmd); err != nil {
			return nil, policy, 0, err
		}
		if p.ObservedArtifact == nil || !hasBehaviorFailure(p.Observations) {
			return nil, policy, 0, fmt.Errorf("a fix requires the original tested instructions and a behavioral failure")
		}
		prompt, err := applyInstructionEdits(p.ObservedArtifact.AgentPrompt, cmd.InstructionEdits, p.limits())
		if err != nil {
			return nil, policy, 0, err
		}
		candidate = *p.ObservedArtifact
		candidate.AgentPrompt = prompt
		candidate.ParentID = &p.ObservedArtifact.ID
	default:
		return nil, policy, 0, fmt.Errorf("unsupported mutation")
	}
	candidate.ID = deterministicID(o.ID, "artifact")
	candidate.SourceMessageID = p.sourceMessageID()
	candidate.CreatedAt = operationTime(o)
	candidate.Accepted = false
	candidate.Dismissed = false
	candidate.ProposalMessageID = nil
	if _, err := r.Service.Compiler.Compile(candidate.Blueprint, o.Models.Evaluator, candidate.ID, p.limits()); err != nil {
		return nil, policy, 0, err
	}
	if intent != "suggest_fix" {
		var err error
		policy, err = reconcilePolicy(rules, p, o, intent == "prepare_tests")
		if err != nil {
			return nil, policy, 0, err
		}
	}
	return &candidate, policy, changed, nil
}

func suiteCaseCount(blueprint json.RawMessage) int {
	var suite struct {
		Cases []json.RawMessage `json:"cases"`
	}
	_ = json.Unmarshal(blueprint, &suite)
	return len(suite.Cases)
}
func countChangedCases(before, after json.RawMessage) int {
	rows := func(bp json.RawMessage) map[string]string {
		var s struct {
			Cases []json.RawMessage `json:"cases"`
		}
		_ = json.Unmarshal(bp, &s)
		m := map[string]string{}
		for _, c := range s.Cases {
			var k struct {
				Key string `json:"key"`
			}
			_ = json.Unmarshal(c, &k)
			m[k.Key], _ = CanonicalJSONHash(c)
		}
		return m
	}
	a, b := rows(before), rows(after)
	n := 0
	for k, v := range a {
		if b[k] != v {
			n++
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			n++
		}
	}
	return n
}
func checkScopedRepair(patch editSuiteCommand, v SuiteValidation, policy PolicySnapshot) error {
	allowed := map[string]bool{}
	for _, c := range v.Cases {
		if c.Status != SuiteSupported {
			allowed[c.CaseKey] = true
		}
	}
	for _, change := range patch.CaseChanges {
		if change.Action != "update" || !allowed[change.CaseKey] {
			return fmt.Errorf("repair may only update rejected existing cases")
		}
	}
	if patch.Criteria != nil && v.SharedCriteria.Status == SuiteSupported {
		return fmt.Errorf("repair changed supported shared rules")
	}
	// A missing-rule finding permits adding that rule, not rewriting other
	// rules that the reviewer explicitly found supported.
	proposed := map[string]PolicyRule{}
	for _, rule := range patch.Rules {
		proposed[rule.ID] = rule
	}
	supportedRules := map[string]bool{}
	for _, rule := range v.Rules {
		if rule.Status == SuiteSupported {
			supportedRules[rule.RuleID] = true
		}
	}
	for _, rule := range policy.Rules {
		if supportedRules[rule.ID] && Hash(raw(proposed[rule.ID])) != Hash(raw(rule)) {
			return fmt.Errorf("repair changed a supported rule")
		}
	}
	if v.PolicyReconciliation.Status == SuiteSupported {
		allowedRules := map[string]bool{}
		for _, r := range v.Rules {
			if r.Status != SuiteSupported {
				allowedRules[r.RuleID] = true
			}
		}
		old := map[string]PolicyRule{}
		for _, rule := range policy.Rules {
			old[rule.ID] = rule
		}
		if len(patch.Rules) != len(policy.Rules) {
			return fmt.Errorf("repair changed unrelated rule coverage")
		}
		for _, rule := range patch.Rules {
			prior, ok := old[rule.ID]
			if !ok || !allowedRules[rule.ID] && Hash(raw(prior)) != Hash(raw(rule)) {
				return fmt.Errorf("repair changed a supported rule")
			}
		}
	}
	return nil
}
func validationQuestion(v *SuiteValidation) string {
	for _, p := range v.Problems {
		if p.Status == SuiteUnclear {
			return "One rule needs clarification before these tests are ready: " + boundedDiagnostic(p.Reason)
		}
	}
	return "What should a correct reply do in the situation that is still unclear? Your previous tests are unchanged."
}
