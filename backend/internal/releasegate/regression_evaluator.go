package releasegate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const (
	regressionRuleNoBlockingFailure    = "no_blocking_regression_failure"
	regressionRuleNoNewBlockingFailure = "no_new_blocking_failure_vs_baseline"
	regressionRuleMaxWarningFailures   = "max_warning_regression_failures"
)

type RegressionReplayStepRef struct {
	SequenceNumber int64  `json:"sequence_number"`
	EventType      string `json:"event_type,omitempty"`
	Kind           string `json:"kind,omitempty"`
}

type RegressionEvidenceRef struct {
	ScoringResultID   uuid.UUID                 `json:"scoring_result_id"`
	ScoringResultType string                    `json:"scoring_result_type"`
	ReplayStepRefs    []RegressionReplayStepRef `json:"replay_step_refs,omitempty"`
}

type RegressionGateViolation struct {
	Rule             string                `json:"rule"`
	Severity         string                `json:"severity"`
	RegressionCaseID uuid.UUID             `json:"regression_case_id"`
	SuiteID          uuid.UUID             `json:"suite_id"`
	ObservedCount    *int                  `json:"observed_count,omitempty"`
	Evidence         RegressionEvidenceRef `json:"evidence"`
}

type RegressionCaseEvaluation struct {
	RegressionCaseID uuid.UUID
	SuiteID          uuid.UUID
	Severity         string
	Failed           bool
	Evidence         *RegressionEvidenceRef
}

type RegressionGateOutcome struct {
	Verdict             Verdict
	ReasonCode          string
	Summary             string
	Warnings            []string
	TriggeredConditions []string
	Violations          []RegressionGateViolation
}

func EvaluateRegressionGateRules(candidate, baseline []RegressionCaseEvaluation, rules *RegressionGateRules) RegressionGateOutcome {
	if rules == nil {
		return RegressionGateOutcome{Verdict: VerdictPass}
	}

	inScopeCandidate := filterRegressionCases(candidate, rules.SuiteIDs)
	inScopeBaseline := filterRegressionCases(baseline, rules.SuiteIDs)

	outcome := RegressionGateOutcome{Verdict: VerdictPass}
	failViolations := make([]RegressionGateViolation, 0)
	warnViolations := make([]RegressionGateViolation, 0)
	conditions := make([]string, 0)

	candidateByCaseID := make(map[uuid.UUID]RegressionCaseEvaluation, len(inScopeCandidate))
	for _, item := range inScopeCandidate {
		candidateByCaseID[item.RegressionCaseID] = item
	}
	baselineByCaseID := make(map[uuid.UUID]RegressionCaseEvaluation, len(inScopeBaseline))
	for _, item := range inScopeBaseline {
		baselineByCaseID[item.RegressionCaseID] = item
	}

	if rules.NoBlockingRegressionFailure {
		for _, item := range inScopeCandidate {
			if item.Severity != "blocking" || !item.Failed || item.Evidence == nil {
				continue
			}
			failViolations = append(failViolations, RegressionGateViolation{
				Rule:             regressionRuleNoBlockingFailure,
				Severity:         item.Severity,
				RegressionCaseID: item.RegressionCaseID,
				SuiteID:          item.SuiteID,
				Evidence:         cloneRegressionEvidence(*item.Evidence),
			})
			conditions = append(conditions, fmt.Sprintf("%s:%s", regressionRuleNoBlockingFailure, item.RegressionCaseID))
		}
	}

	if rules.NoNewBlockingFailureVsBaseline {
		missingBaselineEvidence := 0
		for _, item := range inScopeCandidate {
			if item.Severity != "blocking" || !item.Failed || item.Evidence == nil {
				continue
			}
			baselineItem, ok := baselineByCaseID[item.RegressionCaseID]
			if !ok {
				missingBaselineEvidence++
				continue
			}
			if !baselineItem.Failed {
				failViolations = append(failViolations, RegressionGateViolation{
					Rule:             regressionRuleNoNewBlockingFailure,
					Severity:         item.Severity,
					RegressionCaseID: item.RegressionCaseID,
					SuiteID:          item.SuiteID,
					Evidence:         cloneRegressionEvidence(*item.Evidence),
				})
				conditions = append(conditions, fmt.Sprintf("%s:%s", regressionRuleNoNewBlockingFailure, item.RegressionCaseID))
			}
		}
		if missingBaselineEvidence > 0 {
			outcome.Warnings = append(outcome.Warnings, fmt.Sprintf(
				"baseline regression evidence unavailable for %d blocking case(s); skipped %s",
				missingBaselineEvidence,
				regressionRuleNoNewBlockingFailure,
			))
		}
	}

	if rules.MaxWarningRegressionFailures != nil {
		failedWarnings := make([]RegressionCaseEvaluation, 0)
		for _, item := range inScopeCandidate {
			if item.Severity == "warning" && item.Failed && item.Evidence != nil {
				failedWarnings = append(failedWarnings, item)
			}
		}
		if len(failedWarnings) > *rules.MaxWarningRegressionFailures {
			count := len(failedWarnings)
			for _, item := range failedWarnings {
				observedCount := count
				warnViolations = append(warnViolations, RegressionGateViolation{
					Rule:             regressionRuleMaxWarningFailures,
					Severity:         item.Severity,
					RegressionCaseID: item.RegressionCaseID,
					SuiteID:          item.SuiteID,
					ObservedCount:    &observedCount,
					Evidence:         cloneRegressionEvidence(*item.Evidence),
				})
				conditions = append(conditions, fmt.Sprintf("%s:%s", regressionRuleMaxWarningFailures, item.RegressionCaseID))
			}
		}
	}

	sortRegressionViolations(failViolations)
	sortRegressionViolations(warnViolations)
	sort.Strings(conditions)

	outcome.TriggeredConditions = conditions
	switch {
	case len(failViolations) > 0:
		outcome.Verdict = VerdictFail
		outcome.ReasonCode = reasonCodeForRegressionRule(failViolations[0].Rule)
		outcome.Summary = "release gate failed because regression cases violated policy"
		outcome.Violations = append(outcome.Violations, failViolations...)
		outcome.Violations = append(outcome.Violations, warnViolations...)
	case len(warnViolations) > 0:
		outcome.Verdict = VerdictWarn
		outcome.ReasonCode = reasonCodeForRegressionRule(warnViolations[0].Rule)
		outcome.Summary = "release gate produced warnings because warning-severity regression failures exceeded the configured limit"
		outcome.Violations = append(outcome.Violations, warnViolations...)
	default:
		outcome.Violations = nil
	}

	return outcome
}

