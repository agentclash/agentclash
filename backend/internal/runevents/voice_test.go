package runevents

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVoiceMediaEventTypesAreValid(t *testing.T) {
	eventTypes := []Type{
		EventTypeMediaSessionStarted,
		EventTypeMediaFrameReceived,
		EventTypeSpeechStarted,
		EventTypeSpeechStopped,
		EventTypeTranscriptPartial,
		EventTypeTranscriptFinal,
		EventTypeTurnCompleted,
		EventTypeAgentAudioStarted,
		EventTypeAgentAudioCompleted,
		EventTypeBargeInDetected,
		EventTypeAudioBufferCleared,
		EventTypeDTMFReceived,
		EventTypeTelephonyAnswered,
		EventTypeTelephonyVoicemail,
		EventTypeTransferStarted,
		EventTypeTransferBridged,
		EventTypeVoiceMetricRecorded,
	}

	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			envelope := Envelope{
				EventID:       "evt-" + strings.ReplaceAll(string(eventType), ".", "-"),
				SchemaVersion: SchemaVersionV1,
				RunID:         uuid.New(),
				RunAgentID:    uuid.New(),
				EventType:     eventType,
				Source:        SourceVoiceAdapter,
				OccurredAt:    time.Date(2026, 5, 13, 11, 30, 0, 0, time.UTC),
				Payload:       json.RawMessage(`{"fixture":true}`),
				Summary: SummaryMetadata{
					TurnIndex:     intPtr(2),
					Speaker:       "caller",
					Channel:       "input",
					MetricKey:     "latency_ms",
					EvidenceLevel: EvidenceLevelVoiceStructured,
				},
			}

			if err := envelope.ValidatePending(); err != nil {
				t.Fatalf("ValidatePending returned error: %v", err)
			}
		})
	}
}

func TestVoiceMediaEventRejectsUnknownType(t *testing.T) {
	envelope := Envelope{
		EventID:       "evt-unknown",
		SchemaVersion: SchemaVersionV1,
		RunID:         uuid.New(),
		RunAgentID:    uuid.New(),
		EventType:     Type("media.session.strated"),
		Source:        SourceVoiceAdapter,
		OccurredAt:    time.Date(2026, 5, 13, 11, 30, 0, 0, time.UTC),
		Payload:       json.RawMessage(`{}`),
	}

	if err := envelope.ValidatePending(); !errors.Is(err, ErrInvalidEventType) {
		t.Fatalf("ValidatePending error = %v, want ErrInvalidEventType", err)
	}
}

func TestNormalizeVoiceAdapterEventMapsRawEventToCanonicalEnvelope(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	occurredAt := time.Date(2026, 5, 13, 11, 31, 0, 0, time.UTC)
	payload := json.RawMessage(`{"frame_index":7,"sample_rate_hz":16000}`)

	envelope, err := NormalizeVoiceAdapterEvent(VoiceAdapterEvent{
		EventID:      "fake-evt-7",
		RunID:        runID,
		RunAgentID:   runAgentID,
		RawEventType: "fake.media_frame",
		Source:       SourceVoiceAdapter,
		OccurredAt:   occurredAt,
		Payload:      payload,
		Summary: SummaryMetadata{
			TurnIndex:     intPtr(3),
			Speaker:       "caller",
			Channel:       "inbound",
			MetricKey:     "input_audio_ms",
			EvidenceLevel: EvidenceLevelVoiceStructured,
		},
	})
	if err != nil {
		t.Fatalf("NormalizeVoiceAdapterEvent returned error: %v", err)
	}

	if strings.Contains(string(envelope.EventType), "fake") {
		t.Fatalf("canonical event type leaked raw adapter name: %q", envelope.EventType)
	}
	if envelope.EventType != EventTypeMediaFrameReceived {
		t.Fatalf("event type = %q, want %q", envelope.EventType, EventTypeMediaFrameReceived)
	}
	if envelope.RunID != runID {
		t.Fatalf("run id = %s, want %s", envelope.RunID, runID)
	}
	if envelope.RunAgentID != runAgentID {
		t.Fatalf("run agent id = %s, want %s", envelope.RunAgentID, runAgentID)
	}
	if envelope.Source != SourceVoiceAdapter {
		t.Fatalf("source = %q, want %q", envelope.Source, SourceVoiceAdapter)
	}
	if !envelope.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", envelope.OccurredAt, occurredAt)
	}
	if string(envelope.Payload) != string(payload) {
		t.Fatalf("payload = %s, want %s", envelope.Payload, payload)
	}
	if envelope.Summary.TurnIndex == nil || *envelope.Summary.TurnIndex != 3 || envelope.Summary.Speaker != "caller" || envelope.Summary.Channel != "inbound" {
		t.Fatalf("summary voice fields not preserved: %+v", envelope.Summary)
	}
	if envelope.Summary.MetricKey != "input_audio_ms" {
		t.Fatalf("metric key = %q, want input_audio_ms", envelope.Summary.MetricKey)
	}
	if envelope.Summary.EvidenceLevel != EvidenceLevelVoiceStructured {
		t.Fatalf("evidence level = %q, want %q", envelope.Summary.EvidenceLevel, EvidenceLevelVoiceStructured)
	}
	if envelope.Summary.IdempotencyKey != "fake-evt-7" {
		t.Fatalf("idempotency key = %q, want fake-evt-7", envelope.Summary.IdempotencyKey)
	}
}

