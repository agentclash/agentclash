package scoring

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type OutputState string

const (
	OutputStateAvailable   OutputState = "available"
	OutputStateUnavailable OutputState = "unavailable"
	OutputStateError       OutputState = "error"
)

type EvaluationStatus string

const (
	EvaluationStatusComplete EvaluationStatus = "complete"
	EvaluationStatusPartial  EvaluationStatus = "partial"
	EvaluationStatusFailed   EvaluationStatus = "failed"
)

type EvidenceInput struct {
	ChallengeIdentityID uuid.UUID       `json:"challenge_identity_id"`
	ChallengeKey        string          `json:"challenge_key"`
	CaseKey             string          `json:"case_key,omitempty"`
	ItemKey             string          `json:"item_key"`
	Payload             json.RawMessage `json:"payload"`
	Inputs              map[string]EvidenceValue    `json:"inputs,omitempty"`
	Expectations        map[string]EvidenceValue    `json:"expectations,omitempty"`
	Artifacts           map[string]EvidenceArtifact `json:"artifacts,omitempty"`
}

type EvidenceValue struct {
	Kind        string          `json:"kind,omitempty"`
	Value       json.RawMessage `json:"value,omitempty"`
	ArtifactKey string          `json:"artifact_key,omitempty"`
	Source      string          `json:"source,omitempty"`
	Path        string          `json:"path,omitempty"`
}

type EvidenceArtifact struct {
	Key       string `json:"key"`
	Kind      string `json:"kind,omitempty"`
	Path      string `json:"path"`
	MediaType string `json:"media_type,omitempty"`
}

