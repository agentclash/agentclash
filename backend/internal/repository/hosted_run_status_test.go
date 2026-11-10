package repository

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/runtime/hostedruns"
)

// allHostedRunExecutionStatuses matches the CHECK constraint from migration
// 00090.
var allHostedRunExecutionStatuses = []HostedRunExecutionStatus{
	HostedRunExecutionStatusStarting,
	HostedRunExecutionStatusAccepted,
	HostedRunExecutionStatusRunning,
	HostedRunExecutionStatusCompleted,
	HostedRunExecutionStatusFailed,
	HostedRunExecutionStatusTimedOut,
	HostedRunExecutionStatusUnknown,
}

func TestHostedRunExecutionTransitionsNameEveryLegalMove(t *testing.T) {
	type move struct{ from, to HostedRunExecutionStatus }
	legal := map[move]string{
		{HostedRunExecutionStatusStarting, HostedRunExecutionStatusUnknown}:   "MarkHostedRunUnknown, MarkHostedRunExecutionTimedOut",
		{HostedRunExecutionStatusStarting, HostedRunExecutionStatusAccepted}:  "MarkHostedRunExecutionAccepted",
		{HostedRunExecutionStatusUnknown, HostedRunExecutionStatusAccepted}:   "MarkHostedRunExecutionAccepted",
		{HostedRunExecutionStatusStarting, HostedRunExecutionStatusFailed}:    "MarkHostedRunExecutionFailed, CommitHostedCallback",
		{HostedRunExecutionStatusUnknown, HostedRunExecutionStatusFailed}:     "MarkHostedRunExecutionFailed, CommitHostedCallback",
		{HostedRunExecutionStatusAccepted, HostedRunExecutionStatusUnknown}:   "MarkHostedRunExecutionTimedOut",
		{HostedRunExecutionStatusRunning, HostedRunExecutionStatusUnknown}:    "MarkHostedRunExecutionTimedOut",
		{HostedRunExecutionStatusStarting, HostedRunExecutionStatusRunning}:   "CommitHostedCallback",
		{HostedRunExecutionStatusAccepted, HostedRunExecutionStatusRunning}:   "CommitHostedCallback",
		{HostedRunExecutionStatusUnknown, HostedRunExecutionStatusRunning}:    "CommitHostedCallback",
		{HostedRunExecutionStatusStarting, HostedRunExecutionStatusCompleted}: "CommitHostedCallback",
		{HostedRunExecutionStatusAccepted, HostedRunExecutionStatusCompleted}: "CommitHostedCallback",
		{HostedRunExecutionStatusRunning, HostedRunExecutionStatusCompleted}:  "CommitHostedCallback",
		{HostedRunExecutionStatusUnknown, HostedRunExecutionStatusCompleted}:  "CommitHostedCallback",
		{HostedRunExecutionStatusAccepted, HostedRunExecutionStatusFailed}:    "CommitHostedCallback",
		{HostedRunExecutionStatusRunning, HostedRunExecutionStatusFailed}:     "CommitHostedCallback",
	}

	for _, from := range allHostedRunExecutionStatuses {
		if !from.Valid() {
			t.Fatalf("status %q from the CHECK constraint is not in the transition table", from)
		}
		for _, to := range allHostedRunExecutionStatuses {
			_, want := legal[move{from, to}]
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("CanTransitionTo(%s -> %s) = %v, want %v", from, to, got, want)
			}
		}
	}
	if len(hostedRunExecutionTransitions) != len(allHostedRunExecutionStatuses) {
		t.Fatalf("transition table has %d statuses, CHECK constraint has %d", len(hostedRunExecutionTransitions), len(allHostedRunExecutionStatuses))
	}
}

func TestHostedRunExecutionTerminalStatuses(t *testing.T) {
	terminal := map[HostedRunExecutionStatus]bool{
		HostedRunExecutionStatusCompleted: true,
		HostedRunExecutionStatusFailed:    true,
		HostedRunExecutionStatusTimedOut:  true,
	}
	for _, status := range allHostedRunExecutionStatuses {
		if got := status.Terminal(); got != terminal[status] {
			t.Errorf("%s.Terminal() = %v, want %v", status, got, terminal[status])
		}
	}
	if _, err := ParseHostedRunExecutionStatus("cancelled"); err == nil {
		t.Fatal("ParseHostedRunExecutionStatus accepted a status outside the CHECK constraint")
	}
}

