package reasoning

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
)

func TestStartRequestValidateRequiresRunID(t *testing.T) {
	r := validStartRequest()
	r.RunID = uuid.Nil
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for nil run_id")
	} else if !errors.Is(err, ErrInvalidStartRequest) {
		t.Fatalf("expected ErrInvalidStartRequest, got %v", err)
	}
}

func TestStartRequestValidateRequiresRunAgentID(t *testing.T) {
	r := validStartRequest()
	r.RunAgentID = uuid.Nil
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for nil run_agent_id")
	}
}

func TestStartRequestValidateRequiresIdempotencyKey(t *testing.T) {
	r := validStartRequest()
	r.IdempotencyKey = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty idempotency_key")
	}
}

func TestStartRequestValidateRequiresExecutionContext(t *testing.T) {
	r := validStartRequest()
	r.ExecutionContext = nil
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for nil execution_context")
	}
}

func TestStartRequestValidateRequiresCallbackURL(t *testing.T) {
	r := validStartRequest()
	r.CallbackURL = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty callback_url")
	}
}

func TestStartRequestValidateRequiresCallbackToken(t *testing.T) {
	r := validStartRequest()
	r.CallbackToken = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty callback_token")
	}
}

func TestStartRequestValidateRequiresDeadlineAt(t *testing.T) {
	r := validStartRequest()
	r.DeadlineAt = time.Time{}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for zero deadline_at")
	}
}

func TestStartRequestValidateAcceptsValid(t *testing.T) {
	r := validStartRequest()
	if err := r.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolResultValidateRequiresToolCallID(t *testing.T) {
	r := ToolResult{Status: ToolResultStatusCompleted}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty tool_call_id")
	}
}

func TestToolResultValidateRejectsInvalidStatus(t *testing.T) {
	r := ToolResult{ToolCallID: "tc-1", Status: "invalid"}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestToolResultValidateAcceptsAllStatuses(t *testing.T) {
	for _, status := range []ToolResultStatus{
		ToolResultStatusCompleted,
		ToolResultStatusBlocked,
		ToolResultStatusSkipped,
		ToolResultStatusFailed,
	} {
		r := ToolResult{ToolCallID: "tc-1", Status: status}
		if err := r.Validate(); err != nil {
			t.Errorf("unexpected error for status %q: %v", status, err)
		}
	}
}

func TestToolResultsBatchValidateRequiresIdempotencyKey(t *testing.T) {
	b := ToolResultsBatch{
		ToolResults: []ToolResult{{ToolCallID: "tc-1", Status: ToolResultStatusCompleted}},
	}
	if err := b.Validate(); err == nil {
		t.Fatal("expected error for empty idempotency_key")
	}
}

func TestCancelRequestValidateRequiresIdempotencyKey(t *testing.T) {
	r := CancelRequest{Reason: "timeout"}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty idempotency_key")
	}
}

func TestCancelRequestValidateAcceptsValid(t *testing.T) {
	r := CancelRequest{IdempotencyKey: "key-1", Reason: "timeout"}
	if err := r.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartRequestJSONRoundTrip(t *testing.T) {
	original := validStartRequest()
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded StartRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.RunID != original.RunID {
		t.Errorf("run_id mismatch: %v != %v", decoded.RunID, original.RunID)
	}
	if decoded.RunAgentID != original.RunAgentID {
		t.Errorf("run_agent_id mismatch: %v != %v", decoded.RunAgentID, original.RunAgentID)
	}
	if len(decoded.Tools) != len(original.Tools) {
		t.Errorf("tools length mismatch: %d != %d", len(decoded.Tools), len(original.Tools))
	}
	if decoded.Tools[0].Name != original.Tools[0].Name {
		t.Errorf("tool name mismatch: %q != %q", decoded.Tools[0].Name, original.Tools[0].Name)
	}
}

func validStartRequest() StartRequest {
	return StartRequest{
		RunID:          uuid.New(),
		RunAgentID:     uuid.New(),
		IdempotencyKey: "idem-key-1",
		ExecutionContext: json.RawMessage(`{"deployment":{}}`),
		Tools: []provider.ToolDefinition{
			{
				Name:        "read_file",
				Description: "Read a file from the sandbox workspace.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
			},
		},
		CallbackURL:   "http://localhost:8080/v1/integrations/reasoning-runs/test/events",
		CallbackToken: "test-token",
		DeadlineAt:    time.Now().Add(5 * time.Minute),
	}
}
