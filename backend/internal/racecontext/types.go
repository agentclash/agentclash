// Package racecontext holds the shared types and pure functions for the
// race-context feature from issue #400: live peer-standings injected into
// each agent's prompt mid-run. The package has no infrastructure
// dependencies (no Redis, no Postgres) so both the engine (reader/consumer)
// and the pubsub layer (writer/producer) can import it without creating a
// cycle through worker.
package racecontext

import (
	"time"

	"github.com/google/uuid"
)

// StandingsEntry is the per-agent snapshot stored in the standings store,
// keyed by run_agent_id. The writer (pubsub.StandingsRecorder) merges
// partial updates from events; the reader (engine executor) treats the
// entry as a point-in-time view.
type StandingsEntry struct {
	RunAgentID  uuid.UUID      `json:"run_agent_id"`
	Model       string         `json:"model,omitempty"`
	Step        int            `json:"step"`
	ToolCalls   int            `json:"tool_calls"`
	TokensUsed  int64          `json:"tokens_used"`
	State       StandingsState `json:"state"`
	SubmittedAt *time.Time     `json:"submitted_at,omitempty"`
	FailedAt    *time.Time     `json:"failed_at,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	LastEventAt time.Time      `json:"last_event_at"`
}

// StandingsState matches the states documented in the race-context issue
// edge-case matrix: not_started, running, submitted, failed, timed_out.
type StandingsState string

const (
	StandingsStateNotStarted StandingsState = "not_started"
	StandingsStateRunning    StandingsState = "running"
	StandingsStateSubmitted  StandingsState = "submitted"
	StandingsStateFailed     StandingsState = "failed"
	StandingsStateTimedOut   StandingsState = "timed_out"
)

// standingsTransitions names every legal state change. A write of the same
// state is always allowed and is not listed. submitted, failed and timed_out
// are final: a late event for an agent that has finished must not move it
// back to running, because each move into a final state fires a peer trigger
// in the other agents' prompts.
var standingsTransitions = map[StandingsState]map[StandingsState]struct{}{
	StandingsStateNotStarted: {
		StandingsStateRunning:   {},
		StandingsStateSubmitted: {},
		StandingsStateFailed:    {},
		StandingsStateTimedOut:  {},
	},
	StandingsStateRunning: {
		StandingsStateSubmitted: {},
		StandingsStateFailed:    {},
		StandingsStateTimedOut:  {},
	},
	StandingsStateSubmitted: {},
	StandingsStateFailed:    {},
	StandingsStateTimedOut:  {},
}

// Valid reports whether s is a known standings state.
func (s StandingsState) Valid() bool {
	_, ok := standingsTransitions[s]
	return ok
}

// CanTransitionTo reports whether a standings entry can move from s to next.
func (s StandingsState) CanTransitionTo(next StandingsState) bool {
	nextStates, ok := standingsTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStates[next]
	return ok
}

// Terminal reports whether s is final.
func (s StandingsState) Terminal() bool {
	nextStates, ok := standingsTransitions[s]
	return ok && len(nextStates) == 0
}

// HashKey returns the Redis hash key used by the pubsub-layer store.
// Kept in this package so readers can name the key without importing Redis.
func HashKey(runID uuid.UUID) string {
	return "run:" + runID.String() + ":standings"
}

// FieldName returns the per-agent hash field name.
func FieldName(runAgentID uuid.UUID) string {
	return "agent:" + runAgentID.String()
}
