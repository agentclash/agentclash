package scoring

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
	ChallengeIdentityID uuid.UUID                   `json:"challenge_identity_id"`
	RegressionCaseID    *uuid.UUID                  `json:"regression_case_id,omitempty"`
	ChallengeKey        string                      `json:"challenge_key"`
	CaseKey             string                      `json:"case_key,omitempty"`
	ItemKey             string                      `json:"item_key"`
	Payload             json.RawMessage             `json:"payload"`
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
	Type           string          `json:"type"`
	Source         string          `json:"source"`
	SequenceNumber int64           `json:"sequence_number,omitempty"`
	OccurredAt     time.Time       `json:"occurred_at"`
	Payload        json.RawMessage `json:"payload"`
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
	LLMJudgeResults  []LLMJudgeResult    `json:"llm_judge_results,omitempty"`
	MetricResults    []MetricResult      `json:"metric_results"`
	DimensionResults []DimensionResult   `json:"dimension_results"`
	DimensionScores  map[string]*float64 `json:"dimension_scores"`
	OverallScore     *float64            `json:"overall_score,omitempty"`
	Passed           *bool               `json:"passed,omitempty"`
	OverallReason    string              `json:"overall_reason,omitempty"`
	Strategy         ScoringStrategy     `json:"strategy,omitempty"`
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
	RegressionCaseID    *uuid.UUID      `json:"regression_case_id,omitempty"`
	Reason              string          `json:"reason,omitempty"`
	Source              *Source         `json:"source,omitempty"`
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
	RegressionCaseID    *uuid.UUID      `json:"regression_case_id,omitempty"`
	Reason              string          `json:"reason,omitempty"`
	Metadata            json.RawMessage `json:"metadata"`
}

type DimensionResult struct {
	Dimension       string      `json:"dimension"`
	Score           *float64    `json:"score,omitempty"`
	State           OutputState `json:"state"`
	Reason          string      `json:"reason,omitempty"`
	BetterDirection string      `json:"better_direction,omitempty"`
}

type LLMJudgeResult struct {
	JudgeKey        string          `json:"judge_key"`
	Mode            string          `json:"mode"`
	NormalizedScore *float64        `json:"normalized_score,omitempty"`
	Payload         json.RawMessage `json:"payload"`
	Confidence      *string         `json:"confidence,omitempty"`
	Variance        *float64        `json:"variance,omitempty"`
	SampleCount     int32           `json:"sample_count"`
	ModelCount      int32           `json:"model_count"`
	Reason          string          `json:"reason,omitempty"`
}

var errJudgeModeUnsupported = errors.New("only deterministic evaluation specs are supported")

func DecodeDefinition(definition json.RawMessage) (EvaluationSpec, error) {
	if len(bytes.TrimSpace(definition)) == 0 {
		return EvaluationSpec{}, ValidationErrors{{Field: "evaluation_spec.definition", Message: "is required"}}
	}

	var spec EvaluationSpec
	if err := strictUnmarshal(definition, &spec); err != nil {
		return EvaluationSpec{}, fmt.Errorf("decode evaluation spec definition: %w", err)
	}

	normalizeEvaluationSpec(&spec)
	if err := ValidateEvaluationSpec(spec); err != nil {
		return EvaluationSpec{}, err
	}
	return spec, nil
}

func EvaluateRunAgent(input EvaluationInput, spec EvaluationSpec) (RunAgentEvaluation, error) {
	return evaluateRunAgentWithResolvedJudges(input, spec, nil, false)
}

func EvaluateRunAgentWithLLMJudgeResults(input EvaluationInput, spec EvaluationSpec, llmJudgeResults []LLMJudgeResult) (RunAgentEvaluation, error) {
	return evaluateRunAgentWithResolvedJudges(input, spec, llmJudgeResults, true)
}