type Event struct {
	Type       string          `json:"type"`
	Source     string          `json:"source"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

type EvaluationInput struct {
	RunAgentID       uuid.UUID       `json:"run_agent_id"`
	EvaluationSpecID uuid.UUID       `json:"evaluation_spec_id"`
	ChallengeInputs  []EvidenceInput `json:"challenge_inputs"`
	Events           []Event         `json:"events"`
}

type RunAgentEvaluation struct {
	RunAgentID       uuid.UUID           `json:"run_agent_id"`
	EvaluationSpecID uuid.UUID           `json:"evaluation_spec_id"`
	Status           EvaluationStatus    `json:"status"`
	ValidatorResults []ValidatorResult   `json:"validator_results"`
	MetricResults    []MetricResult      `json:"metric_results"`
	DimensionResults []DimensionResult   `json:"dimension_results"`
	DimensionScores  map[string]*float64 `json:"dimension_scores"`
	Warnings         []string            `json:"warnings,omitempty"`
}

type ValidatorResult struct {
	Key                 string          `json:"key"`
	Type                ValidatorType   `json:"type"`
	State               OutputState     `json:"state"`
	Verdict             string          `json:"verdict,omitempty"`
	NormalizedScore     *float64        `json:"normalized_score,omitempty"`
	Target              string          `json:"target"`
	ExpectedFrom        string          `json:"expected_from"`
	ActualValue         *string         `json:"actual_value,omitempty"`
	ExpectedValue       *string         `json:"expected_value,omitempty"`
	ChallengeIdentityID *uuid.UUID      `json:"challenge_identity_id,omitempty"`
	Reason              string          `json:"reason,omitempty"`
	RawOutput           json.RawMessage `json:"raw_output"`
}

type MetricResult struct {
	Key                 string          `json:"key"`
	Type                MetricType      `json:"type"`
	State               OutputState     `json:"state"`
	Collector           string          `json:"collector"`
	Unit                string          `json:"unit,omitempty"`
	NumericValue        *float64        `json:"numeric_value,omitempty"`
	TextValue           *string         `json:"text_value,omitempty"`
	BooleanValue        *bool           `json:"boolean_value,omitempty"`
	ChallengeIdentityID *uuid.UUID      `json:"challenge_identity_id,omitempty"`
	Reason              string          `json:"reason,omitempty"`
	Metadata            json.RawMessage `json:"metadata"`
}

type DimensionResult struct {
	Dimension ScorecardDimension `json:"dimension"`
	Score     *float64           `json:"score,omitempty"`
	State     OutputState        `json:"state"`
	Reason    string             `json:"reason,omitempty"`
}

var errJudgeModeUnsupported = errors.New("only deterministic evaluation specs are supported")

func DecodeDefinition(definition json.RawMessage) (EvaluationSpec, error) {
	if len(bytes.TrimSpace(definition)) == 0 {
		return EvaluationSpec{}, ValidationErrors{{Field: "evaluation_spec.definition", Message: "is required"}}
	}

	var spec EvaluationSpec
	if err := json.Unmarshal(definition, &spec); err != nil {
		return EvaluationSpec{}, fmt.Errorf("decode evaluation spec definition: %w", err)
	}

	normalizeEvaluationSpec(&spec)
	if err := ValidateEvaluationSpec(spec); err != nil {
		return EvaluationSpec{}, err
	}
	return spec, nil
}

func EvaluateRunAgent(input EvaluationInput, spec EvaluationSpec) (RunAgentEvaluation, error) {
	normalizeEvaluationSpec(&spec)
	if err := ValidateEvaluationSpec(spec); err != nil {
		return RunAgentEvaluation{}, err
	}
	if spec.JudgeMode != JudgeModeDeterministic {
		return RunAgentEvaluation{}, errJudgeModeUnsupported
	}

	events := append([]Event(nil), input.Events...)
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].OccurredAt.Before(events[j].OccurredAt)
	})

	evidence := buildEvidence(input.ChallengeInputs, events)
	validatorResults, warnings := evaluateValidators(spec.Validators, evidence)
	metricResults, metricWarnings := evaluateMetrics(spec.Metrics, evidence, validatorResults, spec)
	warnings = append(warnings, metricWarnings...)

	dimensionResults := evaluateDimensions(spec, evidence, validatorResults, metricResults)
	warnings = append(warnings, dimensionWarnings(dimensionResults)...)
	dimensionScores := make(map[string]*float64, len(dimensionResults))
	for _, result := range dimensionResults {
		score := result.Score
		if score != nil {
			cloned := *score
			score = &cloned
		}
		dimensionScores[string(result.Dimension)] = score
	}

	status := EvaluationStatusComplete
	if len(dimensionResults) == 0 {
		status = EvaluationStatusPartial
	}

	hasDimensionScore := false
	for _, dimension := range dimensionResults {
		if dimension.State != OutputStateAvailable {
			status = EvaluationStatusPartial
		}
		if dimension.Score != nil {
			hasDimensionScore = true
		}
	}
	if !hasDimensionScore {
		status = EvaluationStatusPartial
	}

	return RunAgentEvaluation{
		RunAgentID:       input.RunAgentID,
		EvaluationSpecID: input.EvaluationSpecID,
		Status:           status,
		ValidatorResults: validatorResults,
		MetricResults:    metricResults,
		DimensionResults: dimensionResults,
		DimensionScores:  dimensionScores,
		Warnings:         uniqueStrings(warnings),
	}, nil
}

type extractedEvidence struct {
	finalOutput               *string
	finalOutputChallengeID    *uuid.UUID
	challengeInputValue       *string
	challengeInputChallengeID *uuid.UUID
	caseInput                 *EvidenceInput
	caseInputReason           string
	startedAt                 *time.Time
	firstOutputAt             *time.Time
	terminalAt                *time.Time
	completedSuccessfully     *bool
	failureCount              int
	inputTokens               *float64
	outputTokens              *float64
	totalTokens               *float64
	modelUsage                []pricedUsage
	observedModels            []modelRef
	stepDurations             []stepDurationEvidence
	warnings                  []string
}

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

func buildEvidence(challengeInputs []EvidenceInput, events []Event) extractedEvidence {
	evidence := extractedEvidence{}
	evidence.challengeInputValue, evidence.challengeInputChallengeID, evidence.warnings = resolveChallengeInputValue(challengeInputs)
	evidence.caseInput, evidence.caseInputReason = resolveCaseInput(challengeInputs)

	var (
		inputFromCalls  float64
		outputFromCalls float64
		totalFromCalls  float64
		usageFromCalls  bool
		stepStartedAt   = map[int]time.Time{}
		usageByModel    = map[string]*pricedUsage{}
		seenModels      = map[string]modelRef{}
	)

	for _, event := range events {
		payload := decodePayload(event.Payload)
		switch event.Type {
		case "system.run.started":
			if evidence.startedAt == nil {
				occurredAt := event.OccurredAt.UTC()
				evidence.startedAt = &occurredAt
			}
		case "system.step.started":
			if stepIndex, ok := intValue(payload, "step_index"); ok {
				stepStartedAt[stepIndex] = event.OccurredAt.UTC()
			}
		case "system.step.completed":
			stepIndex, ok := intValue(payload, "step_index")
			if !ok {
				break
			}
			startedAt, ok := stepStartedAt[stepIndex]
			if !ok {
				break
			}
			completedAt := event.OccurredAt.UTC()
			evidence.stepDurations = append(evidence.stepDurations, stepDurationEvidence{
				StepIndex:   stepIndex,
				DurationMS:  float64(completedAt.Sub(startedAt).Milliseconds()),
				StartedAt:   startedAt.Format(time.RFC3339Nano),
				CompletedAt: completedAt.Format(time.RFC3339Nano),
			})
		case "model.output.delta", "system.output.finalized":
			if evidence.firstOutputAt == nil {
				occurredAt := event.OccurredAt.UTC()
				evidence.firstOutputAt = &occurredAt
			}
			if event.Type == "system.output.finalized" && evidence.finalOutput == nil {
				if output, ok := stringValue(payload, "final_output"); ok {
					evidence.finalOutput = &output
				} else if output, ok := extractLooseString(payload["output"]); ok {
					evidence.finalOutput = &output
				}
			}
		case "system.run.completed":
			occurredAt := event.OccurredAt.UTC()
			evidence.terminalAt = &occurredAt
			completed := true
			evidence.completedSuccessfully = &completed
			if output, ok := stringValue(payload, "final_output"); ok {
				evidence.finalOutput = &output
			}
			if value, ok := numericValue(payload, "input_tokens"); ok {
				evidence.inputTokens = floatPtr(value)
			}
			if value, ok := numericValue(payload, "output_tokens"); ok {
				evidence.outputTokens = floatPtr(value)
			}
			if value, ok := numericValue(payload, "total_tokens"); ok {
				evidence.totalTokens = floatPtr(value)
			}
			if value, ok := usageValue(payload, "input_tokens"); ok && evidence.inputTokens == nil {
				evidence.inputTokens = floatPtr(value)
			}
			if value, ok := usageValue(payload, "output_tokens"); ok && evidence.outputTokens == nil {
				evidence.outputTokens = floatPtr(value)
			}
			if value, ok := usageValue(payload, "total_tokens"); ok && evidence.totalTokens == nil {
				evidence.totalTokens = floatPtr(value)
			}
			if value, ok := numericValue(payload, "latency_ms"); ok && evidence.startedAt != nil {
				evidence.terminalAt = timePtr(evidence.startedAt.Add(time.Duration(value) * time.Millisecond))
			}
		case "system.run.failed":
			occurredAt := event.OccurredAt.UTC()
			evidence.terminalAt = &occurredAt
			completed := false
			evidence.completedSuccessfully = &completed
			evidence.failureCount++
		case "tool.call.failed", "sandbox.command.failed":
			evidence.failureCount++
		case "model.call.completed":
			providerKey, _ := stringValue(payload, "provider_key")
			providerModelID, _ := stringValue(payload, "provider_model_id")
			if providerModelID == "" {
				providerModelID, _ = stringValue(payload, "model")
			}
			if providerKey != "" || providerModelID != "" {
				seenModels[providerKey+"\x00"+providerModelID] = modelRef{
					ProviderKey:     providerKey,
					ProviderModelID: providerModelID,
				}
			}
			if value, ok := usageValue(payload, "input_tokens"); ok {
				inputFromCalls += value
				usageFromCalls = true
				addModelUsage(usageByModel, providerKey, providerModelID, "input_tokens", value)
			}
			if value, ok := usageValue(payload, "output_tokens"); ok {
				outputFromCalls += value
				usageFromCalls = true
				addModelUsage(usageByModel, providerKey, providerModelID, "output_tokens", value)
			}
			if value, ok := usageValue(payload, "total_tokens"); ok {
				totalFromCalls += value
				usageFromCalls = true
				addModelUsage(usageByModel, providerKey, providerModelID, "total_tokens", value)
			}
		case "model.call.started":
			providerKey, _ := stringValue(payload, "provider_key")
			providerModelID, _ := stringValue(payload, "model")
			if providerModelID == "" {
				providerModelID, _ = stringValue(payload, "provider_model_id")
			}
			if providerKey != "" || providerModelID != "" {
				seenModels[providerKey+"\x00"+providerModelID] = modelRef{
					ProviderKey:     providerKey,
					ProviderModelID: providerModelID,
				}
			}
		}
	}

	if evidence.inputTokens == nil && usageFromCalls {
		evidence.inputTokens = floatPtr(inputFromCalls)
	}
	if evidence.outputTokens == nil && usageFromCalls {
		evidence.outputTokens = floatPtr(outputFromCalls)
	}
	if evidence.totalTokens == nil && usageFromCalls {
		if totalFromCalls > 0 {
			evidence.totalTokens = floatPtr(totalFromCalls)
		} else if evidence.inputTokens != nil && evidence.outputTokens != nil {
			evidence.totalTokens = floatPtr(*evidence.inputTokens + *evidence.outputTokens)
		}
	}
	for _, usage := range usageByModel {
		if usage.TotalTokens == 0 && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
			usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		}
		evidence.modelUsage = append(evidence.modelUsage, *usage)
	}
	for _, ref := range seenModels {
		evidence.observedModels = append(evidence.observedModels, ref)
	}
	sort.SliceStable(evidence.modelUsage, func(i, j int) bool {
		if evidence.modelUsage[i].ProviderKey == evidence.modelUsage[j].ProviderKey {
			return evidence.modelUsage[i].ProviderModelID < evidence.modelUsage[j].ProviderModelID
		}
		return evidence.modelUsage[i].ProviderKey < evidence.modelUsage[j].ProviderKey
	})
	sort.SliceStable(evidence.observedModels, func(i, j int) bool {
		if evidence.observedModels[i].ProviderKey == evidence.observedModels[j].ProviderKey {
			return evidence.observedModels[i].ProviderModelID < evidence.observedModels[j].ProviderModelID
		}
		return evidence.observedModels[i].ProviderKey < evidence.observedModels[j].ProviderKey
	})
	if evidence.finalOutput == nil {
		evidence.warnings = append(evidence.warnings, "final output evidence is unavailable")
	}
	if evidence.completedSuccessfully == nil {
		evidence.warnings = append(evidence.warnings, "terminal run evidence is unavailable")
	}

	return evidence
}

func evaluateValidators(validators []ValidatorDeclaration, evidence extractedEvidence) ([]ValidatorResult, []string) {
	results := make([]ValidatorResult, 0, len(validators))
	warnings := append([]string(nil), evidence.warnings...)
	for _, validator := range validators {
		result := ValidatorResult{
			Key:          validator.Key,
			Type:         validator.Type,
			Target:       validator.Target,
			ExpectedFrom: validator.ExpectedFrom,
		}

		actualValue, actualChallengeID, actualReason, actualErr := resolveEvidenceValue(validator.Target, evidence)
		expectedValue, expectedChallengeID, expectedReason, expectedErr := resolveEvidenceValue(validator.ExpectedFrom, evidence)

		if actualErr != nil {
			result.State = OutputStateError
			result.Reason = actualErr.Error()
			result.RawOutput = mustMarshalJSON(map[string]any{
				"state":  result.State,
				"reason": result.Reason,
			})
			results = append(results, result)
			continue
		}
		if expectedErr != nil {
			result.State = OutputStateError
			result.Reason = expectedErr.Error()
			result.RawOutput = mustMarshalJSON(map[string]any{
				"state":  result.State,
				"reason": result.Reason,
			})
			results = append(results, result)
			continue
		}
		if actualValue == nil || expectedValue == nil {
			result.State = OutputStateUnavailable
			result.Reason = firstNonEmpty(actualReason, expectedReason, "evidence is unavailable")
			if actualChallengeID != nil {
				result.ChallengeIdentityID = actualChallengeID
			} else {
				result.ChallengeIdentityID = expectedChallengeID
			}
			result.RawOutput = mustMarshalJSON(map[string]any{
				"state":  result.State,
				"reason": result.Reason,
			})
			results = append(results, result)
			continue
		}

		result.ActualValue = stringPtr(*actualValue)
		result.ExpectedValue = stringPtr(*expectedValue)
		if actualChallengeID != nil {
			result.ChallengeIdentityID = actualChallengeID
		} else {
			result.ChallengeIdentityID = expectedChallengeID
		}

		outcome := applyValidator(validator, *actualValue, *expectedValue)
		result.Verdict = outcome.verdict
		result.NormalizedScore = outcome.normalizedScore
		result.Reason = outcome.reason
		if outcome.verdict == "error" {
			result.State = OutputStateError
		} else {
			result.State = OutputStateAvailable
		}
		result.RawOutput = mustMarshalJSON(mergeEvidence(map[string]any{
			"state":            result.State,
			"verdict":          result.Verdict,
			"normalized_score": result.NormalizedScore,
			"reason":           result.Reason,
			"target":           validator.Target,
			"expected_from":    validator.ExpectedFrom,
			"actual_value":     result.ActualValue,
			"expected_value":   result.ExpectedValue,
		}, outcome.evidence))
		results = append(results, result)
	}
	return results, warnings
}

func applyValidator(validator ValidatorDeclaration, actual string, expected string) validatorOutcome {
	pass := false
	reason := ""

	switch validator.Type {
	case ValidatorTypeExactMatch:
		pass = actual == expected
	case ValidatorTypeContains:
		pass = strings.Contains(actual, expected)
	case ValidatorTypeRegexMatch:
		pattern, err := regexp.Compile(expected)
		if err != nil {
			return validatorOutcome{verdict: "error", reason: fmt.Sprintf("invalid regex pattern: %v", err)}
		}
		pass = pattern.MatchString(actual)
	case ValidatorTypeBooleanAssert:
		actualBool, err := strconvBool(actual)
		if err != nil {
			return validatorOutcome{verdict: "error", reason: fmt.Sprintf("parse actual boolean assertion value: %v", err)}
		}
		expectedBool, err := strconvBool(expected)
		if err != nil {
			return validatorOutcome{verdict: "error", reason: fmt.Sprintf("parse expected boolean assertion value: %v", err)}
		}
		pass = actualBool == expectedBool
	case ValidatorTypeJSONSchema:
		return validateJSONSchema(actual, expected)
	case ValidatorTypeJSONPathMatch:
		return validateJSONPathMatch(actual, expected)
	default:
		return validatorOutcome{verdict: "error", reason: fmt.Sprintf("unsupported validator type %q", validator.Type)}
	}

	if pass {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1), reason: reason}
	}
	return validatorOutcome{verdict: "fail", normalizedScore: floatPtr(0), reason: reason}
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

func modelCostMetric(evidence extractedEvidence, spec EvaluationSpec) (*float64, string, json.RawMessage) {
	return computeModelCostUSD(evidence, spec)
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

func computeModelCostUSD(evidence extractedEvidence, spec EvaluationSpec) (*float64, string, json.RawMessage) {
	if len(spec.Pricing.Models) == 0 {
		return nil, "model pricing is unavailable", nil
	}

	usageByModel := evidence.modelUsage
	if len(usageByModel) == 0 {
		ref, ok := singleObservedModel(evidence)
		if !ok {
			return nil, "model usage evidence is unavailable", mustMarshalJSON(map[string]any{
				"observed_models": evidence.observedModels,
			})
		}
		if evidence.inputTokens == nil && evidence.outputTokens == nil && evidence.totalTokens == nil {
			return nil, "model usage evidence is unavailable", mustMarshalJSON(map[string]any{
				"provider_key":      ref.ProviderKey,
				"provider_model_id": ref.ProviderModelID,
			})
		}
		usage := pricedUsage{
			ProviderKey:     ref.ProviderKey,
			ProviderModelID: ref.ProviderModelID,
		}
		if evidence.inputTokens != nil {
			usage.InputTokens = *evidence.inputTokens
		}
		if evidence.outputTokens != nil {
			usage.OutputTokens = *evidence.outputTokens
		}
		if evidence.totalTokens != nil {
			usage.TotalTokens = *evidence.totalTokens
		} else {
			usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		}
		usageByModel = []pricedUsage{usage}
	}

	type breakdownRow struct {
		ProviderKey     string  `json:"provider_key"`
		ProviderModelID string  `json:"provider_model_id"`
		InputTokens     float64 `json:"input_tokens"`
		OutputTokens    float64 `json:"output_tokens"`
		TotalTokens     float64 `json:"total_tokens"`
		CostUSD         float64 `json:"cost_usd"`
	}

	breakdown := make([]breakdownRow, 0, len(usageByModel))
	totalCost := 0.0
	for _, usage := range usageByModel {
		pricing, ok := lookupPricing(spec.Pricing.Models, usage.ProviderKey, usage.ProviderModelID)
		if !ok {
			return nil, fmt.Sprintf("model pricing is unavailable for provider %q model %q", usage.ProviderKey, usage.ProviderModelID), mustMarshalJSON(map[string]any{
				"provider_key":      usage.ProviderKey,
				"provider_model_id": usage.ProviderModelID,
			})
		}
		modelCost := (usage.InputTokens/1_000_000)*pricing.InputCostPerMillionTokens +
			(usage.OutputTokens/1_000_000)*pricing.OutputCostPerMillionTokens
		totalCost += modelCost
		breakdown = append(breakdown, breakdownRow{
			ProviderKey:     usage.ProviderKey,
			ProviderModelID: usage.ProviderModelID,
			InputTokens:     usage.InputTokens,
			OutputTokens:    usage.OutputTokens,
			TotalTokens:     usage.TotalTokens,
			CostUSD:         modelCost,
		})
	}

	return floatPtr(totalCost), "", mustMarshalJSON(map[string]any{
		"state":      OutputStateAvailable,
		"breakdown":  breakdown,
		"total_usd":  totalCost,
		"priced_run": true,
	})
}

func lookupPricing(models []ModelPricing, providerKey string, providerModelID string) (ModelPricing, bool) {
	normalizedModelID := normalizePricedModelID(providerModelID)
	for _, model := range models {
		if model.ProviderKey == providerKey && model.ProviderModelID == providerModelID {
			return model, true
		}
	}
	if normalizedModelID != providerModelID {
		for _, model := range models {
			if model.ProviderKey == providerKey && model.ProviderModelID == normalizedModelID {
				return model, true
			}
		}
	}
	return ModelPricing{}, false
}

func normalizePricedModelID(modelID string) string {
	trimmed := strings.TrimSpace(modelID)
	parts := strings.Split(trimmed, "-")
	if len(parts) < 4 {
		return trimmed
	}
	last := parts[len(parts)-1]
	secondLast := parts[len(parts)-2]
	thirdLast := parts[len(parts)-3]
	if len(thirdLast) == 4 && len(secondLast) == 2 && len(last) == 2 &&
		isDigits(thirdLast) && isDigits(secondLast) && isDigits(last) {
		return strings.Join(parts[:len(parts)-3], "-")
	}
	return trimmed
}

func singleObservedModel(evidence extractedEvidence) (modelRef, bool) {
	if len(evidence.observedModels) != 1 {
		return modelRef{}, false
	}
	return evidence.observedModels[0], true
}

func resolveEvidenceValue(source string, evidence extractedEvidence) (*string, *uuid.UUID, string, error) {
	switch {
	case source == "final_output":
		if evidence.finalOutput == nil {
			return nil, evidence.finalOutputChallengeID, "final output evidence is unavailable", nil
		}
		return stringPtr(*evidence.finalOutput), evidence.finalOutputChallengeID, "", nil
	case source == "run.final_output":
		if evidence.finalOutput == nil {
			return nil, evidence.finalOutputChallengeID, "final output evidence is unavailable", nil
		}
		return stringPtr(*evidence.finalOutput), evidence.finalOutputChallengeID, "", nil
	case source == "challenge_input":
		if evidence.challengeInputValue == nil {
			return nil, evidence.challengeInputChallengeID, "challenge input evidence is unavailable", nil
		}
		return stringPtr(*evidence.challengeInputValue), evidence.challengeInputChallengeID, "", nil
	case strings.HasPrefix(source, "case."):
		if evidence.caseInput == nil {
			return nil, evidence.challengeInputChallengeID, firstNonEmpty(evidence.caseInputReason, "case evidence is unavailable"), nil
		}
		return resolveCaseEvidence(source, *evidence.caseInput)
	case strings.HasPrefix(source, "artifact."):
		if evidence.caseInput == nil {
			return nil, evidence.challengeInputChallengeID, firstNonEmpty(evidence.caseInputReason, "case evidence is unavailable"), nil
		}
		return resolveArtifactEvidence(source, *evidence.caseInput)
	case strings.HasPrefix(source, "literal:"):
		value := strings.TrimPrefix(source, "literal:")
		return &value, nil, "", nil
	default:
		return nil, nil, "", fmt.Errorf("unsupported evidence source %q", source)
	}
}

func resolveChallengeInputValue(inputs []EvidenceInput) (*string, *uuid.UUID, []string) {
	if len(inputs) == 0 {
		return nil, nil, []string{"challenge input set is unavailable"}
	}
	if len(inputs) > 1 {
		return nil, nil, []string{"challenge input is ambiguous across multiple items"}
	}

	var decoded any
	if err := json.Unmarshal(inputs[0].Payload, &decoded); err == nil {
		if value, ok := extractLooseString(decoded); ok {
			return &value, uuidPtrOrNil(inputs[0].ChallengeIdentityID), nil
		}
	}

	payload := decodePayload(inputs[0].Payload)
	if value, ok := extractLooseString(payload); ok {
		return &value, uuidPtrOrNil(inputs[0].ChallengeIdentityID), nil
	}

	normalized := bytes.TrimSpace(inputs[0].Payload)
	if len(normalized) == 0 {
		return nil, uuidPtrOrNil(inputs[0].ChallengeIdentityID), []string{"challenge input payload is empty"}
	}
	value := string(normalized)
	return &value, uuidPtrOrNil(inputs[0].ChallengeIdentityID), nil
}

func resolveCaseInput(inputs []EvidenceInput) (*EvidenceInput, string) {
	if len(inputs) == 0 {
		return nil, "case evidence is unavailable"
	}
	// Case-oriented evidence currently resolves only when one canonical case is
	// in scope. Multi-case packs will need per-case scoring expansion later.
	if len(inputs) > 1 {
		return nil, "case evidence is ambiguous across multiple cases"
	}
	selected := inputs[0]
	return &selected, ""
}

func resolveCaseEvidence(source string, input EvidenceInput) (*string, *uuid.UUID, string, error) {
	segments := strings.Split(source, ".")
	if len(segments) < 2 {
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", source)
	}

	switch segments[1] {
	case "payload":
		return resolveJSONEvidence(input.Payload, segments[2:], uuidPtrOrNil(input.ChallengeIdentityID))
	case "inputs":
		if len(segments) < 3 || strings.TrimSpace(segments[2]) == "" {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", source)
		}
		value, ok := input.Inputs[segments[2]]
		if !ok {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("case input %q is unavailable", segments[2]), nil
		}
		return resolveEvidenceField(value, input, segments[3:])
	case "expectations":
		if len(segments) < 3 || strings.TrimSpace(segments[2]) == "" {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", source)
		}
		value, ok := input.Expectations[segments[2]]
		if !ok {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("case expectation %q is unavailable", segments[2]), nil
		}
		return resolveEvidenceField(value, input, segments[3:])
	default:
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", source)
	}
}

func resolveArtifactEvidence(source string, input EvidenceInput) (*string, *uuid.UUID, string, error) {
	segments := strings.Split(source, ".")
	if len(segments) < 2 || strings.TrimSpace(segments[1]) == "" {
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", source)
	}

	artifact, ok := input.Artifacts[segments[1]]
	if !ok {
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("artifact %q is unavailable", segments[1]), nil
	}
	return resolveArtifactValue(artifact, segments[2:], uuidPtrOrNil(input.ChallengeIdentityID))
}

func resolveEvidenceField(value EvidenceValue, input EvidenceInput, extra []string) (*string, *uuid.UUID, string, error) {
	return resolveEvidenceFieldWithDepth(value, input, extra, 0)
}

func resolveEvidenceFieldWithDepth(value EvidenceValue, input EvidenceInput, extra []string, depth int) (*string, *uuid.UUID, string, error) {
	if depth > 8 {
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("evidence reference chain exceeds maximum depth")
	}
	switch {
	case len(bytes.TrimSpace(value.Value)) > 0:
		return resolveJSONEvidence(value.Value, extra, uuidPtrOrNil(input.ChallengeIdentityID))
	case value.Source != "":
		switch {
		case strings.HasPrefix(value.Source, "input:"):
			inputKey := strings.TrimSpace(strings.TrimPrefix(value.Source, "input:"))
			if inputKey == "" {
				return nil, uuidPtrOrNil(input.ChallengeIdentityID), "referenced input is unavailable", nil
			}
			referenced, ok := input.Inputs[inputKey]
			if !ok {
				return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("case input %q is unavailable", inputKey), nil
			}
			return resolveEvidenceFieldWithDepth(referenced, input, extra, depth+1)
		case strings.HasPrefix(value.Source, "artifact:"):
			artifactKey := strings.TrimSpace(strings.TrimPrefix(value.Source, "artifact:"))
			if artifactKey == "" {
				return nil, uuidPtrOrNil(input.ChallengeIdentityID), "referenced artifact is unavailable", nil
			}
			artifact, ok := input.Artifacts[artifactKey]
			if !ok {
				return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("artifact %q is unavailable", artifactKey), nil
			}
			return resolveArtifactValue(artifact, extra, uuidPtrOrNil(input.ChallengeIdentityID))
		default:
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", value.Source)
		}
	case value.ArtifactKey != "":
		artifact, ok := input.Artifacts[value.ArtifactKey]
		if !ok {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("artifact %q is unavailable", value.ArtifactKey), nil
		}
		return resolveArtifactValue(artifact, extra, uuidPtrOrNil(input.ChallengeIdentityID))
	case strings.TrimSpace(value.Path) != "":
		encoded, err := json.Marshal(value.Path)
		if err != nil {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", err
		}
		return resolveJSONString(encoded, extra, uuidPtrOrNil(input.ChallengeIdentityID))
	default:
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), "evidence is unavailable", nil
	}
}

func resolveArtifactValue(artifact EvidenceArtifact, extra []string, challengeID *uuid.UUID) (*string, *uuid.UUID, string, error) {
	if len(extra) == 0 {
		return stringPtr(artifact.Path), challengeID, "", nil
	}
	payload, err := json.Marshal(map[string]any{
		"key":        artifact.Key,
		"kind":       artifact.Kind,
		"path":       artifact.Path,
		"media_type": artifact.MediaType,
	})
	if err != nil {
		return nil, challengeID, "", err
	}
	return resolveJSONEvidence(payload, extra, challengeID)
}

func resolveJSONEvidence(raw json.RawMessage, extra []string, challengeID *uuid.UUID) (*string, *uuid.UUID, string, error) {
	return resolveJSONString(raw, extra, challengeID)
}

func resolveJSONString(raw json.RawMessage, extra []string, challengeID *uuid.UUID) (*string, *uuid.UUID, string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, challengeID, "evidence is unavailable", nil
	}
	if len(extra) == 0 {
		value := stringifyEvidenceJSON(trimmed)
		return &value, challengeID, "", nil
	}

	var decoded any
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return nil, challengeID, "", fmt.Errorf("resolve evidence path: %w", err)
	}
	value, ok := walkEvidenceValue(decoded, extra)
	if !ok {
		return nil, challengeID, "evidence path is unavailable", nil
	}
	stringified, err := stringifyEvidenceValue(value)
	if err != nil {
		return nil, challengeID, "", err
	}
	return &stringified, challengeID, "", nil
}

func walkEvidenceValue(value any, segments []string) (any, bool) {
	current := value
	for _, segment := range segments {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func stringifyEvidenceJSON(raw []byte) string {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	stringified, err := stringifyEvidenceValue(decoded)
	if err != nil {
		return string(raw)
	}
	return stringified
}

func stringifyEvidenceValue(value any) (string, error) {
	if resolved, ok := extractLooseString(value); ok {
		return resolved, nil
	}
	switch typed := value.(type) {
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%v", typed)), nil
	case nil:
		return "null", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func decodePayload(payload json.RawMessage) map[string]any {
	if len(bytes.TrimSpace(payload)) == 0 {
		return map[string]any{}
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return map[string]any{}
	}
	return decoded
}

func stringValue(payload map[string]any, key string) (string, bool) {
	value, ok := payload[key]
	if !ok {
		return "", false
	}
	return extractLooseString(value)
}

func intValue(payload map[string]any, key string) (int, bool) {
	value, ok := numericValue(payload, key)
	if !ok {
		return 0, false
	}
	return int(value), true
}

func extractLooseString(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case json.RawMessage:
		return string(bytes.TrimSpace(typed)), len(bytes.TrimSpace(typed)) > 0
	case map[string]any:
		for _, candidate := range []string{"value", "content", "text", "answer"} {
			if item, ok := typed[candidate]; ok {
				if resolved, ok := extractLooseString(item); ok {
					return resolved, true
				}
			}
		}
	case []any:
		if len(typed) == 1 {
			return extractLooseString(typed[0])
		}
	}
	return "", false
}

func numericValue(payload map[string]any, key string) (float64, bool) {
	value, ok := payload[key]
	if !ok {
		return 0, false
	}
	return anyNumber(value)
}

func usageValue(payload map[string]any, key string) (float64, bool) {
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		return 0, false
	}
	return numericValue(usage, key)
}

func addModelUsage(usageByModel map[string]*pricedUsage, providerKey string, providerModelID string, field string, value float64) {
	key := providerKey + "\x00" + providerModelID
	usage, ok := usageByModel[key]
	if !ok {
		usage = &pricedUsage{
			ProviderKey:     providerKey,
			ProviderModelID: providerModelID,
		}
		usageByModel[key] = usage
	}
	switch field {
	case "input_tokens":
		usage.InputTokens += value
	case "output_tokens":
		usage.OutputTokens += value
	case "total_tokens":
		usage.TotalTokens += value
	}
}

func anyNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	}
	return 0, false
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func strconvBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("unsupported boolean value %q", value)
	}
}

func mustMarshalJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func floatPtr(value float64) *float64 {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func uuidPtrOrNil(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil {
		return nil
	}
	cloned := value
	return &cloned
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
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
