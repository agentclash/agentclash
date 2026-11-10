package racecontext

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMergeEntryIsAdditive(t *testing.T) {
	agentID := uuid.New()
	existing := StandingsEntry{
		RunAgentID: agentID,
		Step:       3,
		ToolCalls:  2,
		TokensUsed: 100,
		State:      StandingsStateRunning,
		Model:      "gpt-5",
	}
	update := StandingsEntry{
		RunAgentID: agentID,
		Step:       2, // lower than existing; should NOT regress
		ToolCalls:  1,
		TokensUsed: 50,
		Model:      "", // empty; should NOT clobber model name
	}
	merged := MergeEntry(existing, update)

	if merged.Step != 3 {
		t.Errorf("step regressed: got %d, want 3 (max preserved)", merged.Step)
	}
	if merged.ToolCalls != 3 {
		t.Errorf("tool_calls = %d, want 3 (additive)", merged.ToolCalls)
	}
	if merged.TokensUsed != 150 {
		t.Errorf("tokens = %d, want 150 (additive)", merged.TokensUsed)
	}
	if merged.Model != "gpt-5" {
		t.Errorf("model clobbered to %q, want preserved gpt-5", merged.Model)
	}
	if merged.State != StandingsStateRunning {
		t.Errorf("state = %q, want running (empty update didn't clobber)", merged.State)
	}
}

func TestHashKeyAndFieldName(t *testing.T) {
	runID := uuid.MustParse("12345678-1234-1234-1234-123456789abc")
	agentID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	if key := HashKey(runID); key != "run:12345678-1234-1234-1234-123456789abc:standings" {
		t.Errorf("HashKey = %q", key)
	}
	if field := FieldName(agentID); field != "agent:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Errorf("FieldName = %q", field)
	}
}

func TestNoopStoreIsInert(t *testing.T) {
	store := NoopStore{}
	if err := store.Update(context.Background(), uuid.New(), StandingsEntry{}); err != nil {
		t.Errorf("noop Update returned error: %v", err)
	}
	snap, err := store.Snapshot(context.Background(), uuid.New())
	if err != nil {
		t.Errorf("noop Snapshot returned error: %v", err)
	}
	if len(snap) != 0 {
		t.Errorf("noop Snapshot returned %d entries, want 0", len(snap))
	}
	if err := store.Close(); err != nil {
		t.Errorf("noop Close returned error: %v", err)
	}
}

func TestStandingsTransitionsNameEveryLegalMove(t *testing.T) {
	all := []StandingsState{
		StandingsStateNotStarted,
		StandingsStateRunning,
		StandingsStateSubmitted,
		StandingsStateFailed,
		StandingsStateTimedOut,
	}
	type move struct{ from, to StandingsState }
	legal := map[move]bool{
		{StandingsStateNotStarted, StandingsStateRunning}:   true,
		{StandingsStateNotStarted, StandingsStateSubmitted}: true,
		{StandingsStateNotStarted, StandingsStateFailed}:    true,
		{StandingsStateNotStarted, StandingsStateTimedOut}:  true,
		{StandingsStateRunning, StandingsStateSubmitted}:    true,
		{StandingsStateRunning, StandingsStateFailed}:       true,
		{StandingsStateRunning, StandingsStateTimedOut}:     true,
	}
	terminal := map[StandingsState]bool{
		StandingsStateSubmitted: true,
		StandingsStateFailed:    true,
		StandingsStateTimedOut:  true,
	}
	for _, from := range all {
		if !from.Valid() {
			t.Fatalf("state %q is not in the transition table", from)
		}
		if got := from.Terminal(); got != terminal[from] {
			t.Errorf("%s.Terminal() = %v, want %v", from, got, terminal[from])
		}
		for _, to := range all {
			if got := from.CanTransitionTo(to); got != legal[move{from, to}] {
				t.Errorf("CanTransitionTo(%s -> %s) = %v, want %v", from, to, got, legal[move{from, to}])
			}
		}
	}
	if len(standingsTransitions) != len(all) {
		t.Fatalf("transition table has %d states, want %d", len(standingsTransitions), len(all))
	}
}

