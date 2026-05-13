package multimodaltrace

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const (
	testRunID      = "11111111-1111-1111-1111-111111111111"
	testRunAgentID = "22222222-2222-2222-2222-222222222222"
)

func TestTraceRoundTripGoldenFixtures(t *testing.T) {
	fixtures := map[string]string{
		"text_only":  `{"trace_id":"trace-text-only","schema_version":"2026-05-13","run_id":"11111111-1111-1111-1111-111111111111","run_agent_id":"22222222-2222-2222-2222-222222222222","segments":[{"segment_id":"seg-001","sequence_number":1,"kind":"text_input","actor":"user","occurred_at":"2026-05-13T09:00:00Z","text":{"text":"I was charged twice.","language":"en-US"}},{"segment_id":"seg-002","sequence_number":2,"kind":"text_output","actor":"agent","occurred_at":"2026-05-13T09:00:02Z","text":{"text":"I can help check the duplicate charge.","language":"en-US"}}]}`,
		"audio_only": `{"trace_id":"trace-audio-only","schema_version":"2026-05-13","run_id":"11111111-1111-1111-1111-111111111111","run_agent_id":"22222222-2222-2222-2222-222222222222","segments":[{"segment_id":"seg-001","sequence_number":1,"kind":"audio_input","actor":"user","occurred_at":"2026-05-13T09:01:00Z","audio":{"artifact_ref":"artifacts/caller.wav","format":"wav","channel":"caller","duration_ms":1400}},{"segment_id":"seg-002","sequence_number":2,"kind":"audio_output","actor":"agent","occurred_at":"2026-05-13T09:01:02Z","audio":{"artifact_ref":"artifacts/agent.wav","format":"wav","channel":"agent","duration_ms":2100}}]}`,
		"mixed":      `{"trace_id":"trace-mixed-support","schema_version":"2026-05-13","run_id":"11111111-1111-1111-1111-111111111111","run_agent_id":"22222222-2222-2222-2222-222222222222","segments":[{"segment_id":"seg-001","sequence_number":1,"kind":"audio_input","actor":"user","occurred_at":"2026-05-13T09:02:00Z","audio":{"artifact_ref":"artifacts/caller-001.wav","format":"wav","channel":"caller","duration_ms":1200}},{"segment_id":"seg-002","sequence_number":2,"kind":"transcript_final","actor":"system","occurred_at":"2026-05-13T09:02:01Z","transcript":{"text":"Please refund the duplicate charge.","language":"en-US","source_segment_id":"seg-001"}},{"segment_id":"seg-003","sequence_number":3,"kind":"tool_call","actor":"agent","occurred_at":"2026-05-13T09:02:02Z","tool_call":{"call_id":"call-refund-1","tool_name":"refund_api","arguments":{"amount_cents":4200,"reason":"duplicate_charge"}}},{"segment_id":"seg-004","sequence_number":4,"kind":"tool_result","actor":"tool","occurred_at":"2026-05-13T09:02:03Z","tool_result":{"call_id":"call-refund-1","tool_name":"refund_api","result":{"status":"created","refund_id":"rf_123"}}},{"segment_id":"seg-005","sequence_number":5,"kind":"structured_output","actor":"agent","occurred_at":"2026-05-13T09:02:04Z","structured_output":{"schema_ref":"voice.support.resolution.v1","output":{"resolution":"refund_created","refund_id":"rf_123"}}},{"segment_id":"seg-006","sequence_number":6,"kind":"timing_marker","actor":"evaluator","occurred_at":"2026-05-13T09:02:04Z","timing_marker":{"key":"end_of_speech_to_first_agent_output","value_ms":800}},{"segment_id":"seg-007","sequence_number":7,"kind":"media_control","actor":"system","occurred_at":"2026-05-13T09:02:05Z","media_control":{"action":"audio_buffer_cleared","target_segment_id":"seg-001"}}]}`,
	}

	for name, golden := range fixtures {
		t.Run(name, func(t *testing.T) {
			var trace Trace
			if err := json.Unmarshal([]byte(golden), &trace); err != nil {
				t.Fatalf("unmarshal golden trace: %v", err)
			}
			if err := trace.Validate(); err != nil {
				t.Fatalf("validate golden trace: %v", err)
			}
			roundTripped, err := json.Marshal(trace)
			if err != nil {
				t.Fatalf("marshal golden trace: %v", err)
			}
			if string(roundTripped) != golden {
				t.Fatalf("round-trip JSON mismatch\nwant: %s\n got: %s", golden, string(roundTripped))
			}
		})
	}
}

func TestTraceValidateRequiresCoreFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Trace)
		wantErr string
	}{
		{
			name: "trace_id",
			mutate: func(trace *Trace) {
				trace.TraceID = ""
			},
			wantErr: "trace_id is required",
		},
		{
			name: "schema_version",
			mutate: func(trace *Trace) {
				trace.SchemaVersion = "bad-version"
			},
			wantErr: "invalid multimodal trace schema version",
		},
		{
			name: "run_id",
			mutate: func(trace *Trace) {
				trace.RunID = uuid.Nil
			},
			wantErr: "run_id is required",
		},
		{
			name: "run_agent_id",
			mutate: func(trace *Trace) {
				trace.RunAgentID = uuid.Nil
			},
			wantErr: "run_agent_id is required",
		},
		{
			name: "segment_id",
			mutate: func(trace *Trace) {
				trace.Segments[0].SegmentID = ""
			},
			wantErr: "segment_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := validTrace()
			tt.mutate(&trace)
			err := trace.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestTraceValidateSegmentKinds(t *testing.T) {
	tests := []struct {
		name    string
		segment Segment
	}{
		{name: "audio_input", segment: validSegment(SegmentKindAudioInput)},
		{name: "audio_output", segment: validSegment(SegmentKindAudioOutput)},
		{name: "text_input", segment: validSegment(SegmentKindTextInput)},
		{name: "text_output", segment: validSegment(SegmentKindTextOutput)},
		{name: "transcript_partial", segment: validSegment(SegmentKindTranscriptPartial)},
		{name: "transcript_final", segment: validSegment(SegmentKindTranscriptFinal)},
		{name: "tool_call", segment: validSegment(SegmentKindToolCall)},
		{name: "tool_result", segment: validSegment(SegmentKindToolResult)},
		{name: "structured_output", segment: validSegment(SegmentKindStructuredOutput)},
		{name: "timing_marker", segment: validSegment(SegmentKindTimingMarker)},
		{name: "media_control", segment: validSegment(SegmentKindMediaControl)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.segment.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}

	segment := validSegment(SegmentKindTextInput)
	segment.Kind = SegmentKind("unknown_kind")
	err := segment.Validate()
	if !errors.Is(err, ErrInvalidSegmentKind) {
		t.Fatalf("Validate() error = %v, want ErrInvalidSegmentKind", err)
	}
}

func TestTraceValidateSegmentOrdering(t *testing.T) {
	t.Run("non_positive_sequence_number", func(t *testing.T) {
		trace := validTrace()
		trace.Segments[0].SequenceNumber = 0
		err := trace.Validate()
		if err == nil || !strings.Contains(err.Error(), "sequence_number must be greater than zero") {
			t.Fatalf("Validate() error = %v, want non-positive sequence error", err)
		}
	})

	t.Run("duplicate_sequence_number", func(t *testing.T) {
		trace := validTrace()
		trace.Segments = append(trace.Segments, validSegment(SegmentKindTextOutput))
		trace.Segments[1].SegmentID = "seg-002"
		trace.Segments[1].SequenceNumber = trace.Segments[0].SequenceNumber
		err := trace.Validate()
		if err == nil || !strings.Contains(err.Error(), "duplicate sequence_number") {
			t.Fatalf("Validate() error = %v, want duplicate sequence error", err)
		}
	})

	t.Run("sequence_numbers_must_increase", func(t *testing.T) {
		trace := validTrace()
		trace.Segments = append(trace.Segments, validSegment(SegmentKindTextOutput))
		trace.Segments[1].SegmentID = "seg-002"
		trace.Segments[0].SequenceNumber = 2
		trace.Segments[1].SequenceNumber = 1
		err := trace.Validate()
		if err == nil || !strings.Contains(err.Error(), "sequence_number must increase monotonically") {
			t.Fatalf("Validate() error = %v, want monotonic sequence error", err)
		}
	})
}

func TestTraceValidateReferences(t *testing.T) {
	traceTests := []struct {
		name    string
		mutate  func(*Trace)
		wantErr string
	}{
		{
			name: "transcript_source_segment_missing",
			mutate: func(trace *Trace) {
				trace.Segments[1].Transcript.SourceSegmentID = "missing-segment"
			},
			wantErr: `transcript.source_segment_id "missing-segment" does not reference a prior segment`,
		},
		{
			name: "transcript_source_segment_must_be_audio",
			mutate: func(trace *Trace) {
				textSegment := validSegment(SegmentKindTextInput)
				textSegment.SegmentID = trace.Segments[0].SegmentID
				textSegment.SequenceNumber = trace.Segments[0].SequenceNumber
				textSegment.OccurredAt = trace.Segments[0].OccurredAt
				trace.Segments[0] = textSegment
			},
			wantErr: `transcript.source_segment_id "seg-001" must reference an audio segment`,
		},
		{
			name: "media_control_target_missing",
			mutate: func(trace *Trace) {
				trace.Segments[5].MediaControl.TargetSegmentID = "missing-segment"
			},
			wantErr: `media_control.target_segment_id "missing-segment" does not reference a prior segment`,
		},
		{
			name: "tool_result_call_missing",
			mutate: func(trace *Trace) {
				trace.Segments[3].ToolResult.CallID = "missing-call"
			},
			wantErr: `tool_result.call_id "missing-call" does not reference a prior tool_call`,
		},
		{
			name: "tool_result_tool_name_mismatch",
			mutate: func(trace *Trace) {
				trace.Segments[3].ToolResult.ToolName = "different_tool"
			},
			wantErr: `tool_result.tool_name "different_tool" does not match prior tool_call.tool_name "refund_api"`,
		},
		{
			name: "duplicate_segment_id",
			mutate: func(trace *Trace) {
				trace.Segments[1].SegmentID = trace.Segments[0].SegmentID
			},
			wantErr: `duplicate segment_id "seg-001"`,
		},
		{
			name: "duplicate_tool_call_id",
			mutate: func(trace *Trace) {
				duplicateCall := validSegment(SegmentKindToolCall)
				duplicateCall.SegmentID = "seg-007"
				duplicateCall.SequenceNumber = 7
				duplicateCall.ToolCall.CallID = trace.Segments[2].ToolCall.CallID
				trace.Segments = append(trace.Segments, duplicateCall)
			},
			wantErr: `duplicate tool_call.call_id "call-refund-1"`,
		},
	}

	for _, tt := range traceTests {
		t.Run(tt.name, func(t *testing.T) {
			trace := validReferenceTrace()
			tt.mutate(&trace)
			err := trace.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}

	t.Run("valid_cross_references", func(t *testing.T) {
		if err := validReferenceTrace().Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})

	tests := []struct {
		name    string
		segment Segment
		wantErr string
	}{
		{
			name: "audio_artifact_ref_required",
			segment: segmentWith(func(segment *Segment) {
				segment.Kind = SegmentKindAudioInput
				segment.Audio = &AudioPayload{Format: "wav"}
			}),
			wantErr: "audio.artifact_ref is required",
		},
		{
			name: "tool_call_arguments_must_be_json",
			segment: segmentWith(func(segment *Segment) {
				segment.Kind = SegmentKindToolCall
				segment.ToolCall = &ToolCallPayload{CallID: "call-1", ToolName: "refund_api", Arguments: json.RawMessage(`{bad`)}
			}),
			wantErr: "tool_call.arguments must be valid JSON",
		},
		{
			name: "structured_output_must_be_json",
			segment: segmentWith(func(segment *Segment) {
				segment.Kind = SegmentKindStructuredOutput
				segment.StructuredOutput = &StructuredOutputPayload{Output: json.RawMessage(`{bad`)}
			}),
			wantErr: "structured_output.output must be valid JSON",
		},
		{
			name: "media_control_action_required",
			segment: segmentWith(func(segment *Segment) {
				segment.Kind = SegmentKindMediaControl
				segment.MediaControl = &MediaControlPayload{TargetSegmentID: "seg-001"}
			}),
			wantErr: "media_control.action is required",
		},
		{
			name: "transcript_confidence_range",
			segment: segmentWith(func(segment *Segment) {
				confidence := 1.2
				segment.Kind = SegmentKindTranscriptFinal
				segment.Transcript = &TranscriptPayload{Text: "hello", Confidence: &confidence}
			}),
			wantErr: "transcript.confidence must be between 0 and 1",
		},
		{
			name: "exactly_one_payload",
			segment: segmentWith(func(segment *Segment) {
				segment.Kind = SegmentKindTextInput
				segment.Text = &TextPayload{Text: "hello"}
				segment.Audio = &AudioPayload{ArtifactRef: "artifacts/audio.wav", Format: "wav"}
			}),
			wantErr: "segment must contain exactly one payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.segment.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func validTrace() Trace {
	return Trace{
		TraceID:       "trace-test",
		SchemaVersion: SchemaVersionV1,
		RunID:         uuid.MustParse(testRunID),
		RunAgentID:    uuid.MustParse(testRunAgentID),
		Segments: []Segment{
			validSegment(SegmentKindTextInput),
		},
	}
}

func validReferenceTrace() Trace {
	trace := validTrace()
	trace.TraceID = "trace-reference-test"
	trace.Segments = []Segment{
		validSegment(SegmentKindAudioInput),
		validSegment(SegmentKindTranscriptFinal),
		validSegment(SegmentKindToolCall),
		validSegment(SegmentKindToolResult),
		validSegment(SegmentKindTextOutput),
		validSegment(SegmentKindMediaControl),
	}
	for i := range trace.Segments {
		trace.Segments[i].SegmentID = fmt.Sprintf("seg-%03d", i+1)
		trace.Segments[i].SequenceNumber = int64(i + 1)
		trace.Segments[i].OccurredAt = mustParseTime(fmt.Sprintf("2026-05-13T09:00:0%dZ", i))
	}
	trace.Segments[1].Transcript.SourceSegmentID = trace.Segments[0].SegmentID
	trace.Segments[2].ToolCall.CallID = "call-refund-1"
	trace.Segments[2].ToolCall.ToolName = "refund_api"
	trace.Segments[3].ToolResult.CallID = "call-refund-1"
	trace.Segments[3].ToolResult.ToolName = "refund_api"
	trace.Segments[5].MediaControl.TargetSegmentID = trace.Segments[0].SegmentID
	return trace
}

func validSegment(kind SegmentKind) Segment {
	segment := Segment{
		SegmentID:      "seg-001",
		SequenceNumber: 1,
		Kind:           kind,
		Actor:          ActorUser,
		OccurredAt:     mustParseTime("2026-05-13T09:00:00Z"),
	}
	switch kind {
	case SegmentKindAudioInput, SegmentKindAudioOutput:
		segment.Audio = &AudioPayload{ArtifactRef: "artifacts/audio.wav", Format: "wav", DurationMS: 1000}
	case SegmentKindTextInput, SegmentKindTextOutput:
		segment.Text = &TextPayload{Text: "hello", Language: "en-US"}
	case SegmentKindTranscriptPartial, SegmentKindTranscriptFinal:
		segment.Transcript = &TranscriptPayload{Text: "hello", Language: "en-US", SourceSegmentID: "seg-audio"}
	case SegmentKindToolCall:
		segment.Actor = ActorAgent
		segment.ToolCall = &ToolCallPayload{CallID: "call-1", ToolName: "refund_api", Arguments: json.RawMessage(`{"amount_cents":4200}`)}
	case SegmentKindToolResult:
		segment.Actor = ActorTool
		segment.ToolResult = &ToolResultPayload{CallID: "call-1", ToolName: "refund_api", Result: json.RawMessage(`{"status":"created"}`)}
	case SegmentKindStructuredOutput:
		segment.Actor = ActorAgent
		segment.StructuredOutput = &StructuredOutputPayload{SchemaRef: "support.resolution.v1", Output: json.RawMessage(`{"resolution":"refund_created"}`)}
	case SegmentKindTimingMarker:
		segment.Actor = ActorEvaluator
		segment.TimingMarker = &TimingMarkerPayload{Key: "end_of_speech_to_first_agent_output", ValueMS: 800}
	case SegmentKindMediaControl:
		segment.Actor = ActorSystem
		segment.MediaControl = &MediaControlPayload{Action: "audio_buffer_cleared", TargetSegmentID: "seg-001"}
	}
	return segment
}

func segmentWith(mutator func(*Segment)) Segment {
	segment := validSegment(SegmentKindTextInput)
	segment.Text = nil
	segment.Audio = nil
	segment.Transcript = nil
	segment.ToolCall = nil
	segment.ToolResult = nil
	segment.StructuredOutput = nil
	segment.TimingMarker = nil
	segment.MediaControl = nil
	mutator(&segment)
	return segment
}

func mustParseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return parsed
}
