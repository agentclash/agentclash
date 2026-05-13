package voicescorecard

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/voiceeval"
)

const SchemaVersionV1 = "2026-05-13"

var ErrInvalidInput = errors.New("invalid voice scorecard input")

type Expectations struct {
	TaskSuccessField    string
	TaskSuccessValue    any
	ExpectedToolName    string
	ExpectedToolArgs    json.RawMessage
	ForbiddenPhrases    []string
	MaxTurns            int
	LatencyTargetMS     int64
	LatencyMaxMS        int64
	CostUSD             float64
	RequireInterruption bool
	RequireMediaPolicy  bool

	MinDialogueRetentionRatio      float64
	MinBackgroundPreservationRatio float64
	MaxSpeechDropRisk              float64
}

type Scorecard struct {
	SchemaVersion  string      `json:"schema_version"`
	Type           string      `json:"type"`
	OverallScore   float64     `json:"overall_score"`
	Passed         bool        `json:"passed"`
	HardGateFailed bool        `json:"hard_gate_failed"`
	Dimensions     []Dimension `json:"dimensions"`
	DegradedKeys   []string    `json:"degraded_keys,omitempty"`
}

type Dimension struct {
	Key      string          `json:"key"`
	Name     string          `json:"name"`
	Score    float64         `json:"score"`
	State    voiceeval.State `json:"state"`
	HardGate bool            `json:"hard_gate"`
	Checks   []CheckDetail   `json:"checks,omitempty"`
	Metrics  []MetricDetail  `json:"metrics,omitempty"`
}

type CheckDetail struct {
	Key     string          `json:"key"`
	State   voiceeval.State `json:"state"`
	Message string          `json:"message,omitempty"`
}

type MetricDetail struct {
	Key      string          `json:"key"`
	State    voiceeval.State `json:"state"`
	Value    *float64        `json:"value,omitempty"`
	ValueMS  *int64          `json:"value_ms,omitempty"`
	TargetMS *int64          `json:"target_ms,omitempty"`
	MaxMS    *int64          `json:"max_ms,omitempty"`
	Min      *float64        `json:"min,omitempty"`
	Max      *float64        `json:"max,omitempty"`
	ValueUSD *float64        `json:"value_usd,omitempty"`
	Message  string          `json:"message,omitempty"`
}

