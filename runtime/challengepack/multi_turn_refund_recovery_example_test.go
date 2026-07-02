package challengepack_test

import (
	"os"
	"testing"

	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/scoring"
)

func TestExamplePack_MultiTurnRefundRecovery_LoadsAndValidates(t *testing.T) {
	path := repoRelative(t, "examples/challenge-packs/multi-turn-refund-recovery.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pack: %v", err)
	}
	bundle, err := challengepack.ParseYAML(data)
	if err != nil {
		t.Fatalf("ParseYAML: %v", err)
	}
	if bundle.Pack.Slug != "multi-turn-refund-recovery" {
		t.Fatalf("slug = %q; want multi-turn-refund-recovery", bundle.Pack.Slug)
	}
	if bundle.Version.ExecutionMode != challengepack.ExecutionModeMultiTurn {
		t.Fatalf("execution_mode = %q; want multi_turn", bundle.Version.ExecutionMode)
	}
	if len(bundle.InputSets) != 1 || len(bundle.InputSets[0].Cases) != 1 {
		t.Fatalf("expected one default case; got input_sets=%d", len(bundle.InputSets))
	}
	sim := bundle.InputSets[0].Cases[0].UserSimulator
	if sim == nil {
		t.Fatal("user_simulator is required for multi_turn packs")
	}
	if len(sim.Phases) < 3 {
		t.Fatalf("expected hybrid phases; got %d", len(sim.Phases))
	}
	spec := bundle.Version.EvaluationSpec
	if spec.JudgeMode != scoring.JudgeModeHybrid {
		t.Fatalf("judge_mode = %q; want hybrid", spec.JudgeMode)
	}
	foundJudge := false
	for _, judge := range spec.LLMJudges {
		if judge.Key == "recovery_after_mismatch" {
			foundJudge = true
			for _, ref := range judge.ContextFrom {
				if ref == "transcript.from_mismatch" {
					return
				}
			}
			t.Fatal("recovery_after_mismatch judge missing transcript.from_mismatch context")
		}
	}
	if !foundJudge {
		t.Fatal("expected recovery_after_mismatch llm_judge")
	}
}
