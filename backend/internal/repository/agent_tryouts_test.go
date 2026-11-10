package repository

import "testing"

func TestAgentTryoutStatusTransitions(t *testing.T) {
	statuses := []AgentTryoutStatus{
		AgentTryoutStatusQueued,
		AgentTryoutStatusRunning,
		AgentTryoutStatusCompleted,
		AgentTryoutStatusFailed,
		AgentTryoutStatusCancelled,
	}
	allowed := map[AgentTryoutStatus][]AgentTryoutStatus{
		AgentTryoutStatusQueued:  {AgentTryoutStatusRunning, AgentTryoutStatusFailed, AgentTryoutStatusCancelled},
		AgentTryoutStatusRunning: {AgentTryoutStatusCompleted, AgentTryoutStatusFailed, AgentTryoutStatusCancelled},
	}

	for _, from := range statuses {
		for _, to := range statuses {
			want := false
			for _, next := range allowed[from] {
				if next == to {
					want = true
				}
			}
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("CanTransitionTo(%s, %s) = %v, want %v", from, to, got, want)
			}
			// A write to the same status is always allowed (retries, summary
			// refreshes); any other write follows the table.
			wantWrite := want || from == to
			if got := from.AllowsWrite(to); got != wantWrite {
				t.Errorf("AllowsWrite(%s, %s) = %v, want %v", from, to, got, wantWrite)
			}
		}
	}
}

func TestAgentTryoutStatusTerminal(t *testing.T) {
	tests := map[AgentTryoutStatus]bool{
		AgentTryoutStatusQueued:    false,
		AgentTryoutStatusRunning:   false,
		AgentTryoutStatusCompleted: true,
		AgentTryoutStatusFailed:    true,
		AgentTryoutStatusCancelled: true,
		AgentTryoutStatus("bogus"): false,
	}
	for status, want := range tests {
		if got := status.Terminal(); got != want {
			t.Errorf("Terminal(%q) = %v, want %v", status, got, want)
		}
	}
	if AgentTryoutStatus("bogus").AllowsWrite("bogus") {
		t.Error("AllowsWrite accepted an unknown status")
	}
}

func TestAgentTryoutTurnStatusTransitions(t *testing.T) {
	tests := []struct {
		from AgentTryoutTurnStatus
		to   AgentTryoutTurnStatus
		want bool
	}{
		{AgentTryoutTurnStatusPending, AgentTryoutTurnStatusProcessing, true},
		{AgentTryoutTurnStatusProcessing, AgentTryoutTurnStatusDone, true},
		{AgentTryoutTurnStatusPending, AgentTryoutTurnStatusDone, false},
		{AgentTryoutTurnStatusProcessing, AgentTryoutTurnStatusPending, false},
		{AgentTryoutTurnStatusDone, AgentTryoutTurnStatusProcessing, false},
		{AgentTryoutTurnStatusDone, AgentTryoutTurnStatusPending, false},
		{AgentTryoutTurnStatus("bogus"), AgentTryoutTurnStatusDone, false},
	}
	for _, tc := range tests {
		if got := tc.from.CanTransitionTo(tc.to); got != tc.want {
			t.Errorf("CanTransitionTo(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
	if AgentTryoutTurnStatus("bogus").Valid() {
		t.Error("unknown turn status reported valid")
	}
}
