package multimodaltrace

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const SchemaVersionV1 = "2026-05-13"

type SegmentKind string

const (
	SegmentKindAudioInput        SegmentKind = "audio_input"
	SegmentKindAudioOutput       SegmentKind = "audio_output"
	SegmentKindTextInput         SegmentKind = "text_input"
	SegmentKindTextOutput        SegmentKind = "text_output"
	SegmentKindTranscriptPartial SegmentKind = "transcript_partial"
	SegmentKindTranscriptFinal   SegmentKind = "transcript_final"
	SegmentKindToolCall          SegmentKind = "tool_call"
	SegmentKindToolResult        SegmentKind = "tool_result"
	SegmentKindStructuredOutput  SegmentKind = "structured_output"
	SegmentKindTimingMarker      SegmentKind = "timing_marker"
	SegmentKindMediaControl      SegmentKind = "media_control"
)

type Actor string

const (
	ActorUser      Actor = "user"
	ActorAgent     Actor = "agent"
	ActorSystem    Actor = "system"
	ActorTool      Actor = "tool"
	ActorEvaluator Actor = "evaluator"
)

var (
	ErrInvalidSchemaVersion = errors.New("invalid multimodal trace schema version")
	ErrInvalidSegmentKind   = errors.New("invalid multimodal trace segment kind")
	ErrInvalidActor         = errors.New("invalid multimodal trace actor")
)

type Trace struct {
	TraceID       string    `json:"trace_id"`
	SchemaVersion string    `json:"schema_version"`
	RunID         uuid.UUID `json:"run_id"`
	RunAgentID    uuid.UUID `json:"run_agent_id"`
	Segments      []Segment `json:"segments"`
}

type Segment struct {
	SegmentID        string                   `json:"segment_id"`
	SequenceNumber   int64                    `json:"sequence_number"`
	Kind             SegmentKind              `json:"kind"`
	Actor            Actor                    `json:"actor"`
	OccurredAt       time.Time                `json:"occurred_at"`
	Text             *TextPayload             `json:"text,omitempty"`
	Audio            *AudioPayload            `json:"audio,omitempty"`
	Transcript       *TranscriptPayload       `json:"transcript,omitempty"`
	ToolCall         *ToolCallPayload         `json:"tool_call,omitempty"`
	ToolResult       *ToolResultPayload       `json:"tool_result,omitempty"`
	StructuredOutput *StructuredOutputPayload `json:"structured_output,omitempty"`
	TimingMarker     *TimingMarkerPayload     `json:"timing_marker,omitempty"`
	MediaControl     *MediaControlPayload     `json:"media_control,omitempty"`
}

type TextPayload struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
}

type AudioPayload struct {
	ArtifactRef string `json:"artifact_ref"`
	Format      string `json:"format"`
	Channel     string `json:"channel,omitempty"`
	DurationMS  int64  `json:"duration_ms,omitempty"`
}

type TranscriptPayload struct {
	Text            string   `json:"text"`
	Language        string   `json:"language,omitempty"`
	Confidence      *float64 `json:"confidence,omitempty"`
	SourceSegmentID string   `json:"source_segment_id,omitempty"`
}

type ToolCallPayload struct {
	CallID    string          `json:"call_id"`
	ToolName  string          `json:"tool_name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResultPayload struct {
	CallID   string          `json:"call_id"`
	ToolName string          `json:"tool_name"`
	Result   json.RawMessage `json:"result,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type StructuredOutputPayload struct {
	SchemaRef string          `json:"schema_ref,omitempty"`
	Output    json.RawMessage `json:"output"`
}

type TimingMarkerPayload struct {
	Key     string `json:"key"`
	ValueMS int64  `json:"value_ms"`
}

type MediaControlPayload struct {
	Action          string `json:"action"`
	TargetSegmentID string `json:"target_segment_id,omitempty"`
}

