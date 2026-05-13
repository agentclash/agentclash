package voiceeval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/agentclash/agentclash/backend/internal/runevents"
)

var ErrInvalidInput = errors.New("invalid voice eval input")

type Input struct {
	Trace  multimodaltrace.Trace
	Events []runevents.Envelope
}

type State string

const (
	StatePassed      State = "passed"
	StateFailed      State = "failed"
	StateUnavailable State = "unavailable"
)

const (
	KeyTaskSuccess                         = "task_success"
	KeyToolCallName                        = "tool_call_name"
	KeyToolCallArguments                   = "tool_call_arguments"
	KeyNoForbiddenPhrase                   = "no_forbidden_phrase"
	KeyMaxTurns                            = "max_turns"
	KeyInterruptionHandled                 = "interruption_handled"
	KeyTotalDurationMS                     = "total_duration_ms"
	KeyEndOfUserTurnToFirstAgentOutputMS   = "end_of_user_turn_to_first_agent_output_ms"
	KeyEndOfUserTextToFirstAgentTextLegacy = "end_of_user_text_to_first_agent_text"
	KeyDialogueRetentionRatio              = "dialogue_retention_ratio"
	KeyBackgroundPreservationRatio         = "background_preservation_ratio"
	KeySpeechDropRisk                      = "speech_drop_risk"
)

type CheckResult struct {
	Key     string
	State   State
	Message string
}

type MetricResult struct {
	Key     string
	State   State
	ValueMS int64
	Message string
}

type RatioMetricResult struct {
	Key     string
	State   State
	Value   float64
	Message string
}

func ValidateTaskSuccessStructured(input Input, key string, field string, expected any) CheckResult {
	output, ok := latestStructuredOutput(input.Trace)
	if !ok {
		return unavailableCheck(key, "structured output not found")
	}
	actual, ok := output[field]
	if !ok {
		return unavailableCheck(key, fmt.Sprintf("structured output field %q not found", field))
	}
	if valuesEqual(actual, expected) {
		return passedCheck(key)
	}
	return failedCheck(key, fmt.Sprintf("structured output field %q = %v, want %v", field, actual, expected))
}

func ValidateTaskSuccessEvent(input Input, key string, expectedEventType runevents.Type) CheckResult {
	if len(input.Events) == 0 {
		return unavailableCheck(key, "events not found")
	}
	for _, event := range input.Events {
		if event.EventType == runevents.EventTypeSystemRunFailed {
			return failedCheck(key, "system run failed event found")
		}
	}
	for _, event := range input.Events {
		if event.EventType == expectedEventType {
			return passedCheck(key)
		}
	}
	return unavailableCheck(key, fmt.Sprintf("event %q not found", expectedEventType))
}

func ValidateExactToolCallName(input Input, key string, expectedToolName string) CheckResult {
	call, ok := firstToolCall(input.Trace)
	if !ok {
		return unavailableCheck(key, "tool call not found")
	}
	if call.ToolName == expectedToolName {
		return passedCheck(key)
	}
	return failedCheck(key, fmt.Sprintf("tool name = %q, want %q", call.ToolName, expectedToolName))
}

func ValidateExactToolCallArguments(input Input, key string, expected json.RawMessage) CheckResult {
	call, ok := firstToolCall(input.Trace)
	if !ok {
		return unavailableCheck(key, "tool call not found")
	}
	if !json.Valid(expected) {
		return unavailableCheck(key, "expected arguments are not valid JSON")
	}
	if !json.Valid(call.Arguments) {
		return unavailableCheck(key, "observed arguments are not valid JSON")
	}
	if bytes.Equal(normalizeJSON(call.Arguments), normalizeJSON(expected)) {
		return passedCheck(key)
	}
	return failedCheck(key, fmt.Sprintf("tool arguments = %s, want %s", normalizeJSON(call.Arguments), normalizeJSON(expected)))
}

func ValidateNoForbiddenPhrase(input Input, key string, phrase string) CheckResult {
	needle := strings.ToLower(strings.TrimSpace(phrase))
	if needle == "" {
		return unavailableCheck(key, "forbidden phrase is empty")
	}
	for _, text := range allTraceText(input.Trace) {
		if strings.Contains(strings.ToLower(text), needle) {
			return failedCheck(key, fmt.Sprintf("forbidden phrase %q found", phrase))
		}
	}
	return passedCheck(key)
}

