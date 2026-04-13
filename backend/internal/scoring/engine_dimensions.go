package scoring

import "fmt"

func evaluateDimensions(spec EvaluationSpec, evidence extractedEvidence, validators []ValidatorResult, metrics []MetricResult) []DimensionResult {
	dimensions := spec.Scorecard.Dimensions
	results := make([]DimensionResult, 0, len(dimensions))
	for _, dimension := range dimensions {
		result := DimensionResult{Dimension: dimension}
		switch dimension {
		case ScorecardDimensionCorrectness:
			score, reason, state := correctnessScore(validators)
			result.Score = score
			result.Reason = reason
			result.State = state
		case ScorecardDimensionReliability:
			score, reason, state := reliabilityScore(metrics)
			result.Score = score
			result.Reason = reason
			result.State = state
		case ScorecardDimensionLatency:
			score, reason, state := latencyScore(spec, evidence)
			result.Score = score
			result.Reason = reason
			result.State = state
		case ScorecardDimensionCost:
			score, reason, state := costScore(spec, evidence)
			result.Score = score
			result.Reason = reason
			result.State = state
		default:
			result.State = OutputStateError
			result.Reason = fmt.Sprintf("unsupported dimension %q", dimension)
		}
		results = append(results, result)
	}
	return results
}

func dimensionWarnings(results []DimensionResult) []string {
	warnings := make([]string, 0, len(results))
	for _, result := range results {
		switch result.Dimension {
		case ScorecardDimensionLatency, ScorecardDimensionCost:
			if result.State == OutputStateUnavailable && result.Reason != "" {
				warnings = append(warnings, result.Reason)
			}
		}
	}
	return warnings
}

func correctnessScore(validators []ValidatorResult) (*float64, string, OutputState) {
	if len(validators) == 0 {
		return nil, "no validators declared", OutputStateUnavailable
	}
	var total float64
	for _, validator := range validators {
		if validator.State != OutputStateAvailable || validator.NormalizedScore == nil {
			return nil, "correctness requires all validators to be available", OutputStateUnavailable
		}
		total += *validator.NormalizedScore
	}
	score := total / float64(len(validators))
	return &score, "", OutputStateAvailable
}

func reliabilityScore(metrics []MetricResult) (*float64, string, OutputState) {
	completed := findMetric(metrics, "run_completed_successfully")
	failures := findMetric(metrics, "run_failure_count")
	if completed == nil || failures == nil {
		return nil, "reliability requires completion and failure-count metrics", OutputStateUnavailable
	}
	if completed.State != OutputStateAvailable || completed.BooleanValue == nil {
		return nil, "completion metric is unavailable", OutputStateUnavailable
	}
	if failures.State != OutputStateAvailable || failures.NumericValue == nil {
		return nil, "failure-count metric is unavailable", OutputStateUnavailable
	}

	score := 0.0
	if *completed.BooleanValue && *failures.NumericValue == 0 {
		score = 1
	}
	return &score, "", OutputStateAvailable
}

func latencyScore(spec EvaluationSpec, evidence extractedEvidence) (*float64, string, OutputState) {
	value, reason, _ := totalLatencyMetric(evidence)
	if value == nil {
		return nil, reason, OutputStateUnavailable
	}

	config := spec.Scorecard.Normalization.Latency
	if config == nil || config.TargetMS == nil {
		return nil, "latency normalization config is unavailable", OutputStateUnavailable
	}
	maxMS, ok := latencyMaxMS(spec)
	if !ok {
		return nil, "latency normalization config is unavailable", OutputStateUnavailable
	}
	score := normalizeLowerIsBetter(*value, *config.TargetMS, maxMS)
	return &score, "", OutputStateAvailable
}

func costScore(spec EvaluationSpec, evidence extractedEvidence) (*float64, string, OutputState) {
	value, reason, _ := computeModelCostUSD(evidence, spec)
	if value == nil {
		return nil, reason, OutputStateUnavailable
	}

	config := spec.Scorecard.Normalization.Cost
	if config == nil || config.TargetUSD == nil {
		return nil, "cost normalization config is unavailable", OutputStateUnavailable
	}
	maxUSD, ok := costMaxUSD(spec)
	if !ok {
		return nil, "cost normalization config is unavailable", OutputStateUnavailable
	}
	score := normalizeLowerIsBetter(*value, *config.TargetUSD, maxUSD)
	return &score, "", OutputStateAvailable
}

func latencyMaxMS(spec EvaluationSpec) (float64, bool) {
	if spec.Scorecard.Normalization.Latency != nil && spec.Scorecard.Normalization.Latency.MaxMS != nil {
		return *spec.Scorecard.Normalization.Latency.MaxMS, true
	}
	if spec.RuntimeLimits.MaxDurationMS != nil {
		return float64(*spec.RuntimeLimits.MaxDurationMS), true
	}
	return 0, false
}

func costMaxUSD(spec EvaluationSpec) (float64, bool) {
	if spec.Scorecard.Normalization.Cost != nil && spec.Scorecard.Normalization.Cost.MaxUSD != nil {
		return *spec.Scorecard.Normalization.Cost.MaxUSD, true
	}
	if spec.RuntimeLimits.MaxCostUSD != nil {
		return *spec.RuntimeLimits.MaxCostUSD, true
	}
	return 0, false
}

func normalizeLowerIsBetter(value float64, target float64, max float64) float64 {
	if value <= target {
		return 1
	}
	if value >= max {
		return 0
	}
	if max <= target {
		return 0
	}
	return 1 - ((value - target) / (max - target))
}
