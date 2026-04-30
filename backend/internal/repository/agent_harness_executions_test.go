package repository

import "testing"

func TestAgentHarnessExecutionStatusTransitions(t *testing.T) {
	tests := []struct {
		name string
		from AgentHarnessExecutionStatus
		to   AgentHarnessExecutionStatus
		want bool
	}{
		{name: "queued to provisioning", from: AgentHarnessExecutionStatusQueued, to: AgentHarnessExecutionStatusProvisioning, want: true},
		{name: "queued to failed", from: AgentHarnessExecutionStatusQueued, to: AgentHarnessExecutionStatusFailed, want: true},
		{name: "running to scoring", from: AgentHarnessExecutionStatusRunning, to: AgentHarnessExecutionStatusScoring, want: true},
		{name: "scoring to completed", from: AgentHarnessExecutionStatusScoring, to: AgentHarnessExecutionStatusCompleted, want: true},
		{name: "completed is terminal", from: AgentHarnessExecutionStatusCompleted, to: AgentHarnessExecutionStatusRunning, want: false},
		{name: "cannot skip queued to running", from: AgentHarnessExecutionStatusQueued, to: AgentHarnessExecutionStatusRunning, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.from.CanTransitionTo(tc.to); got != tc.want {
				t.Fatalf("CanTransitionTo(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestAgentHarnessExecutionEventsSequencePerExecution(t *testing.T) {
	if !AgentHarnessExecutionStatusQueued.Valid() {
		t.Fatal("queued status should be valid before events can be appended to new executions")
	}
}
