package runevents

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMultiTurnEventTypesAreValid(t *testing.T) {
	eventTypes := []Type{
		EventTypeTurnUserMessage,
		EventTypeTurnUserSimulated,
		EventTypeTurnAssistantMessage,
		EventTypeTurnAwaitingHuman,
		EventTypeTurnStateCaptured,
		EventTypeConversationCompleted,
	}

	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			envelope := baseMultiTurnEnvelope(eventType, json.RawMessage(`{"content":"hello","actor":"scripted","phase_id":"open"}`))
			if err := envelope.ValidatePending(); err != nil {
				t.Fatalf("ValidatePending() error = %v", err)
			}
		})
	}
}

func TestValidateMultiTurnEventRequiredFields(t *testing.T) {
	t.Run("turn.user.message requires turn_index and content", func(t *testing.T) {
		envelope := baseMultiTurnEnvelope(EventTypeTurnUserMessage, json.RawMessage(`{"content":"","actor":"scripted","phase_id":"open"}`))
		if err := ValidateMultiTurnEvent(envelope); err == nil {
			t.Fatal("expected validation error for empty content")
		}
	})

	t.Run("turn.assistant.message requires content", func(t *testing.T) {
		envelope := baseMultiTurnEnvelope(EventTypeTurnAssistantMessage, json.RawMessage(`{"content":"done","phase_id":"open"}`))
		envelope.Summary.TurnIndex = intPtr(0)
		envelope.Summary.PhaseID = "open"
		if err := ValidateMultiTurnEvent(envelope); err != nil {
			t.Fatalf("ValidateMultiTurnEvent() error = %v", err)
		}
	})

	t.Run("native turn.completed requires actor and mismatch", func(t *testing.T) {
		envelope := baseMultiTurnEnvelope(EventTypeTurnCompleted, json.RawMessage(`{}`))
		envelope.Source = SourceNativeEngine
		envelope.Summary.TurnIndex = intPtr(1)
		envelope.Summary.PhaseID = "open"
		if err := ValidateMultiTurnEvent(envelope); err == nil {
			t.Fatal("expected validation error for missing actor/mismatch")
		}
	})
}

func TestVoiceTurnCompletedStillValidWithoutPhaseID(t *testing.T) {
	envelope := Envelope{
		EventID:       "evt-voice-turn",
		SchemaVersion: SchemaVersionV1,
		RunID:         uuid.New(),
		RunAgentID:    uuid.New(),
		EventType:     EventTypeTurnCompleted,
		Source:        SourceVoiceAdapter,
		OccurredAt:    time.Date(2026, 5, 13, 11, 30, 0, 0, time.UTC),
		Payload:       json.RawMessage(`{"turn_id":"turn-001"}`),
		Summary: SummaryMetadata{
			TurnIndex:     intPtr(1),
			Speaker:       "agent",
			EvidenceLevel: EvidenceLevelVoiceStructured,
		},
	}
	if err := envelope.ValidatePending(); err != nil {
		t.Fatalf("ValidatePending() error = %v", err)
	}
	if err := ValidateMultiTurnEvent(envelope); err != nil {
		t.Fatalf("ValidateMultiTurnEvent() voice event error = %v", err)
	}
}

