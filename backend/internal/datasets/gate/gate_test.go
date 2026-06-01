package gate

import (
	"testing"

	"github.com/google/uuid"
)

func strPtr(value string) *string { return &value }
func f64Ptr(value float64) *float64 { return &value }

func TestEvaluatePassWhenCandidateMatchesBaseline(t *testing.T) {
	exampleID := uuid.New()
	pass := "pass"
	baseline := []ExampleOutcome{{DatasetExampleID: exampleID, Verdict: &pass, NormalizedScore: f64Ptr(1)}}
	candidate := []ExampleOutcome{{DatasetExampleID: exampleID, Verdict: &pass, NormalizedScore: f64Ptr(1)}}

	result := Evaluate(baseline, candidate, Thresholds{})
	if !result.Pass {
		t.Fatalf("Pass = false, want true")
	}
	if result.RegressionCount != 0 {
		t.Fatalf("RegressionCount = %d, want 0", result.RegressionCount)
	}
}

func TestEvaluateFailsOnNewlyFailingExample(t *testing.T) {
	exampleID := uuid.New()
	pass := "pass"
	fail := "fail"
	baseline := []ExampleOutcome{{DatasetExampleID: exampleID, Verdict: &pass, NormalizedScore: f64Ptr(1)}}
	candidate := []ExampleOutcome{{DatasetExampleID: exampleID, Verdict: &fail, NormalizedScore: f64Ptr(0)}}

	result := Evaluate(baseline, candidate, Thresholds{MaxRegressions: intPtr(0)})
	if result.Pass {
		t.Fatal("Pass = true, want false")
	}
	if len(result.Regressions) != 1 || result.Regressions[0].Reason != "newly_failing" {
		t.Fatalf("regressions = %#v", result.Regressions)
	}
}

func TestEvaluateHonorsMinPassRate(t *testing.T) {
	pass := "pass"
	fail := "fail"
	baseline := []ExampleOutcome{}
	candidate := []ExampleOutcome{
		{DatasetExampleID: uuid.New(), Verdict: &pass},
		{DatasetExampleID: uuid.New(), Verdict: &fail},
	}

	minPassRate := 0.9
	result := Evaluate(baseline, candidate, Thresholds{MinPassRate: &minPassRate})
	if result.Pass {
		t.Fatal("Pass = true, want false")
	}
	if result.PassRate != 0.5 {
		t.Fatalf("PassRate = %v, want 0.5", result.PassRate)
	}
}

func intPtr(value int) *int { return &value }

func TestEvaluateFailsOnEmptyCandidate(t *testing.T) {
	pass := "pass"
	baseline := []ExampleOutcome{{DatasetExampleID: uuid.New(), Verdict: &pass}}
	result := Evaluate(baseline, nil, Thresholds{})
	if result.Pass {
		t.Fatal("Pass = true, want false")
	}
	if len(result.FailedThresholds) != 1 || result.FailedThresholds[0] != "no_candidate_outcomes" {
		t.Fatalf("FailedThresholds = %#v", result.FailedThresholds)
	}
}

func TestEvaluateFailsOnMissingBaselineExample(t *testing.T) {
	exampleA := uuid.New()
	exampleB := uuid.New()
	pass := "pass"
	baseline := []ExampleOutcome{
		{DatasetExampleID: exampleA, Verdict: &pass},
		{DatasetExampleID: exampleB, Verdict: &pass},
	}
	candidate := []ExampleOutcome{{DatasetExampleID: exampleA, Verdict: &pass}}

	result := Evaluate(baseline, candidate, Thresholds{})
	if result.Pass {
		t.Fatal("Pass = true, want false")
	}
	if len(result.Regressions) != 1 || result.Regressions[0].Reason != "missing_example" {
		t.Fatalf("regressions = %#v", result.Regressions)
	}
	if result.Regressions[0].DatasetExampleID != exampleB {
		t.Fatalf("missing example = %v, want %v", result.Regressions[0].DatasetExampleID, exampleB)
	}
}

func TestEvaluateIgnoresScoreRegressionOnAlreadyFailingBaseline(t *testing.T) {
	exampleID := uuid.New()
	fail := "fail"
	baseline := []ExampleOutcome{{DatasetExampleID: exampleID, Verdict: &fail, NormalizedScore: f64Ptr(0.5)}}
	candidate := []ExampleOutcome{{DatasetExampleID: exampleID, Verdict: &fail, NormalizedScore: f64Ptr(0.1)}}

	result := Evaluate(baseline, candidate, Thresholds{})
	if !result.Pass {
		t.Fatal("Pass = false, want true")
	}
	if len(result.Regressions) != 0 {
		t.Fatalf("regressions = %#v", result.Regressions)
	}
}