func Generate(input voiceeval.Input, expectations Expectations) (Scorecard, error) {
	if err := voiceeval.ValidateInput(input); err != nil {
		return Scorecard{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	if expectations.TaskSuccessField == "" {
		return Scorecard{}, fmt.Errorf("%w: task success field is required", ErrInvalidInput)
	}
	if expectations.ExpectedToolName == "" {
		return Scorecard{}, fmt.Errorf("%w: expected tool name is required", ErrInvalidInput)
	}
	if len(expectations.ExpectedToolArgs) == 0 || !json.Valid(expectations.ExpectedToolArgs) {
		return Scorecard{}, fmt.Errorf("%w: expected tool args must be valid JSON", ErrInvalidInput)
	}
	if expectations.MaxTurns <= 0 {
		return Scorecard{}, fmt.Errorf("%w: max turns must be positive", ErrInvalidInput)
	}

	dimensions := []Dimension{
		businessSuccessDimension(input, expectations),
		interactionQualityDimension(input, expectations),
		latencyDimension(input, expectations),
		robustnessDimension(input, expectations),
		toolDataCorrectnessDimension(input, expectations),
	}
	if expectations.RequireMediaPolicy {
		dimensions = append(dimensions, mediaPolicyDimension(input, expectations))
	}
	dimensions = append(dimensions, costDimension(expectations))
	scorecard := Scorecard{
		SchemaVersion: SchemaVersionV1,
		Type:          "voice",
		Dimensions:    dimensions,
	}
	scorecard.DegradedKeys = degradedKeys(dimensions)
	scorecard.HardGateFailed = hardGateFailed(dimensions)
	if scorecard.HardGateFailed {
		scorecard.OverallScore = 0
		scorecard.Passed = false
		return scorecard, nil
	}
	scorecard.OverallScore = roundScore(averageDimensionScore(dimensions))
	scorecard.Passed = scorecard.OverallScore == 1 && len(scorecard.DegradedKeys) == 0
	return scorecard, nil
}

func businessSuccessDimension(input voiceeval.Input, expectations Expectations) Dimension {
	checks := []CheckDetail{
		checkDetail(voiceeval.ValidateTaskSuccessStructured(input, voiceeval.KeyTaskSuccess, expectations.TaskSuccessField, expectations.TaskSuccessValue)),
		checkDetail(voiceeval.ValidateTaskSuccessEvent(input, "task_success_event", runevents.EventTypeSystemRunCompleted)),
	}
	return checkDimension("task_business_success", "Task / business success", true, checks)
}

func interactionQualityDimension(input voiceeval.Input, expectations Expectations) Dimension {
	checks := make([]CheckDetail, 0, max(1, len(expectations.ForbiddenPhrases)))
	if len(expectations.ForbiddenPhrases) == 0 {
		checks = append(checks, CheckDetail{Key: voiceeval.KeyNoForbiddenPhrase, State: voiceeval.StatePassed})
	}
	for _, phrase := range expectations.ForbiddenPhrases {
		checks = append(checks, checkDetail(voiceeval.ValidateNoForbiddenPhrase(input, voiceeval.KeyNoForbiddenPhrase, phrase)))
	}
	checks = append(checks, transcriptAvailableCheck(input))
	return checkDimension("interaction_quality", "Interaction quality", true, checks)
}

func latencyDimension(input voiceeval.Input, expectations Expectations) Dimension {
	totalDuration := metricDetail(voiceeval.MetricTotalDuration(input, voiceeval.KeyTotalDurationMS))
	latency := metricDetail(voiceeval.MetricEndOfUserTurnToFirstAgentOutput(input, voiceeval.KeyEndOfUserTurnToFirstAgentOutputMS))
	if expectations.LatencyTargetMS > 0 {
		latency.TargetMS = ptrInt64(expectations.LatencyTargetMS)
	}
	if expectations.LatencyMaxMS > 0 {
		latency.MaxMS = ptrInt64(expectations.LatencyMaxMS)
	}

	state := voiceeval.StatePassed
	score := 1.0
	for _, metric := range []MetricDetail{totalDuration, latency} {
		if metric.State == voiceeval.StateUnavailable {
			state = voiceeval.StateUnavailable
			score = math.Min(score, 0.5)
		}
		if metric.State == voiceeval.StateFailed {
			state = voiceeval.StateFailed
			score = 0
		}
	}
	if latency.State == voiceeval.StatePassed && latency.ValueMS != nil && expectations.LatencyMaxMS > 0 && *latency.ValueMS > expectations.LatencyMaxMS {
		state = voiceeval.StateFailed
		score = 0
		latency.State = voiceeval.StateFailed
		latency.Message = fmt.Sprintf("latency_ms = %d, max_ms = %d", *latency.ValueMS, expectations.LatencyMaxMS)
	} else if latency.State == voiceeval.StatePassed && latency.ValueMS != nil && expectations.LatencyTargetMS > 0 && *latency.ValueMS > expectations.LatencyTargetMS {
		state = voiceeval.StateFailed
		score = 0.5
		latency.State = voiceeval.StateFailed
		latency.Message = fmt.Sprintf("latency_ms = %d, target_ms = %d", *latency.ValueMS, expectations.LatencyTargetMS)
	}
	return Dimension{
		Key:     "latency",
		Name:    "Latency",
		Score:   score,
		State:   state,
		Metrics: []MetricDetail{totalDuration, latency},
	}
}

func robustnessDimension(input voiceeval.Input, expectations Expectations) Dimension {
	checks := []CheckDetail{
		checkDetail(voiceeval.ValidateMaxTurns(input, voiceeval.KeyMaxTurns, expectations.MaxTurns)),
	}
	if expectations.RequireInterruption {
		checks = append(checks, checkDetail(voiceeval.ValidateInterruptionHandled(input, voiceeval.KeyInterruptionHandled)))
	}
	return checkDimension("robustness", "Robustness", false, checks)
}

func toolDataCorrectnessDimension(input voiceeval.Input, expectations Expectations) Dimension {
	checks := []CheckDetail{
		checkDetail(voiceeval.ValidateExactToolCallName(input, voiceeval.KeyToolCallName, expectations.ExpectedToolName)),
		checkDetail(voiceeval.ValidateExactToolCallArguments(input, voiceeval.KeyToolCallArguments, expectations.ExpectedToolArgs)),
	}
	return checkDimension("tool_data_correctness", "Tool / data correctness", true, checks)
}

func mediaPolicyDimension(input voiceeval.Input, expectations Expectations) Dimension {
	minDialogueRetention := ratioOrDefault(expectations.MinDialogueRetentionRatio, 0.85)
	minBackgroundPreservation := ratioOrDefault(expectations.MinBackgroundPreservationRatio, 0.75)
	maxSpeechDropRisk := ratioOrDefault(expectations.MaxSpeechDropRisk, 0.15)

	dialogueRetention := ratioMetricDetail(voiceeval.MetricRecordedRatio(input, voiceeval.KeyDialogueRetentionRatio))
	dialogueRetention.Min = ptrFloat64(minDialogueRetention)
	backgroundPreservation := ratioMetricDetail(voiceeval.MetricRecordedRatio(input, voiceeval.KeyBackgroundPreservationRatio))
	backgroundPreservation.Min = ptrFloat64(minBackgroundPreservation)
	speechDropRisk := ratioMetricDetail(voiceeval.MetricRecordedRatio(input, voiceeval.KeySpeechDropRisk))
	speechDropRisk.Max = ptrFloat64(maxSpeechDropRisk)

	metrics := []MetricDetail{dialogueRetention, backgroundPreservation, speechDropRisk}
	state := voiceeval.StatePassed
	score := 1.0
	for idx := range metrics {
		metric := &metrics[idx]
		switch metric.State {
		case voiceeval.StateUnavailable:
			if state != voiceeval.StateFailed {
				state = voiceeval.StateUnavailable
				score = math.Min(score, 0.5)
			}
		case voiceeval.StatePassed:
			if metric.Value == nil {
				continue
			}
			switch metric.Key {
			case voiceeval.KeyDialogueRetentionRatio:
				if *metric.Value < minDialogueRetention {
					metric.State = voiceeval.StateFailed
					metric.Message = fmt.Sprintf("value = %.4f, min = %.4f", *metric.Value, minDialogueRetention)
					state = voiceeval.StateFailed
					score = 0
				}
			case voiceeval.KeyBackgroundPreservationRatio:
				if *metric.Value < minBackgroundPreservation {
					metric.State = voiceeval.StateFailed
					metric.Message = fmt.Sprintf("value = %.4f, min = %.4f", *metric.Value, minBackgroundPreservation)
					state = voiceeval.StateFailed
					score = 0
				}
			case voiceeval.KeySpeechDropRisk:
				if *metric.Value > maxSpeechDropRisk {
					metric.State = voiceeval.StateFailed
					metric.Message = fmt.Sprintf("value = %.4f, max = %.4f", *metric.Value, maxSpeechDropRisk)
					state = voiceeval.StateFailed
					score = 0
				}
			}
		}
	}

	return Dimension{
		Key:      "media_policy",
		Name:     "Media policy",
		Score:    score,
		State:    state,
		HardGate: true,
		Metrics:  metrics,
	}
}

func costDimension(expectations Expectations) Dimension {
	cost := expectations.CostUSD
	return Dimension{
		Key:   "cost",
		Name:  "Cost",
		Score: 1,
		State: voiceeval.StatePassed,
		Metrics: []MetricDetail{
			{
				Key:      "total_cost_usd",
				State:    voiceeval.StatePassed,
				ValueUSD: &cost,
			},
		},
	}
}

func checkDimension(key string, name string, hardGate bool, checks []CheckDetail) Dimension {
	state := voiceeval.StatePassed
	score := 1.0
	for _, check := range checks {
		switch check.State {
		case voiceeval.StateFailed:
			state = voiceeval.StateFailed
			score = 0
		case voiceeval.StateUnavailable:
			if state != voiceeval.StateFailed {
				state = voiceeval.StateUnavailable
				score = math.Min(score, 0.5)
			}
		}
	}
	return Dimension{
		Key:      key,
		Name:     name,
		Score:    score,
		State:    state,
		HardGate: hardGate,
		Checks:   checks,
	}
}

func checkDetail(result voiceeval.CheckResult) CheckDetail {
	return CheckDetail{
		Key:     result.Key,
		State:   result.State,
		Message: result.Message,
	}
}

func transcriptAvailableCheck(input voiceeval.Input) CheckDetail {
	for _, segment := range input.Trace.Segments {
		if segment.Kind == multimodaltrace.SegmentKindTranscriptFinal && segment.Transcript != nil && segment.Transcript.Text != "" {
			return CheckDetail{Key: "transcript_available", State: voiceeval.StatePassed}
		}
	}
	return CheckDetail{Key: "transcript_available", State: voiceeval.StateUnavailable, Message: "final transcript evidence not found"}
}

func metricDetail(result voiceeval.MetricResult) MetricDetail {
	detail := MetricDetail{
		Key:     result.Key,
		State:   result.State,
		Message: result.Message,
	}
	if result.State == voiceeval.StatePassed {
		detail.ValueMS = ptrInt64(result.ValueMS)
	}
	return detail
}

func ratioMetricDetail(result voiceeval.RatioMetricResult) MetricDetail {
	detail := MetricDetail{
		Key:     result.Key,
		State:   result.State,
		Message: result.Message,
	}
	if result.State == voiceeval.StatePassed {
		detail.Value = ptrFloat64(result.Value)
	}
	return detail
}

func degradedKeys(dimensions []Dimension) []string {
	var keys []string
	for _, dimension := range dimensions {
		if dimension.State == voiceeval.StateUnavailable {
			keys = append(keys, dimension.Key)
		}
		for _, check := range dimension.Checks {
			if check.State == voiceeval.StateUnavailable {
				keys = append(keys, check.Key)
			}
		}
		for _, metric := range dimension.Metrics {
			if metric.State == voiceeval.StateUnavailable {
				keys = append(keys, metric.Key)
			}
		}
	}
	return keys
}

func hardGateFailed(dimensions []Dimension) bool {
	for _, dimension := range dimensions {
		if dimension.HardGate && dimension.State == voiceeval.StateFailed {
			return true
		}
	}
	return false
}

func averageDimensionScore(dimensions []Dimension) float64 {
	if len(dimensions) == 0 {
		return 0
	}
	var total float64
	for _, dimension := range dimensions {
		total += dimension.Score
	}
	return total / float64(len(dimensions))
}

func roundScore(score float64) float64 {
	return math.Round(score*10000) / 10000
}

func ptrInt64(value int64) *int64 {
	return &value
}

func ptrFloat64(value float64) *float64 {
	return &value
}

func ratioOrDefault(value float64, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}
