package scoring

import (
	"math"
	"strings"
	"testing"
)

func approxEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestComputeOverallScore_WeightedDefaultWeightsAverage(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{Key: "a"}, {Key: "b"},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "a", Score: floatPtr(0.6), State: OutputStateAvailable},
		{Dimension: "b", Score: floatPtr(0.8), State: OutputStateAvailable},
	}

	overall, passed, reason := computeOverallScore(spec, results)
	if overall == nil || *overall != 0.7 {
		t.Fatalf("overall = %v, want 0.7", overall)
	}
	if passed == nil || !*passed {
		t.Fatalf("passed = %v, want true", passed)
	}
	if reason != "" {
		t.Fatalf("reason = %q, want empty", reason)
	}
}

func TestComputeOverallScore_WeightedHonoursExplicitWeights(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{Key: "a", Weight: floatPtr(3)},
				{Key: "b", Weight: floatPtr(1)},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "a", Score: floatPtr(1.0), State: OutputStateAvailable},
		{Dimension: "b", Score: floatPtr(0.0), State: OutputStateAvailable},
	}

	overall, _, _ := computeOverallScore(spec, results)
	if overall == nil || *overall != 0.75 {
		t.Fatalf("overall = %v, want 0.75", overall)
	}
}

func TestComputeOverallScore_WeightedSkipsUnavailableDimensions(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{Key: "a"}, {Key: "b"},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "a", Score: floatPtr(0.5), State: OutputStateAvailable},
		{Dimension: "b", State: OutputStateUnavailable},
	}

	overall, passed, _ := computeOverallScore(spec, results)
	if overall == nil || *overall != 0.5 {
		t.Fatalf("overall = %v, want 0.5", overall)
	}
	if passed == nil || !*passed {
		t.Fatalf("passed = %v, want true (no gates)", passed)
	}
}

func TestComputeOverallScore_WeightedAllUnavailable(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{Key: "a"}, {Key: "b"},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "a", State: OutputStateUnavailable},
		{Dimension: "b", State: OutputStateUnavailable},
	}

	overall, passed, reason := computeOverallScore(spec, results)
	if overall != nil {
		t.Fatalf("overall = %v, want nil", overall)
	}
	if passed != nil {
		t.Fatalf("passed = %v, want nil", passed)
	}
	if reason == "" {
		t.Fatalf("reason should be populated")
	}
}

func TestComputeOverallScore_BinaryAllPass(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyBinary,
			Dimensions: []DimensionDeclaration{
				{Key: "a", PassThreshold: floatPtr(0.5)},
				{Key: "b", PassThreshold: floatPtr(0.5)},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "a", Score: floatPtr(0.8), State: OutputStateAvailable},
		{Dimension: "b", Score: floatPtr(0.6), State: OutputStateAvailable},
	}

	overall, passed, _ := computeOverallScore(spec, results)
	if overall == nil || *overall != 1.0 {
		t.Fatalf("overall = %v, want 1.0", overall)
	}
	if passed == nil || !*passed {
		t.Fatalf("passed = %v, want true", passed)
	}
}

func TestComputeOverallScore_BinaryBelowThresholdFails(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyBinary,
			Dimensions: []DimensionDeclaration{
				{Key: "a", PassThreshold: floatPtr(0.9)},
				{Key: "b", PassThreshold: floatPtr(0.5)},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "a", Score: floatPtr(0.8), State: OutputStateAvailable},
		{Dimension: "b", Score: floatPtr(0.6), State: OutputStateAvailable},
	}

	overall, passed, reason := computeOverallScore(spec, results)
	if overall == nil || *overall != 0.0 {
		t.Fatalf("overall = %v, want 0", overall)
	}
	if passed == nil || *passed {
		t.Fatalf("passed = %v, want false", passed)
	}
	if !strings.Contains(reason, `"a"`) {
		t.Fatalf("reason = %q, want to mention dimension a", reason)
	}
}