func TestNormalizeVoiceAdapterEventsAssignsDeterministicSequences(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	baseTime := time.Date(2026, 5, 13, 11, 32, 0, 0, time.UTC)
	events := []VoiceAdapterEvent{
		voiceAdapterEventForSequence(runID, runAgentID, "evt-c", "fake.turn_completed", baseTime.Add(2*time.Second)),
		voiceAdapterEventForSequence(runID, runAgentID, "evt-b", "fake.transcript_final", baseTime),
		voiceAdapterEventForSequence(runID, runAgentID, "evt-a", "fake.speech_started", baseTime),
	}

	first, err := NormalizeVoiceAdapterEvents(events)
	if err != nil {
		t.Fatalf("NormalizeVoiceAdapterEvents returned error: %v", err)
	}
	second, err := NormalizeVoiceAdapterEvents([]VoiceAdapterEvent{events[2], events[0], events[1]})
	if err != nil {
		t.Fatalf("NormalizeVoiceAdapterEvents second run returned error: %v", err)
	}

	wantOrder := []string{"evt-a", "evt-b", "evt-c"}
	for idx, wantEventID := range wantOrder {
		if first[idx].EventID != wantEventID {
			t.Fatalf("first[%d].EventID = %q, want %q", idx, first[idx].EventID, wantEventID)
		}
		if first[idx].SequenceNumber != int64(idx+1) {
			t.Fatalf("first[%d].SequenceNumber = %d, want %d", idx, first[idx].SequenceNumber, idx+1)
		}
		if second[idx].EventID != wantEventID {
			t.Fatalf("second[%d].EventID = %q, want %q", idx, second[idx].EventID, wantEventID)
		}
		if second[idx].SequenceNumber != first[idx].SequenceNumber {
			t.Fatalf("second[%d].SequenceNumber = %d, want %d", idx, second[idx].SequenceNumber, first[idx].SequenceNumber)
		}
		if err := first[idx].ValidatePersisted(); err != nil {
			t.Fatalf("first[%d] persisted validation failed: %v", idx, err)
		}
	}
}

func voiceAdapterEventForSequence(runID uuid.UUID, runAgentID uuid.UUID, eventID string, rawEventType string, occurredAt time.Time) VoiceAdapterEvent {
	return VoiceAdapterEvent{
		EventID:      eventID,
		RunID:        runID,
		RunAgentID:   runAgentID,
		RawEventType: rawEventType,
		OccurredAt:   occurredAt,
		Payload:      json.RawMessage(`{"fixture":true}`),
		Summary: SummaryMetadata{
			TurnIndex: intPtr(1),
			Speaker:   "assistant",
			Channel:   "outbound",
		},
	}
}

func TestSummaryMetadataPreservesExplicitZeroTurnIndex(t *testing.T) {
	encoded, err := json.Marshal(SummaryMetadata{
		TurnIndex: intPtr(0),
		Speaker:   "caller",
	})
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	if !strings.Contains(string(encoded), `"turn_index":0`) {
		t.Fatalf("encoded summary = %s, want explicit zero turn_index", encoded)
	}

	encoded, err = json.Marshal(SummaryMetadata{Speaker: "caller"})
	if err != nil {
		t.Fatalf("marshal summary without turn index: %v", err)
	}
	if strings.Contains(string(encoded), "turn_index") {
		t.Fatalf("encoded summary = %s, want omitted turn_index when nil", encoded)
	}
}

func intPtr(value int) *int {
	return &value
}
