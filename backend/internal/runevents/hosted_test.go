package runevents

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/hostedruns"
	"github.com/google/uuid"
)

func TestNormalizeHostedEventMapsCompletedRunFinishedToCanonicalEnvelope(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	finalStatus := hostedruns.FinalStatusCompleted
	event := hostedruns.Event{
		RunAgentID:    runAgentID,
		ExternalRunID: "ext-123",
		EventType:     hostedruns.EventTypeRunFinished,
		OccurredAt:    time.Date(2026, 3, 15, 11, 22, 33, 0, time.UTC),
		FinalStatus:   &finalStatus,
		Output:        json.RawMessage(`{"answer":"done"}`),
	}

	envelope, err := NormalizeHostedEvent(runID, event)
	if err != nil {
		t.Fatalf("NormalizeHostedEvent returned error: %v", err)
	}

	if envelope.EventType != EventTypeSystemRunCompleted {
		t.Fatalf("event type = %q, want %q", envelope.EventType, EventTypeSystemRunCompleted)
	}
	if envelope.Source != SourceHostedExternal {
		t.Fatalf("source = %q, want %q", envelope.Source, SourceHostedExternal)
	}
	if envelope.Summary.Status != "completed" {
		t.Fatalf("summary status = %q, want completed", envelope.Summary.Status)
	}
	if envelope.Summary.EvidenceLevel != EvidenceLevelHostedBlackBox {
		t.Fatalf("evidence level = %q, want %q", envelope.Summary.EvidenceLevel, EvidenceLevelHostedBlackBox)
	}
	if envelope.SequenceNumber != 0 {
		t.Fatalf("sequence number = %d, want 0 before persistence", envelope.SequenceNumber)
	}
	if err := envelope.ValidatePending(); err != nil {
		t.Fatalf("pending envelope should validate: %v", err)
	}
}

func TestNormalizeHostedEventTreatsMetadataAsStructuredEvidence(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	event := hostedruns.Event{
		RunAgentID:    runAgentID,
		ExternalRunID: "ext-structured",
		EventType:     hostedruns.EventTypeFinalAnswer,
		OccurredAt:    time.Date(2026, 3, 15, 11, 22, 33, 0, time.UTC),
		Output:        json.RawMessage(`{"answer":"done"}`),
		Metadata:      json.RawMessage(`{"trace":"available"}`),
	}

	envelope, err := NormalizeHostedEvent(runID, event)
	if err != nil {
		t.Fatalf("NormalizeHostedEvent returned error: %v", err)
	}

	if envelope.EventType != EventTypeSystemOutputFinalized {
		t.Fatalf("event type = %q, want %q", envelope.EventType, EventTypeSystemOutputFinalized)
	}
	if envelope.Summary.EvidenceLevel != EvidenceLevelHostedStructured {
		t.Fatalf("evidence level = %q, want %q", envelope.Summary.EvidenceLevel, EvidenceLevelHostedStructured)
	}
}

func TestNormalizeHostedEventMapsRunStartedToCanonicalEnvelope(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	event := hostedruns.Event{
		RunAgentID:    runAgentID,
		ExternalRunID: "ext-start",
		EventType:     hostedruns.EventTypeRunStarted,
		OccurredAt:    time.Date(2026, 3, 15, 11, 22, 33, 0, time.UTC),
	}

	envelope, err := NormalizeHostedEvent(runID, event)
	if err != nil {
		t.Fatalf("NormalizeHostedEvent returned error: %v", err)
	}
	if envelope.EventType != EventTypeSystemRunStarted {
		t.Fatalf("event type = %q, want %q", envelope.EventType, EventTypeSystemRunStarted)
	}
	if envelope.Summary.Status != "running" {
		t.Fatalf("summary status = %q, want running", envelope.Summary.Status)
	}
}

func TestNormalizeHostedEventMapsErrorToCanonicalEnvelope(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	event := hostedruns.Event{
		RunAgentID:    runAgentID,
		ExternalRunID: "ext-error",
		EventType:     hostedruns.EventTypeError,
		OccurredAt:    time.Date(2026, 3, 15, 11, 22, 33, 0, time.UTC),
		ErrorMessage:  stringPtr("boom"),
	}

	envelope, err := NormalizeHostedEvent(runID, event)
	if err != nil {
		t.Fatalf("NormalizeHostedEvent returned error: %v", err)
	}
	if envelope.EventType != EventTypeSystemRunFailed {
		t.Fatalf("event type = %q, want %q", envelope.EventType, EventTypeSystemRunFailed)
	}
	if envelope.Summary.Status != "failed" {
		t.Fatalf("summary status = %q, want failed", envelope.Summary.Status)
	}
}

