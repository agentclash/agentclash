package voiceeval

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/agentclash/agentclash/backend/internal/runevents"
)

func TestVoiceValidatorsAgainstTextSimGolden(t *testing.T) {
	input := loadGoldenInput(t)

	checks := []struct {
		name string
		got  CheckResult
		want State
	}{
		{"task success structured pass", ValidateTaskSuccessStructured(input, KeyTaskSuccess, "resolution", "refund_created"), StatePassed},
		{"task success structured fail", ValidateTaskSuccessStructured(input, KeyTaskSuccess, "resolution", "not_found"), StateFailed},
		{"task success structured missing evidence", ValidateTaskSuccessStructured(withoutStructuredOutput(input), KeyTaskSuccess, "resolution", "refund_created"), StateUnavailable},
		{"task success event pass", ValidateTaskSuccessEvent(input, KeyTaskSuccess, runevents.EventTypeSystemRunCompleted), StatePassed},
		{"task success event fail", ValidateTaskSuccessEvent(withFailedRunEvent(input), KeyTaskSuccess, runevents.EventTypeSystemRunCompleted), StateFailed},
		{"task success event missing evidence", ValidateTaskSuccessEvent(withoutEvents(input), KeyTaskSuccess, runevents.EventTypeSystemRunCompleted), StateUnavailable},
		{"tool name pass", ValidateExactToolCallName(input, KeyToolCallName, "refund_api"), StatePassed},
		{"tool name fail", ValidateExactToolCallName(input, KeyToolCallName, "crm_lookup"), StateFailed},
		{"tool name missing evidence", ValidateExactToolCallName(withoutToolCall(input), KeyToolCallName, "refund_api"), StateUnavailable},
		{"tool args pass", ValidateExactToolCallArguments(input, KeyToolCallArguments, json.RawMessage(`{"reason":"duplicate_charge","account_id":"acct_123","amount_cents":4200,"currency":"USD","idempotency_key":"voice-fixture-42-refund"}`)), StatePassed},
		{"tool args fail", ValidateExactToolCallArguments(input, KeyToolCallArguments, json.RawMessage(`{"reason":"wrong"}`)), StateFailed},
		{"tool args missing evidence", ValidateExactToolCallArguments(withoutToolCall(input), KeyToolCallArguments, json.RawMessage(`{"reason":"duplicate_charge"}`)), StateUnavailable},
		{"tool args degraded", ValidateExactToolCallArguments(input, KeyToolCallArguments, json.RawMessage(`{`)), StateUnavailable},
		{"tool args degraded observed", ValidateExactToolCallArguments(withInvalidToolArguments(input), KeyToolCallArguments, json.RawMessage(`{"reason":"duplicate_charge"}`)), StateUnavailable},
		{"forbidden phrase pass", ValidateNoForbiddenPhrase(input, KeyNoForbiddenPhrase, "unauthorized transfer"), StatePassed},
		{"forbidden phrase fail", ValidateNoForbiddenPhrase(input, KeyNoForbiddenPhrase, "duplicate charge"), StateFailed},
		{"forbidden phrase degraded", ValidateNoForbiddenPhrase(input, KeyNoForbiddenPhrase, ""), StateUnavailable},
		{"max turns pass", ValidateMaxTurns(input, KeyMaxTurns, 1), StatePassed},
		{"max turns fail", ValidateMaxTurns(withSecondUserTurn(input), KeyMaxTurns, 1), StateFailed},
		{"max turns missing evidence", ValidateMaxTurns(withoutUserTurns(input), KeyMaxTurns, 1), StateUnavailable},
		{"interruption handled fail", ValidateInterruptionHandled(input, KeyInterruptionHandled), StateFailed},
		{"interruption handled pass", ValidateInterruptionHandled(withBargeIn(input), KeyInterruptionHandled), StatePassed},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got.State != tc.want {
				t.Fatalf("state = %q, want %q; result=%+v", tc.got.State, tc.want, tc.got)
			}
		})
	}
}

