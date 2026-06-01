package engine

import (
	"testing"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
)

func TestExecuteToolCallsPreservesSubmitToolMessageWhenRequested(t *testing.T) {
	executor := NewNativeExecutor(&provider.FakeClient{}, nil, NoopObserver{})
	registry := &Registry{visible: nativePrimitiveTools(sandbox.ToolPolicy{})}

	toolCalls := []provider.ToolCall{{
		ID:        "call-submit",
		Name:      submitToolName,
		Arguments: []byte(`{"answer":"done"}`),
	}}

	withoutPreserve, finalOutput, completed, _, err := executor.executeToolCalls(
		t.Context(),
		nil,
		registry,
		sandbox.ToolPolicy{},
		nil,
		0,
		toolCalls,
		false,
	)
	if err != nil {
		t.Fatalf("executeToolCalls returned error: %v", err)
	}
	if !completed || finalOutput != "done" {
		t.Fatalf("completed=%v finalOutput=%q, want completed with done", completed, finalOutput)
	}
	if len(withoutPreserve) != 0 {
		t.Fatalf("tool message count = %d, want 0 when submit message is stripped", len(withoutPreserve))
	}

	withPreserve, finalOutput, completed, _, err := executor.executeToolCalls(
		t.Context(),
		nil,
		registry,
		sandbox.ToolPolicy{},
		nil,
		0,
		toolCalls,
		true,
	)
	if err != nil {
		t.Fatalf("executeToolCalls returned error: %v", err)
	}
	if !completed || finalOutput != "done" {
		t.Fatalf("completed=%v finalOutput=%q, want completed with done", completed, finalOutput)
	}
	if len(withPreserve) != 1 {
		t.Fatalf("tool message count = %d, want 1 when submit message is preserved", len(withPreserve))
	}
	if withPreserve[0].ToolCallID != "call-submit" {
		t.Fatalf("tool call id = %q, want call-submit", withPreserve[0].ToolCallID)
	}
}
