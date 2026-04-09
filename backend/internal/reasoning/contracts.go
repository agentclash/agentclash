package reasoning

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
)

const (
	ReasoningRunEventSignal = "reasoning_run_event"
)

var (
	ErrInvalidStartRequest   = errors.New("invalid reasoning start request")
	ErrInvalidToolResult     = errors.New("invalid reasoning tool result")
	ErrInvalidCancelRequest  = errors.New("invalid reasoning cancel request")
	ErrReasoningRunNotFound  = errors.New("reasoning run execution not found")
	ErrInvalidReasoningEvent = errors.New("invalid reasoning event")
)

// StartRequest is sent from Go to Python to begin a reasoning run.
type StartRequest struct {
	RunID            uuid.UUID                `json:"run_id"`
	RunAgentID       uuid.UUID                `json:"run_agent_id"`
	IdempotencyKey   string                   `json:"idempotency_key"`
	ExecutionContext json.RawMessage          `json:"execution_context"`
	Tools            []provider.ToolDefinition `json:"tools"`
	CallbackURL      string                   `json:"callback_url"`
	CallbackToken    string                   `json:"callback_token"`
	DeadlineAt       time.Time                `json:"deadline_at"`
}

func (r StartRequest) Validate() error {
	if r.RunID == uuid.Nil {
		return errors.Join(ErrInvalidStartRequest, errors.New("run_id is required"))
	}
	if r.RunAgentID == uuid.Nil {
		return errors.Join(ErrInvalidStartRequest, errors.New("run_agent_id is required"))
	}
	if r.IdempotencyKey == "" {
		return errors.Join(ErrInvalidStartRequest, errors.New("idempotency_key is required"))
	}
	if len(r.ExecutionContext) == 0 {
		return errors.Join(ErrInvalidStartRequest, errors.New("execution_context is required"))
	}
	if r.CallbackURL == "" {
		return errors.Join(ErrInvalidStartRequest, errors.New("callback_url is required"))
	}
	if r.CallbackToken == "" {
		return errors.Join(ErrInvalidStartRequest, errors.New("callback_token is required"))
	}
	if r.DeadlineAt.IsZero() {
		return errors.Join(ErrInvalidStartRequest, errors.New("deadline_at is required"))
	}
	return nil
}

// StartResponse is returned by Python when a reasoning run is accepted.
type StartResponse struct {
	Accepted       bool   `json:"accepted"`
	ReasoningRunID string `json:"reasoning_run_id"`
	Error          string `json:"error,omitempty"`
}

// ToolResultStatus indicates the outcome of a single tool execution.
type ToolResultStatus string

const (
	ToolResultStatusCompleted ToolResultStatus = "completed"
	ToolResultStatusBlocked   ToolResultStatus = "blocked"
	ToolResultStatusSkipped   ToolResultStatus = "skipped"
	ToolResultStatusFailed    ToolResultStatus = "failed"
)

// ToolResult is the outcome of executing a single tool call.
type ToolResult struct {
	ToolCallID   string           `json:"tool_call_id"`
	Status       ToolResultStatus `json:"status"`
	Content      string           `json:"content,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
}

func (r ToolResult) Validate() error {
	if r.ToolCallID == "" {
		return errors.Join(ErrInvalidToolResult, errors.New("tool_call_id is required"))
	}
	switch r.Status {
	case ToolResultStatusCompleted, ToolResultStatusBlocked, ToolResultStatusSkipped, ToolResultStatusFailed:
	default:
		return errors.Join(ErrInvalidToolResult, errors.New("status must be one of: completed, blocked, skipped, failed"))
	}
	return nil
}

// ToolResultsBatch is sent from Go to Python with tool execution outcomes.
type ToolResultsBatch struct {
	IdempotencyKey string       `json:"idempotency_key"`
	ToolResults    []ToolResult `json:"tool_results"`
}

func (b ToolResultsBatch) Validate() error {
	if b.IdempotencyKey == "" {
		return errors.Join(ErrInvalidToolResult, errors.New("idempotency_key is required"))
	}
	for i, result := range b.ToolResults {
		if err := result.Validate(); err != nil {
			return errors.Join(err, errors.New("tool_results["+itoa(i)+"]"))
		}
	}
	return nil
}

// CancelRequest is sent from Go to Python to stop a reasoning run.
type CancelRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	Reason         string `json:"reason"`
}

func (r CancelRequest) Validate() error {
	if r.IdempotencyKey == "" {
		return errors.Join(ErrInvalidCancelRequest, errors.New("idempotency_key is required"))
	}
	return nil
}

// CancelResponse is returned by Python when a cancellation is acknowledged.
type CancelResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

// ReasoningEventSignal is the minimal payload sent via Temporal signal
// when an actionable event arrives at the callback handler. The full
// event data lives in reasoning_run_executions, not in the signal.
type ReasoningEventSignal struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
}

func itoa(n int) string {
	const digits = "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	buf := make([]byte, 0, 4)
	for n > 0 {
		buf = append(buf, digits[n%10])
		n /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
