package domain

import "testing"

func TestEvalSessionStatusTransitions(t *testing.T) {
	tests := []struct {
		name string
		from EvalSessionStatus
		to   EvalSessionStatus
		want bool
	}{
		{name: "queued to running", from: EvalSessionStatusQueued, to: EvalSessionStatusRunning, want: true},
		{name: "queued to cancelled", from: EvalSessionStatusQueued, to: EvalSessionStatusCancelled, want: true},
		{name: "running to aggregating", from: EvalSessionStatusRunning, to: EvalSessionStatusAggregating, want: true},
		{name: "running to failed", from: EvalSessionStatusRunning, to: EvalSessionStatusFailed, want: true},
		{name: "aggregating to completed", from: EvalSessionStatusAggregating, to: EvalSessionStatusCompleted, want: true},
		{name: "aggregating to cancelled", from: EvalSessionStatusAggregating, to: EvalSessionStatusCancelled, want: true},
		{name: "queued to completed denied", from: EvalSessionStatusQueued, to: EvalSessionStatusCompleted, want: false},
		{name: "running to completed denied", from: EvalSessionStatusRunning, to: EvalSessionStatusCompleted, want: false},
		{name: "completed to failed denied", from: EvalSessionStatusCompleted, to: EvalSessionStatusFailed, want: false},
		{name: "cancelled to running denied", from: EvalSessionStatusCancelled, to: EvalSessionStatusRunning, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.from.CanTransitionTo(tc.to); got != tc.want {
				t.Fatalf("CanTransitionTo(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestParseEvalSessionStatusRejectsInvalidValues(t *testing.T) {
	if _, err := ParseEvalSessionStatus("draft"); err == nil {
		t.Fatal("expected invalid eval session status error")
	}
	if _, err := ParseEvalSessionStatus(""); err == nil {
		t.Fatal("expected empty eval session status error")
	}
}
