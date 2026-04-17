package scoring

import "fmt"

func evaluateDimensions(spec EvaluationSpec, evidence extractedEvidence, validators []ValidatorResult, metrics []MetricResult, llmJudges []LLMJudgeResult) []DimensionResult {
	dimensions := spec.Scorecard.Dimensions
	results := make([]DimensionResult, 0, len(dimensions))
	for _, dim := range dimensions {
		result := DimensionResult{Dimension: dim.Key, BetterDirection: dim.BetterDirection}
		var score *float64
		var reason string
		var state OutputState

		switch dim.Source {
		case DimensionSourceValidators:
			score, reason, state = averageScopedValidators(dim.Validators, validators)
		case DimensionSourceReliability:
			score, reason, state = reliabilityScore(metrics)
		case DimensionSourceLatency:
			score, reason, state = latencyDimensionScore(dim, evidence)
		case DimensionSourceCost:
			score, reason, state = costDimensionScore(dim, evidence, spec)
		case DimensionSourceBehavioral:
			score, reason, state = behavioralDimensionScore(spec, evidence, validators)
		case DimensionSourceMetric:
			score, reason, state = metricDimensionScore(dim, metrics)
		case DimensionSourceLLMJudge:
			score, reason, state = llmJudgeDimensionScore(dim, llmJudges)
		default:
			state = OutputStateError
			reason = fmt.Sprintf("unsupported dimension source %q", dim.Source)
		}

		result.Score = score
		result.Reason = reason
		result.State = state
		results = append(results, result)
	}
	return results
}

func dimensionWarnings(results []DimensionResult, dims []DimensionDeclaration) []string {
	sourceByKey := make(map[string]DimensionSource, len(dims))
	for _, d := range dims {
		sourceByKey[d.Key] = d.Source
	}
	warnings := make([]string, 0, len(results))
	for _, result := range results {
		src := sourceByKey[result.Dimension]
		if src == DimensionSourceLatency || src == DimensionSourceCost || src == DimensionSourceBehavioral || src == DimensionSourceMetric || src == DimensionSourceLLMJudge {
			if result.State == OutputStateUnavailable && result.Reason != "" {
				warnings = append(warnings, result.Reason)
			}
		}
	}
	return warnings
}

func averageScopedValidators(scope []string, validators []ValidatorResult) (*float64, string, OutputState) {
	selected := validators
	if len(scope) > 0 {
		allowed := make(map[string]struct{}, len(scope))
		for _, k := range scope {
			allowed[k] = struct{}{}
		}
		selected = make([]ValidatorResult, 0, len(scope))
		for _, v := range validators {
			if _, ok := allowed[v.Key]; ok {
				selected = append(selected, v)
			}
		}
	}
	if len(selected) == 0 {
		return nil, "no validators in scope", OutputStateUnavailable
	}
	var total float64
	for _, v := range selected {
		if v.State != OutputStateAvailable || v.NormalizedScore == nil {
			return nil, "all scoped validators must be available", OutputStateUnavailable
		}
		total += *v.NormalizedScore
	}
	score := total / float64(len(selected))
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

func latencyDimensionScore(dim DimensionDeclaration, evidence extractedEvidence) (*float64, string, OutputState) {
	value, reason, _ := totalLatencyMetric(evidence)
	if value == nil {
		return nil, reason, OutputStateUnavailable
	}
	if dim.Normalization == nil || dim.Normalization.Target == nil || dim.Normalization.Max == nil {
		return nil, "latency normalization config is unavailable", OutputStateUnavailable
	}
	score := normalizeLowerIsBetter(*value, *dim.Normalization.Target, *dim.Normalization.Max)
	return &score, "", OutputStateAvailable
}

func costDimensionScore(dim DimensionDeclaration, evidence extractedEvidence, spec EvaluationSpec) (*float64, string, OutputState) {
	value, reason, _ := computeModelCostUSD(evidence, spec)
	if value == nil {
		return nil, reason, OutputStateUnavailable
	}
	if dim.Normalization == nil || dim.Normalization.Target == nil || dim.Normalization.Max == nil {
		return nil, "cost normalization config is unavailable", OutputStateUnavailable
	}
	score := normalizeLowerIsBetter(*value, *dim.Normalization.Target, *dim.Normalization.Max)
	return &score, "", OutputStateAvailable
}

func metricDimensionScore(dim DimensionDeclaration, metrics []MetricResult) (*float64, string, OutputState) {
	if dim.Metric == "" {
		return nil, "dimension metric key is not set", OutputStateError
	}
	metric := findMetricByKey(metrics, dim.Metric)
	if metric == nil || metric.State != OutputStateAvailable || metric.NumericValue == nil {
		return nil, fmt.Sprintf("metric %q is unavailable", dim.Metric), OutputStateUnavailable
	}
	if dim.Normalization == nil || dim.Normalization.Target == nil || dim.Normalization.Max == nil {
		return nil, "metric normalization config is unavailable", OutputStateUnavailable
	}
	var score float64
	switch dim.BetterDirection {
	case "lower":
		score = normalizeLowerIsBetter(*metric.NumericValue, *dim.Normalization.Target, *dim.Normalization.Max)
	case "higher":
		score = normalizeHigherIsBetter(*metric.NumericValue, *dim.Normalization.Target, *dim.Normalization.Max)
	default:
		return nil, fmt.Sprintf("unsupported better_direction %q", dim.BetterDirection), OutputStateError
	}
	return &score, "", OutputStateAvailable
}

func llmJudgeDimensionScore(dim DimensionDeclaration, results []LLMJudgeResult) (*float64, string, OutputState) {
	if dim.JudgeKey == "" {
		return nil, "dimension judge_key is not set", OutputStateError
	}
	for _, result := range results {
		if result.JudgeKey != dim.JudgeKey {
			continue
		}
		if result.NormalizedScore == nil {
			return nil, firstNonEmpty(result.Reason, fmt.Sprintf("llm judge %q is unavailable", dim.JudgeKey)), OutputStateUnavailable
		}
		score := *result.NormalizedScore
		return &score, "", OutputStateAvailable
	}
	return nil, fmt.Sprintf("llm judge %q is unavailable", dim.JudgeKey), OutputStateUnavailable
}

func findMetricByKey(metrics []MetricResult, key string) *MetricResult {
	for i := range metrics {
		if metrics[i].Key == key {
			return &metrics[i]
		}
	}
	return nil
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

func normalizeHigherIsBetter(value float64, target float64, floor float64) float64 {
	if value >= target {
		return 1
	}
	if value <= floor {
		return 0
	}
	if target <= floor {
		return 0
	}
	return (value - floor) / (target - floor)
}
