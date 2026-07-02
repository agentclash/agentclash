package engine

import "github.com/agentclash/agentclash/runtime/runner"

var ErrHumanTurnTimeout = runner.ErrHumanTurnTimeout

type HumanTurnRequest = runner.HumanTurnRequest
type HumanTurnGate = runner.HumanTurnGate
type PollHumanTurnGate = runner.PollHumanTurnGate
type HumanTurnStore = runner.HumanTurnStore

func NoopHumanTurnGate() HumanTurnGate {
	return runner.NoopHumanTurnGate()
}

func NewPollHumanTurnGate(store HumanTurnStore) PollHumanTurnGate {
	return runner.NewPollHumanTurnGate(store)
}
