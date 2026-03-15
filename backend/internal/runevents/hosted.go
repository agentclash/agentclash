package runevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/hostedruns"
	"github.com/google/uuid"
)

func NormalizeHostedEvent(runID uuid.UUID, event hostedruns.Event) (Envelope, error) {
	if err := event.Validate(); err != nil {
		return Envelope{}, err
	}

	payload, err := hostedEventPayload(event)
	if err != nil {
		return Envelope{}, err
	}

	envelope := Envelope{
		EventID:        HostedEventID(event),
		SchemaVersion:  SchemaVersionV1,
		RunID:          runID,
		RunAgentID:     event.RunAgentID,
		SequenceNumber: 0,
		EventType:      normalizeHostedEventType(event),
		Source:         SourceHostedExternal,
		OccurredAt:     event.OccurredAt.UTC(),
		Payload:        payload,
		Summary: SummaryMetadata{
			Status:         hostedEventStatus(event),
			ExternalRunID:  event.ExternalRunID,
			EvidenceLevel:  hostedEvidenceLevel(event),
			IdempotencyKey: HostedEventID(event),
		},
	}
	return envelope, envelope.ValidatePending()
}

func HostedEventID(event hostedruns.Event) string {
	finalStatus := ""
	if event.FinalStatus != nil {
		finalStatus = *event.FinalStatus
	}
	return fmt.Sprintf(
		"hosted:%s:%s:%s:%s:%s",
		event.RunAgentID.String(),
		event.ExternalRunID,
		event.EventType,
		event.OccurredAt.UTC().Format(time.RFC3339Nano),
		finalStatus,
	)
}

func normalizeHostedEventType(event hostedruns.Event) Type {
	switch event.EventType {
	case hostedruns.EventTypeRunStarted:
		return EventTypeSystemRunStarted
	case hostedruns.EventTypeFinalAnswer:
		return EventTypeSystemOutputFinalized
	case hostedruns.EventTypeError:
		return EventTypeSystemRunFailed
	case hostedruns.EventTypeRunFinished:
		if event.FinalStatus != nil && *event.FinalStatus == hostedruns.FinalStatusCompleted {
			return EventTypeSystemRunCompleted
		}
		return EventTypeSystemRunFailed
	default:
		return EventTypeSystemRunFailed
	}
}

func hostedEventStatus(event hostedruns.Event) string {
	switch event.EventType {
	case hostedruns.EventTypeRunStarted:
		return "running"
	case hostedruns.EventTypeFinalAnswer:
		return "running"
	case hostedruns.EventTypeError:
		return "failed"
	case hostedruns.EventTypeRunFinished:
		if event.FinalStatus != nil && *event.FinalStatus == hostedruns.FinalStatusCompleted {
			return "completed"
		}
		return "failed"
	default:
		return "failed"
	}
}

func hostedEvidenceLevel(event hostedruns.Event) EvidenceLevel {
	if len(event.Metadata) == 0 {
		return EvidenceLevelHostedBlackBox
	}
	return EvidenceLevelHostedStructured
}

func hostedEventPayload(event hostedruns.Event) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]any{
		"raw_event_type":  event.EventType,
		"external_run_id": event.ExternalRunID,
		"final_status":    cloneStringPtr(event.FinalStatus),
		"output":          normalizeJSON(event.Output),
		"error_message":   cloneStringPtr(event.ErrorMessage),
		"latency_ms":      cloneInt64Ptr(event.LatencyMS),
		"metadata":        normalizeJSON(event.Metadata),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal hosted event payload: %w", err)
	}
	return payload, nil
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func normalizeJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}
