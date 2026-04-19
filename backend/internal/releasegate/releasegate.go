package releasegate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

type Verdict string

const (
	VerdictPass                 Verdict = "pass"
	VerdictWarn                 Verdict = "warn"
	VerdictFail                 Verdict = "fail"
	VerdictInsufficientEvidence Verdict = "insufficient_evidence"
)

type EvidenceStatus string

const (
	EvidenceStatusSufficient   EvidenceStatus = "sufficient"
	EvidenceStatusInsufficient EvidenceStatus = "insufficient"
)

type Policy struct {
	PolicyKey                   string `json:"policy_key"`
	PolicyVersion               int    `json:"policy_version"`
	RequireComparable           bool   `json:"require_comparable"`
	RequireEvidenceQuality      bool   `json:"require_evidence_quality"`
	FailOnCandidateFailure      bool   `json:"fail_on_candidate_failure"`
	FailOnBothFailedDifferently bool   `json:"fail_on_both_failed_differently"`
	// RequireScorecardPass demands that the candidate's scorecard-level
	// pass verdict (carried by ComparisonSummary.ScorecardPass.Candidate)
	// is explicitly true. A false verdict produces VerdictFail; a nil
	// verdict — e.g. a legacy row that never persisted Passed — produces
	// VerdictInsufficientEvidence so operators can fix the spec instead of
	// silently shipping a regression.
	//
	// omitempty is load-bearing: the serialized policy feeds a SHA-256
	// fingerprint that uniquely identifies a gate row in the DB. Emitting
	// "require_scorecard_pass": false for every legacy policy would change
	// their fingerprints and orphan every persisted gate row. With
	// omitempty, pre-Phase-5 policies serialize identically and their
	// fingerprints stay stable.
	RequireScorecardPass bool                          `json:"require_scorecard_pass,omitempty"`
	RequiredDimensions   []string                      `json:"required_dimensions,omitempty"`
	Dimensions           map[string]DimensionThreshold `json:"dimensions,omitempty"`
	RegressionGateRules  *RegressionGateRules          `json:"regression_gate_rules,omitempty"`
}

type DimensionThreshold struct {
	WarnDelta *float64 `json:"warn_delta,omitempty"`
	FailDelta *float64 `json:"fail_delta,omitempty"`
}

type RegressionGateRules struct {
	NoBlockingRegressionFailure    bool     `json:"no_blocking_regression_failure,omitempty"`
	NoNewBlockingFailureVsBaseline bool     `json:"no_new_blocking_failure_vs_baseline,omitempty"`
	MaxWarningRegressionFailures   *int     `json:"max_warning_regression_failures,omitempty"`
	SuiteIDs                       []string `json:"suite_ids,omitempty"`
}

type ComparisonSummary struct {
	SchemaVersion           string                    `json:"schema_version"`
	Status                  string                    `json:"status"`
	ReasonCode              string                    `json:"reason_code,omitempty"`
	BaselineRefs            ComparisonRunRefs         `json:"baseline_refs"`
	CandidateRefs           ComparisonRunRefs         `json:"candidate_refs"`
	DimensionDeltas         map[string]DimensionDelta `json:"dimension_deltas,omitempty"`
	ScorecardPass           *ScorecardPassSummary     `json:"scorecard_pass,omitempty"`
	FailureDivergence       FailureDivergence         `json:"failure_divergence"`
	ReplaySummaryDivergence ReplayDivergence          `json:"replay_summary_divergence"`
	EvidenceQuality         compareEvidenceQuality    `json:"evidence_quality"`
}

type ComparisonRunRefs struct {
	RunID               uuid.UUID  `json:"run_id"`
	RunAgentID          *uuid.UUID `json:"run_agent_id,omitempty"`
	EvaluationSpecID    *uuid.UUID `json:"evaluation_spec_id,omitempty"`
	ChallengeInputSetID *uuid.UUID `json:"challenge_input_set_id,omitempty"`
}

