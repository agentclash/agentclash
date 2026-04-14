package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/engine"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestNativeModelInvokerPersistsCanonicalEventsForMultiStepRun(t *testing.T) {
	session := sandbox.NewFakeSession("sandbox-observer")
	recorder := &fakeRunEventRecorder{}
	client := &scriptedProviderClient{
		steps: []providerStep{
			{
				deltas: []provider.StreamDelta{
					{
						Kind:      provider.StreamDeltaKindText,
						Timestamp: time.Date(2026, 3, 16, 9, 0, 0, 100_000_000, time.UTC),
						Text:      "working",
					},
				},
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-write",
							Name:      "write_file",
							Arguments: []byte(`{"path":"/workspace/result.txt","content":"done"}`),
						},
					},
				},
			},
			{
				deltas: []provider.StreamDelta{
					{
						Kind:      provider.StreamDeltaKindToolCall,
						Timestamp: time.Date(2026, 3, 16, 9, 0, 1, 200_000_000, time.UTC),
						ToolCall: provider.ToolCallFragment{
							Index:             0,
							NameFragment:      "submit",
							ArgumentsFragment: `{"answer":"final answer"}`,
						},
					},
				},
				response: provider.Response{
					ProviderKey:     "openai",
					ProviderModelID: "gpt-4.1",
					FinishReason:    "tool_calls",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-submit",
							Name:      "submit",
							Arguments: []byte(`{"answer":"final answer"}`),
						},
					},
				},
			},
		},
	}

	invoker := NewNativeModelInvokerWithObserverFactory(
		client,
		&sandbox.FakeProvider{NextSession: session},
		NewNativeRunEventObserverFactory(recorder),
	)

	result, err := invoker.InvokeNativeModel(context.Background(), nativeModelExecutionContext())
	if err != nil {
		t.Fatalf("InvokeNativeModel returned error: %v", err)
	}
	if result.FinalOutput != "final answer" {
		t.Fatalf("final output = %q, want final answer", result.FinalOutput)
	}

	if len(recorder.events) != 14 {
		t.Fatalf("event count = %d, want 14", len(recorder.events))
	}
	assertEventTypeSequence(t, recorder.events, []runevents.Type{
		runevents.EventTypeSystemRunStarted,
		runevents.EventTypeSystemStepStarted,
		runevents.EventTypeModelCallStarted,
		runevents.EventTypeModelOutputDelta,
		runevents.EventTypeModelCallCompleted,
		runevents.EventTypeToolCallCompleted,
		runevents.EventTypeSystemStepCompleted,
		runevents.EventTypeSystemStepStarted,
		runevents.EventTypeModelCallStarted,
		runevents.EventTypeModelOutputDelta,
		runevents.EventTypeModelCallCompleted,
		runevents.EventTypeToolCallCompleted,
		runevents.EventTypeSystemStepCompleted,
		runevents.EventTypeSystemRunCompleted,
	})
	if got := recorder.events[3].OccurredAt; !got.Equal(time.Date(2026, 3, 16, 9, 0, 0, 100_000_000, time.UTC)) {
		t.Fatalf("first model output timestamp = %s, want streamed delta timestamp", got)
	}
	var toolPayload map[string]any
	if err := json.Unmarshal(recorder.events[5].Payload, &toolPayload); err != nil {
		t.Fatalf("decode tool payload: %v", err)
	}
	if toolPayload["tool_category"] != string(engine.ToolCategoryPrimitive) {
		t.Fatalf("tool category = %#v, want primitive", toolPayload["tool_category"])
	}
}

func TestNativeModelInvokerPersistsTerminalFailureEvent(t *testing.T) {
	recorder := &fakeRunEventRecorder{}
	invoker := NewNativeModelInvokerWithObserverFactory(
		&provider.FakeClient{
			Err: provider.NewFailure("openai", provider.FailureCodeUnavailable, "upstream unavailable", true, errors.New("boom")),
		},
		sandbox.UnconfiguredProvider{},
		NewNativeRunEventObserverFactory(recorder),
	)

	_, err := invoker.InvokeNativeModel(context.Background(), nativeModelExecutionContext())
	if err == nil {
		t.Fatalf("expected invocation error")
	}

	if len(recorder.events) < 2 {
		t.Fatalf("event count = %d, want at least 2", len(recorder.events))
	}
	last := recorder.events[len(recorder.events)-1]
	if last.EventType != runevents.EventTypeSystemRunFailed {
		t.Fatalf("last event type = %q, want %q", last.EventType, runevents.EventTypeSystemRunFailed)
	}
}

func TestNativeRunEventObserverRecordsFailureOriginForToolFailures(t *testing.T) {
	recorder := &fakeRunEventRecorder{}
	observer := &NativeRunEventObserver{
		recorder:         recorder,
		executionContext: nativeModelExecutionContext(),
	}

	err := observer.OnToolExecution(context.Background(), engine.ToolExecutionRecord{
		ToolCall: provider.ToolCall{
			ID:        "call-1",
			Name:      "check_inventory",
			Arguments: []byte(`{"sku":"WIDGET-42"}`),
		},
		Result: provider.ToolResult{
			ToolCallID: "call-1",
			Content:    `{"error":"check_inventory failed: answer is required"}`,
			IsError:    true,
		},
		ToolCategory:         engine.ToolCategoryComposed,
		ResolvedToolName:     "submit",
		ResolvedToolCategory: engine.ToolCategoryPrimitive,
		FailureOrigin:        engine.ToolFailureOriginPrimitive,
	})
	if err != nil {
		t.Fatalf("OnToolExecution returned error: %v", err)
	}
	if len(recorder.events) != 2 {
		t.Fatalf("event count = %d, want 2 including run start", len(recorder.events))
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.events[1].Payload, &payload); err != nil {
		t.Fatalf("decode tool payload: %v", err)
	}
	if payload["failure_origin"] != string(engine.ToolFailureOriginPrimitive) {
		t.Fatalf("failure_origin = %#v, want primitive", payload["failure_origin"])
	}
	if payload["resolved_tool_name"] != "submit" {
		t.Fatalf("resolved_tool_name = %#v, want submit", payload["resolved_tool_name"])
	}
}

