package cmd

// Canonical run/job lifecycle statuses, aligned with the backend's run state
// machine (backend/internal/domain/run.go). Every follow/poll loop in the CLI
// must use isTerminalRunStatus rather than hand-rolling its own switch — the
// divergent copies are exactly how `dataset generate --follow` looped forever
// on a cancelled job. The `schema` command's status registry consumes these
// same constants, so the documented contract and the code cannot drift.
const (
	runStatusPending   = "pending"
	runStatusRunning   = "running"
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
