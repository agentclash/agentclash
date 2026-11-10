package racecontext

import (
	"context"

	"github.com/google/uuid"
)

// Store is the per-run standings backing store. Implementations must be
// safe for concurrent use across multiple agents within the same run. The
// contract is weak on consistency: readers may see stale or partial data,
// which is realistic for a live race and acceptable for the newswire use
// case.
type Store interface {
	// Update applies a partial update for one agent. Non-zero fields in
	// `updates` merge into existing state per the documented rules
	// (step is max-wins, tokens/tool_calls are additive, empty strings
	// don't clobber, final states stay final; see MergeEntry). The merge
	// must be atomic, so concurrent updates for one run lose no increment.
	// updates.RunAgentID must be set.
	Update(ctx context.Context, runID uuid.UUID, updates StandingsEntry) error
	// Snapshot returns the current standings for all agents in a run.
	// An empty map is returned when the run has no recorded standings.
	Snapshot(ctx context.Context, runID uuid.UUID) (map[uuid.UUID]StandingsEntry, error)
	// Close releases any resources held by the implementation.
	Close() error
}

// NoopStore is the fallback when Redis (or any other backing store) is
// not configured. All operations succeed silently and Snapshot returns an
// empty map, so injection becomes a no-op automatically.
type NoopStore struct{}

var _ Store = NoopStore{}

func (NoopStore) Update(context.Context, uuid.UUID, StandingsEntry) error { return nil }
func (NoopStore) Snapshot(context.Context, uuid.UUID) (map[uuid.UUID]StandingsEntry, error) {
	return map[uuid.UUID]StandingsEntry{}, nil
}
func (NoopStore) Close() error { return nil }

// MergeEntry applies non-zero fields from `updates` onto `existing` per the
// store contract. Exposed (and exported) so writer implementations share
// the same semantics and the rules are testable in one place.
//
// The state follows the standings transition table: a state change that
// the table does not allow is ignored, so a final state (submitted, failed,
// timed_out) stays final. A final entry also keeps its step, because the
// newswire shows it as "submitted at step N" or "FAILED at step N". The
// submitted and failed times are kept from the first write of the state
// that they belong to. Tool calls and tokens stay additive in every state,
// because a late event still reports real usage.
func MergeEntry(existing, updates StandingsEntry) StandingsEntry {
	if updates.RunAgentID != uuid.Nil {
		existing.RunAgentID = updates.RunAgentID
	}
	if updates.Model != "" {
		existing.Model = updates.Model
	}
	if updates.Step > existing.Step && !existing.State.Terminal() {
		existing.Step = updates.Step
	}
	existing.ToolCalls += updates.ToolCalls
	existing.TokensUsed += updates.TokensUsed
	if updates.State != "" && acceptsState(existing.State, updates.State) {
		existing.State = updates.State
		switch updates.State {
		case StandingsStateSubmitted:
			if existing.SubmittedAt == nil {
				existing.SubmittedAt = updates.SubmittedAt
			}
		case StandingsStateFailed, StandingsStateTimedOut:
			if existing.FailedAt == nil {
				existing.FailedAt = updates.FailedAt
			}
		}
	}
	if updates.StartedAt != nil && existing.StartedAt == nil {
		existing.StartedAt = updates.StartedAt
	}
	return existing
}

// acceptsState reports whether an entry in state `current` can take state
// `next`. An entry without a state has not started yet.
func acceptsState(current, next StandingsState) bool {
	if !next.Valid() {
		return false
	}
	if current == "" {
		current = StandingsStateNotStarted
	}
	return current == next || current.CanTransitionTo(next)
}
