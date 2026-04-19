package releasegate

import (
	"testing"

	"github.com/google/uuid"
)

func TestNormalizePolicyPreservesLegacyPoliciesWithoutRegressionRules(t *testing.T) {
	policy := DefaultPolicy()

	firstJSON, firstFingerprint, err := PolicySnapshot(policy)
	if err != nil {
		t.Fatalf("PolicySnapshot returned error: %v", err)
	}

	policy.RegressionGateRules = &RegressionGateRules{}
	secondJSON, secondFingerprint, err := PolicySnapshot(policy)
	if err != nil {
		t.Fatalf("PolicySnapshot with empty regression rules returned error: %v", err)
	}

	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("policy snapshot changed:\n%s\n%s", firstJSON, secondJSON)
	}
	if firstFingerprint != secondFingerprint {
		t.Fatalf("fingerprint = %q, want %q", secondFingerprint, firstFingerprint)
	}
}

func TestNormalizePolicyNormalizesRegressionGateRules(t *testing.T) {
	policy := DefaultPolicy()
	policy.RegressionGateRules = &RegressionGateRules{
		NoBlockingRegressionFailure:  true,
		MaxWarningRegressionFailures: intPtr(2),
		SuiteIDs: []string{
			"B0E20D4F-1A0B-4BE7-BE65-9A5D7D96E6B0",
			"b0e20d4f-1a0b-4be7-be65-9a5d7d96e6b0",
			" 6af2a0c0-342f-47a6-8210-b7ff1f6f44f8 ",
		},
	}

	normalized, err := NormalizePolicy(policy)
	if err != nil {
		t.Fatalf("NormalizePolicy returned error: %v", err)
	}

	if normalized.RegressionGateRules == nil {
		t.Fatal("normalized regression rules missing")
	}
	if got, want := len(normalized.RegressionGateRules.SuiteIDs), 2; got != want {
		t.Fatalf("suite id count = %d, want %d", got, want)
	}

	policy.RegressionGateRules.MaxWarningRegressionFailures = intPtr(-1)
	if _, err := NormalizePolicy(policy); err == nil {
		t.Fatal("expected negative max_warning_regression_failures to fail")
	}
}

func TestEvaluateRegressionRulesNoBlockingFailure(t *testing.T) {
	caseID := uuid.New()
	suiteID := uuid.New()

	outcome := EvaluateRegressionGateRules([]RegressionCaseEvaluation{
		{
			RegressionCaseID: caseID,
			SuiteID:          suiteID,
			Severity:         "blocking",
			Failed:           true,
			Evidence:         &RegressionEvidenceRef{ScoringResultID: uuid.New(), ScoringResultType: "judge_result"},
		},
	}, nil, &RegressionGateRules{NoBlockingRegressionFailure: true})

	if outcome.Verdict != VerdictFail {
		t.Fatalf("verdict = %q, want %q", outcome.Verdict, VerdictFail)
	}
	if outcome.ReasonCode != "regression_blocking_failure" {
		t.Fatalf("reason code = %q, want regression_blocking_failure", outcome.ReasonCode)
	}
	if len(outcome.Violations) != 1 {
		t.Fatalf("violation count = %d, want 1", len(outcome.Violations))
	}
	if outcome.Violations[0].RegressionCaseID != caseID {
		t.Fatalf("regression case id = %s, want %s", outcome.Violations[0].RegressionCaseID, caseID)
	}
}

func TestEvaluateRegressionRulesNoNewBlockingFailureVsBaseline(t *testing.T) {
	caseID := uuid.New()
	suiteID := uuid.New()

	outcome := EvaluateRegressionGateRules(
		[]RegressionCaseEvaluation{{
			RegressionCaseID: caseID,
			SuiteID:          suiteID,
			Severity:         "blocking",
			Failed:           true,
			Evidence:         &RegressionEvidenceRef{ScoringResultID: uuid.New(), ScoringResultType: "judge_result"},
		}},
		[]RegressionCaseEvaluation{{
			RegressionCaseID: caseID,
			SuiteID:          suiteID,
			Severity:         "blocking",
			Failed:           false,
		}},
		&RegressionGateRules{NoNewBlockingFailureVsBaseline: true},
	)

	if outcome.Verdict != VerdictFail {
		t.Fatalf("verdict = %q, want %q", outcome.Verdict, VerdictFail)
	}
	if outcome.ReasonCode != "regression_new_blocking_failure" {
		t.Fatalf("reason code = %q, want regression_new_blocking_failure", outcome.ReasonCode)
	}
}

func TestEvaluateRegressionRulesWarningThreshold(t *testing.T) {
	suiteID := uuid.New()
	outcome := EvaluateRegressionGateRules([]RegressionCaseEvaluation{
		{
			RegressionCaseID: uuid.New(),
			SuiteID:          suiteID,
			Severity:         "warning",
			Failed:           true,
			Evidence:         &RegressionEvidenceRef{ScoringResultID: uuid.New(), ScoringResultType: "judge_result"},
		},
		{
			RegressionCaseID: uuid.New(),
			SuiteID:          suiteID,
			Severity:         "warning",
			Failed:           true,
			Evidence:         &RegressionEvidenceRef{ScoringResultID: uuid.New(), ScoringResultType: "metric_result"},
		},
	}, nil, &RegressionGateRules{MaxWarningRegressionFailures: intPtr(1)})

	if outcome.Verdict != VerdictWarn {
		t.Fatalf("verdict = %q, want %q", outcome.Verdict, VerdictWarn)
	}
	if got := len(outcome.Violations); got != 2 {
		t.Fatalf("violation count = %d, want 2", got)
	}
	if outcome.Violations[0].ObservedCount == nil || *outcome.Violations[0].ObservedCount != 2 {
		t.Fatalf("observed count = %#v, want 2", outcome.Violations[0].ObservedCount)
	}
}