func TestNativeRunEventObserverOmitsFailureDepthForSuccessfulChains(t *testing.T) {
	recorder := &fakeRunEventRecorder{}
	observer := &NativeRunEventObserver{
		recorder:         recorder,
		executionContext: nativeModelExecutionContext(),
	}

	err := observer.OnToolExecution(context.Background(), engine.ToolExecutionRecord{
		ToolCall: provider.ToolCall{
			ID:        "call-2",
			Name:      "outer",
			Arguments: []byte(`{"val":"hello"}`),
		},
		Result: provider.ToolResult{
			ToolCallID: "call-2",
			Content:    `{"val":"hello"}`,
			IsError:    false,
		},
		ToolCategory:         engine.ToolCategoryComposed,
		ResolvedToolName:     "passthrough",
		ResolvedToolCategory: engine.ToolCategoryPrimitive,
		ResolutionChain:      []string{"outer", "inner", "passthrough"},
		FailureDepth:         0,
	})
	if err != nil {
		t.Fatalf("OnToolExecution returned error: %v", err)
	}
	if len(recorder.events) != 2 {
		t.Fatalf("event count = %d, want 2 including run start", len(recorder.events))
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.events[1].Payload, &payload); err != nil {
		t.Fatalf("decode tool payload: %v", err)
	}
	if _, ok := payload["failure_depth"]; ok {
		t.Fatalf("failure_depth should be omitted for successful chained executions: %#v", payload)
	}
	if got, ok := payload["resolution_chain"].([]any); !ok || len(got) != 3 {
		t.Fatalf("resolution_chain = %#v, want 3-item chain", payload["resolution_chain"])
	}
}

func TestNativeRunEventObserverRecordsCodeExecutionVerification(t *testing.T) {
	recorder := &fakeRunEventRecorder{}
	observer := &NativeRunEventObserver{
		recorder:         recorder,
		executionContext: nativeModelExecutionContext(),
	}

	err := observer.OnPostExecutionVerification(context.Background(), []engine.PostExecutionVerificationResult{
		{
			Key:  "tests_pass",
			Type: "code_execution",
			Payload: []byte(`{
				"validator_key":"tests_pass",
				"target":"file:generated_code",
				"test_command":"python -m pytest tests/ -q",
				"timeout_ms":30000,
				"exit_code":0,
				"passed_tests":1,
				"total_tests":1
			}`),
		},
	})
	if err != nil {
		t.Fatalf("OnPostExecutionVerification returned error: %v", err)
	}

	if len(recorder.events) != 2 {
		t.Fatalf("event count = %d, want 2 including run start", len(recorder.events))
	}
	if recorder.events[1].EventType != runevents.EventTypeGraderVerificationCodeExecuted {
		t.Fatalf("event type = %q, want %q", recorder.events[1].EventType, runevents.EventTypeGraderVerificationCodeExecuted)
	}
}

type fakeRunEventRecorder struct {
	events []repository.RunEvent
}

func (f *fakeRunEventRecorder) RecordRunEvent(_ context.Context, params repository.RecordRunEventParams) (repository.RunEvent, error) {
	event := repository.RunEvent{
		ID:             int64(len(f.events) + 1),
		RunID:          params.Event.RunID,
		RunAgentID:     params.Event.RunAgentID,
		SequenceNumber: int64(len(f.events) + 1),
		EventType:      params.Event.EventType,
		Source:         params.Event.Source,
		OccurredAt:     params.Event.OccurredAt,
		Payload:        append([]byte(nil), params.Event.Payload...),
	}
	f.events = append(f.events, event)
	return event, nil
}

type providerStep struct {
	deltas   []provider.StreamDelta
	response provider.Response
	err      error
}

type scriptedProviderClient struct {
	steps []providerStep
	index int
}

func (c *scriptedProviderClient) InvokeModel(_ context.Context, _ provider.Request) (provider.Response, error) {
	if c.index >= len(c.steps) {
		return provider.Response{}, errors.New("unexpected provider invocation")
	}
	step := c.steps[c.index]
	c.index++
	if step.err != nil {
		return provider.Response{}, step.err
	}
	return step.response, nil
}

func (c *scriptedProviderClient) StreamModel(_ context.Context, _ provider.Request, onDelta func(provider.StreamDelta) error) (provider.Response, error) {
	if c.index >= len(c.steps) {
		return provider.Response{}, errors.New("unexpected provider invocation")
	}
	step := c.steps[c.index]
	c.index++
	if step.err != nil {
		return provider.Response{}, step.err
	}
	for _, delta := range step.deltas {
		if onDelta != nil {
			if err := onDelta(delta); err != nil {
				return provider.Response{}, err
			}
		}
	}
	return step.response, nil
}

func assertEventTypeSequence(t *testing.T, events []repository.RunEvent, want []runevents.Type) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d", len(events), len(want))
	}
	for i, event := range events {
		if event.EventType != want[i] {
			t.Fatalf("event[%d] type = %q, want %q", i, event.EventType, want[i])
		}
		if event.Source != runevents.SourceNativeEngine {
			t.Fatalf("event[%d] source = %q, want %q", i, event.Source, runevents.SourceNativeEngine)
		}
		if event.SequenceNumber != int64(i+1) {
			t.Fatalf("event[%d] sequence number = %d, want %d", i, event.SequenceNumber, i+1)
		}
	}
}
