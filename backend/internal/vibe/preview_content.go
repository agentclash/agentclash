package vibe

import (
	"fmt"
	"regexp"
	"strings"
)

// This is a conservative authoring check for explicit action promises, not a
// semantic safety classifier. Examples stay untouched: they may intentionally
// contain these requests. The capability policy and human review still apply.
var actionPromises = regexp.MustCompile(`(?i)\b(confirm (?:the |their |your |a |an )?(?:booking|appointment|reservation)|will confirm (?:it|this|the booking) later|(?:escalate|transfer|connect) .{0,60}(?:immediately|to a human|to an? (?:agent|person|operator))|(?:book|schedule) (?:the |their |your |an? )(?:appointment|reservation)|(?:will|shall) (?:call|contact|notify|email|follow up)|send (?:an? |the )?(?:confirmation|notification|email))\b`)
var negation = regexp.MustCompile(`(?i)\b(?:not|never|cannot|can't|unable|unavailable|no|without|simulated|hypothetical)\b`)
var confidenceQuota = regexp.MustCompile(`(?i)(?:at least|minimum of)\s+(?:one|two|three|\d+).{0,70}high[ -]confidence`)
var evidenceQuota = regexp.MustCompile(`(?i)\b(?:at least|minimum of)\s+(?:one|two|three|\d+)\s+(?:qualifying\s+)?(?:companies|signals|recommendations|leads|prospects)\b`)
var audioCriteria = regexp.MustCompile(`(?i)\b(?:natural pacing|dead air|call latency|acoustic interruption|speech recognition accuracy)\b`)
var unavailableCriterion = regexp.MustCompile(`(?i)\b(?:(?:cannot|can't|do not|never) (?:measure|test|assess|evaluate)|unavailable|out of scope|not measured|no quota|no minimum)\b`)

const previewEvidencePolicy = "Evidence rules: use supplied facts only. With insufficient or contradictory evidence, asking for clarification or returning no qualifying recommendation is valid. Never require invented facts or external actions. Apply additional criteria only when supported by the supplied evidence.\n\n"

func PreviewCriteria(criteria string) string {
	if strings.HasPrefix(criteria, previewEvidencePolicy) {
		return criteria
	}
	return previewEvidencePolicy + criteria
}

func validatePreviewText(text string) error {
	for _, sentence := range strings.FieldsFunc(text, func(r rune) bool { return r == '.' || r == '\n' || r == ';' || r == '!' }) {
		if actionPromises.MatchString(sentence) && !negation.MatchString(sentence) {
			return fmt.Errorf("unconnected preview promises an external action; state that it is unavailable or explicitly simulated, without a future follow-up promise")
		}
	}
	return nil
}

func validatePreviewProposal(a assistantReply) error {
	if a.Draft == nil {
		return nil
	}
	for _, text := range []string{a.Reply, a.Draft.AgentPrompt, a.Draft.SuccessCriteria} {
		if err := validatePreviewText(text); err != nil {
			return err
		}
	}
	for _, q := range a.Changes {
		if q.Action != "remove" {
			if err := validatePreviewText(q.Statement); err != nil {
				return err
			}
		}
	}
	return ValidatePreviewCriteria(a.Draft.SuccessCriteria)
}

// Manual criterion edits create new contracts but cannot grant capabilities.
func ValidatePreviewCriteria(criteria string) error {
	criteria = strings.TrimPrefix(criteria, previewEvidencePolicy)
	for _, sentence := range strings.FieldsFunc(criteria, func(r rune) bool { return r == '.' || r == '\n' || r == ';' }) {
		if unavailableCriterion.MatchString(sentence) {
			continue
		}
		text := normalizedRequirement(sentence)
		if confidenceQuota.MatchString(text) || evidenceQuota.MatchString(text) {
			return fmt.Errorf("remove the mandatory recommendation/confidence quota; confidence follows evidence and no qualifying recommendation must be allowed")
		}
		if audioCriteria.MatchString(text) {
			return fmt.Errorf("text previews cannot measure audio or telephony; put these checks in a test_plan with the required evidence")
		}
	}
	return validatePreviewText(criteria)
}
