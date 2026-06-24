package hostedruns

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	TraceLevelBlackBox        = "black_box"
	TraceLevelStructuredTrace = "structured_trace"

	EventTypeRunStarted  = "run_started"
	EventTypeFinalAnswer = "final_answer"
	EventTypeError       = "error"
	EventTypeRunFinished = "run_finished"

	FinalStatusCompleted = "completed"
	FinalStatusFailed    = "failed"
)

var (
	ErrInvalidTraceLevel  = errors.New("invalid hosted trace level")
	ErrInvalidEventType   = errors.New("invalid hosted event type")
	ErrInvalidFinalStatus = errors.New("invalid hosted final status")
)

type StartRequest struct {
	RunID                  uuid.UUID       `json:"run_id"`
	RunAgentID             uuid.UUID       `json:"run_agent_id"`
	ChallengePackVersionID uuid.UUID       `json:"challenge_pack_version_id"`
	TaskPayload            json.RawMessage `json:"task_payload"`
	TraceLevel             string          `json:"trace_level"`
	CallbackURL            string          `json:"callback_url"`
	CallbackToken          string          `json:"callback_token"`
	DeadlineAt             time.Time       `json:"deadline_at"`
}

type StartResponse struct {
	Accepted      bool   `json:"accepted"`
	ExternalRunID string `json:"external_run_id"`
}

type Event struct {
	RunAgentID    uuid.UUID       `json:"run_agent_id"`
	ExternalRunID string          `json:"external_run_id"`
	EventType     string          `json:"event_type"`
	OccurredAt    time.Time       `json:"occurred_at"`
	FinalStatus   *string         `json:"final_status,omitempty"`
	Output        json.RawMessage `json:"output,omitempty"`
	ErrorMessage  *string         `json:"error_message,omitempty"`
	LatencyMS     *int64          `json:"latency_ms,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

func (r StartRequest) Validate() error {
	if r.TraceLevel != TraceLevelBlackBox && r.TraceLevel != TraceLevelStructuredTrace {
		return fmt.Errorf("%w: %q", ErrInvalidTraceLevel, r.TraceLevel)
	}
	if r.CallbackURL == "" {
		return errors.New("callback_url is required")
	}
	if r.CallbackToken == "" {
		return errors.New("callback_token is required")
	}
	if r.DeadlineAt.IsZero() {
		return errors.New("deadline_at is required")
	}
	return nil
}

func (e Event) Validate() error {
	switch e.EventType {
	case EventTypeRunStarted, EventTypeFinalAnswer, EventTypeError, EventTypeRunFinished:
	default:
		return fmt.Errorf("%w: %q", ErrInvalidEventType, e.EventType)
	}
	if e.RunAgentID == uuid.Nil {
		return errors.New("run_agent_id is required")
	}
	if e.ExternalRunID == "" {
		return errors.New("external_run_id is required")
	}
	if e.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}
	if e.EventType == EventTypeError && e.ErrorMessage == nil {
		return errors.New("error_message is required for error events")
	}
	if e.EventType == EventTypeRunFinished {
		if e.FinalStatus == nil {
			return errors.New("final_status is required for run_finished events")
		}
		if *e.FinalStatus != FinalStatusCompleted && *e.FinalStatus != FinalStatusFailed {
			return fmt.Errorf("%w: %q", ErrInvalidFinalStatus, *e.FinalStatus)
		}
		if *e.FinalStatus == FinalStatusFailed && e.ErrorMessage == nil {
			return errors.New("error_message is required for failed run_finished events")
		}
	}
	return nil
}

func NormalizeMetadata(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return json.RawMessage(`{}`)
	}
	return payload
}
