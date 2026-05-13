package voicedeployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/google/uuid"
)

var (
	ErrInvalidScript   = errors.New("invalid fake voice deployment script")
	ErrTurnNotFound    = errors.New("fake voice deployment turn not found")
	ErrUnexpectedInput = errors.New("unexpected fake voice deployment input")
)

type Deployment interface {
	Invoke(context.Context, Invocation) (Result, error)
}

type Invocation struct {
	TraceID       string
	RunID         uuid.UUID
	RunAgentID    uuid.UUID
	TurnID        string
	InputSegments []multimodaltrace.Segment
}

type Result struct {
	Outcome  Outcome
	Segments []multimodaltrace.Segment
}

type Outcome string

const (
	OutcomePass Outcome = "pass"
	OutcomeFail Outcome = "fail"
)

type Script struct {
	ScenarioKey string `json:"scenario_key"`
	Turns       []Turn `json:"turns"`
}

type Turn struct {
	TurnID            string         `json:"turn_id"`
	ExpectedInputText string         `json:"expected_input_text"`
	Outcome           Outcome        `json:"outcome,omitempty"`
	Response          ResponseScript `json:"response"`
}

type ResponseScript struct {
	TextResponse        *TextResponse        `json:"text_response,omitempty"`
	TranscriptResponse  *TranscriptResponse  `json:"transcript_response,omitempty"`
	StructuredOutput    *StructuredOutput    `json:"structured_output,omitempty"`
	ExpectedToolCall    *ExpectedToolCall    `json:"expected_tool_call,omitempty"`
	SpokenAudioArtifact *SpokenAudioArtifact `json:"spoken_audio_artifact,omitempty"`
	TimingMarkers       []TimingMarker       `json:"timing_markers,omitempty"`
}

type TextResponse struct {
	Text      string `json:"text"`
	Language  string `json:"language,omitempty"`
	OffsetMS  int64  `json:"offset_ms"`
	SegmentID string `json:"segment_id,omitempty"`
}

