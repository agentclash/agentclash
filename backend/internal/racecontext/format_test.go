package racecontext

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func fixedAgent(s string) uuid.UUID {
	// Produces stable UUIDs for deterministic ordering in tests.
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(s))
}

func TestFormatStandingsShowsRunningPeers(t *testing.T) {
	self := fixedAgent("self")
	peerA := fixedAgent("peerA")
	peerB := fixedAgent("peerB")
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	snapshot := map[uuid.UUID]StandingsEntry{
		self: {
			RunAgentID: self,
			Model:      "grok-4",
			Step:       9,
			ToolCalls:  5,
			TokensUsed: 2800,
			State:      StandingsStateRunning,
		},
		peerA: {
			RunAgentID: peerA,
			Model:      "gpt-5",
			Step:       12,
			ToolCalls:  6,
			TokensUsed: 4000,
			State:      StandingsStateRunning,
		},
		peerB: {
			RunAgentID: peerB,
			Model:      "gemini-2.5-pro",
			Step:       11,
			ToolCalls:  8,
			TokensUsed: 4400,
			State:      StandingsStateRunning,
		},
	}

	text, tokens := Format(FormatInput{
		Snapshot:       snapshot,
		SelfRunAgentID: self,
		SelfStepIndex:  9,
		Now:            now,
	})

	lines := strings.Split(text, "\n")
	if lines[0] != "[RACE UPDATE · your step 9]" {
		t.Fatalf("header = %q", lines[0])
	}
	if lines[1] != "3 agents running, 0 submitted." {
		t.Fatalf("summary = %q", lines[1])
	}
	// Rank: 12 > 11 > 9 — peer A first, peer B second, self last.
	if !strings.Contains(lines[2], "gpt-5") || !strings.Contains(lines[2], "12 steps") {
		t.Errorf("line 2 (top-ranked peer) = %q", lines[2])
	}
	if !strings.Contains(lines[3], "gemini-2.5-pro") {
		t.Errorf("line 3 = %q", lines[3])
	}
	if !strings.Contains(lines[4], "you (grok-4)") || !strings.Contains(lines[4], "9 steps") {
		t.Errorf("self line = %q", lines[4])
	}
	if tokens <= 0 {
		t.Errorf("tokens = %d, want > 0", tokens)
	}
}

func TestFormatStandingsPinsSubmittersToTop(t *testing.T) {
	self := fixedAgent("self")
	submitter := fixedAgent("submitter")
	laggard := fixedAgent("laggard")
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	started := now.Add(-5 * time.Minute)
	submittedAt := now.Add(-12 * time.Second)

	snapshot := map[uuid.UUID]StandingsEntry{
		self: {
			RunAgentID: self, Model: "grok-4", Step: 8, State: StandingsStateRunning,
		},
		submitter: {
			RunAgentID: submitter, Model: "claude-sonnet-4-6", Step: 14,
			State: StandingsStateSubmitted, StartedAt: &started, SubmittedAt: &submittedAt,
		},
		laggard: {
			RunAgentID: laggard, Model: "mistral-large", Step: 11, State: StandingsStateRunning,
		},
	}

	text, _ := Format(FormatInput{
		Snapshot: snapshot, SelfRunAgentID: self, SelfStepIndex: 8, Now: now,
	})
	lines := strings.Split(text, "\n")
	if !strings.Contains(lines[1], "2 agents running, 1 submitted") {
		t.Fatalf("summary = %q", lines[1])
	}
	if !strings.Contains(lines[2], "claude-sonnet-4-6") || !strings.Contains(lines[2], "submitted at step 14") {
		t.Errorf("submitter should be pinned to top, got line 2 = %q", lines[2])
	}
	if !strings.Contains(lines[2], "4m48s elapsed") {
		t.Errorf("elapsed duration wrong in %q", lines[2])
	}
	if !strings.Contains(lines[2], "verifying") {
		t.Errorf("submitter should show `· verifying` mid-scoring, got %q", lines[2])
	}
}