func TestMergeEntryKeepsFinalStates(t *testing.T) {
	firstAt := time.Date(2026, 1, 1, 0, 0, 10, 0, time.UTC)
	laterAt := firstAt.Add(time.Second)
	for _, final := range []StandingsState{StandingsStateSubmitted, StandingsStateFailed, StandingsStateTimedOut} {
		t.Run(string(final), func(t *testing.T) {
			existing := StandingsEntry{Step: 4, ToolCalls: 2, TokensUsed: 100, State: final}
			if final == StandingsStateSubmitted {
				existing.SubmittedAt = &firstAt
			} else {
				existing.FailedAt = &firstAt
			}

			// A late step_started event must not reopen the entry.
			merged := MergeEntry(existing, StandingsEntry{Step: 5, State: StandingsStateRunning})
			if merged.State != final {
				t.Fatalf("state = %q, want %q to stay final", merged.State, final)
			}
			if merged.Step != 4 {
				t.Fatalf("step = %d, want 4 to stay at the final step", merged.Step)
			}

			// A different final state does not replace the first one.
			other := StandingsStateFailed
			if final == StandingsStateFailed {
				other = StandingsStateSubmitted
			}
			merged = MergeEntry(merged, StandingsEntry{State: other, SubmittedAt: &laterAt, FailedAt: &laterAt})
			if merged.State != final {
				t.Fatalf("state = %q, want %q to stay final", merged.State, final)
			}

			// A repeat of the same final state is allowed but keeps the first time.
			merged = MergeEntry(merged, StandingsEntry{State: final, SubmittedAt: &laterAt, FailedAt: &laterAt})
			if final == StandingsStateSubmitted && !merged.SubmittedAt.Equal(firstAt) {
				t.Fatalf("submitted_at = %s, want the first time %s", merged.SubmittedAt, firstAt)
			}
			if final != StandingsStateSubmitted && !merged.FailedAt.Equal(firstAt) {
				t.Fatalf("failed_at = %s, want the first time %s", merged.FailedAt, firstAt)
			}

			// Late usage still counts.
			merged = MergeEntry(merged, StandingsEntry{ToolCalls: 1, TokensUsed: 10})
			if merged.ToolCalls != 3 || merged.TokensUsed != 110 {
				t.Fatalf("usage = %d tool calls, %d tokens, want 3 and 110", merged.ToolCalls, merged.TokensUsed)
			}
		})
	}
}

func TestMergeEntryMovesForward(t *testing.T) {
	submittedAt := time.Date(2026, 1, 1, 0, 0, 10, 0, time.UTC)
	merged := MergeEntry(StandingsEntry{}, StandingsEntry{Step: 1, State: StandingsStateRunning})
	if merged.State != StandingsStateRunning {
		t.Fatalf("new entry state = %q, want running", merged.State)
	}
	merged = MergeEntry(merged, StandingsEntry{State: StandingsStateNotStarted})
	if merged.State != StandingsStateRunning {
		t.Fatalf("state = %q, want running (not_started is not a legal next state)", merged.State)
	}
	merged = MergeEntry(merged, StandingsEntry{State: StandingsState("paused")})
	if merged.State != StandingsStateRunning {
		t.Fatalf("state = %q, want running (unknown states are ignored)", merged.State)
	}
	merged = MergeEntry(merged, StandingsEntry{State: StandingsStateSubmitted, SubmittedAt: &submittedAt})
	if merged.State != StandingsStateSubmitted || merged.SubmittedAt == nil || !merged.SubmittedAt.Equal(submittedAt) {
		t.Fatalf("entry = %+v, want submitted at %s", merged, submittedAt)
	}
}
