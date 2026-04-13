package scoring

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type modelRef struct {
	ProviderKey     string
	ProviderModelID string
}

type pricedUsage struct {
	ProviderKey     string
	ProviderModelID string
	InputTokens     float64
	OutputTokens    float64
	TotalTokens     float64
}

type stepDurationEvidence struct {
	StepIndex   int     `json:"step_index"`
	DurationMS  float64 `json:"duration_ms"`
	StartedAt   string  `json:"started_at"`
	CompletedAt string  `json:"completed_at"`
}

func evaluateMetrics(metrics []MetricDeclaration, evidence extractedEvidence, validators []ValidatorResult, spec EvaluationSpec) ([]MetricResult, []string) {
	results := make([]MetricResult, 0, len(metrics))
	warnings := append([]string(nil), evidence.warnings...)
	for _, metric := range metrics {
		result := MetricResult{
			Key:       metric.Key,
			Type:      metric.Type,
			Collector: metric.Collector,
			Unit:      metric.Unit,
		}

		state, numericValue, textValue, boolValue, reason, metadata := collectMetric(metric, evidence, validators, spec)
		result.State = state
		result.NumericValue = numericValue
		result.TextValue = textValue
		result.BooleanValue = boolValue
		result.Reason = reason
		result.Metadata = metadata
		if evidence.challengeInputChallengeID != nil {
			result.ChallengeIdentityID = evidence.challengeInputChallengeID
		}
		results = append(results, result)
	}
	return results, warnings
}

func collectMetric(metric MetricDeclaration, evidence extractedEvidence, validators []ValidatorResult, spec EvaluationSpec) (OutputState, *float64, *string, *bool, string, json.RawMessage) {
	switch metric.Collector {
	case "run_total_latency_ms":
		value, reason, metadata := totalLatencyMetric(evidence)
		if value == nil {
			return unavailableMetricWithMetadata(reason, metric, metadata)
		}
		return OutputStateAvailable, value, nil, nil, "", metadata
	case "run_ttft_ms":
		value, reason, metadata := ttftMetric(evidence)
		if value == nil {
			return unavailableMetricWithMetadata(reason, metric, metadata)
		}
		return OutputStateAvailable, value, nil, nil, "", metadata
	case "run_input_tokens":
		if evidence.inputTokens == nil {
			return unavailableMetric("input token usage is unavailable", metric)
		}
		return OutputStateAvailable, floatPtr(*evidence.inputTokens), nil, nil, "", mustMarshalJSON(map[string]any{
			"state":     OutputStateAvailable,
			"collector": metric.Collector,
		})
	case "run_output_tokens":
		if evidence.outputTokens == nil {
			return unavailableMetric("output token usage is unavailable", metric)
		}
		return OutputStateAvailable, floatPtr(*evidence.outputTokens), nil, nil, "", mustMarshalJSON(map[string]any{
			"state":     OutputStateAvailable,
			"collector": metric.Collector,
		})
	case "run_total_tokens":
		if evidence.totalTokens == nil {
			return unavailableMetric("total token usage is unavailable", metric)
		}
		return OutputStateAvailable, floatPtr(*evidence.totalTokens), nil, nil, "", mustMarshalJSON(map[string]any{
			"state":     OutputStateAvailable,
			"collector": metric.Collector,
		})
	case "run_model_cost_usd":
		value, reason, metadata := modelCostMetric(evidence, spec)
		if value == nil {
			return unavailableMetricWithMetadata(reason, metric, metadata)
		}
		return OutputStateAvailable, value, nil, nil, "", metadata
	case "run_completed_successfully":
		if evidence.completedSuccessfully == nil {
			return unavailableMetric("terminal success evidence is unavailable", metric)
		}
		value := *evidence.completedSuccessfully
		return OutputStateAvailable, nil, nil, &value, "", mustMarshalJSON(map[string]any{
			"state":     OutputStateAvailable,
			"collector": metric.Collector,
		})
	case "run_failure_count":
		value := float64(evidence.failureCount)
		return OutputStateAvailable, &value, nil, nil, "", mustMarshalJSON(map[string]any{
			"state":     OutputStateAvailable,
			"collector": metric.Collector,
		})
	case "validator_pass_rate":
		if len(validators) == 0 {
			return unavailableMetric("validator pass rate requires validator results", metric)
		}
		var total float64
		for _, validator := range validators {
			if validator.State != OutputStateAvailable || validator.NormalizedScore == nil {
				return unavailableMetric("validator pass rate requires all validators to be available", metric)
			}
			total += *validator.NormalizedScore
		}
		value := total / float64(len(validators))
		return OutputStateAvailable, &value, nil, nil, "", mustMarshalJSON(map[string]any{
			"state":     OutputStateAvailable,
			"collector": metric.Collector,
		})
	default:
		return errorMetric(fmt.Sprintf("collector %q is not supported", metric.Collector), metric)
	}
}

