package runevents

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/hostedruns"
	"github.com/google/uuid"
)

type hostedTraceDocument struct {
	TraceEvents []hostedTraceEventDocument `json:"trace_events"`
	Events      []hostedTraceEventDocument `json:"events"`
}

type hostedTraceEventDocument struct {
	EventType  string          `json:"event_type"`
	OccurredAt *time.Time      `json:"occurred_at,omitempty"`
	Source     string          `json:"source,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Summary    json.RawMessage `json:"summary,omitempty"`
	EventID    string          `json:"event_id,omitempty"`
}

type hostedTraceSummaryDocument struct {
	Status          string `json:"status,omitempty"`
	StepIndex       int    `json:"step_index,omitempty"`
	ProviderKey     string `json:"provider_key,omitempty"`
	ProviderModelID string `json:"provider_model_id,omitempty"`
	ToolName        string `json:"tool_name,omitempty"`
	ToolCategory    string `json:"tool_category,omitempty"`
	SandboxAction   string `json:"sandbox_action,omitempty"`
	MetricKey       string `json:"metric_key,omitempty"`
	ExternalRunID   string `json:"external_run_id,omitempty"`
	EvidenceLevel   string `json:"evidence_level,omitempty"`
	IdempotencyKey  string `json:"idempotency_key,omitempty"`
}

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

func NormalizeHostedTraceEvents(runID uuid.UUID, event hostedruns.Event) ([]Envelope, error) {
	if len(event.Metadata) == 0 {
		return nil, nil
	}
	var doc hostedTraceDocument
	if err := json.Unmarshal(event.Metadata, &doc); err != nil {
		return nil, nil
	}
	traceEvents := doc.TraceEvents
	if len(traceEvents) == 0 {
		traceEvents = doc.Events
	}
	if len(traceEvents) == 0 {
		return nil, nil
	}

	envelopes := make([]Envelope, 0, len(traceEvents))
	for idx, traceEvent := range traceEvents {
		eventType := Type(traceEvent.EventType)
		if !isValidType(eventType) {
			return nil, fmt.Errorf("invalid hosted trace event type %q", traceEvent.EventType)
		}
		source := SourceHostedExternal
		if traceEvent.Source != "" {
			source = Source(traceEvent.Source)
			if !isValidSource(source) {
				return nil, fmt.Errorf("invalid hosted trace event source %q", traceEvent.Source)
			}
		}
		occurredAt := event.OccurredAt.UTC()
		if traceEvent.OccurredAt != nil && !traceEvent.OccurredAt.IsZero() {
			occurredAt = traceEvent.OccurredAt.UTC()
		}
		summary, err := normalizeHostedTraceSummary(traceEvent.Summary, event)
		if err != nil {
			return nil, fmt.Errorf("normalize hosted trace summary[%d]: %w", idx, err)
		}
		if summary.ExternalRunID == "" {
			summary.ExternalRunID = event.ExternalRunID
		}
		if summary.EvidenceLevel == "" {
			summary.EvidenceLevel = EvidenceLevelHostedStructured
		}
		eventID := strings.TrimSpace(traceEvent.EventID)
		if eventID == "" {
			eventID = fmt.Sprintf("%s:trace:%d", HostedEventID(event), idx+1)
		}
		if summary.IdempotencyKey == "" {
			summary.IdempotencyKey = eventID
		}
		envelope := Envelope{
			EventID:        eventID,
			SchemaVersion:  SchemaVersionV1,
			RunID:          runID,
			RunAgentID:     event.RunAgentID,
			SequenceNumber: 0,
			EventType:      eventType,
			Source:         source,
			OccurredAt:     occurredAt,
			Payload:        normalizeJSON(traceEvent.Payload),
			Summary:        summary,
		}
		if err := envelope.ValidatePending(); err != nil {
			return nil, err
		}
		envelopes = append(envelopes, envelope)
	}
	sort.SliceStable(envelopes, func(i, j int) bool {
		if envelopes[i].OccurredAt.Equal(envelopes[j].OccurredAt) {
			return envelopes[i].EventID < envelopes[j].EventID
		}
		return envelopes[i].OccurredAt.Before(envelopes[j].OccurredAt)
	})
	return envelopes, nil
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

func normalizeHostedTraceSummary(raw json.RawMessage, event hostedruns.Event) (SummaryMetadata, error) {
	summary := SummaryMetadata{
		ExternalRunID: event.ExternalRunID,
		EvidenceLevel: EvidenceLevelHostedStructured,
	}
	if len(raw) == 0 {
		return summary, nil
	}
	var decoded hostedTraceSummaryDocument
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return SummaryMetadata{}, err
	}
	summary.Status = decoded.Status
	summary.StepIndex = decoded.StepIndex
	summary.ProviderKey = decoded.ProviderKey
	summary.ProviderModelID = decoded.ProviderModelID
	summary.ToolName = decoded.ToolName
	summary.ToolCategory = decoded.ToolCategory
	summary.SandboxAction = decoded.SandboxAction
	summary.MetricKey = decoded.MetricKey
	if decoded.ExternalRunID != "" {
		summary.ExternalRunID = decoded.ExternalRunID
	}
	if decoded.EvidenceLevel != "" {
		summary.EvidenceLevel = EvidenceLevel(decoded.EvidenceLevel)
	}
	summary.IdempotencyKey = decoded.IdempotencyKey
	return summary, nil
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
