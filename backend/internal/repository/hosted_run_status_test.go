package repository

import "testing"

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
