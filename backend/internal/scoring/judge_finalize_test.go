package scoring

import (
	"encoding/json"
	"testing"
	"time"
)

// --- FinalizeRunAgentEvaluation unit tests (Phase 4 of issue #148) ---

// buildDeterministicEvaluation constructs a RunAgentEvaluation that
// mimics what EvaluateRunAgent returns for a hybrid spec with one
// validator-sourced dim and one llm_judge-sourced dim. The llm_judge
// dim comes back as unavailable (Phase 1 stub behavior), matching
// the real call path from Phase 4's JudgeRunAgent activity.
func buildDeterministicEvaluation() (RunAgentEvaluation, EvaluationSpec) {
	correctnessScore := 0.75
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{Key: "correctness", Source: DimensionSourceValidators, BetterDirection: "higher", Weight: floatPtr(0.5)},
				{Key: "tone", Source: DimensionSourceLLMJudge, JudgeKey: "professional_tone", BetterDirection: "higher", Weight: floatPtr(0.5)},
			},
		},
	}
	eval := RunAgentEvaluation{
		Status:   EvaluationStatusPartial, // llm_judge unavailable → partial
		Strategy: ScoringStrategyWeighted,
		DimensionResults: []DimensionResult{
			{Dimension: "correctness", Score: &correctnessScore, State: OutputStateAvailable, BetterDirection: "higher"},
			{Dimension: "tone", Score: nil, State: OutputStateUnavailable, Reason: "llm_judge dimensions require the judge evaluator (#148 phase 3+)", BetterDirection: "higher"},
		},
		DimensionScores: map[string]*float64{
			"correctness": &correctnessScore,
			"tone":        nil,
		},
	}
	return eval, spec
}

func TestFinalizeRunAgentEvaluation_NilJudgesIsNoop(t *testing.T) {
	eval, spec := buildDeterministicEvaluation()

	finalized := FinalizeRunAgentEvaluation(eval, spec, nil)

	if finalized.Status != eval.Status {
		t.Errorf("Status = %q, want unchanged %q", finalized.Status, eval.Status)
	}
	if len(finalized.DimensionResults) != len(eval.DimensionResults) {
		t.Errorf("DimensionResults count = %d, want %d", len(finalized.DimensionResults), len(eval.DimensionResults))
	}
	// The llm_judge dim must stay unavailable — no judges supplied.
	toneDim := finalized.DimensionResults[1]
	if toneDim.State != OutputStateUnavailable {
		t.Errorf("tone dim state = %q, want unavailable (no judges merged)", toneDim.State)
	}
}

func TestFinalizeRunAgentEvaluation_MergesAssertionPass(t *testing.T) {
	eval, spec := buildDeterministicEvaluation()

	judgeScore := 1.0
	judges := []JudgeResult{
		{
			Key:             "professional_tone",
			Mode:            JudgeMethodAssertion,
			State:           OutputStateAvailable,
			NormalizedScore: &judgeScore,
			Confidence:      "high",
			SampleCount:     3,
			ModelCount:      1,
		},
	}

	finalized := FinalizeRunAgentEvaluation(eval, spec, judges)

	toneDim := finalized.DimensionResults[1]
	if toneDim.State != OutputStateAvailable {
		t.Fatalf("tone dim state = %q, want available after merge", toneDim.State)
	}
	if toneDim.Score == nil || *toneDim.Score != 1.0 {
		t.Fatalf("tone dim score = %v, want 1.0", toneDim.Score)
	}
	// Status should upgrade from partial → complete now that both
	// dims are available.
	if finalized.Status != EvaluationStatusComplete {
		t.Errorf("Status = %q, want complete after merge", finalized.Status)
	}
	// Overall score = (0.75 * 0.5 + 1.0 * 0.5) / 1.0 = 0.875
	if finalized.OverallScore == nil {
		t.Fatal("OverallScore is nil after merge")
	}
	if *finalized.OverallScore != 0.875 {
		t.Errorf("OverallScore = %v, want 0.875", *finalized.OverallScore)
	}
	// DimensionScores map must reflect the merged score.
	if got := finalized.DimensionScores["tone"]; got == nil || *got != 1.0 {
		t.Errorf("DimensionScores[tone] = %v, want 1.0", got)
	}
}

