package releasegate

import (
	"encoding/json"
	"testing"
)

func TestEvaluatePassesWhenThresholdsHold(t *testing.T) {
	evaluation, err := Evaluate(testSummary(t, `{
		"status":"comparable",
		"dimension_deltas":{
			"correctness":{"delta":-0.01,"better_direction":"higher","state":"available"},
			"reliability":{"delta":-0.01,"better_direction":"higher","state":"available"},
			"latency":{"delta":0.02,"better_direction":"lower","state":"available"},
			"cost":{"delta":0.03,"better_direction":"lower","state":"available"}
		},
		"failure_divergence":{"candidate_failed_baseline_succeeded":false,"both_failed_differently":false},
		"replay_summary_divergence":{"state":"available"},
		"evidence_quality":{}
	}`), DefaultPolicy())
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if evaluation.Verdict != VerdictPass {
		t.Fatalf("verdict = %q, want %q", evaluation.Verdict, VerdictPass)
	}
	if evaluation.ReasonCode != "within_thresholds" {
		t.Fatalf("reason code = %q, want within_thresholds", evaluation.ReasonCode)
	}
}

func TestEvaluateWarnsWhenWarnThresholdCrossed(t *testing.T) {
	evaluation, err := Evaluate(testSummary(t, `{
		"status":"comparable",
		"dimension_deltas":{
			"correctness":{"delta":-0.03,"better_direction":"higher","state":"available"},
			"reliability":{"delta":0,"better_direction":"higher","state":"available"},
			"latency":{"delta":0,"better_direction":"lower","state":"available"},
			"cost":{"delta":0,"better_direction":"lower","state":"available"}
		},
		"failure_divergence":{"candidate_failed_baseline_succeeded":false,"both_failed_differently":false},
		"replay_summary_divergence":{"state":"available"},
		"evidence_quality":{}
	}`), DefaultPolicy())
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if evaluation.Verdict != VerdictWarn {
		t.Fatalf("verdict = %q, want %q", evaluation.Verdict, VerdictWarn)
	}
	if evaluation.ReasonCode != "threshold_warn_correctness" {
		t.Fatalf("reason code = %q, want threshold_warn_correctness", evaluation.ReasonCode)
	}
}

func TestEvaluateFailsWhenFailThresholdCrossed(t *testing.T) {
	evaluation, err := Evaluate(testSummary(t, `{
		"status":"comparable",
		"dimension_deltas":{
			"correctness":{"delta":-0.06,"better_direction":"higher","state":"available"},
			"reliability":{"delta":0,"better_direction":"higher","state":"available"},
			"latency":{"delta":0,"better_direction":"lower","state":"available"},
			"cost":{"delta":0,"better_direction":"lower","state":"available"}
		},
		"failure_divergence":{"candidate_failed_baseline_succeeded":false,"both_failed_differently":false},
		"replay_summary_divergence":{"state":"available"},
		"evidence_quality":{}
	}`), DefaultPolicy())
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if evaluation.Verdict != VerdictFail {
		t.Fatalf("verdict = %q, want %q", evaluation.Verdict, VerdictFail)
	}
	if evaluation.ReasonCode != "threshold_fail_correctness" {
		t.Fatalf("reason code = %q, want threshold_fail_correctness", evaluation.ReasonCode)
	}
}

func TestEvaluateReturnsInsufficientEvidenceWhenMissingFieldsPresent(t *testing.T) {
	evaluation, err := Evaluate(testSummary(t, `{
		"status":"comparable",
		"dimension_deltas":{
			"correctness":{"delta":0,"better_direction":"higher","state":"available"},
			"reliability":{"delta":0,"better_direction":"higher","state":"available"},
			"latency":{"delta":0,"better_direction":"lower","state":"available"},
			"cost":{"delta":0,"better_direction":"lower","state":"available"}
		},
		"failure_divergence":{"candidate_failed_baseline_succeeded":false,"both_failed_differently":false},
		"replay_summary_divergence":{"state":"unavailable"},
		"evidence_quality":{"missing_fields":["replay_summary_divergence"]}
	}`), DefaultPolicy())
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if evaluation.Verdict != VerdictInsufficientEvidence {
		t.Fatalf("verdict = %q, want %q", evaluation.Verdict, VerdictInsufficientEvidence)
	}
	if evaluation.ReasonCode != "comparison_evidence_missing" {
		t.Fatalf("reason code = %q, want comparison_evidence_missing", evaluation.ReasonCode)
	}
}

func TestEvaluateReturnsInsufficientEvidenceWhenComparisonNotComparable(t *testing.T) {
	evaluation, err := Evaluate(testSummary(t, `{
		"status":"not_comparable",
		"reason_code":"missing_scorecard",
		"failure_divergence":{"candidate_failed_baseline_succeeded":false,"both_failed_differently":false},
		"replay_summary_divergence":{"state":"unavailable"},
		"evidence_quality":{}
	}`), DefaultPolicy())
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if evaluation.Verdict != VerdictInsufficientEvidence {
		t.Fatalf("verdict = %q, want %q", evaluation.Verdict, VerdictInsufficientEvidence)
	}
	if evaluation.ReasonCode != "comparison_not_comparable" {
		t.Fatalf("reason code = %q, want comparison_not_comparable", evaluation.ReasonCode)
	}
}

func TestPolicySnapshotFingerprintStable(t *testing.T) {
	policy := DefaultPolicy()
	firstJSON, firstFingerprint, err := PolicySnapshot(policy)
	if err != nil {
		t.Fatalf("first PolicySnapshot returned error: %v", err)
	}
	secondJSON, secondFingerprint, err := PolicySnapshot(policy)
	if err != nil {
		t.Fatalf("second PolicySnapshot returned error: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("snapshot mismatch:\n%s\n%s", firstJSON, secondJSON)
	}
	if firstFingerprint != secondFingerprint {
		t.Fatalf("fingerprint = %q, want %q", firstFingerprint, secondFingerprint)
	}
}

func testSummary(t *testing.T, payload string) ComparisonSummary {
	t.Helper()

	var summary ComparisonSummary
	if err := json.Unmarshal([]byte(payload), &summary); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	return summary
}
