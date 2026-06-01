package scoring

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildTranscriptFromEvents_GroupsTurnPayloads(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	events := []Event{
		{Type: "turn.user.message", SequenceNumber: 1, OccurredAt: now, Payload: mustJSON(t, map[string]any{
			"turn_index": 0, "phase_id": "open", "actor": "scripted", "content": "Refund please",
		})},
		{Type: "turn.assistant.message", SequenceNumber: 2, OccurredAt: now, Payload: mustJSON(t, map[string]any{
			"turn_index": 0, "content": "Sure, checking order.",
		})},
		{Type: "turn.completed", SequenceNumber: 3, OccurredAt: now, Payload: mustJSON(t, map[string]any{
			"turn_index": 0, "mismatch": true,
		})},
	}

	turns := buildTranscriptFromEvents(events)
	if len(turns) != 1 {
		t.Fatalf("len(turns) = %d; want 1", len(turns))
	}
	if turns[0].UserMessage != "Refund please" || turns[0].AssistantMessage != "Sure, checking order." {
		t.Fatalf("unexpected turn content: %+v", turns[0])
	}
	if !turns[0].Mismatch {
		t.Fatal("expected mismatch=true on turn.completed")
	}

	full, reason, err := resolveTranscriptEvidence("transcript.full", turns)
	if err != nil || reason != "" {
		t.Fatalf("transcript.full err=%v reason=%q", err, reason)
	}
	if full == nil || !strings.Contains(*full, "user[0]: Refund please") {
		t.Fatalf("transcript.full = %v", full)
	}

	fromMismatch, reason, err := resolveTranscriptEvidence("transcript.from_mismatch", turns)
	if err != nil || reason != "" {
		t.Fatalf("transcript.from_mismatch err=%v reason=%q", err, reason)
	}
	if fromMismatch == nil {
		t.Fatal("expected transcript from mismatch")
	}
}

func TestMultiTurnRecoveryScore_RecoversWhenValidatorsPass(t *testing.T) {
	t.Parallel()

	turns := []transcriptTurnEvidence{{TurnIndex: 0, Mismatch: true}}
	score, reason, state := multiTurnRecoveryScore(turns, []ValidatorResult{{
		Key: "ok", State: OutputStateAvailable, NormalizedScore: floatPtr(1),
	}})
	if state != OutputStateAvailable || score == nil || *score != 1 {
		t.Fatalf("recovery score = %v state=%v reason=%q; want 1 available", score, state, reason)
	}
}

func mustJSON(t *testing.T, v map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