func TestPlanHostedCallback(t *testing.T) {
	completed := hostedruns.FinalStatusCompleted
	failed := hostedruns.FinalStatusFailed
	message := "remote failed"
	output := json.RawMessage(`{"answer":"late"}`)
	started := hostedruns.Event{EventType: hostedruns.EventTypeRunStarted, Output: output}
	finishedOK := hostedruns.Event{EventType: hostedruns.EventTypeRunFinished, FinalStatus: &completed, Output: output}
	finishedFailed := hostedruns.Event{EventType: hostedruns.EventTypeRunFinished, FinalStatus: &failed, ErrorMessage: &message}
	remoteError := hostedruns.Event{EventType: hostedruns.EventTypeError, ErrorMessage: &message}

	tests := []struct {
		name      string
		current   HostedRunExecutionStatus
		waitEnded bool
		duplicate bool
		event     hostedruns.Event
		want      hostedCallbackPlan
		wantErr   error
	}{
		{
			name:    "start before acceptance",
			current: HostedRunExecutionStatusStarting,
			event:   started,
			want:    hostedCallbackPlan{Status: HostedRunExecutionStatusRunning, Lifecycle: true, Output: true},
		},
		{
			name:    "running repeats running",
			current: HostedRunExecutionStatusRunning,
			event:   started,
			want:    hostedCallbackPlan{Status: HostedRunExecutionStatusRunning, Lifecycle: true, Output: true},
		},
		{
			name:    "lost acceptance is reconciled by a completion while the workflow waits",
			current: HostedRunExecutionStatusUnknown,
			event:   finishedOK,
			want:    hostedCallbackPlan{Status: HostedRunExecutionStatusCompleted, Terminal: true, Lifecycle: true, Output: true, Notify: true},
		},
		{
			name:    "remote error fails a running execution",
			current: HostedRunExecutionStatusRunning,
			event:   remoteError,
			want:    hostedCallbackPlan{Status: HostedRunExecutionStatusFailed, Terminal: true, Lifecycle: true, Notify: true},
		},
		{
			name:      "completion after the deadline keeps unknown",
			current:   HostedRunExecutionStatusUnknown,
			waitEnded: true,
			event:     finishedOK,
			want:      hostedCallbackPlan{Status: HostedRunExecutionStatusUnknown, Terminal: true},
		},
		{
			name:      "start after the run agent failed keeps accepted",
			current:   HostedRunExecutionStatusAccepted,
			waitEnded: true,
			event:     started,
			want:      hostedCallbackPlan{Status: HostedRunExecutionStatusAccepted},
		},
		{
			name:    "legacy timed_out row keeps its status",
			current: HostedRunExecutionStatusTimedOut,
			event:   finishedOK,
			want:    hostedCallbackPlan{Status: HostedRunExecutionStatusTimedOut, Terminal: true},
		},
		{
			name:      "redelivered completion keeps completed",
			current:   HostedRunExecutionStatusCompleted,
			duplicate: true,
			event:     finishedOK,
			want:      hostedCallbackPlan{Status: HostedRunExecutionStatusCompleted, Terminal: true, Lifecycle: true, Output: true, Notify: true},
		},
		{
			name:      "redelivered start keeps completed output",
			current:   HostedRunExecutionStatusCompleted,
			duplicate: true,
			event:     started,
			want:      hostedCallbackPlan{Status: HostedRunExecutionStatusCompleted, Lifecycle: true},
		},
		{
			name:      "completed rejects new evidence",
			current:   HostedRunExecutionStatusCompleted,
			waitEnded: true,
			event:     finishedOK,
			wantErr:   ErrHostedEventConflict,
		},
		{
			name:      "failed rejects a different outcome",
			current:   HostedRunExecutionStatusFailed,
			duplicate: true,
			event:     finishedOK,
			wantErr:   ErrHostedEventConflict,
		},
		{
			name:      "failed keeps a redelivered failure",
			current:   HostedRunExecutionStatusFailed,
			duplicate: true,
			event:     finishedFailed,
			want:      hostedCallbackPlan{Status: HostedRunExecutionStatusFailed, Terminal: true, Lifecycle: true, Notify: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := planHostedCallback(tt.current, tt.waitEnded, tt.duplicate, tt.event)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("plan = %+v, want %+v", got, tt.want)
			}
		})
	}
}