func (t Trace) Validate() error {
	if t.TraceID == "" {
		return errors.New("trace_id is required")
	}
	if t.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("%w: %q", ErrInvalidSchemaVersion, t.SchemaVersion)
	}
	if t.RunID == uuid.Nil {
		return errors.New("run_id is required")
	}
	if t.RunAgentID == uuid.Nil {
		return errors.New("run_agent_id is required")
	}
	seenSequences := make(map[int64]struct{}, len(t.Segments))
	seenSegmentIDs := make(map[string]Segment, len(t.Segments))
	seenToolCalls := make(map[string]ToolCallPayload)
	var previousSequence int64
	for i, segment := range t.Segments {
		if err := segment.Validate(); err != nil {
			return fmt.Errorf("segments[%d]: %w", i, err)
		}
		if _, ok := seenSegmentIDs[segment.SegmentID]; ok {
			return fmt.Errorf("segments[%d]: duplicate segment_id %q", i, segment.SegmentID)
		}
		if _, ok := seenSequences[segment.SequenceNumber]; ok {
			return fmt.Errorf("segments[%d]: duplicate sequence_number %d", i, segment.SequenceNumber)
		}
		if previousSequence > 0 && segment.SequenceNumber <= previousSequence {
			return fmt.Errorf("segments[%d]: sequence_number must increase monotonically", i)
		}
		if err := validateSegmentReferences(i, segment, seenSegmentIDs, seenToolCalls); err != nil {
			return err
		}
		seenSequences[segment.SequenceNumber] = struct{}{}
		seenSegmentIDs[segment.SegmentID] = segment
		if segment.ToolCall != nil {
			if _, ok := seenToolCalls[segment.ToolCall.CallID]; ok {
				return fmt.Errorf("segments[%d]: duplicate tool_call.call_id %q", i, segment.ToolCall.CallID)
			}
			seenToolCalls[segment.ToolCall.CallID] = *segment.ToolCall
		}
		previousSequence = segment.SequenceNumber
	}
	return nil
}

func (s Segment) Validate() error {
	if s.SegmentID == "" {
		return errors.New("segment_id is required")
	}
	if s.SequenceNumber <= 0 {
		return errors.New("sequence_number must be greater than zero")
	}
	if !s.Kind.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidSegmentKind, s.Kind)
	}
	if !s.Actor.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidActor, s.Actor)
	}
	if s.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}
	if s.payloadCount() > 1 {
		return errors.New("segment must contain exactly one payload")
	}
	return s.validatePayloadForKind()
}

func (k SegmentKind) IsValid() bool {
	switch k {
	case SegmentKindAudioInput,
		SegmentKindAudioOutput,
		SegmentKindTextInput,
		SegmentKindTextOutput,
		SegmentKindTranscriptPartial,
		SegmentKindTranscriptFinal,
		SegmentKindToolCall,
		SegmentKindToolResult,
		SegmentKindStructuredOutput,
		SegmentKindTimingMarker,
		SegmentKindMediaControl:
		return true
	default:
		return false
	}
}

func (a Actor) IsValid() bool {
	switch a {
	case ActorUser, ActorAgent, ActorSystem, ActorTool, ActorEvaluator:
		return true
	default:
		return false
	}
}

func validateSegmentReferences(index int, segment Segment, priorSegments map[string]Segment, priorToolCalls map[string]ToolCallPayload) error {
	if segment.Transcript != nil && segment.Transcript.SourceSegmentID != "" {
		source, ok := priorSegments[segment.Transcript.SourceSegmentID]
		if !ok {
			return fmt.Errorf("segments[%d]: transcript.source_segment_id %q does not reference a prior segment", index, segment.Transcript.SourceSegmentID)
		}
		if source.Kind != SegmentKindAudioInput && source.Kind != SegmentKindAudioOutput {
			return fmt.Errorf("segments[%d]: transcript.source_segment_id %q must reference an audio segment", index, segment.Transcript.SourceSegmentID)
		}
	}
	if segment.MediaControl != nil && segment.MediaControl.TargetSegmentID != "" {
		if _, ok := priorSegments[segment.MediaControl.TargetSegmentID]; !ok {
			return fmt.Errorf("segments[%d]: media_control.target_segment_id %q does not reference a prior segment", index, segment.MediaControl.TargetSegmentID)
		}
	}
	if segment.ToolResult != nil {
		call, ok := priorToolCalls[segment.ToolResult.CallID]
		if !ok {
			return fmt.Errorf("segments[%d]: tool_result.call_id %q does not reference a prior tool_call", index, segment.ToolResult.CallID)
		}
		if call.ToolName != segment.ToolResult.ToolName {
			return fmt.Errorf("segments[%d]: tool_result.tool_name %q does not match prior tool_call.tool_name %q", index, segment.ToolResult.ToolName, call.ToolName)
		}
	}
	return nil
}

