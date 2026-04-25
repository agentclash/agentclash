package racecontext

import (
	"context"
	"testing"

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
