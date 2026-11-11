package vibe

import (
	"fmt"
	"time"
)

// AttemptState is the state of one vibe_attempts row. The CHECK constraint
// in migration 00070 names the same four states.
type AttemptState string

const (
	// AttemptDispatching is the first state. BeginAttempt commits it before
	// the provider call.
	AttemptDispatching AttemptState = "DISPATCHING"
	// AttemptSucceeded means that the call succeeded and its cost is known.
	AttemptSucceeded AttemptState = "SUCCEEDED"
	// AttemptUncertain means that the attempt ended, but its cost is not
	// known. The operation keeps its full hold.
	AttemptUncertain AttemptState = "UNCERTAIN"
	// AttemptReconciled means that the attempt ended without success, and
	// its cost is known. reconciliation_evidence records how the cost became
	// known: the provider response, a receipt, or an assumed cost.
	AttemptReconciled AttemptState = "RECONCILED"
)

// attemptTransitions names every state change that a writer can make. A
// write that keeps the state is not listed.
//
// Writers:
//   - EndAttempt: DISPATCHING -> SUCCEEDED, UNCERTAIN, RECONCILED. A late
//     response with a cost moves an abandoned UNCERTAIN attempt to RECONCILED.
//   - ReconcileCost: UNCERTAIN -> RECONCILED.
//   - sweepAttempts: DISPATCHING -> UNCERTAIN after AttemptAbandonAfter, or
//     DISPATCHING -> RECONCILED if a receipt gave the cost first. Then
//     UNCERTAIN -> RECONCILED after AttemptChargeAfter.
var attemptTransitions = map[AttemptState]map[AttemptState]struct{}{
	AttemptDispatching: {
		AttemptSucceeded:  {},
		AttemptUncertain:  {},
		AttemptReconciled: {},
	},
	AttemptUncertain: {
		AttemptReconciled: {},
	},
	AttemptSucceeded:  {},
	AttemptReconciled: {},
}

func (s AttemptState) Valid() bool {
	_, ok := attemptTransitions[s]
	return ok
}

func (s AttemptState) CanTransitionTo(next AttemptState) bool {
	nextStates, ok := attemptTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStates[next]
	return ok
}

// Terminal reports whether the state is final.
func (s AttemptState) Terminal() bool {
	nextStates, ok := attemptTransitions[s]
	return ok && len(nextStates) == 0
}

func attemptTransitionFault(from, to AttemptState) error {
	return fault("invalid_state", fmt.Sprintf("Provider attempt cannot transition from %s to %s.", from, to))
}

const (
	// paidActivityTimeout bounds the vibe.execute activity. Each provider
	// call of an operation runs inside this activity.
	paidActivityTimeout = 16 * time.Minute
	// attemptJournalTimeout bounds each journal write of a provider call.
	// EndAttempt can run for this time after the activity context ends.
	attemptJournalTimeout = 5 * time.Second
	// AttemptAbandonAfter is the age at which the sweep ends a DISPATCHING
	// attempt. No live provider call is older.
	AttemptAbandonAfter = paidActivityTimeout + 4*time.Minute
	// receiptWindow is the age range in which the receipt loop asks the
	// provider for the cost of an attempt.
	receiptWindow = 24 * time.Hour
	// AttemptChargeAfter is the age at which the sweep gives an UNCERTAIN
	// attempt with no cost an assumed cost.
	AttemptChargeAfter = receiptWindow + time.Hour
	// attemptSweepBatch limits the attempts that one sweep pass moves.
	attemptSweepBatch = 100
)