func TestVoiceMetricsAgainstTextSimGolden(t *testing.T) {
	input := loadGoldenInput(t)

	metrics := []struct {
		name    string
		got     MetricResult
		want    State
		wantMS  int64
		checkMS bool
	}{
		{"total duration", MetricTotalDuration(input, KeyTotalDurationMS), StatePassed, 5000, true},
		{"total duration degraded event order", MetricTotalDuration(withBackwardsEventTime(input), KeyTotalDurationMS), StateUnavailable, 0, false},
		{"latency from voice metric event", MetricEndOfUserTurnToFirstAgentOutput(input, KeyEndOfUserTurnToFirstAgentOutputMS), StatePassed, 1200, true},
		{"latency degraded metric event", MetricEndOfUserTurnToFirstAgentOutput(withInvalidVoiceMetric(input), KeyEndOfUserTurnToFirstAgentOutputMS), StateUnavailable, 0, false},
		{"latency missing metric value", MetricEndOfUserTurnToFirstAgentOutput(withMissingVoiceMetricValue(input), KeyEndOfUserTurnToFirstAgentOutputMS), StateUnavailable, 0, false},
		{"latency unavailable without user input", MetricEndOfUserTurnToFirstAgentOutput(withoutUserTurns(withoutTimingMarkers(withoutVoiceMetric(input))), KeyEndOfUserTurnToFirstAgentOutputMS), StateUnavailable, 0, false},
		{"latency unavailable when agent precedes user", MetricEndOfUserTurnToFirstAgentOutput(agentBeforeUser(withoutTimingMarkers(withoutVoiceMetric(input))), KeyEndOfUserTurnToFirstAgentOutputMS), StateUnavailable, 0, false},
		{"latency unavailable without explicit evidence", MetricEndOfUserTurnToFirstAgentOutput(withoutTimingMarkers(withoutVoiceMetric(input)), KeyEndOfUserTurnToFirstAgentOutputMS), StateUnavailable, 0, false},
		{"total duration unavailable with one event", MetricTotalDuration(withOnlyOneEvent(input), KeyTotalDurationMS), StateUnavailable, 0, false},
		{"total duration unavailable with one segment", MetricTotalDuration(withOnlyOneSegment(withoutEvents(input)), KeyTotalDurationMS), StateUnavailable, 0, false},
	}

	for _, tc := range metrics {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got.State != tc.want {
				t.Fatalf("state = %q, want %q; result=%+v", tc.got.State, tc.want, tc.got)
			}
			if tc.checkMS && tc.got.ValueMS != tc.wantMS {
				t.Fatalf("value_ms = %d, want %d", tc.got.ValueMS, tc.wantMS)
			}
		})
	}
}

func TestValidateInputRejectsMalformedEvidence(t *testing.T) {
	input := loadGoldenInput(t)
	input.Trace.Segments[0].SegmentID = ""

	if err := ValidateInput(input); err == nil {
		t.Fatalf("ValidateInput returned nil, want error")
	}
}

func TestStableKeys(t *testing.T) {
	keys := map[string]string{
		"task success":                  KeyTaskSuccess,
		"tool call name":                KeyToolCallName,
		"tool call arguments":           KeyToolCallArguments,
		"no forbidden phrase":           KeyNoForbiddenPhrase,
		"max turns":                     KeyMaxTurns,
		"interruption handled":          KeyInterruptionHandled,
		"total duration ms":             KeyTotalDurationMS,
		"end user turn to agent output": KeyEndOfUserTurnToFirstAgentOutputMS,
	}
	want := map[string]string{
		"task success":                  "task_success",
		"tool call name":                "tool_call_name",
		"tool call arguments":           "tool_call_arguments",
		"no forbidden phrase":           "no_forbidden_phrase",
		"max turns":                     "max_turns",
		"interruption handled":          "interruption_handled",
		"total duration ms":             "total_duration_ms",
		"end user turn to agent output": "end_of_user_turn_to_first_agent_output_ms",
	}
	for name, key := range keys {
		if key != want[name] {
			t.Fatalf("%s key = %q, want %q", name, key, want[name])
		}
	}
}

func loadGoldenInput(t *testing.T) Input {
	t.Helper()
	traceData, err := os.ReadFile("../voicetextsim/testdata/support_billing_expected_trace.json")
	if err != nil {
		t.Fatalf("read trace golden: %v", err)
	}
	eventsData, err := os.ReadFile("../voicetextsim/testdata/support_billing_expected_events.json")
	if err != nil {
		t.Fatalf("read events golden: %v", err)
	}
	var trace multimodaltrace.Trace
	if err := json.Unmarshal(traceData, &trace); err != nil {
		t.Fatalf("decode trace golden: %v", err)
	}
	var events []runevents.Envelope
	if err := json.Unmarshal(eventsData, &events); err != nil {
		t.Fatalf("decode events golden: %v", err)
	}
	input := Input{Trace: trace, Events: events}
	if err := ValidateInput(input); err != nil {
		t.Fatalf("ValidateInput(golden) returned error: %v", err)
	}
	return input
}

func withoutEvents(input Input) Input {
	input.Events = nil
	return input
}

func withFailedRunEvent(input Input) Input {
	input.Events = append([]runevents.Envelope(nil), input.Events...)
	failed := input.Events[len(input.Events)-1]
	failed.EventID = failed.EventID + ":failed"
	failed.SequenceNumber++
	failed.EventType = runevents.EventTypeSystemRunFailed
	failed.Summary.Status = "failed"
	input.Events = append(input.Events, failed)
	return input
}

func withoutVoiceMetric(input Input) Input {
	events := make([]runevents.Envelope, 0, len(input.Events))
	for _, event := range input.Events {
		if event.EventType == runevents.EventTypeVoiceMetricRecorded {
			continue
		}
		events = append(events, event)
	}
	input.Events = events
	return input
}

