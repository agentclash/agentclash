package repository

import "fmt"

// HostedRunExecutionStatus is the status of one hosted_run_executions row.
//
// The row records what AgentClash knows about the remote run. This is a
// separate fact from the run-agent status. For example, a run agent fails
// when the callback deadline passes, but the remote outcome stays unknown.
type HostedRunExecutionStatus string

const (
	// HostedRunExecutionStatusStarting is the first status. The dispatch
	// can be claimed, but the remote has not answered yet.
	HostedRunExecutionStatusStarting HostedRunExecutionStatus = "starting"
	// HostedRunExecutionStatusAccepted means that the remote accepted the
	// dispatch and returned its external run ID.
	HostedRunExecutionStatusAccepted HostedRunExecutionStatus = "accepted"
	// HostedRunExecutionStatusRunning means that a non-terminal callback
	// arrived.
	HostedRunExecutionStatusRunning HostedRunExecutionStatus = "running"
	// HostedRunExecutionStatusCompleted means that a terminal callback
	// reported success.
	HostedRunExecutionStatusCompleted HostedRunExecutionStatus = "completed"
	// HostedRunExecutionStatusFailed means that the remote rejected the
	// dispatch or a terminal callback reported failure.
	HostedRunExecutionStatusFailed HostedRunExecutionStatus = "failed"
	// HostedRunExecutionStatusUnknown means that AgentClash cannot tell what
	// the remote did. Two writes set it:
	//
	//   - The dispatch result was lost. The dispatch is never sent again,
	//     and the workflow still waits for callbacks until the deadline.
	//   - The callback deadline passed. The wait is over, the run agent
	//     fails, and finished_at is set. The outcome stays unknown for
	//     reconciliation; a late callback is recorded as evidence only.
	HostedRunExecutionStatusUnknown HostedRunExecutionStatus = "unknown"
	// HostedRunExecutionStatusTimedOut is on rows that binaries before
	// migration 00090 wrote at the callback deadline. Current code does not
	// write it. The status is final.
	HostedRunExecutionStatusTimedOut HostedRunExecutionStatus = "timed_out"
)

// hostedRunExecutionTransitions names every status change that a writer can
// make. A write to the same status is always allowed and is not listed.
//
// Writers:
//   - MarkHostedRunUnknown: starting -> unknown.
//   - MarkHostedRunExecutionAccepted: starting, unknown -> accepted.
//   - MarkHostedRunExecutionFailed: starting, unknown -> failed.
//   - MarkHostedRunExecutionTimedOut: starting, accepted, running -> unknown.
//   - CommitHostedCallback: starting, accepted, unknown -> running, and
//     starting, accepted, running, unknown -> completed, failed.
var hostedRunExecutionTransitions = map[HostedRunExecutionStatus]map[HostedRunExecutionStatus]struct{}{
	HostedRunExecutionStatusStarting: {
		HostedRunExecutionStatusAccepted:  {},
		HostedRunExecutionStatusRunning:   {},
		HostedRunExecutionStatusCompleted: {},
		HostedRunExecutionStatusFailed:    {},
		HostedRunExecutionStatusUnknown:   {},
	},
	HostedRunExecutionStatusAccepted: {
		HostedRunExecutionStatusRunning:   {},
		HostedRunExecutionStatusCompleted: {},
		HostedRunExecutionStatusFailed:    {},
		HostedRunExecutionStatusUnknown:   {},
	},
	HostedRunExecutionStatusRunning: {
		HostedRunExecutionStatusCompleted: {},
		HostedRunExecutionStatusFailed:    {},
		HostedRunExecutionStatusUnknown:   {},
	},
	HostedRunExecutionStatusUnknown: {
		HostedRunExecutionStatusAccepted:  {},
		HostedRunExecutionStatusRunning:   {},
		HostedRunExecutionStatusCompleted: {},
		HostedRunExecutionStatusFailed:    {},
	},
	HostedRunExecutionStatusCompleted: {},
	HostedRunExecutionStatusFailed:    {},
	HostedRunExecutionStatusTimedOut:  {},
}

func ParseHostedRunExecutionStatus(raw string) (HostedRunExecutionStatus, error) {
	status := HostedRunExecutionStatus(raw)
	if !status.Valid() {
		return "", fmt.Errorf("invalid hosted run execution status: %q", raw)
	}
	return status, nil
}

func (s HostedRunExecutionStatus) Valid() bool {
	_, ok := hostedRunExecutionTransitions[s]
	return ok
}

func (s HostedRunExecutionStatus) CanTransitionTo(next HostedRunExecutionStatus) bool {
	nextStatuses, ok := hostedRunExecutionTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

// Terminal reports whether the status is final.
func (s HostedRunExecutionStatus) Terminal() bool {
	nextStatuses, ok := hostedRunExecutionTransitions[s]
	return ok && len(nextStatuses) == 0
}