func TestNormalizeHostedEventMapsFailedRunFinishedToCanonicalEnvelope(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	finalStatus := hostedruns.FinalStatusFailed
	event := hostedruns.Event{
		RunAgentID:    runAgentID,
		ExternalRunID: "ext-failed-finish",
		EventType:     hostedruns.EventTypeRunFinished,
		OccurredAt:    time.Date(2026, 3, 15, 11, 22, 33, 0, time.UTC),
		FinalStatus:   &finalStatus,
		ErrorMessage:  stringPtr("failed"),
	}

	envelope, err := NormalizeHostedEvent(runID, event)
	if err != nil {
		t.Fatalf("NormalizeHostedEvent returned error: %v", err)
	}
	if envelope.EventType != EventTypeSystemRunFailed {
		t.Fatalf("event type = %q, want %q", envelope.EventType, EventTypeSystemRunFailed)
	}
	if envelope.Summary.Status != "failed" {
		t.Fatalf("summary status = %q, want failed", envelope.Summary.Status)
	}
}

func TestNormalizeHostedTraceEventsExtractsStructuredTraceFragments(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	occurredAt := time.Date(2026, 3, 15, 11, 22, 33, 0, time.UTC)
	event := hostedruns.Event{
		RunAgentID:    runAgentID,
		ExternalRunID: "ext-trace",
		EventType:     hostedruns.EventTypeFinalAnswer,
		OccurredAt:    occurredAt,
		Metadata: json.RawMessage(`{
			"trace_events": [
				{
					"event_type": "system.step.started",
					"occurred_at": "2026-03-15T11:22:30Z",
					"payload": {"step_index": 1, "subagent_key": "triage", "subagent_label": "Triage"},
					"summary": {"status": "running", "step_index": 1}
				},
				{
					"event_type": "model.call.completed",
					"payload": {"provider_model_id": "gpt-4.1", "subagent_label": "Triage"},
					"summary": {"status": "completed", "provider_model_id": "gpt-4.1"}
				}
			]
		}`),
	}

	envelopes, err := NormalizeHostedTraceEvents(runID, event)
	if err != nil {
		t.Fatalf("NormalizeHostedTraceEvents returned error: %v", err)
	}
	if len(envelopes) != 2 {
		t.Fatalf("trace envelope count = %d, want 2", len(envelopes))
	}
	if envelopes[0].EventType != EventTypeSystemStepStarted {
		t.Fatalf("first trace event type = %q, want %q", envelopes[0].EventType, EventTypeSystemStepStarted)
	}
	if envelopes[0].Summary.EvidenceLevel != EvidenceLevelHostedStructured {
		t.Fatalf("first evidence level = %q, want %q", envelopes[0].Summary.EvidenceLevel, EvidenceLevelHostedStructured)
	}
	if envelopes[1].EventType != EventTypeModelCallCompleted {
		t.Fatalf("second trace event type = %q, want %q", envelopes[1].EventType, EventTypeModelCallCompleted)
	}
	if envelopes[1].Summary.ProviderModelID != "gpt-4.1" {
		t.Fatalf("second provider model id = %q, want gpt-4.1", envelopes[1].Summary.ProviderModelID)
	}
}

func TestEnvelopeValidatePersistedRequiresPositiveSequenceNumber(t *testing.T) {
	envelope := Envelope{
		EventID:       "evt-1",
		SchemaVersion: SchemaVersionV1,
		RunID:         uuid.New(),
		RunAgentID:    uuid.New(),
		EventType:     EventTypeSystemRunStarted,
		Source:        SourceHostedExternal,
		OccurredAt:    time.Now().UTC(),
		Payload:       json.RawMessage(`{}`),
	}

	if err := envelope.ValidatePersisted(); err == nil {
		t.Fatalf("expected persisted validation to fail without sequence number")
	}
}

func TestEnvelopeValidatePendingAllowsEmptyPayload(t *testing.T) {
	envelope := Envelope{
		EventID:       "evt-1",
		SchemaVersion: SchemaVersionV1,
		RunID:         uuid.New(),
		RunAgentID:    uuid.New(),
		EventType:     EventTypeSystemRunStarted,
		Source:        SourceHostedExternal,
		OccurredAt:    time.Now().UTC(),
	}

	if err := envelope.ValidatePending(); err != nil {
		t.Fatalf("ValidatePending returned error for empty payload: %v", err)
	}
	if len(envelope.Payload) != 0 {
		t.Fatalf("payload length = %d, want 0", len(envelope.Payload))
	}
}

func stringPtr(value string) *string {
	return &value
}
