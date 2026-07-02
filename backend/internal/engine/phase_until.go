package engine

import (
	"strings"

	"github.com/agentclash/agentclash/runtime/challengepack"
)

type untilEvalContext struct {
	AssistantText string
	UserMessage   string
	TurnIndex     int
	PhaseTurns    int
	MaxPhaseTurns int
}

func phaseUntilSatisfied(conditions []string, ctx untilEvalContext) bool {
	if len(conditions) == 0 {
		return false
	}
	for _, raw := range conditions {
		cond := strings.TrimSpace(raw)
		if cond == "" {
			continue
		}
		if !untilConditionMet(cond, ctx) {
			return false
		}
	}
	return true
}

func untilConditionMet(condition string, ctx untilEvalContext) bool {
	switch {
	case condition == "max_turns":
		if ctx.MaxPhaseTurns <= 0 {
			return false
		}
		return ctx.PhaseTurns >= ctx.MaxPhaseTurns
	case strings.HasPrefix(condition, "assistant_emitted:"):
		token := strings.TrimSpace(strings.TrimPrefix(condition, "assistant_emitted:"))
		return token != "" && strings.Contains(strings.ToLower(ctx.AssistantText), strings.ToLower(token))
	case strings.HasPrefix(condition, "user_token:"):
		token := strings.TrimSpace(strings.TrimPrefix(condition, "user_token:"))
		return token != "" && strings.Contains(strings.ToLower(ctx.UserMessage), strings.ToLower(token))
	default:
		return false
	}
}

func shouldRunPhase(phase challengepack.UserSimulatorPhase, lastMismatch bool) bool {
	trigger := strings.TrimSpace(phase.Trigger)
	if trigger == "" {
		trigger = challengepack.UserSimulatorTriggerAlways
	}
	switch trigger {
	case challengepack.UserSimulatorTriggerNever:
		return false
	case challengepack.UserSimulatorTriggerOnAssistantMismatch:
		return lastMismatch
	case challengepack.UserSimulatorTriggerManual:
		return true
	default:
		return true
	}
}
