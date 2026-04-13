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

