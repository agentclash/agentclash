package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

type fakeStandingsStore struct {
	mu      sync.Mutex
	calls   []standingsCall
	nextErr error
}

type standingsCall struct {
	runID   uuid.UUID
	updates StandingsEntry
}

func (f *fakeStandingsStore) Update(_ context.Context, runID uuid.UUID, updates StandingsEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, standingsCall{runID: runID, updates: updates})
	return f.nextErr
}

func (f *fakeStandingsStore) Snapshot(context.Context, uuid.UUID) (map[uuid.UUID]StandingsEntry, error) {
	return nil, nil
}

func (f *fakeStandingsStore) Close() error { return nil }

func TestStandingsRecorderRoutesEventTypes(t *testing.T) {
	runID := uuid.New()
	agentID := uuid.New()
	inner := &fakeRecorder{
		returnEvent: repository.RunEvent{RunID: runID, RunAgentID: agentID, SequenceNumber: 1},
	}
	store := &fakeStandingsStore{}
	recorder := NewStandingsRecorder(inner, store, slog.Default())

	modelPayload, _ := json.Marshal(map[string]any{
		"provider_model_id": "claude-sonnet-4-6",
		"usage":             map[string]int64{"total_tokens": 250},
	})

	cases := []struct {
		name        string
		eventType   runevents.Type
		stepIndex   int
		payload     json.RawMessage
		wantStore   bool
		wantState   StandingsState
		wantStep    int
		wantTokens  int64
		wantTool    int
		wantModel   string
	}{
		{
			name:      "run.started → state running",
			eventType: runevents.EventTypeSystemRunStarted,
			wantStore: true,
			wantState: StandingsStateRunning,
		},
		{
			name:      "step.started → step update",
			eventType: runevents.EventTypeSystemStepStarted,
			stepIndex: 5,
			wantStore: true,
			wantStep:  5,
			wantState: StandingsStateRunning,
		},
		{
			name:      "tool.call.completed → tool tick",
			eventType: runevents.EventTypeToolCallCompleted,
			wantStore: true,
			wantTool:  1,
		},
		{
			name:       "model.call.completed → tokens + model",
			eventType:  runevents.EventTypeModelCallCompleted,
			payload:    modelPayload,
			wantStore:  true,
			wantTokens: 250,
			wantModel:  "claude-sonnet-4-6",
		},
		{
			name:      "output.finalized → submitted",
			eventType: runevents.EventTypeSystemOutputFinalized,
			wantStore: true,
			wantState: StandingsStateSubmitted,
		},
		{
			name:      "run.failed → failed",
			eventType: runevents.EventTypeSystemRunFailed,
			wantStore: true,
			wantState: StandingsStateFailed,
		},
		{
			name:      "unrelated event → no store call",
			eventType: runevents.EventTypeModelOutputDelta,
			wantStore: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store.mu.Lock()
			store.calls = nil
			store.mu.Unlock()

			env := runevents.Envelope{
				RunID:      runID,
				RunAgentID: agentID,
				EventType:  tc.eventType,
				Summary:    runevents.SummaryMetadata{StepIndex: tc.stepIndex},
				Payload:    tc.payload,
			}
			_, err := recorder.RecordRunEvent(context.Background(), repository.RecordRunEventParams{Event: env})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			store.mu.Lock()
			defer store.mu.Unlock()
			if !tc.wantStore {
				if len(store.calls) != 0 {
					t.Fatalf("store should not be called, got %d calls", len(store.calls))
				}
				return
			}
			if len(store.calls) != 1 {
				t.Fatalf("expected 1 store call, got %d", len(store.calls))
			}
			call := store.calls[0]
			if call.runID != runID {
				t.Errorf("store runID = %s, want %s", call.runID, runID)
			}
			if call.updates.RunAgentID != agentID {
				t.Errorf("store runAgentID = %s, want %s", call.updates.RunAgentID, agentID)
			}
			if tc.wantState != "" && call.updates.State != tc.wantState {
				t.Errorf("state = %q, want %q", call.updates.State, tc.wantState)
			}
			if call.updates.Step != tc.wantStep {
				t.Errorf("step = %d, want %d", call.updates.Step, tc.wantStep)
			}
			if call.updates.TokensUsed != tc.wantTokens {
				t.Errorf("tokens = %d, want %d", call.updates.TokensUsed, tc.wantTokens)
			}
			if call.updates.ToolCalls != tc.wantTool {
				t.Errorf("tool_calls = %d, want %d", call.updates.ToolCalls, tc.wantTool)
			}
			if call.updates.Model != tc.wantModel {
				t.Errorf("model = %q, want %q", call.updates.Model, tc.wantModel)
			}
		})
	}
}

