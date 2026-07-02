package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPollHumanTurnGateReturnsReadyMessage(t *testing.T) {
	store := &fakeHumanTurnStore{message: "continue"}
	gate := NewPollHumanTurnGate(store)

	got, err := gate.WaitForHumanTurn(context.Background(), HumanTurnRequest{
		RunAgentID: uuid.New(),
		TurnIndex:  2,
	})
	if err != nil {
		t.Fatalf("WaitForHumanTurn: %v", err)
	}
	if got != "continue" {
		t.Fatalf("message = %q; want continue", got)
	}
	if !store.markedAwaiting {
		t.Fatal("expected MarkAwaitingHuman to be called")
	}
}

func TestPollHumanTurnGateExpiresOnTimeout(t *testing.T) {
	store := &fakeHumanTurnStore{}
	gate := NewPollHumanTurnGate(store)

	_, err := gate.WaitForHumanTurn(context.Background(), HumanTurnRequest{
		RunAgentID: uuid.New(),
		TurnIndex:  1,
		Timeout:    time.Nanosecond,
	})
	if !errors.Is(err, ErrHumanTurnTimeout) {
		t.Fatalf("error = %v; want ErrHumanTurnTimeout", err)
	}
	if !store.markedExpired {
		t.Fatal("expected MarkHumanTurnExpired to be called")
	}
}

func TestPollHumanTurnGateReturnsParentCancellation(t *testing.T) {
	store := &fakeHumanTurnStore{}
	gate := NewPollHumanTurnGate(store)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := gate.WaitForHumanTurn(ctx, HumanTurnRequest{
		RunAgentID: uuid.New(),
		TurnIndex:  1,
		Timeout:    time.Hour,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v; want context.Canceled", err)
	}
	if store.markedExpired {
		t.Fatal("parent cancellation must not mark human turn expired")
	}
}

type fakeHumanTurnStore struct {
	message        string
	markedAwaiting bool
	markedExpired  bool
}

func (s *fakeHumanTurnStore) MarkAwaitingHuman(context.Context, HumanTurnRequest) error {
	s.markedAwaiting = true
	return nil
}

func (s *fakeHumanTurnStore) PollHumanMessage(context.Context, uuid.UUID, int) (string, bool, error) {
	return s.message, s.message != "", nil
}

func (s *fakeHumanTurnStore) MarkHumanTurnExpired(context.Context, uuid.UUID, int) error {
	s.markedExpired = true
	return nil
}