func ValidateMaxTurns(input Input, key string, maxTurns int) CheckResult {
	if maxTurns <= 0 {
		return unavailableCheck(key, "max_turns must be positive")
	}
	turns := map[string]struct{}{}
	for _, segment := range input.Trace.Segments {
		if segment.Actor == multimodaltrace.ActorUser {
			turnID := segmentTurnID(segment.SegmentID)
			if turnID != "" {
				turns[turnID] = struct{}{}
			}
		}
	}
	if len(turns) == 0 {
		return unavailableCheck(key, "user turns not found")
	}
	if len(turns) <= maxTurns {
		return passedCheck(key)
	}
	return failedCheck(key, fmt.Sprintf("turns = %d, max_turns = %d", len(turns), maxTurns))
}

func ValidateInterruptionHandled(input Input, key string) CheckResult {
	for _, segment := range input.Trace.Segments {
		if segment.MediaControl != nil && segment.MediaControl.Action == "barge_in_detected" {
			return passedCheck(key)
		}
	}
	for _, event := range input.Events {
		if event.EventType == runevents.EventTypeBargeInDetected {
			return passedCheck(key)
		}
	}
	return failedCheck(key, "barge-in marker not found")
}

func MetricTotalDuration(input Input, key string) MetricResult {
	if len(input.Events) > 0 {
		first, last, ok := orderedEventBounds(input.Events)
		if !ok {
			return unavailableMetric(key, "event timestamps are not monotonic")
		}
		return MetricResult{Key: key, State: StatePassed, ValueMS: millis(last.Sub(first))}
	}
	first, last, ok := orderedSegmentBounds(input.Trace.Segments)
	if !ok {
		return unavailableMetric(key, "monotonic timestamp evidence not found")
	}
	return MetricResult{Key: key, State: StatePassed, ValueMS: millis(last.Sub(first))}
}

func MetricEndOfUserTurnToFirstAgentOutput(input Input, key string) MetricResult {
	if valueMS, found, valid := metricRecordedValue(input.Events, key); found {
		if !valid {
			return unavailableMetric(key, "voice metric event is invalid")
		}
		return MetricResult{Key: key, State: StatePassed, ValueMS: valueMS}
	}
	if key == KeyEndOfUserTurnToFirstAgentOutputMS {
		if valueMS, found, valid := metricRecordedValue(input.Events, KeyEndOfUserTextToFirstAgentTextLegacy); found {
			if !valid {
				return unavailableMetric(key, "voice metric event is invalid")
			}
			return MetricResult{Key: key, State: StatePassed, ValueMS: valueMS}
		}
	}
	for _, segment := range input.Trace.Segments {
		if segment.Kind == multimodaltrace.SegmentKindTimingMarker && segment.TimingMarker != nil &&
			(segment.TimingMarker.Key == key || segment.TimingMarker.Key == KeyEndOfUserTextToFirstAgentTextLegacy) {
			return MetricResult{Key: key, State: StatePassed, ValueMS: segment.TimingMarker.ValueMS}
		}
	}
	return unavailableMetric(key, "explicit end-of-user-turn latency evidence not found")
}

func MetricRecordedRatio(input Input, key string) RatioMetricResult {
	if value, found, valid := ratioMetricRecordedValue(input.Events, key); found {
		if !valid {
			return unavailableRatioMetric(key, "voice ratio metric event is invalid")
		}
		return RatioMetricResult{Key: key, State: StatePassed, Value: value}
	}
	return unavailableRatioMetric(key, "voice ratio metric evidence not found")
}

func ValidateInput(input Input) error {
	if err := input.Trace.Validate(); err != nil {
		return fmt.Errorf("%w: trace: %w", ErrInvalidInput, err)
	}
	for idx, event := range input.Events {
		if err := event.ValidatePersisted(); err != nil {
			return fmt.Errorf("%w: events[%d]: %w", ErrInvalidInput, idx, err)
		}
	}
	return nil
}

func latestStructuredOutput(trace multimodaltrace.Trace) (map[string]any, bool) {
	for idx := len(trace.Segments) - 1; idx >= 0; idx-- {
		segment := trace.Segments[idx]
		if segment.StructuredOutput == nil {
			continue
		}
		var output map[string]any
		if err := json.Unmarshal(segment.StructuredOutput.Output, &output); err != nil {
			return nil, false
		}
		return output, true
	}
	return nil, false
}

