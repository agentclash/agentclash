package voicereplay

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/agentclash/agentclash/backend/internal/voiceartifacts"
)

const SchemaVersionV1 = "2026-05-13"

var ErrInvalidInput = errors.New("invalid voice replay input")

type Projection struct {
	SchemaVersion    string               `json:"schema_version"`
	VoiceSessionID   string               `json:"voice_session_id"`
	RunID            string               `json:"run_id"`
	RunAgentID       string               `json:"run_agent_id"`
	Turns            []Turn               `json:"turns"`
	Artifacts        []ArtifactProjection `json:"artifacts"`
	DegradedEvidence []string             `json:"degraded_evidence,omitempty"`
}

type Turn struct {
	TurnID      string              `json:"turn_id"`
	TurnIndex   int                 `json:"turn_index,omitempty"`
	Events      []EventProjection   `json:"events"`
	Transcripts []TranscriptSegment `json:"transcripts,omitempty"`
	ToolCalls   []ToolCall          `json:"tool_calls,omitempty"`
	Audio       []AudioReference    `json:"audio,omitempty"`
	Metrics     []MetricMarker      `json:"metrics,omitempty"`
}

type EventProjection struct {
	SequenceNumber int64  `json:"sequence_number"`
	EventType      string `json:"event_type"`
	OccurredAt     string `json:"occurred_at"`
	Speaker        string `json:"speaker,omitempty"`
	Channel        string `json:"channel,omitempty"`
}

type TranscriptSegment struct {
	SequenceNumber int64  `json:"sequence_number"`
	SegmentID      string `json:"segment_id,omitempty"`
	Speaker        string `json:"speaker,omitempty"`
	Channel        string `json:"channel,omitempty"`
	Text           string `json:"text"`
	Language       string `json:"language,omitempty"`
}

type ToolCall struct {
	SequenceNumber int64           `json:"sequence_number"`
	CallID         string          `json:"call_id"`
	ToolName       string          `json:"tool_name"`
	Arguments      json.RawMessage `json:"arguments,omitempty"`
	Result         json.RawMessage `json:"result,omitempty"`
	Error          string          `json:"error,omitempty"`
}

type AudioReference struct {
	SequenceNumber int64  `json:"sequence_number"`
	SegmentID      string `json:"segment_id,omitempty"`
	ArtifactKey    string `json:"artifact_key"`
	ArtifactRef    string `json:"artifact_ref,omitempty"`
	DurationMS     int64  `json:"duration_ms,omitempty"`
}

type MetricMarker struct {
	SequenceNumber int64  `json:"sequence_number"`
	Key            string `json:"key"`
	ValueMS        int64  `json:"value_ms"`
}

type ArtifactProjection struct {
	Key            string `json:"key"`
	Kind           string `json:"kind"`
	Location       string `json:"location"`
	Path           string `json:"path,omitempty"`
	Bucket         string `json:"bucket,omitempty"`
	ObjectKey      string `json:"object_key,omitempty"`
	ContentType    string `json:"content_type,omitempty"`
	SizeBytes      int64  `json:"size_bytes,omitempty"`
	ChecksumSHA256 string `json:"checksum_sha256"`
}

func Build(events []runevents.Envelope, manifest voiceartifacts.Manifest) (Projection, error) {
	if err := manifest.Validate(); err != nil {
		return Projection{}, fmt.Errorf("%w: manifest: %w", ErrInvalidInput, err)
	}
	if len(events) == 0 {
		return Projection{}, fmt.Errorf("%w: events are required", ErrInvalidInput)
	}
	ordered := append([]runevents.Envelope(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].SequenceNumber < ordered[j].SequenceNumber
	})
	for idx, event := range ordered {
		if err := event.ValidatePersisted(); err != nil {
			return Projection{}, fmt.Errorf("%w: events[%d]: %w", ErrInvalidInput, idx, err)
		}
	}

	builder := projectionBuilder{
		projection: Projection{
			SchemaVersion:  SchemaVersionV1,
			VoiceSessionID: manifest.VoiceSessionID,
			RunID:          manifest.RunID.String(),
			RunAgentID:     manifest.RunAgentID.String(),
			Artifacts:      artifacts(manifest.Artifacts),
		},
		artifactByKind: artifactsByKind(manifest.Artifacts),
		turnByID:       map[string]int{},
	}

	for _, event := range ordered {
		builder.addEvent(event)
	}
	builder.finalize()
	return builder.projection, nil
}

func StableJSON(projection Projection) ([]byte, error) {
	encoded, err := json.MarshalIndent(projection, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode voice replay projection: %w", err)
	}
	return append(encoded, '\n'), nil
}

type projectionBuilder struct {
	projection     Projection
	artifactByKind map[voiceartifacts.ArtifactKind]voiceartifacts.ArtifactRef
	turnByID       map[string]int
}

