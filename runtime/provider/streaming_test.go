package provider

import (
	"testing"
	"time"
)

func TestStreamAccumulatorBuildsFinalResponse(t *testing.T) {
	startedAt := mustTime(t, "2026-03-15T10:00:00Z")
	accumulator := NewStreamAccumulator("openai", startedAt)

	deltas := []StreamDelta{
		{
			Kind:      StreamDeltaKindText,
			Timestamp: startedAt.Add(25 * time.Millisecond),
			Text:      "hello ",
		},
		{
			Kind:      StreamDeltaKindText,
			Timestamp: startedAt.Add(30 * time.Millisecond),
			Text:      "world",
		},
		{
			Kind:      StreamDeltaKindToolCall,
			Timestamp: startedAt.Add(35 * time.Millisecond),
			ToolCall: ToolCallFragment{
				Index:        0,
				IDFragment:   "call-1",
				NameFragment: "submit",
			},
		},
		{
			Kind:      StreamDeltaKindToolCall,
			Timestamp: startedAt.Add(40 * time.Millisecond),
			ToolCall: ToolCallFragment{
				Index:             0,
				ArgumentsFragment: `{"answer":"done"}`,
			},
		},
		{
			Kind:      StreamDeltaKindTerminal,
			Timestamp: startedAt.Add(55 * time.Millisecond),
			Terminal: StreamTerminal{
				ProviderModelID: "gpt-4.1",
				FinishReason:    "tool_calls",
				Usage: &Usage{
					InputTokens:  5,
					OutputTokens: 7,
					TotalTokens:  12,
				},
			},
		},
	}

	for _, delta := range deltas {
		if err := accumulator.Consume(delta); err != nil {
			t.Fatalf("Consume returned error: %v", err)
		}
	}

	response, err := accumulator.Finalize(startedAt.Add(75 * time.Millisecond))
	if err != nil {
		t.Fatalf("Finalize returned error: %v", err)
	}
	if !response.Streamed {
		t.Fatalf("expected streamed response")
	}
	if response.OutputText != "hello world" {
		t.Fatalf("output text = %q, want hello world", response.OutputText)
	}
	if response.FinishReason != "tool_calls" {
		t.Fatalf("finish reason = %q, want tool_calls", response.FinishReason)
	}
	if response.ProviderModelID != "gpt-4.1" {
		t.Fatalf("provider model id = %q, want gpt-4.1", response.ProviderModelID)
	}
	if response.Usage.TotalTokens != 12 {
		t.Fatalf("total tokens = %d, want 12", response.Usage.TotalTokens)
	}
	if len(response.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(response.ToolCalls))
	}
	if string(response.ToolCalls[0].Arguments) != `{"answer":"done"}` {
		t.Fatalf("tool call arguments = %s, want JSON payload", response.ToolCalls[0].Arguments)
	}
	if response.Timing.TTFT != 25*time.Millisecond {
		t.Fatalf("TTFT = %s, want 25ms", response.Timing.TTFT)
	}
	if response.Timing.TotalLatency != 75*time.Millisecond {
		t.Fatalf("total latency = %s, want 75ms", response.Timing.TotalLatency)
	}
}

func TestStreamAccumulatorRejectsMalformedFinalToolCallArguments(t *testing.T) {
	startedAt := mustTime(t, "2026-03-15T10:00:00Z")
	accumulator := NewStreamAccumulator("openai", startedAt)

	if err := accumulator.Consume(StreamDelta{
		Kind:      StreamDeltaKindToolCall,
		Timestamp: startedAt.Add(20 * time.Millisecond),
		ToolCall: ToolCallFragment{
			Index:             0,
			NameFragment:      "read_file",
			ArgumentsFragment: `{not-json}`,
		},
	}); err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}

	_, err := accumulator.Finalize(startedAt.Add(30 * time.Millisecond))
	if err == nil {
		t.Fatalf("expected error")
	}

	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("expected provider failure, got %T", err)
	}
	if failure.Code != FailureCodeMalformedResponse {
		t.Fatalf("failure code = %s, want %s", failure.Code, FailureCodeMalformedResponse)
	}
}

func TestStreamAccumulatorAllowsMissingUsage(t *testing.T) {
	startedAt := mustTime(t, "2026-03-15T10:00:00Z")
	accumulator := NewStreamAccumulator("openai", startedAt)

	if err := accumulator.Consume(StreamDelta{
		Kind:      StreamDeltaKindText,
		Timestamp: startedAt.Add(15 * time.Millisecond),
		Text:      "partial",
	}); err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}

	response, err := accumulator.Finalize(startedAt.Add(40 * time.Millisecond))
	if err != nil {
		t.Fatalf("Finalize returned error: %v", err)
	}
	if response.Usage.TotalTokens != 0 {
		t.Fatalf("total tokens = %d, want 0 without usage block", response.Usage.TotalTokens)
	}
	if response.Timing.TTFT != 15*time.Millisecond {
		t.Fatalf("TTFT = %s, want 15ms", response.Timing.TTFT)
	}
}

func TestStreamAccumulatorRejectsTooLargeToolCallIndex(t *testing.T) {
	startedAt := mustTime(t, "2026-03-15T10:00:00Z")
	accumulator := NewStreamAccumulator("openai", startedAt)

	err := accumulator.Consume(StreamDelta{
		Kind:      StreamDeltaKindToolCall,
		Timestamp: startedAt.Add(20 * time.Millisecond),
		ToolCall: ToolCallFragment{
			Index:        maxToolCallIndex,
			NameFragment: "read_file",
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestStreamAccumulatorRejectsCompletedAtBeforeStartedAt(t *testing.T) {
	startedAt := mustTime(t, "2026-03-15T10:00:00Z")
	accumulator := NewStreamAccumulator("openai", startedAt)

	_, err := accumulator.Finalize(startedAt.Add(-1 * time.Second))
	if err == nil {
		t.Fatalf("expected error")
	}
}