type TranscriptResponse struct {
	Text       string   `json:"text"`
	Language   string   `json:"language,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
	OffsetMS   int64    `json:"offset_ms"`
	SegmentID  string   `json:"segment_id,omitempty"`
}

type StructuredOutput struct {
	SchemaRef string          `json:"schema_ref,omitempty"`
	Output    json.RawMessage `json:"output"`
	OffsetMS  int64           `json:"offset_ms"`
	SegmentID string          `json:"segment_id,omitempty"`
}

type ExpectedToolCall struct {
	CallID    string          `json:"call_id"`
	ToolName  string          `json:"tool_name"`
	Arguments json.RawMessage `json:"arguments"`
	OffsetMS  int64           `json:"offset_ms"`
	SegmentID string          `json:"segment_id,omitempty"`
}

type SpokenAudioArtifact struct {
	ArtifactRef string `json:"artifact_ref"`
	Format      string `json:"format"`
	Channel     string `json:"channel,omitempty"`
	DurationMS  int64  `json:"duration_ms,omitempty"`
	OffsetMS    int64  `json:"offset_ms"`
	SegmentID   string `json:"segment_id,omitempty"`
}

type TimingMarker struct {
	Key       string `json:"key"`
	ValueMS   int64  `json:"value_ms"`
	OffsetMS  int64  `json:"offset_ms"`
	SegmentID string `json:"segment_id,omitempty"`
}

type FakeDeployment struct {
	turns map[string]Turn
}

func NewFake(script Script) (*FakeDeployment, error) {
	if err := script.Validate(); err != nil {
		return nil, err
	}
	turns := make(map[string]Turn, len(script.Turns))
	for _, turn := range script.Turns {
		turns[turn.TurnID] = turn
	}
	return &FakeDeployment{turns: turns}, nil
}

func (s Script) Validate() error {
	if strings.TrimSpace(s.ScenarioKey) == "" {
		return fmt.Errorf("%w: scenario_key is required", ErrInvalidScript)
	}
	if len(s.Turns) == 0 {
		return fmt.Errorf("%w: turns must contain at least one turn", ErrInvalidScript)
	}
	seen := make(map[string]struct{}, len(s.Turns))
	for idx, turn := range s.Turns {
		if err := turn.Validate(); err != nil {
			return fmt.Errorf("turns[%d]: %w", idx, err)
		}
		if _, ok := seen[turn.TurnID]; ok {
			return fmt.Errorf("turns[%d]: %w: duplicate turn_id %q", idx, ErrInvalidScript, turn.TurnID)
		}
		seen[turn.TurnID] = struct{}{}
	}
	return nil
}

func (t Turn) Validate() error {
	if strings.TrimSpace(t.TurnID) == "" {
		return fmt.Errorf("%w: turn_id is required", ErrInvalidScript)
	}
	if strings.TrimSpace(t.ExpectedInputText) == "" {
		return fmt.Errorf("%w: expected_input_text is required", ErrInvalidScript)
	}
	if t.Outcome != "" && t.Outcome != OutcomePass && t.Outcome != OutcomeFail {
		return fmt.Errorf("%w: unsupported outcome %q", ErrInvalidScript, t.Outcome)
	}
	if err := t.Response.Validate(); err != nil {
		return err
	}
	return nil
}

func (r ResponseScript) Validate() error {
	count := 0
	previousOffset := int64(-1)
	validateOffset := func(label string, offsetMS int64) error {
		if offsetMS < 0 {
			return fmt.Errorf("%w: %s.offset_ms must be non-negative", ErrInvalidScript, label)
		}
		if offsetMS < previousOffset {
			return fmt.Errorf("%w: %s.offset_ms must not go backward", ErrInvalidScript, label)
		}
		previousOffset = offsetMS
		return nil
	}
	if r.ExpectedToolCall != nil {
		count++
		if strings.TrimSpace(r.ExpectedToolCall.CallID) == "" {
			return fmt.Errorf("%w: expected_tool_call.call_id is required", ErrInvalidScript)
		}
		if strings.TrimSpace(r.ExpectedToolCall.ToolName) == "" {
			return fmt.Errorf("%w: expected_tool_call.tool_name is required", ErrInvalidScript)
		}
		if len(r.ExpectedToolCall.Arguments) == 0 || !json.Valid(r.ExpectedToolCall.Arguments) {
			return fmt.Errorf("%w: expected_tool_call.arguments must be valid JSON", ErrInvalidScript)
		}
		if err := validateOffset("expected_tool_call", r.ExpectedToolCall.OffsetMS); err != nil {
			return err
		}
	}
	if r.TextResponse != nil {
		count++
		if strings.TrimSpace(r.TextResponse.Text) == "" {
			return fmt.Errorf("%w: text_response.text is required", ErrInvalidScript)
		}
		if err := validateOffset("text_response", r.TextResponse.OffsetMS); err != nil {
			return err
		}
	}
	if r.SpokenAudioArtifact != nil {
		count++
		if strings.TrimSpace(r.SpokenAudioArtifact.ArtifactRef) == "" {
			return fmt.Errorf("%w: spoken_audio_artifact.artifact_ref is required", ErrInvalidScript)
		}
		if strings.TrimSpace(r.SpokenAudioArtifact.Format) == "" {
			return fmt.Errorf("%w: spoken_audio_artifact.format is required", ErrInvalidScript)
		}
		if r.SpokenAudioArtifact.DurationMS < 0 {
			return fmt.Errorf("%w: spoken_audio_artifact.duration_ms must be non-negative", ErrInvalidScript)
		}
		if err := validateOffset("spoken_audio_artifact", r.SpokenAudioArtifact.OffsetMS); err != nil {
			return err
		}
	}
	if r.TranscriptResponse != nil {
		count++
		if strings.TrimSpace(r.TranscriptResponse.Text) == "" {
			return fmt.Errorf("%w: transcript_response.text is required", ErrInvalidScript)
		}
		if err := validateOffset("transcript_response", r.TranscriptResponse.OffsetMS); err != nil {
			return err
		}
	}
	if r.StructuredOutput != nil {
		count++
		if len(r.StructuredOutput.Output) == 0 || !json.Valid(r.StructuredOutput.Output) {
			return fmt.Errorf("%w: structured_output.output must be valid JSON", ErrInvalidScript)
		}
		if err := validateOffset("structured_output", r.StructuredOutput.OffsetMS); err != nil {
			return err
		}
	}
	for idx, marker := range r.TimingMarkers {
		count++
		if strings.TrimSpace(marker.Key) == "" {
			return fmt.Errorf("%w: timing_markers[%d].key is required", ErrInvalidScript, idx)
		}
		if marker.ValueMS < 0 {
			return fmt.Errorf("%w: timing_markers[%d].value_ms must be non-negative", ErrInvalidScript, idx)
		}
		if err := validateOffset(fmt.Sprintf("timing_markers[%d]", idx), marker.OffsetMS); err != nil {
			return err
		}
	}
	if count == 0 {
		return fmt.Errorf("%w: response must contain at least one scripted segment", ErrInvalidScript)
	}
	return nil
}

func (d *FakeDeployment) Invoke(ctx context.Context, input Invocation) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(input.TraceID) == "" {
		return Result{}, fmt.Errorf("%w: trace_id is required", ErrInvalidScript)
	}
	if input.RunID == uuid.Nil {
		return Result{}, fmt.Errorf("%w: run_id is required", ErrInvalidScript)
	}
	if input.RunAgentID == uuid.Nil {
		return Result{}, fmt.Errorf("%w: run_agent_id is required", ErrInvalidScript)
	}
	turn, ok := d.turns[input.TurnID]
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", ErrTurnNotFound, input.TurnID)
	}
	if err := validateInputSegments(input.InputSegments); err != nil {
		return Result{}, err
	}
	if !currentTurnInputMatches(input.InputSegments, input.TurnID, turn.ExpectedInputText) {
		return Result{}, fmt.Errorf("%w: turn_id=%s expected text %q", ErrUnexpectedInput, input.TurnID, turn.ExpectedInputText)
	}

	outcome := turn.Outcome
	if outcome == "" {
		outcome = OutcomePass
	}
	segments := make([]multimodaltrace.Segment, 0, 6+len(turn.Response.TimingMarkers))
	nextSequence := nextSequenceNumber(input.InputSegments)
	anchor := latestOccurredAt(input.InputSegments)
	appendSegment := func(segment multimodaltrace.Segment) error {
		segment.SequenceNumber = nextSequence
		nextSequence++
		if err := segment.Validate(); err != nil {
			return err
		}
		segments = append(segments, segment)
		return nil
	}

	if call := turn.Response.ExpectedToolCall; call != nil {
		if err := appendSegment(multimodaltrace.Segment{
			SegmentID:  firstNonEmpty(call.SegmentID, input.TurnID+":tool-call"),
			Kind:       multimodaltrace.SegmentKindToolCall,
			Actor:      multimodaltrace.ActorAgent,
			OccurredAt: atOffset(anchor, call.OffsetMS),
			ToolCall: &multimodaltrace.ToolCallPayload{
				CallID:    call.CallID,
				ToolName:  call.ToolName,
				Arguments: normalizeJSON(call.Arguments),
			},
		}); err != nil {
			return Result{}, err
		}
	}
	if text := turn.Response.TextResponse; text != nil {
		if err := appendSegment(multimodaltrace.Segment{
			SegmentID:  firstNonEmpty(text.SegmentID, input.TurnID+":agent-text"),
			Kind:       multimodaltrace.SegmentKindTextOutput,
			Actor:      multimodaltrace.ActorAgent,
			OccurredAt: atOffset(anchor, text.OffsetMS),
			Text: &multimodaltrace.TextPayload{
				Text:     text.Text,
				Language: text.Language,
			},
		}); err != nil {
			return Result{}, err
		}
	}

	audioSegmentID := ""
	if audio := turn.Response.SpokenAudioArtifact; audio != nil {
		audioSegmentID = firstNonEmpty(audio.SegmentID, input.TurnID+":agent-audio")
		if err := appendSegment(multimodaltrace.Segment{
			SegmentID:  audioSegmentID,
			Kind:       multimodaltrace.SegmentKindAudioOutput,
			Actor:      multimodaltrace.ActorAgent,
			OccurredAt: atOffset(anchor, audio.OffsetMS),
			Audio: &multimodaltrace.AudioPayload{
				ArtifactRef: audio.ArtifactRef,
				Format:      audio.Format,
				Channel:     audio.Channel,
				DurationMS:  audio.DurationMS,
			},
		}); err != nil {
			return Result{}, err
		}
	}
	if transcript := turn.Response.TranscriptResponse; transcript != nil {
		if err := appendSegment(multimodaltrace.Segment{
			SegmentID:  firstNonEmpty(transcript.SegmentID, input.TurnID+":agent-transcript"),
			Kind:       multimodaltrace.SegmentKindTranscriptFinal,
			Actor:      multimodaltrace.ActorSystem,
			OccurredAt: atOffset(anchor, transcript.OffsetMS),
			Transcript: &multimodaltrace.TranscriptPayload{
				Text:            transcript.Text,
				Language:        transcript.Language,
				Confidence:      transcript.Confidence,
				SourceSegmentID: audioSegmentID,
			},
		}); err != nil {
			return Result{}, err
		}
	}
	if structured := turn.Response.StructuredOutput; structured != nil {
		if err := appendSegment(multimodaltrace.Segment{
			SegmentID:  firstNonEmpty(structured.SegmentID, input.TurnID+":structured-output"),
			Kind:       multimodaltrace.SegmentKindStructuredOutput,
			Actor:      multimodaltrace.ActorAgent,
			OccurredAt: atOffset(anchor, structured.OffsetMS),
			StructuredOutput: &multimodaltrace.StructuredOutputPayload{
				SchemaRef: structured.SchemaRef,
				Output:    normalizeJSON(structured.Output),
			},
		}); err != nil {
			return Result{}, err
		}
	}
	for idx, marker := range turn.Response.TimingMarkers {
		if err := appendSegment(multimodaltrace.Segment{
			SegmentID:  firstNonEmpty(marker.SegmentID, fmt.Sprintf("%s:timing:%03d", input.TurnID, idx+1)),
			Kind:       multimodaltrace.SegmentKindTimingMarker,
			Actor:      multimodaltrace.ActorEvaluator,
			OccurredAt: atOffset(anchor, marker.OffsetMS),
			TimingMarker: &multimodaltrace.TimingMarkerPayload{
				Key:     marker.Key,
				ValueMS: marker.ValueMS,
			},
		}); err != nil {
			return Result{}, err
		}
	}

	trace := multimodaltrace.Trace{
		TraceID:       input.TraceID,
		SchemaVersion: multimodaltrace.SchemaVersionV1,
		RunID:         input.RunID,
		RunAgentID:    input.RunAgentID,
		Segments:      append(append([]multimodaltrace.Segment(nil), input.InputSegments...), segments...),
	}
	if err := trace.Validate(); err != nil {
		return Result{}, err
	}
	return Result{Outcome: outcome, Segments: segments}, nil
}

func validateInputSegments(segments []multimodaltrace.Segment) error {
	trace := multimodaltrace.Trace{
		TraceID:       "fake-input-validation",
		SchemaVersion: multimodaltrace.SchemaVersionV1,
		RunID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		RunAgentID:    uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Segments:      segments,
	}
	if err := trace.Validate(); err != nil {
		return fmt.Errorf("%w: input_segments: %w", ErrInvalidScript, err)
	}
	return nil
}

func currentTurnInputMatches(segments []multimodaltrace.Segment, turnID string, expected string) bool {
	for _, segment := range segments {
		if segment.Actor != multimodaltrace.ActorUser || !strings.HasPrefix(segment.SegmentID, turnID+":") {
			continue
		}
		if segment.Kind == multimodaltrace.SegmentKindTextInput && segment.Text != nil && segment.Text.Text == expected {
			return true
		}
		if segment.Kind == multimodaltrace.SegmentKindTranscriptFinal && segment.Transcript != nil && segment.Transcript.Text == expected {
			return true
		}
	}
	return false
}

func nextSequenceNumber(segments []multimodaltrace.Segment) int64 {
	var maxSequence int64
	for _, segment := range segments {
		if segment.SequenceNumber > maxSequence {
			maxSequence = segment.SequenceNumber
		}
	}
	return maxSequence + 1
}

func latestOccurredAt(segments []multimodaltrace.Segment) time.Time {
	var latest time.Time
	for _, segment := range segments {
		if segment.OccurredAt.After(latest) {
			latest = segment.OccurredAt
		}
	}
	return latest
}

func atOffset(anchor time.Time, offsetMS int64) time.Time {
	return anchor.Add(time.Duration(offsetMS) * time.Millisecond).UTC()
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