func (b *projectionBuilder) addEvent(event runevents.Envelope) {
	payload := decodePayload(event.Payload)
	turnID := stringField(payload, "turn_id")
	if turnID == "" {
		turnID = "session"
	}
	turn := b.turn(turnID, event.Summary.TurnIndex)
	turn.Events = append(turn.Events, EventProjection{
		SequenceNumber: event.SequenceNumber,
		EventType:      string(event.EventType),
		OccurredAt:     event.OccurredAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
		Speaker:        event.Summary.Speaker,
		Channel:        event.Summary.Channel,
	})

	switch event.EventType {
	case runevents.EventTypeTranscriptFinal, runevents.EventTypeTranscriptPartial:
		text := stringField(payload, "text")
		if text != "" {
			turn.Transcripts = append(turn.Transcripts, TranscriptSegment{
				SequenceNumber: event.SequenceNumber,
				SegmentID:      stringField(payload, "segment_id"),
				Speaker:        event.Summary.Speaker,
				Channel:        event.Summary.Channel,
				Text:           text,
				Language:       stringField(payload, "language"),
			})
		}
	case runevents.EventTypeToolCallStarted, runevents.EventTypeToolCallCompleted, runevents.EventTypeToolCallFailed:
		turn.ToolCalls = append(turn.ToolCalls, ToolCall{
			SequenceNumber: event.SequenceNumber,
			CallID:         stringField(payload, "call_id"),
			ToolName:       stringField(payload, "tool_name"),
			Arguments:      rawField(payload, "arguments"),
			Result:         rawField(payload, "result"),
			Error:          stringField(payload, "error"),
		})
	case runevents.EventTypeAgentAudioStarted, runevents.EventTypeAgentAudioCompleted:
		turn.Audio = append(turn.Audio, b.audioReference(event, payload, voiceartifacts.ArtifactKindAgentAudio))
	case runevents.EventTypeMediaFrameReceived, runevents.EventTypeSpeechStarted, runevents.EventTypeSpeechStopped:
		turn.Audio = append(turn.Audio, b.audioReference(event, payload, voiceartifacts.ArtifactKindCallerAudio))
	case runevents.EventTypeVoiceMetricRecorded:
		turn.Metrics = append(turn.Metrics, MetricMarker{
			SequenceNumber: event.SequenceNumber,
			Key:            stringField(payload, "metric_key"),
			ValueMS:        int64Field(payload, "value_ms"),
		})
	}
	b.projection.Turns[b.turnByID[turnID]] = *turn
}

func (b *projectionBuilder) turn(turnID string, turnIndex *int) *Turn {
	if idx, ok := b.turnByID[turnID]; ok {
		return &b.projection.Turns[idx]
	}
	turn := Turn{TurnID: turnID}
	if turnIndex != nil {
		turn.TurnIndex = *turnIndex
	}
	b.turnByID[turnID] = len(b.projection.Turns)
	b.projection.Turns = append(b.projection.Turns, turn)
	return &b.projection.Turns[len(b.projection.Turns)-1]
}

func (b *projectionBuilder) audioReference(event runevents.Envelope, payload map[string]json.RawMessage, kind voiceartifacts.ArtifactKind) AudioReference {
	ref := AudioReference{
		SequenceNumber: event.SequenceNumber,
		SegmentID:      stringField(payload, "segment_id"),
		ArtifactRef:    stringField(payload, "artifact_ref"),
		DurationMS:     int64Field(payload, "duration_ms"),
	}
	artifact, ok := b.artifactByKind[kind]
	if ok {
		ref.ArtifactKey = artifact.Key
		return ref
	}
	ref.ArtifactKey = string(kind)
	b.projection.DegradedEvidence = append(b.projection.DegradedEvidence, "missing_audio_artifact:"+string(kind))
	return ref
}

func (b *projectionBuilder) finalize() {
	if _, ok := b.artifactByKind[voiceartifacts.ArtifactKindMixedAudio]; !ok {
		b.projection.DegradedEvidence = append(b.projection.DegradedEvidence, "missing_audio_artifact:"+string(voiceartifacts.ArtifactKindMixedAudio))
	}
	b.projection.DegradedEvidence = uniqueStrings(b.projection.DegradedEvidence)
	sort.Strings(b.projection.DegradedEvidence)
}

func artifacts(refs []voiceartifacts.ArtifactRef) []ArtifactProjection {
	out := make([]ArtifactProjection, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ArtifactProjection{
			Key:            ref.Key,
			Kind:           string(ref.Kind),
			Location:       string(ref.Location),
			Path:           ref.Path,
			Bucket:         ref.Bucket,
			ObjectKey:      ref.ObjectKey,
			ContentType:    ref.ContentType,
			SizeBytes:      ref.SizeBytes,
			ChecksumSHA256: ref.ChecksumSHA256,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func artifactsByKind(refs []voiceartifacts.ArtifactRef) map[voiceartifacts.ArtifactKind]voiceartifacts.ArtifactRef {
	byKind := make(map[voiceartifacts.ArtifactKind]voiceartifacts.ArtifactRef, len(refs))
	for _, ref := range refs {
		byKind[ref.Kind] = ref
	}
	return byKind
}

func decodePayload(raw json.RawMessage) map[string]json.RawMessage {
	payload := map[string]json.RawMessage{}
	_ = json.Unmarshal(raw, &payload)
	return payload
}

func rawField(payload map[string]json.RawMessage, key string) json.RawMessage {
	raw := payload[key]
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil
	}
	return encoded
}

func stringField(payload map[string]json.RawMessage, key string) string {
	raw := payload[key]
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func int64Field(payload map[string]json.RawMessage, key string) int64 {
	raw := payload[key]
	var value int64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0
	}
	return value
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}
