package releasegate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
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
	PolicyKey                   string                        `json:"policy_key"`
	PolicyVersion               int                           `json:"policy_version"`
	RequireComparable           bool                          `json:"require_comparable"`
	RequireEvidenceQuality      bool                          `json:"require_evidence_quality"`
	FailOnCandidateFailure      bool                          `json:"fail_on_candidate_failure"`
	FailOnBothFailedDifferently bool                          `json:"fail_on_both_failed_differently"`
	RequiredDimensions          []string                      `json:"required_dimensions,omitempty"`
	Dimensions                  map[string]DimensionThreshold `json:"dimensions,omitempty"`
}

type DimensionThreshold struct {
	WarnDelta *float64 `json:"warn_delta,omitempty"`
	FailDelta *float64 `json:"fail_delta,omitempty"`
}

type ComparisonSummary struct {
	SchemaVersion           string                    `json:"schema_version"`
	Status                  string                    `json:"status"`
	ReasonCode              string                    `json:"reason_code,omitempty"`
	DimensionDeltas         map[string]DimensionDelta `json:"dimension_deltas,omitempty"`
	FailureDivergence       FailureDivergence         `json:"failure_divergence"`
	ReplaySummaryDivergence ReplayDivergence          `json:"replay_summary_divergence"`
	EvidenceQuality         compareEvidenceQuality    `json:"evidence_quality"`
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
	PolicyKey           string                         `json:"policy_key"`
	PolicyVersion       int                            `json:"policy_version"`
	ComparisonStatus    string                         `json:"comparison_status"`
	MissingFields       []string                       `json:"missing_fields,omitempty"`
	Warnings            []string                       `json:"warnings,omitempty"`
	TriggeredConditions []string                       `json:"triggered_conditions,omitempty"`
	RequiredDimensions  []string                       `json:"required_dimensions,omitempty"`
	DimensionResults    map[string]DimensionEvaluation `json:"dimension_results,omitempty"`
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
		!policy.RequireComparable && !policy.RequireEvidenceQuality && !policy.FailOnCandidateFailure && !policy.FailOnBothFailedDifferently {
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

	policy.PolicyKey = strings.TrimSpace(policy.PolicyKey)
	policy.RequiredDimensions = requiredDimensions
	policy.Dimensions = normalizedDimensions
	return policy, nil
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

	for _, dimension := range normalized.RequiredDimensions {
		delta, ok := summary.DimensionDeltas[dimension]
		if !ok || delta.State != "available" || delta.Delta == nil {
			details.TriggeredConditions = []string{"required_dimension_unavailable"}
			details.DimensionResults[dimension] = DimensionEvaluation{
				State:   delta.State,
				Outcome: string(VerdictInsufficientEvidence),
			}
			return Evaluation{
				Verdict:        VerdictInsufficientEvidence,
				ReasonCode:     "required_dimension_unavailable",
				Summary:        fmt.Sprintf("required comparison dimension %s is unavailable", dimension),
				EvidenceStatus: EvidenceStatusInsufficient,
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
