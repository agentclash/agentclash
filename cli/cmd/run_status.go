package cmd

// Terminal run and job statuses. The server owns these values in the run
// transition table (runtime/domain/run.go), which `agentclash schema`
// publishes through domain.Lifecycles(). Every follow and poll loop for runs,
// eval sessions, agent-harness executions, and dataset generation jobs must
// use isTerminalRunStatus. Do not write a separate switch: the divergent
// copies are how `dataset generate --follow` looped forever on a cancelled
// job. Eval sets have one more terminal status; use evalsetStatusTerminal.
const (
	runStatusCompleted = "completed"
	runStatusFailed    = "failed"
	runStatusCancelled = "cancelled"
)

// isTerminalRunStatus reports whether a run/job/execution status is final —
// no follow loop should keep polling past it. The single-l "canceled" is
// accepted defensively: the backend's canonical spelling is "cancelled", but
// an alias here costs nothing and an omission means an infinite poll loop.
func isTerminalRunStatus(status string) bool {
	switch status {
	case runStatusCompleted, runStatusFailed, runStatusCancelled, "canceled":
		return true
	default:
		return false
	}
}