func TestStandingsRecorderSwallowsStoreError(t *testing.T) {
	inner := &fakeRecorder{
		returnEvent: repository.RunEvent{SequenceNumber: 7},
	}
	store := &fakeStandingsStore{nextErr: errors.New("redis down")}
	recorder := NewStandingsRecorder(inner, store, slog.Default())

	env := runevents.Envelope{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		EventType:  runevents.EventTypeSystemRunStarted,
	}
	event, err := recorder.RecordRunEvent(context.Background(), repository.RecordRunEventParams{Event: env})
	if err != nil {
		t.Fatalf("store error must be swallowed, got: %v", err)
	}
	if event.SequenceNumber != 7 {
		t.Fatalf("sequence = %d, want 7", event.SequenceNumber)
	}
}

func TestStandingsRecorderRoutesTimeoutToTimedOutState(t *testing.T) {
	runID := uuid.New()
	agentID := uuid.New()
	inner := &fakeRecorder{
		returnEvent: repository.RunEvent{RunID: runID, RunAgentID: agentID, SequenceNumber: 1},
	}
	store := &fakeStandingsStore{}
	recorder := NewStandingsRecorder(inner, store, slog.Default())

	// Timeout failure carries stop_reason=timeout in its payload.
	payload, _ := json.Marshal(map[string]any{
		"error":       "native execution exceeded runtime budget",
		"stop_reason": "timeout",
	})
	env := runevents.Envelope{
		RunID:      runID,
		RunAgentID: agentID,
		EventType:  runevents.EventTypeSystemRunFailed,
		Payload:    payload,
	}
	if _, err := recorder.RecordRunEvent(context.Background(), repository.RecordRunEventParams{Event: env}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.calls) != 1 {
		t.Fatalf("expected 1 store call, got %d", len(store.calls))
	}
	if got := store.calls[0].updates.State; got != StandingsStateTimedOut {
		t.Fatalf("state = %q, want %q", got, StandingsStateTimedOut)
	}
}

func TestStandingsRecorderRoutesNonTimeoutFailureToFailedState(t *testing.T) {
	runID := uuid.New()
	agentID := uuid.New()
	inner := &fakeRecorder{
		returnEvent: repository.RunEvent{RunID: runID, RunAgentID: agentID, SequenceNumber: 1},
	}
	store := &fakeStandingsStore{}
	recorder := NewStandingsRecorder(inner, store, slog.Default())

	payload, _ := json.Marshal(map[string]any{
		"error":       "sandbox error",
		"stop_reason": "sandbox_error",
	})
	env := runevents.Envelope{
		RunID:      runID,
		RunAgentID: agentID,
		EventType:  runevents.EventTypeSystemRunFailed,
		Payload:    payload,
	}
	if _, err := recorder.RecordRunEvent(context.Background(), repository.RecordRunEventParams{Event: env}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if got := store.calls[0].updates.State; got != StandingsStateFailed {
		t.Fatalf("state = %q, want %q", got, StandingsStateFailed)
	}
}

func TestStandingsRecorderSkipsWhenPersistFails(t *testing.T) {
	inner := &fakeRecorder{returnErr: errors.New("db failed")}
	store := &fakeStandingsStore{}
	recorder := NewStandingsRecorder(inner, store, slog.Default())

	env := runevents.Envelope{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		EventType:  runevents.EventTypeSystemRunStarted,
	}
	_, err := recorder.RecordRunEvent(context.Background(), repository.RecordRunEventParams{Event: env})
	if err == nil {
		t.Fatal("expected persist error to propagate")
	}
	if len(store.calls) != 0 {
		t.Fatalf("store should not be called when persist fails, got %d calls", len(store.calls))
	}
}

func TestDecodeStandingsHashFieldRejectsCorruptFields(t *testing.T) {
	validEntry := `{"run_agent_id":"550e8400-e29b-41d4-a716-446655440000","step":1}`
	nilEmbedded := `{"run_agent_id":"00000000-0000-0000-0000-000000000000","step":1}`

	cases := []struct {
		name  string
		field string
		value string
		ok    bool
	}{
		{"valid entry with embedded uuid", "agent:550e8400-e29b-41d4-a716-446655440000", validEntry, true},
		{"nil embedded id, uuid recovered from field", "agent:550e8400-e29b-41d4-a716-446655440000", nilEmbedded, true},
		{"short field does not panic", "ag", nilEmbedded, false},
		{"empty field does not panic", "", nilEmbedded, false},
		{"bare prefix rejected", "agent:", nilEmbedded, false},
		{"wrong prefix rejected", "model:whatever", nilEmbedded, false},
		{"invalid uuid after prefix rejected", "agent:not-a-uuid", nilEmbedded, false},
		{"malformed json rejected", "agent:550e8400-e29b-41d4-a716-446655440000", `{`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := decodeStandingsHashField(tc.field, []byte(tc.value))
			if ok != tc.ok {
				t.Errorf("ok = %v, want %v", ok, tc.ok)
			}
		})
	}
}