func withInvalidVoiceMetric(input Input) Input {
	input.Events = append([]runevents.Envelope(nil), input.Events...)
	for idx := range input.Events {
		if input.Events[idx].EventType == runevents.EventTypeVoiceMetricRecorded {
			input.Events[idx].Payload = json.RawMessage(`{"metric_key":"end_of_user_text_to_first_agent_text","value_ms":-1}`)
			return input
		}
	}
	return input
}

func withMissingVoiceMetricValue(input Input) Input {
	input.Events = append([]runevents.Envelope(nil), input.Events...)
	for idx := range input.Events {
		if input.Events[idx].EventType == runevents.EventTypeVoiceMetricRecorded {
			input.Events[idx].Payload = json.RawMessage(`{"metric_key":"end_of_user_text_to_first_agent_text"}`)
			return input
		}
	}
	return input
}

func withOnlyOneEvent(input Input) Input {
	input.Events = input.Events[:1]
	return input
}

func withOnlyOneSegment(input Input) Input {
	input.Trace.Segments = input.Trace.Segments[:1]
	return input
}

func withBackwardsEventTime(input Input) Input {
	input.Events = append([]runevents.Envelope(nil), input.Events...)
	input.Events[len(input.Events)-1].OccurredAt = input.Events[0].OccurredAt.Add(-time.Second)
	return input
}

func withInvalidToolArguments(input Input) Input {
	for idx := range input.Trace.Segments {
		if input.Trace.Segments[idx].ToolCall != nil {
			input.Trace.Segments[idx].ToolCall.Arguments = json.RawMessage(`{`)
			return input
		}
	}
	return input
}

func withoutStructuredOutput(input Input) Input {
	return filterSegments(input, func(segment multimodaltrace.Segment) bool {
		return segment.Kind != multimodaltrace.SegmentKindStructuredOutput
	})
}

func withoutToolCall(input Input) Input {
	return filterSegments(input, func(segment multimodaltrace.Segment) bool {
		return segment.Kind != multimodaltrace.SegmentKindToolCall
	})
}

func withoutTimingMarkers(input Input) Input {
	return filterSegments(input, func(segment multimodaltrace.Segment) bool {
		return segment.Kind != multimodaltrace.SegmentKindTimingMarker
	})
}

func withoutUserTurns(input Input) Input {
	return filterSegments(input, func(segment multimodaltrace.Segment) bool {
		return segment.Actor != multimodaltrace.ActorUser
	})
}

func withSecondUserTurn(input Input) Input {
	input.Trace.Segments = append(input.Trace.Segments, multimodaltrace.Segment{
		SegmentID:      "turn-002:user-text",
		SequenceNumber: int64(len(input.Trace.Segments) + 1),
		Kind:           multimodaltrace.SegmentKindTextInput,
		Actor:          multimodaltrace.ActorUser,
		OccurredAt:     input.Trace.Segments[len(input.Trace.Segments)-1].OccurredAt.Add(time.Second),
		Text: &multimodaltrace.TextPayload{
			Text: "I need another refund.",
		},
	})
	return input
}

func withBargeIn(input Input) Input {
	input.Trace.Segments = append(input.Trace.Segments, multimodaltrace.Segment{
		SegmentID:      "turn-001:barge-in",
		SequenceNumber: int64(len(input.Trace.Segments) + 1),
		Kind:           multimodaltrace.SegmentKindMediaControl,
		Actor:          multimodaltrace.ActorSystem,
		OccurredAt:     input.Trace.Segments[len(input.Trace.Segments)-1].OccurredAt.Add(time.Millisecond),
		MediaControl: &multimodaltrace.MediaControlPayload{
			Action:          "barge_in_detected",
			TargetSegmentID: "turn-001:agent-audio",
		},
	})
	return input
}

func agentBeforeUser(input Input) Input {
	for idx := range input.Trace.Segments {
		segment := input.Trace.Segments[idx]
		if segment.Actor == multimodaltrace.ActorAgent &&
			(segment.Kind == multimodaltrace.SegmentKindTextOutput || segment.Kind == multimodaltrace.SegmentKindAudioOutput) {
			input.Trace.Segments[idx].OccurredAt = input.Trace.Segments[0].OccurredAt.Add(-time.Second)
			return input
		}
	}
	return input
}

func filterSegments(input Input, keep func(multimodaltrace.Segment) bool) Input {
	segments := make([]multimodaltrace.Segment, 0, len(input.Trace.Segments))
	nextSequence := int64(1)
	for _, segment := range input.Trace.Segments {
		if !keep(segment) {
			continue
		}
		segment.SequenceNumber = nextSequence
		nextSequence++
		segments = append(segments, segment)
	}
	input.Trace.Segments = segments
	return input
}