func TestFinalizeRunAgentEvaluation_MergesAssertionFail(t *testing.T) {
	eval, spec := buildDeterministicEvaluation()

	judgeScore := 0.0
	judges := []JudgeResult{
		{
			Key:             "professional_tone",
			Mode:            JudgeMethodAssertion,
			State:           OutputStateAvailable,
			NormalizedScore: &judgeScore,
		},
	}

	finalized := FinalizeRunAgentEvaluation(eval, spec, judges)

	toneDim := finalized.DimensionResults[1]
	if toneDim.Score == nil || *toneDim.Score != 0.0 {
		t.Fatalf("tone dim score = %v, want 0.0", toneDim.Score)
	}
	// Overall = (0.75 * 0.5 + 0.0 * 0.5) / 1.0 = 0.375
	if finalized.OverallScore == nil || *finalized.OverallScore != 0.375 {
		t.Errorf("OverallScore = %v, want 0.375", finalized.OverallScore)
	}
}

func TestFinalizeRunAgentEvaluation_UnavailableJudgePreservesUnavailable(t *testing.T) {
	eval, spec := buildDeterministicEvaluation()

	// Judge abstained — all samples UNKNOWN. State=unavailable,
	// NormalizedScore=nil. The merge should propagate this through
	// the dimension so the scorecard correctly reflects the abstain.
	judges := []JudgeResult{
		{
			Key:             "professional_tone",
			Mode:            JudgeMethodAssertion,
			State:           OutputStateUnavailable,
			NormalizedScore: nil,
			Reason:          "every model abstained or errored on assertion",
			Confidence:      "low",
			SampleCount:     3,
		},
	}

	finalized := FinalizeRunAgentEvaluation(eval, spec, judges)

	toneDim := finalized.DimensionResults[1]
	if toneDim.State != OutputStateUnavailable {
		t.Fatalf("tone dim state = %q, want unavailable (abstained judge)", toneDim.State)
	}
	if toneDim.Score != nil {
		t.Errorf("tone dim score = %v, want nil", toneDim.Score)
	}
	if finalized.Status != EvaluationStatusPartial {
		t.Errorf("Status = %q, want partial (one dim still unavailable)", finalized.Status)
	}
}

func TestFinalizeRunAgentEvaluation_MissingJudgeKeyLeavesStub(t *testing.T) {
	eval, spec := buildDeterministicEvaluation()

	// Judge results exist but none match the dim's JudgeKey — a
	// programming or configuration bug that validation should have
	// caught upstream. The finalize path must not crash or produce
	// nonsense; the dim stays as the unavailable stub.
	unrelatedScore := 1.0
	judges := []JudgeResult{
		{
			Key:             "unrelated_judge",
			Mode:            JudgeMethodAssertion,
			State:           OutputStateAvailable,
			NormalizedScore: &unrelatedScore,
		},
	}

	finalized := FinalizeRunAgentEvaluation(eval, spec, judges)

	toneDim := finalized.DimensionResults[1]
	if toneDim.State != OutputStateUnavailable {
		t.Errorf("tone dim state = %q, want unavailable (no matching judge)", toneDim.State)
	}
}

func TestFinalizeRunAgentEvaluation_DoesNotMutateInput(t *testing.T) {
	eval, spec := buildDeterministicEvaluation()

	// Snapshot the caller's state so we can assert it stays unchanged.
	originalDimResultsPtr := &eval.DimensionResults[0]
	originalScoresPtr := eval.DimensionScores

	judgeScore := 1.0
	judges := []JudgeResult{
		{
			Key:             "professional_tone",
			Mode:            JudgeMethodAssertion,
			State:           OutputStateAvailable,
			NormalizedScore: &judgeScore,
		},
	}

	finalized := FinalizeRunAgentEvaluation(eval, spec, judges)

	// The finalized slices/maps must not alias the input.
	if &finalized.DimensionResults[0] == originalDimResultsPtr {
		t.Error("DimensionResults slice aliased — finalize mutated caller input")
	}
	sameMap := &finalized.DimensionScores == &originalScoresPtr
	if sameMap {
		t.Error("DimensionScores map aliased — finalize mutated caller input")
	}
	// Caller's data must still show the pre-merge state.
	if eval.DimensionResults[1].State != OutputStateUnavailable {
		t.Error("caller DimensionResults mutated by finalize")
	}
}