func TestEvaluateRegressionRulesSuiteScope(t *testing.T) {
	allowedSuiteID := uuid.New()
	blockedSuiteID := uuid.New()

	outcome := EvaluateRegressionGateRules([]RegressionCaseEvaluation{
		{
			RegressionCaseID: uuid.New(),
			SuiteID:          allowedSuiteID,
			Severity:         "blocking",
			Failed:           true,
			Evidence:         &RegressionEvidenceRef{ScoringResultID: uuid.New(), ScoringResultType: "judge_result"},
		},
		{
			RegressionCaseID: uuid.New(),
			SuiteID:          blockedSuiteID,
			Severity:         "blocking",
			Failed:           true,
			Evidence:         &RegressionEvidenceRef{ScoringResultID: uuid.New(), ScoringResultType: "judge_result"},
		},
	}, nil, &RegressionGateRules{
		NoBlockingRegressionFailure: true,
		SuiteIDs:                    []string{allowedSuiteID.String()},
	})

	if got := len(outcome.Violations); got != 1 {
		t.Fatalf("violation count = %d, want 1", got)
	}
	if outcome.Violations[0].SuiteID != allowedSuiteID {
		t.Fatalf("suite id = %s, want %s", outcome.Violations[0].SuiteID, allowedSuiteID)
	}
}

func TestEvaluateRegressionRulesMissingBaselineWarns(t *testing.T) {
	outcome := EvaluateRegressionGateRules([]RegressionCaseEvaluation{
		{
			RegressionCaseID: uuid.New(),
			SuiteID:          uuid.New(),
			Severity:         "blocking",
			Failed:           true,
			Evidence:         &RegressionEvidenceRef{ScoringResultID: uuid.New(), ScoringResultType: "judge_result"},
		},
	}, nil, &RegressionGateRules{NoNewBlockingFailureVsBaseline: true})

	if outcome.Verdict != VerdictPass {
		t.Fatalf("verdict = %q, want %q", outcome.Verdict, VerdictPass)
	}
	if len(outcome.Warnings) != 1 {
		t.Fatalf("warning count = %d, want 1", len(outcome.Warnings))
	}
}

func TestEvaluateMergesStandardAndRegressionResults(t *testing.T) {
	base := Evaluation{
		Verdict:        VerdictPass,
		ReasonCode:     "within_thresholds",
		Summary:        "release gate passed",
		EvidenceStatus: EvidenceStatusSufficient,
		Details: EvaluationDetails{
			PolicyKey:        "default",
			PolicyVersion:    1,
			ComparisonStatus: "comparable",
		},
	}
	regression := RegressionGateOutcome{
		Verdict:    VerdictFail,
		ReasonCode: "regression_blocking_failure",
		Summary:    "release gate failed because regression cases violated policy",
		Warnings:   []string{"baseline regression evidence unavailable for 1 blocking case(s); skipped no_new_blocking_failure_vs_baseline"},
		Violations: []RegressionGateViolation{{
			Rule:             regressionRuleNoBlockingFailure,
			Severity:         "blocking",
			RegressionCaseID: uuid.New(),
			SuiteID:          uuid.New(),
			Evidence:         RegressionEvidenceRef{ScoringResultID: uuid.New(), ScoringResultType: "judge_result"},
		}},
	}

	merged := MergeEvaluation(base, regression)
	if merged.Verdict != VerdictFail {
		t.Fatalf("verdict = %q, want %q", merged.Verdict, VerdictFail)
	}
	if merged.ReasonCode != "regression_blocking_failure" {
		t.Fatalf("reason code = %q, want regression_blocking_failure", merged.ReasonCode)
	}
	if len(merged.Details.Warnings) != 1 {
		t.Fatalf("warning count = %d, want 1", len(merged.Details.Warnings))
	}
	if len(merged.Details.RegressionViolations) != 1 {
		t.Fatalf("regression violation count = %d, want 1", len(merged.Details.RegressionViolations))
	}
}

func TestMergeEvaluationPreservesPrimaryThresholdFailure(t *testing.T) {
	base := Evaluation{
		Verdict:        VerdictFail,
		ReasonCode:     "threshold_fail_correctness",
		Summary:        "release gate failed because fail conditions were triggered",
		EvidenceStatus: EvidenceStatusSufficient,
		Details: EvaluationDetails{
			PolicyKey:           "default",
			PolicyVersion:       1,
			ComparisonStatus:    "comparable",
			TriggeredConditions: []string{"threshold_fail_correctness"},
		},
	}
	regression := RegressionGateOutcome{
		Verdict:             VerdictFail,
		ReasonCode:          "regression_blocking_failure",
		Summary:             "release gate failed because regression cases violated policy",
		TriggeredConditions: []string{"no_blocking_regression_failure:123"},
		Warnings:            []string{"baseline warning"},
	}

	merged := MergeEvaluation(base, regression)
	if merged.ReasonCode != "threshold_fail_correctness" {
		t.Fatalf("reason code = %q, want threshold_fail_correctness", merged.ReasonCode)
	}
	if merged.Summary != "release gate failed because fail conditions were triggered" {
		t.Fatalf("summary = %q, want threshold fail summary", merged.Summary)
	}
	if got := len(merged.Details.TriggeredConditions); got != 2 {
		t.Fatalf("triggered condition count = %d, want 2", got)
	}
}

func intPtr(value int) *int {
	return &value
}