// ScorecardPassSummary carries the per-agent scorecard pass verdict through
// the release-gate evaluation. Pointer booleans distinguish "unknown"
// (legacy row, partial evaluation) from an explicit false.
type ScorecardPassSummary struct {
	Baseline  *bool `json:"baseline,omitempty"`
	Candidate *bool `json:"candidate,omitempty"`
}

type DimensionDelta struct {
	BaselineValue   *float64 `json:"baseline_value,omitempty"`
	CandidateValue  *float64 `json:"candidate_value,omitempty"`
	Delta           *float64 `json:"delta,omitempty"`
	BetterDirection string   `json:"better_direction"`
	State           string   `json:"state"`
}

type FailureDivergence struct {
	CandidateFailedBaselineOK bool `json:"candidate_failed_baseline_succeeded"`
	BothFailedDifferently     bool `json:"both_failed_differently"`
}

type ReplayDivergence struct {
	State string `json:"state"`
}

type compareEvidenceQuality struct {
	MissingFields []string `json:"missing_fields,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

type Evaluation struct {
	Verdict        Verdict           `json:"verdict"`
	ReasonCode     string            `json:"reason_code"`
	Summary        string            `json:"summary"`
	EvidenceStatus EvidenceStatus    `json:"evidence_status"`
	Details        EvaluationDetails `json:"details"`
}

type EvaluationDetails struct {
	PolicyKey            string                         `json:"policy_key"`
	PolicyVersion        int                            `json:"policy_version"`
	ComparisonStatus     string                         `json:"comparison_status"`
	MissingFields        []string                       `json:"missing_fields,omitempty"`
	Warnings             []string                       `json:"warnings,omitempty"`
	TriggeredConditions  []string                       `json:"triggered_conditions,omitempty"`
	RequiredDimensions   []string                       `json:"required_dimensions,omitempty"`
	DimensionResults     map[string]DimensionEvaluation `json:"dimension_results,omitempty"`
	RegressionViolations []RegressionGateViolation      `json:"regression_violations,omitempty"`
}

type DimensionEvaluation struct {
	State           string   `json:"state"`
	BetterDirection string   `json:"better_direction,omitempty"`
	ObservedDelta   *float64 `json:"observed_delta,omitempty"`
	WorseningDelta  *float64 `json:"worsening_delta,omitempty"`
	WarnThreshold   *float64 `json:"warn_threshold,omitempty"`
	FailThreshold   *float64 `json:"fail_threshold,omitempty"`
	Outcome         string   `json:"outcome"`
}

func DefaultPolicy() Policy {
	return Policy{
		PolicyKey:                   "default",
		PolicyVersion:               1,
		RequireComparable:           true,
		RequireEvidenceQuality:      true,
		FailOnCandidateFailure:      true,
		FailOnBothFailedDifferently: true,
		RequiredDimensions:          []string{"correctness", "reliability", "latency", "cost"},
		Dimensions: map[string]DimensionThreshold{
			"correctness": {WarnDelta: floatPtr(0.02), FailDelta: floatPtr(0.05)},
			"reliability": {WarnDelta: floatPtr(0.02), FailDelta: floatPtr(0.05)},
			"latency":     {WarnDelta: floatPtr(0.05), FailDelta: floatPtr(0.15)},
			"cost":        {WarnDelta: floatPtr(0.10), FailDelta: floatPtr(0.25)},
		},
	}
}

func NormalizePolicy(policy Policy) (Policy, error) {
	if policy.PolicyKey == "" && policy.PolicyVersion == 0 && len(policy.Dimensions) == 0 && len(policy.RequiredDimensions) == 0 &&
		!policy.RequireComparable && !policy.RequireEvidenceQuality && !policy.FailOnCandidateFailure && !policy.FailOnBothFailedDifferently &&
		regressionGateRulesUnset(policy.RegressionGateRules) {
		policy = DefaultPolicy()
	}

	if policy.PolicyKey == "" {
		return Policy{}, errors.New("policy_key is required")
	}
	if policy.PolicyVersion <= 0 {
		return Policy{}, errors.New("policy_version must be greater than zero")
	}
	if len(policy.Dimensions) == 0 {
		return Policy{}, errors.New("at least one dimension threshold is required")
	}

	requiredDimensions := uniqueSortedStrings(policy.RequiredDimensions)
	if len(requiredDimensions) == 0 {
		requiredDimensions = make([]string, 0, len(policy.Dimensions))
		for key := range policy.Dimensions {
			requiredDimensions = append(requiredDimensions, key)
		}
		sort.Strings(requiredDimensions)
	}

	normalizedDimensions := make(map[string]DimensionThreshold, len(policy.Dimensions))
	for key, threshold := range policy.Dimensions {
		key = strings.TrimSpace(strings.ToLower(key))
		if key == "" {
			return Policy{}, errors.New("dimension threshold key is required")
		}
		if threshold.WarnDelta == nil && threshold.FailDelta == nil {
			return Policy{}, fmt.Errorf("dimension %s must declare warn_delta or fail_delta", key)
		}
		if threshold.WarnDelta != nil && *threshold.WarnDelta < 0 {
			return Policy{}, fmt.Errorf("dimension %s warn_delta must be non-negative", key)
		}
		if threshold.FailDelta != nil && *threshold.FailDelta < 0 {
			return Policy{}, fmt.Errorf("dimension %s fail_delta must be non-negative", key)
		}
		if threshold.WarnDelta != nil && threshold.FailDelta != nil && *threshold.WarnDelta > *threshold.FailDelta {
			return Policy{}, fmt.Errorf("dimension %s warn_delta must be less than or equal to fail_delta", key)
		}
		normalizedDimensions[key] = threshold
	}

	for _, dimension := range requiredDimensions {
		if _, ok := normalizedDimensions[dimension]; !ok {
			return Policy{}, fmt.Errorf("required dimension %s is missing a threshold declaration", dimension)
		}
	}

	normalizedRegressionRules, err := normalizeRegressionGateRules(policy.RegressionGateRules)
	if err != nil {
		return Policy{}, err
	}

	policy.PolicyKey = strings.TrimSpace(policy.PolicyKey)
	policy.RequiredDimensions = requiredDimensions
	policy.Dimensions = normalizedDimensions
	policy.RegressionGateRules = normalizedRegressionRules
	return policy, nil
}

func normalizeRegressionGateRules(rules *RegressionGateRules) (*RegressionGateRules, error) {
	if rules == nil {
		return nil, nil
	}

	normalized := &RegressionGateRules{
		NoBlockingRegressionFailure:    rules.NoBlockingRegressionFailure,
		NoNewBlockingFailureVsBaseline: rules.NoNewBlockingFailureVsBaseline,
		SuiteIDs:                       uniqueSortedStrings(rules.SuiteIDs),
	}
	if rules.MaxWarningRegressionFailures != nil {
		if *rules.MaxWarningRegressionFailures < 0 {
			return nil, errors.New("regression_gate_rules.max_warning_regression_failures must be non-negative")
		}
		value := *rules.MaxWarningRegressionFailures
		normalized.MaxWarningRegressionFailures = &value
	}

	if !normalized.NoBlockingRegressionFailure &&
		!normalized.NoNewBlockingFailureVsBaseline &&
		normalized.MaxWarningRegressionFailures == nil &&
		len(normalized.SuiteIDs) == 0 {
		return nil, nil
	}

	return normalized, nil
}

func regressionGateRulesUnset(rules *RegressionGateRules) bool {
	if rules == nil {
		return true
	}
	hasSuiteID := false
	for _, suiteID := range rules.SuiteIDs {
		if strings.TrimSpace(suiteID) != "" {
			hasSuiteID = true
			break
		}
	}
	return !rules.NoBlockingRegressionFailure &&
		!rules.NoNewBlockingFailureVsBaseline &&
		rules.MaxWarningRegressionFailures == nil &&
		!hasSuiteID
}

func PolicySnapshot(policy Policy) (json.RawMessage, string, error) {
	normalized, err := NormalizePolicy(policy)
	if err != nil {
		return nil, "", err
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(sum[:]), nil
}

func DecodeComparisonSummary(raw json.RawMessage) (ComparisonSummary, error) {
	var summary ComparisonSummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		return ComparisonSummary{}, err
	}
	return summary, nil
}

func Evaluate(summary ComparisonSummary, policy Policy) (Evaluation, error) {
	normalized, err := NormalizePolicy(policy)
	if err != nil {
		return Evaluation{}, err
	}

	details := EvaluationDetails{
		PolicyKey:          normalized.PolicyKey,
		PolicyVersion:      normalized.PolicyVersion,
		ComparisonStatus:   summary.Status,
		MissingFields:      append([]string(nil), summary.EvidenceQuality.MissingFields...),
		Warnings:           append([]string(nil), summary.EvidenceQuality.Warnings...),
		RequiredDimensions: append([]string(nil), normalized.RequiredDimensions...),
		DimensionResults:   make(map[string]DimensionEvaluation, len(normalized.Dimensions)),
	}

	if normalized.RequireComparable && summary.Status != "comparable" {
		details.TriggeredConditions = []string{"comparison_not_comparable"}
		return Evaluation{
			Verdict:        VerdictInsufficientEvidence,
			ReasonCode:     "comparison_not_comparable",
			Summary:        buildInsufficientSummary("comparison is not comparable", summary.ReasonCode, details.MissingFields),
			EvidenceStatus: EvidenceStatusInsufficient,
			Details:        details,
		}, nil
	}

	if normalized.RequireEvidenceQuality && len(details.MissingFields) > 0 {
		details.TriggeredConditions = []string{"comparison_evidence_missing"}
		return Evaluation{
			Verdict:        VerdictInsufficientEvidence,
			ReasonCode:     "comparison_evidence_missing",
			Summary:        buildInsufficientSummary("comparison evidence is incomplete", "", details.MissingFields),
			EvidenceStatus: EvidenceStatusInsufficient,
			Details:        details,
		}, nil
	}

	missingRequired := make([]string, 0, len(normalized.RequiredDimensions))
	for _, dimension := range normalized.RequiredDimensions {
		delta, ok := summary.DimensionDeltas[dimension]
		if !ok || delta.State != "available" || delta.Delta == nil {
			missingRequired = append(missingRequired, dimension)
			details.DimensionResults[dimension] = DimensionEvaluation{
				State:   delta.State,
				Outcome: string(VerdictInsufficientEvidence),
			}
		}
	}
	if len(missingRequired) > 0 {
		triggers := make([]string, 0, len(missingRequired))
		for _, dimension := range missingRequired {
			triggers = append(triggers, "required_dimension_unavailable:"+dimension)
		}
		details.TriggeredConditions = triggers
		return Evaluation{
			Verdict:        VerdictInsufficientEvidence,
			ReasonCode:     "required_dimension_unavailable",
			Summary:        fmt.Sprintf("required comparison dimensions unavailable: %s", strings.Join(missingRequired, ", ")),
			EvidenceStatus: EvidenceStatusInsufficient,
			Details:        details,
		}, nil
	}

	if normalized.RequireScorecardPass {
		candidatePass := scorecardPassValue(summary.ScorecardPass, true)
		if candidatePass == nil {
			details.TriggeredConditions = append(details.TriggeredConditions, "scorecard_pass_unknown")
			return Evaluation{
				Verdict:        VerdictInsufficientEvidence,
				ReasonCode:     "scorecard_pass_unknown",
				Summary:        "candidate scorecard pass verdict is unknown",
				EvidenceStatus: EvidenceStatusInsufficient,
				Details:        details,
			}, nil
		}
		if !*candidatePass {
			details.TriggeredConditions = append(details.TriggeredConditions, "scorecard_not_passed")
			return Evaluation{
				Verdict:        VerdictFail,
				ReasonCode:     "scorecard_not_passed",
				Summary:        "release gate failed because candidate scorecard did not pass",
				EvidenceStatus: EvidenceStatusSufficient,
				Details:        details,
			}, nil
		}
	}

	failReasons := make([]string, 0, 4)
	warnReasons := make([]string, 0, 4)

	if normalized.FailOnCandidateFailure && summary.FailureDivergence.CandidateFailedBaselineOK {
		failReasons = append(failReasons, "candidate_failed_baseline_succeeded")
	}
	if normalized.FailOnBothFailedDifferently && summary.FailureDivergence.BothFailedDifferently {
		failReasons = append(failReasons, "both_failed_differently")
	}

	for dimension, threshold := range normalized.Dimensions {
		delta := summary.DimensionDeltas[dimension]
		result := DimensionEvaluation{
			State:           delta.State,
			BetterDirection: delta.BetterDirection,
			ObservedDelta:   delta.Delta,
			WarnThreshold:   threshold.WarnDelta,
			FailThreshold:   threshold.FailDelta,
			Outcome:         string(VerdictPass),
		}
		worsening := worseningDelta(delta)
		if worsening != nil {
			result.WorseningDelta = worsening
			switch {
			// Thresholds are inclusive: a worsening delta exactly equal to the policy
			// threshold is considered a boundary breach and triggers the verdict.
			case threshold.FailDelta != nil && *worsening >= *threshold.FailDelta:
				result.Outcome = string(VerdictFail)
				failReasons = append(failReasons, "threshold_fail_"+dimension)
			case threshold.WarnDelta != nil && *worsening >= *threshold.WarnDelta:
				result.Outcome = string(VerdictWarn)
				warnReasons = append(warnReasons, "threshold_warn_"+dimension)
			}
		}
		details.DimensionResults[dimension] = result
	}

	sort.Strings(failReasons)
	sort.Strings(warnReasons)
	details.TriggeredConditions = append(details.TriggeredConditions, failReasons...)
	details.TriggeredConditions = append(details.TriggeredConditions, warnReasons...)

	if len(failReasons) > 0 {
		return Evaluation{
			Verdict:        VerdictFail,
			ReasonCode:     failReasons[0],
			Summary:        "release gate failed because fail conditions were triggered",
			EvidenceStatus: EvidenceStatusSufficient,
			Details:        details,
		}, nil
	}
	if len(warnReasons) > 0 {
		return Evaluation{
			Verdict:        VerdictWarn,
			ReasonCode:     warnReasons[0],
			Summary:        "release gate produced warnings because thresholds were crossed",
			EvidenceStatus: EvidenceStatusSufficient,
			Details:        details,
		}, nil
	}
	return Evaluation{
		Verdict:        VerdictPass,
		ReasonCode:     "within_thresholds",
		Summary:        "release gate passed because all thresholds stayed within policy limits",
		EvidenceStatus: EvidenceStatusSufficient,
		Details:        details,
	}, nil
}

// scorecardPassValue returns a pointer-wrapped copy of the requested side of
// the scorecard pass summary, or nil when the side is unknown. Copying
// avoids aliasing the caller's ComparisonSummary so callers can't scribble
// on the decoded payload.
func scorecardPassValue(summary *ScorecardPassSummary, candidate bool) *bool {
	if summary == nil {
		return nil
	}
	source := summary.Baseline
	if candidate {
		source = summary.Candidate
	}
	if source == nil {
		return nil
	}
	value := *source
	return &value
}

func worseningDelta(delta DimensionDelta) *float64 {
	if delta.Delta == nil {
		return nil
	}
	value := *delta.Delta
	switch delta.BetterDirection {
	case "higher":
		value = -value
	case "lower":
	default:
		return nil
	}
	return &value
}

func buildInsufficientSummary(prefix string, comparisonReason string, missing []string) string {
	parts := []string{prefix}
	if comparisonReason != "" {
		parts = append(parts, comparisonReason)
	}
	if len(missing) > 0 {
		parts = append(parts, "missing: "+strings.Join(uniqueSortedStrings(missing), ", "))
	}
	return strings.Join(parts, "; ")
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func floatPtr(value float64) *float64 {
	return &value
}