func (s Segment) payloadCount() int {
	count := 0
	if s.Text != nil {
		count++
	}
	if s.Audio != nil {
		count++
	}
	if s.Transcript != nil {
		count++
	}
	if s.ToolCall != nil {
		count++
	}
	if s.ToolResult != nil {
		count++
	}
	if s.StructuredOutput != nil {
		count++
	}
	if s.TimingMarker != nil {
		count++
	}
	if s.MediaControl != nil {
		count++
	}
	return count
}

func (s Segment) validatePayloadForKind() error {
	switch s.Kind {
	case SegmentKindAudioInput, SegmentKindAudioOutput:
		if s.Audio == nil {
			return errors.New("audio payload is required")
		}
		return s.Audio.Validate()
	case SegmentKindTextInput, SegmentKindTextOutput:
		if s.Text == nil {
			return errors.New("text payload is required")
		}
		return s.Text.Validate()
	case SegmentKindTranscriptPartial, SegmentKindTranscriptFinal:
		if s.Transcript == nil {
			return errors.New("transcript payload is required")
		}
		return s.Transcript.Validate()
	case SegmentKindToolCall:
		if s.ToolCall == nil {
			return errors.New("tool_call payload is required")
		}
		return s.ToolCall.Validate()
	case SegmentKindToolResult:
		if s.ToolResult == nil {
			return errors.New("tool_result payload is required")
		}
		return s.ToolResult.Validate()
	case SegmentKindStructuredOutput:
		if s.StructuredOutput == nil {
			return errors.New("structured_output payload is required")
		}
		return s.StructuredOutput.Validate()
	case SegmentKindTimingMarker:
		if s.TimingMarker == nil {
			return errors.New("timing_marker payload is required")
		}
		return s.TimingMarker.Validate()
	case SegmentKindMediaControl:
		if s.MediaControl == nil {
			return errors.New("media_control payload is required")
		}
		return s.MediaControl.Validate()
	default:
		return fmt.Errorf("%w: %q", ErrInvalidSegmentKind, s.Kind)
	}
}

func (p TextPayload) Validate() error {
	if p.Text == "" {
		return errors.New("text.text is required")
	}
	return nil
}

func (p AudioPayload) Validate() error {
	if p.ArtifactRef == "" {
		return errors.New("audio.artifact_ref is required")
	}
	if p.Format == "" {
		return errors.New("audio.format is required")
	}
	if p.DurationMS < 0 {
		return errors.New("audio.duration_ms must be non-negative")
	}
	return nil
}

func (p TranscriptPayload) Validate() error {
	if p.Text == "" {
		return errors.New("transcript.text is required")
	}
	if p.Confidence != nil && (*p.Confidence < 0 || *p.Confidence > 1) {
		return errors.New("transcript.confidence must be between 0 and 1")
	}
	return nil
}

func (p ToolCallPayload) Validate() error {
	if p.CallID == "" {
		return errors.New("tool_call.call_id is required")
	}
	if p.ToolName == "" {
		return errors.New("tool_call.tool_name is required")
	}
	if len(p.Arguments) == 0 {
		return errors.New("tool_call.arguments is required")
	}
	if !json.Valid(p.Arguments) {
		return errors.New("tool_call.arguments must be valid JSON")
	}
	return nil
}

func (p ToolResultPayload) Validate() error {
	if p.CallID == "" {
		return errors.New("tool_result.call_id is required")
	}
	if p.ToolName == "" {
		return errors.New("tool_result.tool_name is required")
	}
	if len(p.Result) > 0 && !json.Valid(p.Result) {
		return errors.New("tool_result.result must be valid JSON")
	}
	return nil
}

func (p StructuredOutputPayload) Validate() error {
	if len(p.Output) == 0 {
		return errors.New("structured_output.output is required")
	}
	if !json.Valid(p.Output) {
		return errors.New("structured_output.output must be valid JSON")
	}
	return nil
}

func (p TimingMarkerPayload) Validate() error {
	if p.Key == "" {
		return errors.New("timing_marker.key is required")
	}
	if p.ValueMS < 0 {
		return errors.New("timing_marker.value_ms must be non-negative")
	}
	return nil
}

func (p MediaControlPayload) Validate() error {
	if p.Action == "" {
		return errors.New("media_control.action is required")
	}
	return nil
}
