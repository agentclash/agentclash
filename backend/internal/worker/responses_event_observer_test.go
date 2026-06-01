package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/runevents"
)

func TestResponsesInvokerPersistsCanonicalEvents(t *testing.T) {
	recorder := &fakeRunEventRecorder{}
	client := &provider.FakeResearchClient{
		FakeClient: provider.FakeClient{
			Response: provider.Response{
				ProviderKey:     "openai",
				ProviderModelID: "o4-mini-deep-research",
				FinishReason:    "completed",
				OutputText:      `{"ok":true}`,
				Usage:           provider.Usage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14},
			},
		},
	}

	invoker := NewResponsesInvokerWithObserverFactory(
		client,
		NewResponsesRunEventObserverFactory(recorder),
	)

	if _, err := invoker.InvokeResponses(context.Background(), responsesInvokerExecutionContext()); err != nil {
		t.Fatalf("InvokeResponses returned error: %v", err)
	}

	want := []runevents.Type{
		runevents.EventTypeSystemRunStarted,
		runevents.EventTypeModelCallStarted,
		runevents.EventTypeModelCallCompleted,
		runevents.EventTypeSystemOutputFinalized,
		runevents.EventTypeSystemRunCompleted,
	}
	if len(recorder.events) != len(want) {
		t.Fatalf("event count = %d, want %d", len(recorder.events), len(want))
	}
	for i, event := range recorder.events {
		if event.EventType != want[i] {
			t.Fatalf("event[%d] type = %q, want %q", i, event.EventType, want[i])
		}
		if event.Source != runevents.SourceResponsesEngine {
			t.Fatalf("event[%d] source = %q, want responses_engine", i, event.Source)
		}
		if event.SequenceNumber != int64(i+1) {
			t.Fatalf("event[%d] sequence number = %d, want %d", i, event.SequenceNumber, i+1)
		}
		if !json.Valid(event.Payload) {
			t.Fatalf("event[%d] payload is not valid JSON: %s", i, event.Payload)
		}
	}

	var completedPayload map[string]any
	if err := json.Unmarshal(recorder.events[4].Payload, &completedPayload); err != nil {
		t.Fatalf("unmarshal run.completed payload: %v", err)
	}
	if completedPayload["final_output"] != `{"ok":true}` {
		t.Fatalf("run.completed final_output = %v", completedPayload["final_output"])
	}
	if totalTokens, _ := completedPayload["total_tokens"].(float64); totalTokens != 14 {
		t.Fatalf("run.completed total_tokens = %v, want 14", completedPayload["total_tokens"])
	}
}

func TestResponsesInvokerRecordsRunFailedOnProviderError(t *testing.T) {
	recorder := &fakeRunEventRecorder{}
	client := &provider.FakeResearchClient{
		FakeClient: provider.FakeClient{
			Err: provider.NewFailure("openai", provider.FailureCodeAuth, "bad key", false, nil),
		},
	}

	invoker := NewResponsesInvokerWithObserverFactory(
		client,
		NewResponsesRunEventObserverFactory(recorder),
	)

	if _, err := invoker.InvokeResponses(context.Background(), responsesInvokerExecutionContext()); err == nil {
		t.Fatal("expected error")
	}

	var sawFailed bool
	types := make([]runevents.Type, 0, len(recorder.events))
	for _, event := range recorder.events {
		types = append(types, event.EventType)
		if event.EventType == runevents.EventTypeSystemRunFailed {
			sawFailed = true
		}
	}
	if !sawFailed {
		t.Fatalf("expected system.run.failed event; got sequence %v", types)
	}
}
