package simulator_test

import (
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/simulator"
)

func TestTranscriptFromTurns_ClonesInput(t *testing.T) {
	t.Parallel()

	src := []simulator.TranscriptTurn{{Actor: "user", Content: "hello"}}
	cloned := simulator.TranscriptFromTurns(src)
	src[0].Content = "mutated"
	if cloned[0].Content != "hello" {
		t.Fatalf("TranscriptFromTurns() should clone; got %q", cloned[0].Content)
	}
}

func TestActorForEvent_MapsConversationActors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in, want string
	}{
		{"scripted", "user"},
		{"llm", "user"},
		{"human", "user"},
		{"assistant", "assistant"},
	} {
		if got := simulator.ActorForEvent(tc.in); got != tc.want {
			t.Fatalf("ActorForEvent(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
	if strings.TrimSpace(simulator.ActorForEvent("  human  ")) != "user" {
		t.Fatal("ActorForEvent should trim whitespace")
	}
}
