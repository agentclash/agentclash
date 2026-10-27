package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/google/uuid"
)

type capturingRecorder struct {
	events []runevents.Envelope
}

func TestNativeObserverSingleCaseKeepsRunLifecycle(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "completed"
		terminal := runevents.EventTypeSystemRunCompleted
		if failed {
			name = "failed"
			terminal = runevents.EventTypeSystemRunFailed
		}
		t.Run(name, func(t *testing.T) {
			recorder := &capturingRecorder{}
			observer := &NativeRunEventObserver{
				recorder: recorder,
				executionContext: repository.RunAgentExecutionContext{
					Run:      domain.Run{ID: uuid.New()},
					RunAgent: domain.RunAgent{ID: uuid.New()},
					ChallengeInputSet: &repository.ChallengeInputSetExecutionContext{
						Cases: []repository.ChallengeCaseExecutionContext{{CaseKey: "only-case"}},
					},
				},
			}
			ctx := context.Background()
			if err := observer.OnStepStart(ctx, 1); err != nil {
				t.Fatal(err)
			}
			var err error
			if failed {
				err = observer.OnRunFailure(ctx, errors.New("test failure"))
			} else {
				err = observer.OnRunComplete(ctx, engine.Result{FinalOutput: "done", StopReason: engine.StopReasonCompleted})
			}
			if err != nil {
				t.Fatal(err)
			}
			want := []runevents.Type{
				runevents.EventTypeSystemRunStarted,
				runevents.EventTypeSystemStepStarted,
				terminal,
			}
			if len(recorder.events) != len(want) {
				t.Fatalf("event count = %d, want %d", len(recorder.events), len(want))
			}
			for i, event := range recorder.events {
				if event.EventType != want[i] {
					t.Fatalf("event %d = %s, want %s", i, event.EventType, want[i])
				}
			}
			if !failed {
				var payload map[string]any
				if err := json.Unmarshal(recorder.events[2].Payload, &payload); err != nil {
					t.Fatal(err)
				}
				if payload["final_output"] != "done" {
					t.Fatalf("final output = %v, want done", payload["final_output"])
				}
			}
		})
	}
}

func (c *capturingRecorder) RecordRunEvent(_ context.Context, params repository.RecordRunEventParams) (repository.RunEvent, error) {
	c.events = append(c.events, params.Event)
	return repository.RunEvent{
		RunID:      params.Event.RunID,
		RunAgentID: params.Event.RunAgentID,
		EventType:  params.Event.EventType,
		Payload:    params.Event.Payload,
	}, nil
}

func TestNativeObserverSuppressesCaseScopedLifecycle(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	recorder := &capturingRecorder{}
	observer := &NativeRunEventObserver{
		recorder: recorder,
		executionContext: repository.RunAgentExecutionContext{
			ExecutionCaseKey: "refund-1",
			Run:              domain.Run{ID: runID},
			RunAgent:         domain.RunAgent{ID: runAgentID, RunID: runID},
			ChallengeInputSet: &repository.ChallengeInputSetExecutionContext{
				Cases: []repository.ChallengeCaseExecutionContext{
					{CaseKey: "refund-1", ItemKey: "refund-1"},
				},
			},
		},
	}

	if err := observer.OnRunComplete(context.Background(), engine.Result{
		FinalOutput: "done",
		StopReason:  engine.StopReasonCompleted,
	}); err != nil {
		t.Fatalf("OnRunComplete: %v", err)
	}
	if err := observer.OnRunFailure(context.Background(), errors.New("boom")); err != nil {
		t.Fatalf("OnRunFailure: %v", err)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("case-scoped lifecycle events = %d, want 0", len(recorder.events))
	}
}

func TestNativeObserverEmbedsCaseKeyOnStepEvents(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	recorder := &capturingRecorder{}
	observer := &NativeRunEventObserver{
		recorder: recorder,
		executionContext: repository.RunAgentExecutionContext{
			ExecutionCaseKey: "refund-1",
			Run:              domain.Run{ID: runID},
			RunAgent:         domain.RunAgent{ID: runAgentID, RunID: runID},
			ChallengeInputSet: &repository.ChallengeInputSetExecutionContext{
				Cases: []repository.ChallengeCaseExecutionContext{
					{CaseKey: "refund-1", ItemKey: "refund-1"},
				},
			},
		},
	}

	if err := observer.OnStepStart(context.Background(), 0); err != nil {
		t.Fatalf("OnStepStart: %v", err)
	}
	if len(recorder.events) == 0 {
		t.Fatal("expected step events with case_key")
	}
	for _, event := range recorder.events {
		if event.EventType == runevents.EventTypeSystemRunStarted ||
			event.EventType == runevents.EventTypeSystemRunCompleted ||
			event.EventType == runevents.EventTypeSystemRunFailed {
			t.Fatalf("unexpected lifecycle event %s for case-scoped observer", event.EventType)
		}
		if event.Summary.CaseKey != "refund-1" {
			t.Fatalf("summary.case_key = %q, want refund-1 (type=%s)", event.Summary.CaseKey, event.EventType)
		}
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("payload: %v", err)
		}
		if payload["case_key"] != "refund-1" {
			t.Fatalf("payload.case_key = %v, want refund-1 (type=%s)", payload["case_key"], event.EventType)
		}
	}
}

func TestNativeObserverOmitsCaseKeyForMultiCaseContext(t *testing.T) {
	recorder := &capturingRecorder{}
	observer := &NativeRunEventObserver{
		recorder: recorder,
		executionContext: repository.RunAgentExecutionContext{
			Run:      domain.Run{ID: uuid.New()},
			RunAgent: domain.RunAgent{ID: uuid.New()},
			ChallengeInputSet: &repository.ChallengeInputSetExecutionContext{
				Cases: []repository.ChallengeCaseExecutionContext{
					{CaseKey: "a"},
					{CaseKey: "b"},
				},
			},
		},
	}
	if err := observer.OnRunComplete(context.Background(), engine.Result{
		FinalOutput: "done",
		StopReason:  engine.StopReasonCompleted,
	}); err != nil {
		t.Fatalf("OnRunComplete: %v", err)
	}
	for _, event := range recorder.events {
		if event.Summary.CaseKey != "" {
			t.Fatalf("expected empty case_key for multi-case mega-activity, got %q", event.Summary.CaseKey)
		}
		var payload map[string]any
		_ = json.Unmarshal(event.Payload, &payload)
		if _, ok := payload["case_key"]; ok {
			t.Fatalf("payload should not include case_key for multi-case context")
		}
	}
}