func unavailableMetric(reason string, metric MetricDeclaration) (OutputState, *float64, *string, *bool, string, json.RawMessage) {
	return OutputStateUnavailable, nil, nil, nil, reason, mustMarshalJSON(map[string]any{
		"state":     OutputStateUnavailable,
		"collector": metric.Collector,
		"reason":    reason,
	})
}

func unavailableMetricWithMetadata(reason string, metric MetricDeclaration, metadata json.RawMessage) (OutputState, *float64, *string, *bool, string, json.RawMessage) {
	if len(bytes.TrimSpace(metadata)) == 0 {
		return unavailableMetric(reason, metric)
	}
	decoded := decodePayload(metadata)
	decoded["state"] = OutputStateUnavailable
	decoded["collector"] = metric.Collector
	decoded["reason"] = reason
	return OutputStateUnavailable, nil, nil, nil, reason, mustMarshalJSON(decoded)
}

func errorMetric(reason string, metric MetricDeclaration) (OutputState, *float64, *string, *bool, string, json.RawMessage) {
	return OutputStateError, nil, nil, nil, reason, mustMarshalJSON(map[string]any{
		"state":     OutputStateError,
		"collector": metric.Collector,
		"reason":    reason,
	})
}

func findMetric(metrics []MetricResult, collector string) *MetricResult {
	for i := range metrics {
		if metrics[i].Collector == collector {
			return &metrics[i]
		}
	}
	return nil
}

func totalLatencyMetric(evidence extractedEvidence) (*float64, string, json.RawMessage) {
	if evidence.startedAt == nil || evidence.terminalAt == nil {
		return nil, "latency evidence is unavailable", mustMarshalJSON(map[string]any{
			"step_durations": evidence.stepDurations,
		})
	}
	duration := float64(evidence.terminalAt.Sub(*evidence.startedAt).Milliseconds())
	metadata := map[string]any{
		"state":          OutputStateAvailable,
		"started_at":     evidence.startedAt,
		"ended_at":       evidence.terminalAt,
		"step_durations": evidence.stepDurations,
	}
	if evidence.firstOutputAt != nil {
		metadata["first_output_at"] = evidence.firstOutputAt
		metadata["ttft_ms"] = float64(evidence.firstOutputAt.Sub(*evidence.startedAt).Milliseconds())
	}
	return floatPtr(duration), "", mustMarshalJSON(metadata)
}

func ttftMetric(evidence extractedEvidence) (*float64, string, json.RawMessage) {
	if evidence.startedAt == nil || evidence.firstOutputAt == nil {
		return nil, "time-to-first-output evidence is unavailable", nil
	}
	duration := float64(evidence.firstOutputAt.Sub(*evidence.startedAt).Milliseconds())
	return floatPtr(duration), "", mustMarshalJSON(map[string]any{
		"state":           OutputStateAvailable,
		"started_at":      evidence.startedAt,
		"first_output_at": evidence.firstOutputAt,
	})
}