func TestTranscriptFromEventsOrdersAndGroupsTurns(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	occurredAt := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)

	events := []Envelope{
		multiTurnEvent(1, EventTypeTurnUserMessage, runID, runAgentID, occurredAt, SummaryMetadata{
			TurnIndex: intPtr(0),
			PhaseID:   "open",
			Actor:     ConversationActorScripted,
		}, `{"content":"Refund order 42","actor":"scripted","phase_id":"open"}`),
		multiTurnEvent(3, EventTypeTurnAssistantMessage, runID, runAgentID, occurredAt, SummaryMetadata{
			TurnIndex: intPtr(0),
			PhaseID:   "open",
		}, `{"content":"Checking order 42","phase_id":"open"}`),
		multiTurnEvent(2, EventTypeTurnUserSimulated, runID, runAgentID, occurredAt, SummaryMetadata{
			TurnIndex: intPtr(1),
			PhaseID:   "dynamic",
		}, `{"provider_key":"openai","model":"gpt-4.1-mini"}`),
		multiTurnEvent(4, EventTypeTurnCompleted, runID, runAgentID, occurredAt, SummaryMetadata{
			TurnIndex: intPtr(0),
			PhaseID:   "open",
			Actor:     ConversationActorScripted,
			Mismatch:  boolPtr(false),
		}, `{}`),
	}

	transcript, err := TranscriptFromEvents(events)
	if err != nil {
		t.Fatalf("TranscriptFromEvents() error = %v", err)
	}
	if len(transcript) != 2 {
		t.Fatalf("len(transcript) = %d, want 2", len(transcript))
	}
	if transcript[0].UserMessage != "Refund order 42" || transcript[0].AssistantMessage != "Checking order 42" {
		t.Fatalf("turn 0 = %+v, want user+assistant messages", transcript[0])
	}
	if !transcript[0].Completed || transcript[0].FirstSequence != 1 || transcript[0].LastSequence != 4 {
		t.Fatalf("turn 0 metadata = %+v", transcript[0])
	}
	if !transcript[1].UserSimulated || transcript[1].PhaseID != "dynamic" {
		t.Fatalf("turn 1 = %+v, want simulated phase", transcript[1])
	}
}

func TestTranscriptFromEventsPreservesMismatch(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	occurredAt := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)

	events := []Envelope{
		multiTurnEvent(1, EventTypeTurnUserMessage, runID, runAgentID, occurredAt, SummaryMetadata{
			TurnIndex: intPtr(0),
			PhaseID:   "open",
			Actor:     ConversationActorScripted,
		}, `{"content":"Hi","actor":"scripted","phase_id":"open"}`),
		multiTurnEvent(2, EventTypeTurnCompleted, runID, runAgentID, occurredAt, SummaryMetadata{
			TurnIndex: intPtr(0),
			PhaseID:   "open",
			Actor:     ConversationActorScripted,
			Mismatch:  boolPtr(true),
		}, `{}`),
	}

	transcript, err := TranscriptFromEvents(events)
	if err != nil {
		t.Fatalf("TranscriptFromEvents() error = %v", err)
	}
	if len(transcript) != 1 || !transcript[0].Mismatch {
		t.Fatalf("transcript = %+v, want mismatch true", transcript)
	}

	encoded, err := json.Marshal(events[1].Summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	if !strings.Contains(string(encoded), `"mismatch":true`) {
		t.Fatalf("encoded summary = %s, want explicit mismatch true", encoded)
	}
}

func TestValidateMultiTurnEventRejectsInvalidActor(t *testing.T) {
	envelope := baseMultiTurnEnvelope(EventTypeTurnUserMessage, json.RawMessage(`{"content":"Hi","actor":"bot","phase_id":"open"}`))
	envelope.Summary.TurnIndex = intPtr(0)
	envelope.Summary.Actor = "bot"
	if err := ValidateMultiTurnEvent(envelope); !errors.Is(err, ErrMultiTurnInvalidActor) {
		t.Fatalf("error = %v, want ErrMultiTurnInvalidActor", err)
	}
}

func baseMultiTurnEnvelope(eventType Type, payload json.RawMessage) Envelope {
	return Envelope{
		EventID:       "evt-mt",
		SchemaVersion: SchemaVersionV1,
		RunID:         uuid.New(),
		RunAgentID:    uuid.New(),
		EventType:     eventType,
		Source:        SourceNativeEngine,
		OccurredAt:    time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC),
		Payload:       payload,
		Summary:       SummaryMetadata{},
	}
}

func multiTurnEvent(seq int64, eventType Type, runID uuid.UUID, runAgentID uuid.UUID, occurredAt time.Time, summary SummaryMetadata, payload string) Envelope {
	return Envelope{
		EventID:        "evt-" + string(eventType),
		SchemaVersion:  SchemaVersionV1,
		RunID:          runID,
		RunAgentID:     runAgentID,
		SequenceNumber: seq,
		EventType:      eventType,
		Source:         SourceNativeEngine,
		OccurredAt:     occurredAt,
		Payload:        json.RawMessage(payload),
		Summary:        summary,
	}
}

func boolPtr(value bool) *bool {
	return &value
}