func firstToolCall(trace multimodaltrace.Trace) (multimodaltrace.ToolCallPayload, bool) {
	for _, segment := range trace.Segments {
		if segment.ToolCall != nil {
			return *segment.ToolCall, true
		}
	}
	return multimodaltrace.ToolCallPayload{}, false
}

func allTraceText(trace multimodaltrace.Trace) []string {
	var texts []string
	for _, segment := range trace.Segments {
		if segment.Text != nil {
			texts = append(texts, segment.Text.Text)
		}
		if segment.Transcript != nil {
			texts = append(texts, segment.Transcript.Text)
		}
		if segment.StructuredOutput != nil {
			texts = append(texts, string(segment.StructuredOutput.Output))
		}
	}
	return texts
}

func metricRecordedValue(events []runevents.Envelope, key string) (int64, bool, bool) {
	for _, event := range events {
		if event.EventType != runevents.EventTypeVoiceMetricRecorded || event.Summary.MetricKey != key {
			continue
		}
		var payload struct {
			ValueMS *int64 `json:"value_ms"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.ValueMS == nil || *payload.ValueMS < 0 {
			return 0, true, false
		}
		return *payload.ValueMS, true, true
	}
	return 0, false, false
}

func ratioMetricRecordedValue(events []runevents.Envelope, key string) (float64, bool, bool) {
	for _, event := range events {
		if event.EventType != runevents.EventTypeVoiceMetricRecorded || event.Summary.MetricKey != key {
			continue
		}
		var payload struct {
			Value      *float64 `json:"value"`
			ValueRatio *float64 `json:"value_ratio"`
			Ratio      *float64 `json:"ratio"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return 0, true, false
		}
		value := firstFloat(payload.Value, payload.ValueRatio, payload.Ratio)
		if value == nil || *value < 0 || *value > 1 {
			return 0, true, false
		}
		return *value, true, true
	}
	return 0, false, false
}

func firstFloat(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func orderedEventBounds(events []runevents.Envelope) (time.Time, time.Time, bool) {
	if len(events) < 2 {
		return time.Time{}, time.Time{}, false
	}
	first, last := events[0].OccurredAt, events[0].OccurredAt
	for _, event := range events[1:] {
		if event.OccurredAt.Before(last) {
			return time.Time{}, time.Time{}, false
		}
		last = event.OccurredAt
	}
	return first, last, true
}

func orderedSegmentBounds(segments []multimodaltrace.Segment) (time.Time, time.Time, bool) {
	if len(segments) < 2 {
		return time.Time{}, time.Time{}, false
	}
	first, last := segments[0].OccurredAt, segments[0].OccurredAt
	for _, segment := range segments[1:] {
		if segment.OccurredAt.Before(last) {
			return time.Time{}, time.Time{}, false
		}
		last = segment.OccurredAt
	}
	return first, last, true
}

func segmentTurnID(segmentID string) string {
	before, _, ok := strings.Cut(segmentID, ":")
	if !ok {
		return ""
	}
	return before
}

func valuesEqual(actual any, expected any) bool {
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return false
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return false
	}
	return bytes.Equal(normalizeJSON(actualJSON), normalizeJSON(expectedJSON))
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return append(json.RawMessage(nil), raw...)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return append(json.RawMessage(nil), raw...)
	}
	return encoded
}

func passedCheck(key string) CheckResult {
	return CheckResult{Key: key, State: StatePassed}
}

func failedCheck(key string, message string) CheckResult {
	return CheckResult{Key: key, State: StateFailed, Message: message}
}

func unavailableCheck(key string, message string) CheckResult {
	return CheckResult{Key: key, State: StateUnavailable, Message: message}
}

func unavailableMetric(key string, message string) MetricResult {
	return MetricResult{Key: key, State: StateUnavailable, Message: message}
}

func unavailableRatioMetric(key string, message string) RatioMetricResult {
	return RatioMetricResult{Key: key, State: StateUnavailable, Message: message}
}

func millis(duration time.Duration) int64 {
	return int64(duration / time.Millisecond)
}