func MergeEvaluation(base Evaluation, regression RegressionGateOutcome) Evaluation {
	merged := base
	merged.Details.Warnings = dedupeStringsPreserveOrder(append(merged.Details.Warnings, regression.Warnings...))
	merged.Details.TriggeredConditions = uniqueSortedStrings(append(merged.Details.TriggeredConditions, regression.TriggeredConditions...))
	merged.Details.RegressionViolations = append(append([]RegressionGateViolation(nil), merged.Details.RegressionViolations...), regression.Violations...)

	if base.Verdict == VerdictInsufficientEvidence {
		return merged
	}
	if regression.Verdict == VerdictInsufficientEvidence {
		if base.Verdict == VerdictFail {
			return merged
		}
		merged.Verdict = VerdictInsufficientEvidence
		merged.ReasonCode = regression.ReasonCode
		merged.Summary = regression.Summary
		merged.EvidenceStatus = EvidenceStatusInsufficient
		return merged
	}
	if regression.Verdict == VerdictFail {
		if base.Verdict == VerdictFail {
			return merged
		}
		merged.Verdict = VerdictFail
		merged.ReasonCode = regression.ReasonCode
		merged.Summary = regression.Summary
		return merged
	}
	if regression.Verdict == VerdictWarn && base.Verdict == VerdictPass {
		merged.Verdict = VerdictWarn
		merged.ReasonCode = regression.ReasonCode
		merged.Summary = regression.Summary
	}
	return merged
}

func dedupeStringsPreserveOrder(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func filterRegressionCases(items []RegressionCaseEvaluation, suiteIDs []string) []RegressionCaseEvaluation {
	if len(suiteIDs) == 0 {
		return append([]RegressionCaseEvaluation(nil), items...)
	}

	allowed := make(map[string]struct{}, len(suiteIDs))
	for _, suiteID := range suiteIDs {
		allowed[suiteID] = struct{}{}
	}

	filtered := make([]RegressionCaseEvaluation, 0, len(items))
	for _, item := range items {
		if _, ok := allowed[item.SuiteID.String()]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func sortRegressionViolations(items []RegressionGateViolation) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Rule != items[j].Rule {
			return items[i].Rule < items[j].Rule
		}
		if items[i].Severity != items[j].Severity {
			return items[i].Severity < items[j].Severity
		}
		return items[i].RegressionCaseID.String() < items[j].RegressionCaseID.String()
	})
}

func reasonCodeForRegressionRule(rule string) string {
	switch rule {
	case regressionRuleNoBlockingFailure:
		return "regression_blocking_failure"
	case regressionRuleNoNewBlockingFailure:
		return "regression_new_blocking_failure"
	case regressionRuleMaxWarningFailures:
		return "regression_warning_threshold_exceeded"
	default:
		return "regression_policy_violation"
	}
}

func cloneRegressionEvidence(input RegressionEvidenceRef) RegressionEvidenceRef {
	cloned := RegressionEvidenceRef{
		ScoringResultID:   input.ScoringResultID,
		ScoringResultType: input.ScoringResultType,
	}
	if len(input.ReplayStepRefs) > 0 {
		cloned.ReplayStepRefs = append([]RegressionReplayStepRef(nil), input.ReplayStepRefs...)
	}
	return cloned
}
