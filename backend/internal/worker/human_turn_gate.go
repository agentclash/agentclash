package worker

import (
	"context"

	"github.com/agentclash/agentclash/backend/internal/engine"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type repositoryHumanTurnGate struct {
	store *repository.MultiTurnHumanTurnStore
}

func NewRepositoryHumanTurnGate(store *repository.MultiTurnHumanTurnStore) engine.HumanTurnGate {
	if store == nil {
		return engine.NoopHumanTurnGate()
	}
	return repositoryHumanTurnGate{store: store}
}

func (g repositoryHumanTurnGate) WaitForHumanTurn(ctx context.Context, req engine.HumanTurnRequest) (string, error) {
	return engine.NewPollHumanTurnGate(humanTurnStoreAdapter{store: g.store}).WaitForHumanTurn(ctx, req)
}

type humanTurnStoreAdapter struct {
	store *repository.MultiTurnHumanTurnStore
}

func (a humanTurnStoreAdapter) MarkAwaitingHuman(ctx context.Context, req engine.HumanTurnRequest) error {
	return a.store.MarkAwaitingHuman(ctx, repository.HumanTurnWaitParams{
		RunAgentID: req.RunAgentID,
		TurnIndex:  req.TurnIndex,
		PhaseID:    req.PhaseID,
		PromptHint: req.PromptHint,
		Timeout:    req.Timeout,
	})
}

func (a humanTurnStoreAdapter) PollHumanMessage(ctx context.Context, runAgentID uuid.UUID, turnIndex int) (string, bool, error) {
	return a.store.PollHumanMessage(ctx, runAgentID, turnIndex)
}

func (a humanTurnStoreAdapter) MarkHumanTurnExpired(ctx context.Context, runAgentID uuid.UUID, turnIndex int) error {
	return a.store.MarkHumanTurnExpired(ctx, runAgentID, turnIndex)
}
