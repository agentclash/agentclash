package engine

import (
	"context"

	"github.com/agentclash/agentclash/backend/internal/simulator"
)

// TurnMessageGenerator produces the next user message for LLM simulator phases.
type TurnMessageGenerator interface {
	GenerateUserMessage(ctx context.Context, input simulator.Input) (message string, metadata simulator.Metadata, err error)
}

type simulatorTurnGenerator struct {
	generator simulator.Generator
}

func NewSimulatorTurnGenerator(generator simulator.Generator) TurnMessageGenerator {
	return simulatorTurnGenerator{generator: generator}
}

func (g simulatorTurnGenerator) GenerateUserMessage(ctx context.Context, input simulator.Input) (string, simulator.Metadata, error) {
	return g.generator.GenerateUserMessage(ctx, input)
}
