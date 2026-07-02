package runner

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrHumanTurnTimeout = errors.New("human turn timed out")

// HumanTurnRequest identifies an in-run human actor pause.
type HumanTurnRequest struct {
	RunAgentID uuid.UUID
	TurnIndex  int
	PhaseID    string
	PromptHint string
	Timeout    time.Duration
}

// HumanTurnGate blocks until an operator submits the next user message.
type HumanTurnGate interface {
	WaitForHumanTurn(ctx context.Context, req HumanTurnRequest) (message string, err error)
}

type noopHumanTurnGate struct{}

func (noopHumanTurnGate) WaitForHumanTurn(context.Context, HumanTurnRequest) (string, error) {
	return "", errors.New("human turn gate is not configured")
}

func NoopHumanTurnGate() HumanTurnGate {
	return noopHumanTurnGate{}
}

// PollHumanTurnGate polls a HumanTurnStore until a message arrives or timeout.
type PollHumanTurnGate struct {
	store HumanTurnStore
}

type HumanTurnStore interface {
	MarkAwaitingHuman(ctx context.Context, req HumanTurnRequest) error
	PollHumanMessage(ctx context.Context, runAgentID uuid.UUID, turnIndex int) (message string, ready bool, err error)
	MarkHumanTurnExpired(ctx context.Context, runAgentID uuid.UUID, turnIndex int) error
}

func NewPollHumanTurnGate(store HumanTurnStore) PollHumanTurnGate {
	return PollHumanTurnGate{store: store}
}

func (g PollHumanTurnGate) WaitForHumanTurn(ctx context.Context, req HumanTurnRequest) (string, error) {
	if g.store == nil {
		return "", errors.New("human turn store is not configured")
	}
	if err := g.store.MarkAwaitingHuman(ctx, req); err != nil {
		return "", err
	}

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		message, ready, err := g.store.PollHumanMessage(ctx, req.RunAgentID, req.TurnIndex)
		if err != nil {
			return "", err
		}
		if ready {
			if strings.TrimSpace(message) == "" {
				return "", errors.New("human turn message is empty")
			}
			return message, nil
		}

		select {
		case <-ctx.Done():
			if req.Timeout > 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = g.store.MarkHumanTurnExpired(cleanupCtx, req.RunAgentID, req.TurnIndex)
				return "", ErrHumanTurnTimeout
			}
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}