func TestFormatStandingsShowsFailedAndTimedOut(t *testing.T) {
	self := fixedAgent("self")
	failed := fixedAgent("failed")
	timedOut := fixedAgent("timed_out")
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	snapshot := map[uuid.UUID]StandingsEntry{
		self:     {RunAgentID: self, Model: "grok-4", Step: 4, State: StandingsStateRunning},
		failed:   {RunAgentID: failed, Model: "mistral-large", Step: 3, State: StandingsStateFailed},
		timedOut: {RunAgentID: timedOut, Model: "gpt-5", Step: 7, State: StandingsStateTimedOut},
	}

	text, _ := Format(FormatInput{
		Snapshot: snapshot, SelfRunAgentID: self, SelfStepIndex: 4, Now: now,
	})
	if !strings.Contains(text, "mistral-large — FAILED at step 3") {
		t.Errorf("missing FAILED line: %s", text)
	}
	if !strings.Contains(text, "gpt-5 — TIMED OUT at step 7") {
		t.Errorf("missing TIMED OUT line: %s", text)
	}
	// failed/timed_out agents must not be counted as either running or submitted.
	if !strings.Contains(text, "1 agents running, 0 submitted") {
		t.Errorf("summary miscounts failed/timed-out agents; text = %s", text)
	}
}

func TestFormatStandingsHandlesNotStarted(t *testing.T) {
	self := fixedAgent("self")
	unborn := fixedAgent("unborn")
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	snapshot := map[uuid.UUID]StandingsEntry{
		self:   {RunAgentID: self, Model: "grok-4", Step: 3, State: StandingsStateRunning},
		unborn: {RunAgentID: unborn, State: StandingsStateNotStarted},
	}

	text, _ := Format(FormatInput{
		Snapshot: snapshot, SelfRunAgentID: self, SelfStepIndex: 3, Now: now,
	})
	if !strings.Contains(text, "not started") {
		t.Errorf("not-started peer not rendered: %s", text)
	}
	// Unknown-model fallback should use the `agent-<8chars>` label.
	if !strings.Contains(text, "agent-") {
		t.Errorf("expected agent-id fallback label for unknown model: %s", text)
	}
}

func TestFormatStandingsAllSubmittedOneRunning(t *testing.T) {
	self := fixedAgent("self")
	peer1 := fixedAgent("p1")
	peer2 := fixedAgent("p2")
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	started := now.Add(-5 * time.Minute)
	t1 := now.Add(-60 * time.Second)
	t2 := now.Add(-30 * time.Second)

	snapshot := map[uuid.UUID]StandingsEntry{
		self:  {RunAgentID: self, Model: "grok-4", Step: 9, State: StandingsStateRunning},
		peer1: {RunAgentID: peer1, Model: "gpt-5", Step: 13, State: StandingsStateSubmitted, StartedAt: &started, SubmittedAt: &t2},
		peer2: {RunAgentID: peer2, Model: "claude-sonnet-4-6", Step: 11, State: StandingsStateSubmitted, StartedAt: &started, SubmittedAt: &t1},
	}

	text, _ := Format(FormatInput{
		Snapshot: snapshot, SelfRunAgentID: self, SelfStepIndex: 9, Now: now,
	})
	lines := strings.Split(text, "\n")
	if lines[1] != "1 agents running, 2 submitted." {
		t.Fatalf("summary = %q", lines[1])
	}
	// Earliest submission appears first.
	if !strings.Contains(lines[2], "claude-sonnet-4-6") {
		t.Errorf("earliest submitter should be first, got: %s", lines[2])
	}
	if !strings.Contains(lines[3], "gpt-5") {
		t.Errorf("later submitter should be second, got: %s", lines[3])
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("empty string tokens = %d, want 0", got)
	}
	if got := EstimateTokens("abcd"); got != 1 {
		t.Errorf("4 chars = %d tokens, want 1", got)
	}
	if got := EstimateTokens("abcde"); got != 2 {
		t.Errorf("5 chars = %d tokens, want 2 (rounded up)", got)
	}
}
