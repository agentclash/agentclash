package worker

import (
	"context"
	"errors"
	"testing"

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

	if len(recorder.events) < 11 {
		t.Fatalf("event count = %d, want at least 11", len(recorder.events))
	}
	assertEventTypeSequence(t, recorder.events, []runevents.Type{
		runevents.EventTypeSystemRunStarted,
		runevents.EventTypeSystemStepStarted,
		runevents.EventTypeModelCallStarted,
		runevents.EventTypeModelCallCompleted,
		runevents.EventTypeToolCallCompleted,
		runevents.EventTypeSystemStepCompleted,
		runevents.EventTypeSystemStepStarted,
		runevents.EventTypeModelCallStarted,
		runevents.EventTypeModelCallCompleted,
		runevents.EventTypeToolCallCompleted,
		runevents.EventTypeSystemStepCompleted,
		runevents.EventTypeSystemRunCompleted,
	})
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
