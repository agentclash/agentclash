package repository

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

// The CHECK constraint from migration 00063 also permits "discarded". No
// writer sets it, so it is not a valid Go status.
var allChallengePackDraftStatuses = []ChallengePackDraftStatus{
	ChallengePackDraftStatusDraft,
	ChallengePackDraftStatusPublished,
}

func TestChallengePackDraftTransitionsNameEveryLegalMove(t *testing.T) {
	type move struct{ from, to ChallengePackDraftStatus }
	legal := map[move]string{
		{ChallengePackDraftStatusDraft, ChallengePackDraftStatusPublished}: "PublishChallengePackBundle",
	}
	for _, from := range allChallengePackDraftStatuses {
		if !from.Valid() {
			t.Fatalf("status %q is not in the transition table", from)
		}
		for _, to := range allChallengePackDraftStatuses {
			_, want := legal[move{from, to}]
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("CanTransitionTo(%s -> %s) = %v, want %v", from, to, got, want)
			}
		}
		if got, want := from.Terminal(), from == ChallengePackDraftStatusPublished; got != want {
			t.Errorf("%s.Terminal() = %v, want %v", from, got, want)
		}
		if got, want := from.Editable(), from == ChallengePackDraftStatusDraft; got != want {
			t.Errorf("%s.Editable() = %v, want %v", from, got, want)
		}
	}
	if len(challengePackDraftTransitions) != len(allChallengePackDraftStatuses) {
		t.Fatalf("transition table has %d statuses, want %d", len(challengePackDraftTransitions), len(allChallengePackDraftStatuses))
	}
	if _, err := ParseChallengePackDraftStatus("discarded"); err == nil {
		t.Fatal("discarded has no writer and must not parse")
	}
	if ChallengePackDraftStatus("discarded").Editable() {
		t.Fatal("a status that is not in the table must not take edits")
	}
}

func TestChallengePackDraftFinalErrorMatchesInvalidTransition(t *testing.T) {
	err := error(ChallengePackDraftFinalError{ID: uuid.New(), Status: ChallengePackDraftStatusPublished})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("errors.Is(%v, ErrInvalidTransition) = false, want true", err)
	}
	if errors.Is(err, ErrChallengePackDraftNotFound) {
		t.Fatalf("a final draft must not read as a missing draft")
	}
}
