package engine

import (
	"testing"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
	"github.com/agentclash/agentclash/backend/internal/scoring"
)

func TestEvaluateTurnExpects_Contains(t *testing.T) {
	expects := []challengepack.CaseExpectation{
		{Key: "mentions_refund", Kind: string(scoring.ValidatorTypeContains), Value: "refund"},
	}
	if !evaluateTurnExpects("sorry, I cannot help", expects) {
		t.Fatal("expected mismatch when assistant output lacks required substring")
	}
	if evaluateTurnExpects("your refund is processing", expects) {
		t.Fatal("expected match when assistant output contains required substring")
	}
}

func TestShouldRunPhase_OnAssistantMismatch(t *testing.T) {
	phase := challengepack.UserSimulatorPhase{
		ID:      "pushback",
		Actor:   challengepack.UserSimulatorActorScripted,
		Trigger: challengepack.UserSimulatorTriggerOnAssistantMismatch,
	}
	if shouldRunPhase(phase, false) {
		t.Fatal("expected phase to stay inactive without prior mismatch")
	}
	if !shouldRunPhase(phase, true) {
		t.Fatal("expected phase to run after mismatch")
	}
}

func TestPhaseUntilSatisfied_AssistantEmitted(t *testing.T) {
	until := []string{"assistant_emitted:refund"}
	if phaseUntilSatisfied(until, untilEvalContext{AssistantText: "still working on it"}) {
		t.Fatal("expected until unsatisfied without refund token")
	}
	if !phaseUntilSatisfied(until, untilEvalContext{AssistantText: "refund approved"}) {
		t.Fatal("expected until satisfied when assistant emitted token")
	}
}
