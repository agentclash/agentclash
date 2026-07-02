package runevents

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	ConversationActorScripted = "scripted"
	ConversationActorLLM      = "llm"
	ConversationActorHuman    = "human"
)

var (
	ErrMultiTurnSummaryRequired = errors.New("multi-turn event summary field is required")
	ErrMultiTurnPayloadRequired = errors.New("multi-turn event payload field is required")
	ErrMultiTurnInvalidActor    = errors.New("invalid multi-turn conversation actor")
)

type turnUserMessagePayload struct {
	Content string `json:"content"`
	Actor   string `json:"actor"`
	PhaseID string `json:"phase_id"`
}

type turnAssistantMessagePayload struct {
	Content string `json:"content"`
	PhaseID string `json:"phase_id,omitempty"`
}

type turnAwaitingHumanPayload struct {
	PromptHint string `json:"prompt_hint,omitempty"`
	PhaseID    string `json:"phase_id"`
}

type turnStateCapturedPayload struct {
	SnapshotRef string `json:"snapshot_ref"`
}

func IsMultiTurnConversationEventType(eventType Type) bool {
	switch eventType {
	case EventTypeTurnUserMessage,
		EventTypeTurnUserSimulated,
		EventTypeTurnAssistantMessage,
		EventTypeTurnCompleted,
		EventTypeTurnAwaitingHuman,
		EventTypeTurnStateCaptured,
		EventTypeConversationCompleted:
		return true
	default:
		return false
	}
}

func ValidateMultiTurnEvent(envelope Envelope) error {
	if err := envelope.ValidatePending(); err != nil {
		return err
	}
	if !IsMultiTurnConversationEventType(envelope.EventType) {
		return fmt.Errorf("not a multi-turn conversation event type: %q", envelope.EventType)
	}

	switch envelope.EventType {
	case EventTypeTurnUserMessage:
		return validateTurnUserMessage(envelope)
	case EventTypeTurnUserSimulated:
		return validateTurnUserSimulated(envelope)
	case EventTypeTurnAssistantMessage:
		return validateTurnAssistantMessage(envelope)
	case EventTypeTurnCompleted:
		return validateTurnCompleted(envelope)
	case EventTypeTurnAwaitingHuman:
		return validateTurnAwaitingHuman(envelope)
	case EventTypeTurnStateCaptured:
		return validateTurnStateCaptured(envelope)
	case EventTypeConversationCompleted:
		return nil
	default:
		return fmt.Errorf("unsupported multi-turn event type: %q", envelope.EventType)
	}
}

func validateTurnIndex(summary SummaryMetadata) error {
	if summary.TurnIndex == nil {
		return fmt.Errorf("%w: turn_index", ErrMultiTurnSummaryRequired)
	}
	if *summary.TurnIndex < 0 {
		return fmt.Errorf("turn_index must be >= 0")
	}
	return nil
}

func validatePhaseID(summary SummaryMetadata, payloadPhaseID string) error {
	phaseID := strings.TrimSpace(summary.PhaseID)
	if phaseID == "" {
		phaseID = strings.TrimSpace(payloadPhaseID)
	}
	if phaseID == "" {
		return fmt.Errorf("%w: phase_id", ErrMultiTurnSummaryRequired)
	}
	return nil
}

func validateConversationActor(actor string) error {
	switch strings.TrimSpace(actor) {
	case ConversationActorScripted, ConversationActorLLM, ConversationActorHuman:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrMultiTurnInvalidActor, actor)
	}
}

func validateTurnUserMessage(envelope Envelope) error {
	if err := validateTurnIndex(envelope.Summary); err != nil {
		return err
	}
	var payload turnUserMessagePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("decode turn.user.message payload: %w", err)
	}
	if strings.TrimSpace(payload.Content) == "" {
		return fmt.Errorf("%w: content", ErrMultiTurnPayloadRequired)
	}
	actor := strings.TrimSpace(envelope.Summary.Actor)
	if actor == "" {
		actor = strings.TrimSpace(payload.Actor)
	}
	if err := validateConversationActor(actor); err != nil {
		return err
	}
	return validatePhaseID(envelope.Summary, payload.PhaseID)
}

func validateTurnUserSimulated(envelope Envelope) error {
	if err := validateTurnIndex(envelope.Summary); err != nil {
		return err
	}
	return validatePhaseID(envelope.Summary, "")
}

func validateTurnAssistantMessage(envelope Envelope) error {
	if err := validateTurnIndex(envelope.Summary); err != nil {
		return err
	}
	var payload turnAssistantMessagePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("decode turn.assistant.message payload: %w", err)
	}
	if strings.TrimSpace(payload.Content) == "" {
		return fmt.Errorf("%w: content", ErrMultiTurnPayloadRequired)
	}
	return validatePhaseID(envelope.Summary, payload.PhaseID)
}

func validateTurnCompleted(envelope Envelope) error {
	// Voice adapter events may omit phase_id/actor; native multi-turn emitters must set them.
	if envelope.Source == SourceVoiceAdapter || envelope.Source == SourceMediaGateway {
		return nil
	}
	if err := validateTurnIndex(envelope.Summary); err != nil {
		return err
	}
	actor := strings.TrimSpace(envelope.Summary.Actor)
	if actor == "" {
		return fmt.Errorf("%w: actor", ErrMultiTurnSummaryRequired)
	}
	if err := validateConversationActor(actor); err != nil {
		return err
	}
	if envelope.Summary.Mismatch == nil {
		return fmt.Errorf("%w: mismatch", ErrMultiTurnSummaryRequired)
	}
	return validatePhaseID(envelope.Summary, "")
}

func validateTurnAwaitingHuman(envelope Envelope) error {
	if err := validateTurnIndex(envelope.Summary); err != nil {
		return err
	}
	payloadPhaseID := ""
	if len(envelope.Payload) > 0 {
		var payload turnAwaitingHumanPayload
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			return fmt.Errorf("decode turn.awaiting_human payload: %w", err)
		}
		payloadPhaseID = payload.PhaseID
	}
	return validatePhaseID(envelope.Summary, payloadPhaseID)
}

func validateTurnStateCaptured(envelope Envelope) error {
	if envelope.Summary.TurnIndex != nil && *envelope.Summary.TurnIndex < 0 {
		return fmt.Errorf("turn_index must be >= 0")
	}
	if len(envelope.Payload) == 0 {
		return fmt.Errorf("%w: payload", ErrMultiTurnPayloadRequired)
	}
	var payload turnStateCapturedPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("decode turn.state.captured payload: %w", err)
	}
	if strings.TrimSpace(payload.SnapshotRef) == "" {
		return fmt.Errorf("%w: snapshot_ref", ErrMultiTurnPayloadRequired)
	}
	return nil
}

func decodeTurnUserMessageContent(payload json.RawMessage) (string, error) {
	var decoded turnUserMessagePayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", err
	}
	return decoded.Content, nil
}

func decodeTurnAssistantMessageContent(payload json.RawMessage) (string, error) {
	var decoded turnAssistantMessagePayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", err
	}
	return decoded.Content, nil
}