func evaluateRunAgentWithResolvedJudges(
	input EvaluationInput,
	spec EvaluationSpec,
	llmJudgeResults []LLMJudgeResult,
	allowNonDeterministic bool,
) (RunAgentEvaluation, error) {
	normalizeEvaluationSpec(&spec)
	if err := ValidateEvaluationSpec(spec); err != nil {
		return RunAgentEvaluation{}, err
	}
	if spec.JudgeMode != JudgeModeDeterministic && !allowNonDeterministic {
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

	dimensionResults := evaluateDimensions(spec, evidence, validatorResults, metricResults, llmJudgeResults)
	warnings = append(warnings, dimensionWarnings(dimensionResults, spec.Scorecard.Dimensions)...)
	dimensionScores := make(map[string]*float64, len(dimensionResults))
	for _, result := range dimensionResults {
		score := result.Score
		if score != nil {
			cloned := *score
			score = &cloned
		}
		dimensionScores[result.Dimension] = score
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

	overallScore, passed, overallReason := computeOverallScore(spec, dimensionResults)

	return RunAgentEvaluation{
		RunAgentID:       input.RunAgentID,
		EvaluationSpecID: input.EvaluationSpecID,
		Status:           status,
		ValidatorResults: validatorResults,
		LLMJudgeResults:  cloneLLMJudgeResults(llmJudgeResults),
		MetricResults:    metricResults,
		DimensionResults: dimensionResults,
		DimensionScores:  dimensionScores,
		OverallScore:     overallScore,
		Passed:           passed,
		OverallReason:    overallReason,
		Strategy:         spec.Scorecard.Strategy,
		Warnings:         uniqueStrings(warnings),
	}, nil
}

func cloneLLMJudgeResults(results []LLMJudgeResult) []LLMJudgeResult {
	if len(results) == 0 {
		return nil
	}
	cloned := make([]LLMJudgeResult, 0, len(results))
	for _, result := range results {
		cloned = append(cloned, LLMJudgeResult{
			JudgeKey:        result.JudgeKey,
			Mode:            result.Mode,
			NormalizedScore: cloneFloat64Ptr(result.NormalizedScore),
			Payload:         cloneJSON(result.Payload),
			Confidence:      cloneStringPtr(result.Confidence),
			Variance:        cloneFloat64Ptr(result.Variance),
			SampleCount:     result.SampleCount,
			ModelCount:      result.ModelCount,
			Reason:          result.Reason,
		})
	}
	return cloned
}

type scoredDimension struct {
	decl  DimensionDeclaration
	value float64
}

func computeOverallScore(spec EvaluationSpec, results []DimensionResult) (*float64, *bool, string) {
	strategy := spec.Scorecard.Strategy
	if strategy == "" {
		strategy = ScoringStrategyWeighted
	}

	declByKey := make(map[string]DimensionDeclaration, len(spec.Scorecard.Dimensions))
	for _, d := range spec.Scorecard.Dimensions {
		declByKey[d.Key] = d
	}

	resultByKey := make(map[string]DimensionResult, len(results))
	available := make([]scoredDimension, 0, len(results))
	for _, r := range results {
		resultByKey[r.Dimension] = r
		if r.State != OutputStateAvailable || r.Score == nil {
			continue
		}
		decl, ok := declByKey[r.Dimension]
		if !ok {
			continue
		}
		available = append(available, scoredDimension{decl: decl, value: *r.Score})
	}
	if len(available) == 0 {
		switch strategy {
		case ScoringStrategyBinary:
			score := 0.0
			passedVal := false
			key, found := firstUnavailableRequiredDimension(spec.Scorecard.Dimensions, resultByKey, strategy)
			return &score, &passedVal, unavailableGateReason(strategy, key, found)
		case ScoringStrategyHybrid:
			if key, ok := firstUnavailableRequiredDimension(spec.Scorecard.Dimensions, resultByKey, strategy); ok {
				score := 0.0
				passedVal := false
				return &score, &passedVal, unavailableGateReason(strategy, key, true)
			}
		case ScoringStrategyWeighted:
			if key, ok := firstUnavailableRequiredDimension(spec.Scorecard.Dimensions, resultByKey, strategy); ok {
				passedVal := false
				return nil, &passedVal, unavailableGateReason(strategy, key, true)
			}
		}
		return nil, nil, "no dimensions produced an available score"
	}

	anyGateFailed := false
	firstFailedGate := ""
	for _, s := range available {
		gated := s.decl.Gate || strategy == ScoringStrategyBinary
		if !gated || s.decl.PassThreshold == nil {
			continue
		}
		if s.value < *s.decl.PassThreshold {
			anyGateFailed = true
			if firstFailedGate == "" {
				firstFailedGate = s.decl.Key
			}
		}
	}

	firstUnavailableGate, hasUnavailableGate := firstUnavailableRequiredDimension(spec.Scorecard.Dimensions, resultByKey, strategy)
	overallThreshold := spec.Scorecard.PassThreshold

	switch strategy {
	case ScoringStrategyBinary:
		passedVal := !anyGateFailed && !hasUnavailableGate
		score := 0.0
		if passedVal {
			score = 1.0
		}
		reason := ""
		if hasUnavailableGate {
			reason = unavailableGateReason(strategy, firstUnavailableGate, true)
		} else if !passedVal {
			reason = fmt.Sprintf("binary: dimension %q below pass_threshold", firstFailedGate)
		}
		return &score, &passedVal, reason

	case ScoringStrategyHybrid:
		if hasUnavailableGate {
			score := 0.0
			passedVal := false
			return &score, &passedVal, unavailableGateReason(strategy, firstUnavailableGate, true)
		}
		if anyGateFailed {
			score := 0.0
			passedVal := false
			return &score, &passedVal, fmt.Sprintf("hybrid: gated dimension %q below pass_threshold", firstFailedGate)
		}
		// Issue #147 criterion 7: hybrid's weighted overall score is computed
		// over NON-GATE dims only. Gates are hard pass/fail checks and must
		// not skew the weighted mean — otherwise a strict gate that barely
		// passes would drag the overall score down, and a soft dim that
		// tanks below threshold could be rescued by a gate with a high
		// score. The two axes stay independent.
		nonGated := make([]scoredDimension, 0, len(available))
		for _, s := range available {
			if !s.decl.Gate {
				nonGated = append(nonGated, s)
			}
		}
		// Degenerate case: every dim is a gate. The second clause of the
		// hybrid rule ("weighted non-gate >= threshold") is vacuously true
		// when there are no non-gate dims, so the verdict falls out of the
		// gate checks alone. Report score 1.0 because every possible
		// requirement has been satisfied.
		if len(nonGated) == 0 {
			score := 1.0
			passedVal := true
			return &score, &passedVal, "hybrid: all dimensions are gates and all passed"
		}
		score := weightedAverage(nonGated)
		if overallThreshold != nil && score < *overallThreshold {
			passedVal := false
			return &score, &passedVal, fmt.Sprintf("hybrid: non-gate weighted score %.4f below scorecard pass_threshold %.4f", score, *overallThreshold)
		}
		passedVal := true
		return &score, &passedVal, ""

	default:
		score := weightedAverage(available)
		passedVal := !anyGateFailed && !hasUnavailableGate
		reason := ""
		if hasUnavailableGate {
			reason = unavailableGateReason(strategy, firstUnavailableGate, true)
		} else if !passedVal {
			reason = fmt.Sprintf("weighted: gated dimension %q below pass_threshold", firstFailedGate)
		} else if overallThreshold != nil && score < *overallThreshold {
			passedVal = false
			reason = fmt.Sprintf("weighted: overall score %.4f below scorecard pass_threshold %.4f", score, *overallThreshold)
		}
		return &score, &passedVal, reason
	}
}

func firstUnavailableRequiredDimension(
	decls []DimensionDeclaration,
	results map[string]DimensionResult,
	strategy ScoringStrategy,
) (string, bool) {
	for _, decl := range decls {
		required := decl.Gate || strategy == ScoringStrategyBinary
		if !required {
			continue
		}
		result, ok := results[decl.Key]
		if !ok || result.State != OutputStateAvailable || result.Score == nil {
			return decl.Key, true
		}
	}
	return "", false
}

func unavailableGateReason(strategy ScoringStrategy, dimension string, found bool) string {
	if !found {
		return "required dimension is unavailable"
	}
	switch strategy {
	case ScoringStrategyBinary:
		return fmt.Sprintf("binary: dimension %q is unavailable", dimension)
	case ScoringStrategyHybrid:
		return fmt.Sprintf("hybrid: gated dimension %q is unavailable", dimension)
	default:
		return fmt.Sprintf("weighted: gated dimension %q is unavailable", dimension)
	}
}

func weightedAverage(items []scoredDimension) float64 {
	var totalWeight, weightedSum float64
	for _, it := range items {
		w := 1.0
		if it.decl.Weight != nil {
			w = *it.decl.Weight
		}
		totalWeight += w
		weightedSum += w * it.value
	}
	if totalWeight == 0 {
		var sum float64
		for _, it := range items {
			sum += it.value
		}
		return sum / float64(len(items))
	}
	return weightedSum / totalWeight
}