func TestFinalizeRunAgentEvaluation_AllDimsFromJudges(t *testing.T) {
	// A pure-judge spec (no validator-sourced dims). After merge,
	// the status and overall are driven entirely by the judge
	// results. Pins that the function works for the "all llm_judge"
	// extreme, not just hybrid.
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{Key: "tone", Source: DimensionSourceLLMJudge, JudgeKey: "professional_tone", BetterDirection: "higher"},
				{Key: "accuracy", Source: DimensionSourceLLMJudge, JudgeKey: "factual_accuracy", BetterDirection: "higher"},
			},
		},
	}
	eval := RunAgentEvaluation{
		Status:   EvaluationStatusPartial,
		Strategy: ScoringStrategyWeighted,
		DimensionResults: []DimensionResult{
			{Dimension: "tone", State: OutputStateUnavailable, Reason: "stub", BetterDirection: "higher"},
			{Dimension: "accuracy", State: OutputStateUnavailable, Reason: "stub", BetterDirection: "higher"},
		},
		DimensionScores: map[string]*float64{"tone": nil, "accuracy": nil},
	}
	toneScore := 1.0
	accScore := 0.5
	judges := []JudgeResult{
		{Key: "professional_tone", Mode: JudgeMethodAssertion, State: OutputStateAvailable, NormalizedScore: &toneScore},
		{Key: "factual_accuracy", Mode: JudgeMethodAssertion, State: OutputStateAvailable, NormalizedScore: &accScore},
	}

	finalized := FinalizeRunAgentEvaluation(eval, spec, judges)

	if finalized.Status != EvaluationStatusComplete {
		t.Errorf("Status = %q, want complete", finalized.Status)
	}
	// Default weights (nil) → unweighted mean of (1.0, 0.5) = 0.75
	if finalized.OverallScore == nil || *finalized.OverallScore != 0.75 {
		t.Errorf("OverallScore = %v, want 0.75", finalized.OverallScore)
	}
}

// --- ExtractFinalOutputFromEvents unit tests ---

func TestExtractFinalOutputFromEvents_FromFinalizedEvent(t *testing.T) {
	events := []Event{
		{
			Type:       "system.output.finalized",
			OccurredAt: time.Now(),
			Payload:    json.RawMessage(`{"final_output": "hello world"}`),
		},
	}
	got := ExtractFinalOutputFromEvents(events)
	if got != "hello world" {
		t.Errorf("got %q, want hello world", got)
	}
}

func TestExtractFinalOutputFromEvents_FromRunCompletedEvent(t *testing.T) {
	events := []Event{
		{
			Type:       "system.run.completed",
			OccurredAt: time.Now(),
			Payload:    json.RawMessage(`{"final_output": "done"}`),
		},
	}
	got := ExtractFinalOutputFromEvents(events)
	if got != "done" {
		t.Errorf("got %q, want done", got)
	}
}

func TestExtractFinalOutputFromEvents_PrefersFinalizedEventFirst(t *testing.T) {
	// When both events carry a final_output, the first one in the
	// event stream wins. ExtractFinalOutputFromEvents is order-
	// stable by design — callers that need chronology sort upstream.
	events := []Event{
		{
			Type:       "system.output.finalized",
			OccurredAt: time.Now(),
			Payload:    json.RawMessage(`{"final_output": "early"}`),
		},
		{
			Type:       "system.run.completed",
			OccurredAt: time.Now().Add(time.Second),
			Payload:    json.RawMessage(`{"final_output": "late"}`),
		},
	}
	got := ExtractFinalOutputFromEvents(events)
	if got != "early" {
		t.Errorf("got %q, want early", got)
	}
}

func TestExtractFinalOutputFromEvents_FallsBackToOutputField(t *testing.T) {
	// system.output.finalized can carry the output under the
	// "output" key instead of "final_output" — matches the
	// extractLooseString fallback in buildEvidence.
	events := []Event{
		{
			Type:       "system.output.finalized",
			OccurredAt: time.Now(),
			Payload:    json.RawMessage(`{"output": "via output key"}`),
		},
	}
	got := ExtractFinalOutputFromEvents(events)
	if got != "via output key" {
		t.Errorf("got %q, want 'via output key'", got)
	}
}

func TestExtractFinalOutputFromEvents_NoMatchReturnsEmpty(t *testing.T) {
	events := []Event{
		{Type: "system.run.started", OccurredAt: time.Now(), Payload: json.RawMessage(`{}`)},
		{Type: "system.step.completed", OccurredAt: time.Now(), Payload: json.RawMessage(`{}`)},
	}
	got := ExtractFinalOutputFromEvents(events)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestExtractFinalOutputFromEvents_EmptyEventsReturnsEmpty(t *testing.T) {
	got := ExtractFinalOutputFromEvents(nil)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
