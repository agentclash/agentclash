package runevents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type VoiceAdapterEvent struct {
	EventID      string
	RunID        uuid.UUID
	RunAgentID   uuid.UUID
	RawEventType string
	Source       Source
	OccurredAt   time.Time
	Payload      json.RawMessage
	Summary      SummaryMetadata
}

var canonicalVoiceEventTypes = map[Type]struct{}{
	EventTypeMediaSessionStarted: {},
	EventTypeMediaFrameReceived:  {},
	EventTypeSpeechStarted:       {},
	EventTypeSpeechStopped:       {},
	EventTypeTranscriptPartial:   {},
	EventTypeTranscriptFinal:     {},
	EventTypeTurnCompleted:       {},
	EventTypeAgentAudioStarted:   {},
	EventTypeAgentAudioCompleted: {},
	EventTypeBargeInDetected:     {},
	EventTypeAudioBufferCleared:  {},
	EventTypeDTMFReceived:        {},
	EventTypeTelephonyAnswered:   {},
	EventTypeTelephonyVoicemail:  {},
	EventTypeTransferStarted:     {},
	EventTypeTransferBridged:     {},
	EventTypeVoiceMetricRecorded: {},
}

var voiceAdapterRawEventTypes = map[string]Type{
	"fake.media_session_started": EventTypeMediaSessionStarted,
	"fake.media_frame":           EventTypeMediaFrameReceived,
	"fake.speech_started":        EventTypeSpeechStarted,
	"fake.speech_stopped":        EventTypeSpeechStopped,
	"fake.transcript_partial":    EventTypeTranscriptPartial,
	"fake.transcript_final":      EventTypeTranscriptFinal,
	"fake.turn_completed":        EventTypeTurnCompleted,
	"fake.agent_audio_started":   EventTypeAgentAudioStarted,
	"fake.agent_audio_completed": EventTypeAgentAudioCompleted,
	"fake.barge_in":              EventTypeBargeInDetected,
	"fake.audio_buffer_cleared":  EventTypeAudioBufferCleared,
	"fake.dtmf":                  EventTypeDTMFReceived,
	"fake.telephony_answered":    EventTypeTelephonyAnswered,
	"fake.telephony_voicemail":   EventTypeTelephonyVoicemail,
	"fake.transfer_started":      EventTypeTransferStarted,
	"fake.transfer_bridged":      EventTypeTransferBridged,
	"fake.voice_metric_recorded": EventTypeVoiceMetricRecorded,
}

func NormalizeVoiceAdapterEvent(event VoiceAdapterEvent) (Envelope, error) {
	eventType, err := normalizeVoiceAdapterEventType(event.RawEventType)
	if err != nil {
		return Envelope{}, err
	}

	source := event.Source
	if source == "" {
		source = SourceVoiceAdapter
	}

	payload := normalizeJSON(event.Payload)
	summary := event.Summary
	if summary.EvidenceLevel == "" {
		summary.EvidenceLevel = EvidenceLevelVoiceStructured
	}

	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		eventID = deterministicVoiceEventID(event, eventType, payload)
	}
	if summary.IdempotencyKey == "" {
		summary.IdempotencyKey = eventID
	}

	envelope := Envelope{
		EventID:        eventID,
		SchemaVersion:  SchemaVersionV1,
		RunID:          event.RunID,
		RunAgentID:     event.RunAgentID,
		SequenceNumber: 0,
		EventType:      eventType,
		Source:         source,
		OccurredAt:     event.OccurredAt.UTC(),
		Payload:        payload,
		Summary:        summary,
	}
	return envelope, envelope.ValidatePending()
}

func NormalizeVoiceAdapterEvents(events []VoiceAdapterEvent) ([]Envelope, error) {
	envelopes := make([]Envelope, 0, len(events))
	for idx, event := range events {
		envelope, err := NormalizeVoiceAdapterEvent(event)
		if err != nil {
			return nil, fmt.Errorf("normalize voice adapter event[%d]: %w", idx, err)
		}
		envelopes = append(envelopes, envelope)
	}

	sort.SliceStable(envelopes, func(i, j int) bool {
		if !envelopes[i].OccurredAt.Equal(envelopes[j].OccurredAt) {
			return envelopes[i].OccurredAt.Before(envelopes[j].OccurredAt)
		}
		if envelopes[i].EventID != envelopes[j].EventID {
			return envelopes[i].EventID < envelopes[j].EventID
		}
		if envelopes[i].EventType != envelopes[j].EventType {
			return envelopes[i].EventType < envelopes[j].EventType
		}
		return string(envelopes[i].Payload) < string(envelopes[j].Payload)
	})

	for idx := range envelopes {
		envelopes[idx] = envelopes[idx].WithSequenceNumber(int64(idx + 1))
		if err := envelopes[idx].ValidatePersisted(); err != nil {
			return nil, fmt.Errorf("validate sequenced voice adapter event[%d]: %w", idx, err)
		}
	}
	return envelopes, nil
}

func normalizeVoiceAdapterEventType(rawEventType string) (Type, error) {
	trimmed := strings.TrimSpace(rawEventType)
	if eventType, ok := voiceAdapterRawEventTypes[trimmed]; ok {
		return eventType, nil
	}

	eventType := Type(trimmed)
	if _, ok := canonicalVoiceEventTypes[eventType]; ok {
		return eventType, nil
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidEventType, rawEventType)
}

func deterministicVoiceEventID(event VoiceAdapterEvent, eventType Type, payload json.RawMessage) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf(
		"%s\n%s\n%s\n%s\n%s",
		event.RunAgentID.String(),
		strings.TrimSpace(event.RawEventType),
		eventType,
		event.OccurredAt.UTC().Format(time.RFC3339Nano),
		string(payload),
	)))
	return fmt.Sprintf("voice:%s:%s", event.RunAgentID.String(), hex.EncodeToString(hash[:8]))
}