func TestComputeOverallScore_BinaryUnavailableDimensionFails(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyBinary,
			Dimensions: []DimensionDeclaration{
				{Key: "a", PassThreshold: floatPtr(0.5)},
				{Key: "b", PassThreshold: floatPtr(0.5)},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "a", Score: floatPtr(0.8), State: OutputStateAvailable},
		{Dimension: "b", State: OutputStateUnavailable, Reason: "missing evidence"},
	}

	overall, passed, reason := computeOverallScore(spec, results)
	if overall == nil || *overall != 0.0 {
		t.Fatalf("overall = %v, want 0", overall)
	}
	if passed == nil || *passed {
		t.Fatalf("passed = %v, want false", passed)
	}
	if !strings.Contains(reason, `"b"`) || !strings.Contains(reason, "unavailable") {
		t.Fatalf("reason = %q, want unavailable dimension b", reason)
	}
}

func TestComputeOverallScore_HybridGatePassUsesWeightedAverage(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyHybrid,
			Dimensions: []DimensionDeclaration{
				{Key: "gated", Gate: true, PassThreshold: floatPtr(0.5)},
				{Key: "unweighted"},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "gated", Score: floatPtr(0.8), State: OutputStateAvailable},
		{Dimension: "unweighted", Score: floatPtr(0.4), State: OutputStateAvailable},
	}

	overall, passed, _ := computeOverallScore(spec, results)
	if overall == nil {
		t.Fatal("overall is nil")
	}
	if !approxEqual(*overall, 0.6) {
		t.Fatalf("overall = %v, want ~0.6", *overall)
	}
	if passed == nil || !*passed {
		t.Fatalf("passed = %v, want true", passed)
	}
}

func TestComputeOverallScore_HybridGateFailForcesZero(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyHybrid,
			Dimensions: []DimensionDeclaration{
				{Key: "gated", Gate: true, PassThreshold: floatPtr(0.9)},
				{Key: "other"},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "gated", Score: floatPtr(0.8), State: OutputStateAvailable},
		{Dimension: "other", Score: floatPtr(1.0), State: OutputStateAvailable},
	}

	overall, passed, reason := computeOverallScore(spec, results)
	if overall == nil || *overall != 0.0 {
		t.Fatalf("overall = %v, want 0", overall)
	}
	if passed == nil || *passed {
		t.Fatalf("passed = %v, want false", passed)
	}
	if !strings.Contains(reason, "hybrid") || !strings.Contains(reason, `"gated"`) {
		t.Fatalf("reason = %q", reason)
	}
}

func TestComputeOverallScore_HybridUnavailableGatedDimensionFails(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyHybrid,
			Dimensions: []DimensionDeclaration{
				{Key: "gated", Gate: true, PassThreshold: floatPtr(0.9)},
				{Key: "other"},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "gated", State: OutputStateUnavailable, Reason: "missing evidence"},
		{Dimension: "other", Score: floatPtr(1.0), State: OutputStateAvailable},
	}

	overall, passed, reason := computeOverallScore(spec, results)
	if overall == nil || *overall != 0.0 {
		t.Fatalf("overall = %v, want 0", overall)
	}
	if passed == nil || *passed {
		t.Fatalf("passed = %v, want false", passed)
	}
	if !strings.Contains(reason, `"gated"`) || !strings.Contains(reason, "unavailable") {
		t.Fatalf("reason = %q, want unavailable gated dimension", reason)
	}
}

func TestComputeOverallScore_HybridNonGatedDimBelowThresholdPasses(t *testing.T) {
	// Non-gated dims never fail hybrid even if their score is low — that's the
	// whole point of "hybrid": hard gates + soft weights.
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyHybrid,
			Dimensions: []DimensionDeclaration{
				{Key: "gated", Gate: true, PassThreshold: floatPtr(0.5)},
				{Key: "soft", PassThreshold: floatPtr(0.9)},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "gated", Score: floatPtr(0.6), State: OutputStateAvailable},
		{Dimension: "soft", Score: floatPtr(0.1), State: OutputStateAvailable},
	}

	_, passed, _ := computeOverallScore(spec, results)
	if passed == nil || !*passed {
		t.Fatalf("passed = %v, want true", passed)
	}
}

func TestComputeOverallScore_DefaultsStrategyToWeighted(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{{Key: "a"}},
		},
	}
	results := []DimensionResult{
		{Dimension: "a", Score: floatPtr(0.42), State: OutputStateAvailable},
	}

	overall, _, _ := computeOverallScore(spec, results)
	if overall == nil || *overall != 0.42 {
		t.Fatalf("overall = %v, want 0.42", overall)
	}
}

func TestComputeOverallScore_WeightedUnavailableGatedDimensionFails(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{Key: "gated", Gate: true, PassThreshold: floatPtr(0.9)},
				{Key: "other"},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "gated", State: OutputStateUnavailable, Reason: "missing evidence"},
		{Dimension: "other", Score: floatPtr(0.42), State: OutputStateAvailable},
	}

	overall, passed, reason := computeOverallScore(spec, results)
	if overall == nil || *overall != 0.42 {
		t.Fatalf("overall = %v, want 0.42", overall)
	}
	if passed == nil || *passed {
		t.Fatalf("passed = %v, want false", passed)
	}
	if !strings.Contains(reason, `"gated"`) || !strings.Contains(reason, "unavailable") {
		t.Fatalf("reason = %q, want unavailable gated dimension", reason)
	}
}

func TestValidateEvaluationSpec_BinaryRequiresPassThreshold(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "v", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyBinary,
			Dimensions: []DimensionDeclaration{
				{Key: "correctness", Source: DimensionSourceValidators, BetterDirection: "higher"},
			},
		},
	}
	err := ValidateEvaluationSpec(spec)
	if err == nil {
		t.Fatalf("expected validation error for missing pass_threshold")
	}
	if !strings.Contains(err.Error(), "pass_threshold") {
		t.Fatalf("expected pass_threshold error, got %v", err)
	}
}

func TestValidateEvaluationSpec_HybridRequiresAtLeastOneGate(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "v", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyHybrid,
			Dimensions: []DimensionDeclaration{
				{Key: "correctness", Source: DimensionSourceValidators, BetterDirection: "higher"},
			},
		},
	}
	err := ValidateEvaluationSpec(spec)
	if err == nil {
		t.Fatalf("expected validation error for hybrid without gates")
	}
	if !strings.Contains(err.Error(), "hybrid") {
		t.Fatalf("expected hybrid error, got %v", err)
	}
}

func TestValidateEvaluationSpec_PassThresholdOutOfRange(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "v", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{Key: "correctness", Source: DimensionSourceValidators, BetterDirection: "higher", Gate: true, PassThreshold: floatPtr(1.5)},
			},
		},
	}
	err := ValidateEvaluationSpec(spec)
	if err == nil {
		t.Fatalf("expected validation error for pass_threshold > 1")
	}
	if !strings.Contains(err.Error(), "between 0 and 1") {
		t.Fatalf("expected range error, got %v", err)
	}
}

func TestValidateEvaluationSpec_DefaultsStrategyToWeighted(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "v", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{
				{Key: "correctness"},
			},
		},
	}
	if err := ValidateEvaluationSpec(spec); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// Phase 3: custom validators-source and reliability-source dims declared
// without better_direction should default to "higher" during normalization so
// callers don't need to repeat boilerplate and downstream sort/delta paths can
// rely on the field being populated.
func TestNormalizeEvaluationSpec_DefaultsBetterDirectionForValidatorsAndReliability(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "v", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{Key: "safety", Source: DimensionSourceValidators, Validators: []string{"v"}},
				{Key: "uptime", Source: DimensionSourceReliability},
			},
		},
	}

	normalizeEvaluationSpec(&spec)

	for _, dim := range spec.Scorecard.Dimensions {
		if dim.BetterDirection != "higher" {
			t.Fatalf("dim %q better_direction = %q, want higher", dim.Key, dim.BetterDirection)
		}
	}

	// And the spec must still validate end-to-end with no errors now that the
	// direction has been filled in implicitly.
	if err := ValidateEvaluationSpec(spec); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// Phase 3: metric/latency/cost dims have ambiguous directionality and must
// still require an explicit better_direction — normalization must not silently
// paper over missing config.
func TestValidateEvaluationSpec_MetricDimStillRequiresBetterDirection(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "fixture",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "v", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "challenge_input"},
		},
		Metrics: []MetricDeclaration{
			{Key: "hit_rate", Type: "counter", Collector: "events"},
		},
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{
					Key:    "hit_rate_score",
					Source: DimensionSourceMetric,
					Metric: "hit_rate",
					Normalization: &DimensionNormalization{
						Target: floatPtr(1.0),
						Max:    floatPtr(0.0),
					},
				},
			},
		},
	}
	err := ValidateEvaluationSpec(spec)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "better_direction") {
		t.Fatalf("error = %q, want it to mention better_direction", err.Error())
	}
}
