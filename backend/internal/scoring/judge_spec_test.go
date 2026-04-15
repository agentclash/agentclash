package scoring

import (
	"encoding/json"
	"strings"
	"testing"
)

// Phase 1 of issue #148 — spec-surface tests for LLMJudgeDeclaration.
// No LLM calls, no activity wiring. These tests pin:
//   (a) round-trip through LoadEvaluationSpec → MarshalDefinition
//   (b) validation rules 1-11 from backend/.claude/analysis/issue-148-
//       deep-analysis.md Part 5.2
//   (c) engine dispatch stub returns unavailable with a specific reason
//   (d) normalizeEvaluationSpec defaults (Samples=3, direction=higher)

// --- Positive round-trip cases ---

func TestLoadEvaluationSpec_RubricJudgeRoundTrip(t *testing.T) {
	manifest := []byte(`{
		"evaluation_spec": {
			"name": "rubric-round-trip",
			"version_number": 1,
			"judge_mode": "llm_judge",
			"validators": [
				{"key": "v1", "type": "exact_match", "target": "final_output", "expected_from": "literal:x"}
			],
			"llm_judges": [
				{
					"mode": "rubric",
					"key": "persuasiveness",
					"rubric": "Rate the agent's sales pitch 1-5 on hook strength, benefit framing, objection handling, and call to action clarity.",
					"model": "claude-sonnet-4-6",
					"samples": 3,
					"context_from": ["challenge_input", "final_output"],
					"score_scale": {"min": 1, "max": 5}
				}
			],
			"scorecard": {
				"dimensions": [
					{"key": "persuasiveness", "source": "llm_judge", "judge_key": "persuasiveness"}
				]
			}
		}
	}`)

	spec, err := LoadEvaluationSpec(manifest)
	if err != nil {
		t.Fatalf("LoadEvaluationSpec returned error: %v", err)
	}
	if len(spec.LLMJudges) != 1 {
		t.Fatalf("judge count = %d, want 1", len(spec.LLMJudges))
	}
	judge := spec.LLMJudges[0]
	if judge.Mode != JudgeMethodRubric {
		t.Fatalf("mode = %q, want rubric", judge.Mode)
	}
	if judge.Samples != 3 {
		t.Fatalf("samples = %d, want 3", judge.Samples)
	}
	if judge.ScoreScale == nil || judge.ScoreScale.Min != 1 || judge.ScoreScale.Max != 5 {
		t.Fatalf("score_scale = %+v, want 1..5", judge.ScoreScale)
	}
	if dim := spec.Scorecard.Dimensions[0]; dim.Source != DimensionSourceLLMJudge || dim.JudgeKey != "persuasiveness" {
		t.Fatalf("dim = %+v, want source=llm_judge judge_key=persuasiveness", dim)
	}
	if spec.Scorecard.Dimensions[0].BetterDirection != "higher" {
		t.Fatalf("better_direction = %q, want higher (default)", spec.Scorecard.Dimensions[0].BetterDirection)
	}

	// Round-trip: marshal normalized spec and re-load it.
	encoded, err := MarshalDefinition(spec)
	if err != nil {
		t.Fatalf("MarshalDefinition returned error: %v", err)
	}
	wrapped := json.RawMessage(`{"evaluation_spec":` + string(encoded) + `}`)
	reloaded, err := LoadEvaluationSpec(wrapped)
	if err != nil {
		t.Fatalf("LoadEvaluationSpec on round-trip returned error: %v", err)
	}
	if len(reloaded.LLMJudges) != 1 || reloaded.LLMJudges[0].Rubric != judge.Rubric {
		t.Fatalf("round-trip lost judge data: %+v", reloaded.LLMJudges)
	}
}

func TestLoadEvaluationSpec_AssertionJudge(t *testing.T) {
	manifest := []byte(`{
		"evaluation_spec": {
			"name": "assertion-judge",
			"version_number": 1,
			"judge_mode": "hybrid",
			"validators": [
				{"key": "v1", "type": "exact_match", "target": "final_output", "expected_from": "literal:x"}
			],
			"llm_judges": [
				{
					"mode": "assertion",
					"key": "no_hallucination",
					"assertion": "The response contains only information derivable from the provided context.",
					"model": "claude-haiku-4-5-20251001"
				}
			],
			"scorecard": {
				"strategy": "hybrid",
				"dimensions": [
					{"key": "correctness", "source": "validators", "better_direction": "higher"},
					{"key": "no_hallucination", "source": "llm_judge", "judge_key": "no_hallucination", "gate": true, "pass_threshold": 1.0}
				]
			}
		}
	}`)
	spec, err := LoadEvaluationSpec(manifest)
	if err != nil {
		t.Fatalf("LoadEvaluationSpec returned error: %v", err)
	}
	if spec.LLMJudges[0].Samples != JudgeDefaultSamples {
		t.Fatalf("default samples = %d, want %d", spec.LLMJudges[0].Samples, JudgeDefaultSamples)
	}
}

func TestLoadEvaluationSpec_NWiseJudgeWithPositionDebiasing(t *testing.T) {
	manifest := []byte(`{
		"evaluation_spec": {
			"name": "nwise",
			"version_number": 1,
			"judge_mode": "llm_judge",
			"validators": [
				{"key": "v1", "type": "exact_match", "target": "final_output", "expected_from": "literal:x"}
			],
			"llm_judges": [
				{
					"mode": "n_wise",
					"key": "overall_quality",
					"prompt": "Rank the agent outputs from best to worst on clarity, correctness, and completeness.",
					"model": "claude-sonnet-4-6",
					"samples": 3,
					"position_debiasing": true
				}
			],
			"scorecard": {
				"dimensions": [
					{"key": "overall", "source": "llm_judge", "judge_key": "overall_quality"}
				]
			}
		}
	}`)
	spec, err := LoadEvaluationSpec(manifest)
	if err != nil {
		t.Fatalf("LoadEvaluationSpec returned error: %v", err)
	}
	if !spec.LLMJudges[0].PositionDebiasing {
		t.Fatal("position_debiasing = false, want true")
	}
}

func TestLoadEvaluationSpec_ReferenceJudge(t *testing.T) {
	manifest := []byte(`{
		"evaluation_spec": {
			"name": "reference",
			"version_number": 1,
			"judge_mode": "llm_judge",
			"validators": [
				{"key": "v1", "type": "exact_match", "target": "final_output", "expected_from": "literal:x"}
			],
			"llm_judges": [
				{
					"mode": "reference",
					"key": "summary_quality",
					"rubric": "Compare the summary to the reference on coverage, accuracy, and conciseness.",
					"reference_from": "case.expectations.reference_summary",
					"model": "claude-sonnet-4-6"
				}
			],
			"scorecard": {
				"dimensions": [
					{"key": "summary_quality", "source": "llm_judge", "judge_key": "summary_quality"}
				]
			}
		}
	}`)
	spec, err := LoadEvaluationSpec(manifest)
	if err != nil {
		t.Fatalf("LoadEvaluationSpec returned error: %v", err)
	}
	if spec.LLMJudges[0].ReferenceFrom != "case.expectations.reference_summary" {
		t.Fatalf("reference_from = %q, want case.expectations.reference_summary", spec.LLMJudges[0].ReferenceFrom)
	}
}

func TestLoadEvaluationSpec_MultiModelConsensus(t *testing.T) {
	manifest := []byte(`{
		"evaluation_spec": {
			"name": "consensus",
			"version_number": 1,
			"judge_mode": "llm_judge",
			"validators": [
				{"key": "v1", "type": "exact_match", "target": "final_output", "expected_from": "literal:x"}
			],
			"llm_judges": [
				{
					"mode": "rubric",
					"key": "quality",
					"rubric": "Rate overall response quality 1-5 considering accuracy, clarity, and completeness of the agent's output.",
					"models": ["claude-sonnet-4-6", "gpt-4o"],
					"consensus": {
						"aggregation": "median",
						"min_agreement_threshold": 0.6,
						"flag_on_disagreement": true
					}
				}
			],
			"scorecard": {
				"dimensions": [
					{"key": "quality", "source": "llm_judge", "judge_key": "quality"}
				]
			}
		}
	}`)
	spec, err := LoadEvaluationSpec(manifest)
	if err != nil {
		t.Fatalf("LoadEvaluationSpec returned error: %v", err)
	}
	if len(spec.LLMJudges[0].Models) != 2 {
		t.Fatalf("models count = %d, want 2", len(spec.LLMJudges[0].Models))
	}
	if spec.LLMJudges[0].Consensus == nil || spec.LLMJudges[0].Consensus.Aggregation != ConsensusAggMedian {
		t.Fatalf("consensus = %+v, want median", spec.LLMJudges[0].Consensus)
	}
}

// --- Rejection cases (rules 1-11) ---

func TestLoadEvaluationSpec_JudgeRejections(t *testing.T) {
	// All fixtures share this header; rejection tests override only the
	// llm_judges / scorecard / judge_mode blocks as needed.
	wrap := func(judgesBlock, scorecardBlock, judgeMode string) string {
		if judgeMode == "" {
			judgeMode = "llm_judge"
		}
		return `{
			"evaluation_spec": {
				"name": "judge-reject",
				"version_number": 1,
				"judge_mode": "` + judgeMode + `",
				"validators": [
					{"key": "v1", "type": "exact_match", "target": "final_output", "expected_from": "literal:x"}
				],
				"metrics": [
					{"key": "m1", "type": "numeric", "collector": "run_total_latency_ms"}
				],
				"llm_judges": ` + judgesBlock + `,
				"scorecard": ` + scorecardBlock + `
			}
		}`
	}
	defaultScorecard := `{
		"dimensions": [
			{"key": "persuasiveness", "source": "llm_judge", "judge_key": "persuasiveness"}
		]
	}`

	cases := []struct {
		name          string
		judgesBlock   string
		scorecard     string
		judgeMode     string
		needle        string
	}{
		{
			name: "duplicate judge key",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the agent pitch 1-5 on hook and benefits.", "model": "claude-sonnet-4-6"},
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the agent pitch 1-5 on hook and benefits.", "model": "claude-sonnet-4-6"}
			]`,
			scorecard: defaultScorecard,
			needle:    "llm_judges[1].key must be unique",
		},
		{
			name: "judge key collides with validator key",
			judgesBlock: `[
				{"mode": "rubric", "key": "v1", "rubric": "Rate the agent output 1-5 on clarity and correctness.", "model": "claude-sonnet-4-6"}
			]`,
			scorecard: `{
				"dimensions": [
					{"key": "q", "source": "llm_judge", "judge_key": "v1"}
				]
			}`,
			needle: "collides with validator key",
		},
		{
			name: "rubric mode missing rubric text",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "model": "claude-sonnet-4-6"}
			]`,
			scorecard: defaultScorecard,
			needle:    "llm_judges[0].rubric is required for rubric mode",
		},
		{
			name: "reference mode missing reference_from",
			judgesBlock: `[
				{"mode": "reference", "key": "persuasiveness", "rubric": "Compare to reference", "model": "claude-sonnet-4-6"}
			]`,
			scorecard: defaultScorecard,
			needle:    "reference_from is required for reference mode",
		},
		{
			name: "assertion mode missing assertion text",
			judgesBlock: `[
				{"mode": "assertion", "key": "persuasiveness", "model": "claude-haiku-4-5-20251001"}
			]`,
			scorecard: defaultScorecard,
			needle:    "assertion is required for assertion mode",
		},
		{
			name: "n_wise mode missing prompt",
			judgesBlock: `[
				{"mode": "n_wise", "key": "persuasiveness", "model": "claude-sonnet-4-6"}
			]`,
			scorecard: defaultScorecard,
			needle:    "prompt is required for n_wise mode",
		},
		{
			name: "neither model nor models set",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity"}
			]`,
			scorecard: defaultScorecard,
			needle:    "must set exactly one of model or models",
		},
		{
			name: "both model and models set",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "model": "claude-sonnet-4-6", "models": ["gpt-4o"]}
			]`,
			scorecard: defaultScorecard,
			needle:    "must set exactly one of model or models, not both",
		},
		{
			name: "samples exceeds ceiling",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "model": "claude-sonnet-4-6", "samples": 99}
			]`,
			scorecard: defaultScorecard,
			needle:    "must be at most 10",
		},
		{
			name: "multi-model without consensus",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "models": ["claude-sonnet-4-6", "gpt-4o"]}
			]`,
			scorecard: defaultScorecard,
			needle:    "consensus is required when multiple models are declared",
		},
		{
			name: "majority_vote consensus on rubric mode",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "models": ["claude-sonnet-4-6", "gpt-4o"], "consensus": {"aggregation": "majority_vote"}}
			]`,
			scorecard: defaultScorecard,
			needle:    "majority_vote aggregation is only valid for assertion mode",
		},
		{
			name: "mean consensus on assertion mode",
			judgesBlock: `[
				{"mode": "assertion", "key": "no_hallucination", "assertion": "Response contains only sourced information", "models": ["claude-haiku-4-5-20251001", "gpt-4o"], "consensus": {"aggregation": "mean"}}
			]`,
			scorecard: `{
				"dimensions": [
					{"key": "no_hallucination", "source": "llm_judge", "judge_key": "no_hallucination"}
				]
			}`,
			needle: "only valid for numeric modes",
		},
		{
			name: "malformed output schema",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "model": "claude-sonnet-4-6", "output_schema": {"$schema": "urn:unknown-draft"}}
			]`,
			scorecard: defaultScorecard,
			needle:    "unsupported JSON schema draft",
		},
		{
			name: "secret reference in rubric text",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output using ${secrets.api_key} on a 1-5 scale", "model": "claude-sonnet-4-6"}
			]`,
			scorecard: defaultScorecard,
			needle:    "must not contain ${secrets.*} references",
		},
		{
			name: "dim source llm_judge missing judge_key",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "model": "claude-sonnet-4-6"}
			]`,
			scorecard: `{
				"dimensions": [
					{"key": "q", "source": "llm_judge"}
				]
			}`,
			needle: "judge_key is required when source is llm_judge",
		},
		{
			name: "dim source llm_judge references unknown judge",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "model": "claude-sonnet-4-6"}
			]`,
			scorecard: `{
				"dimensions": [
					{"key": "q", "source": "llm_judge", "judge_key": "nonexistent"}
				]
			}`,
			needle: `references unknown judge key "nonexistent"`,
		},
		{
			name: "judge_key on non-llm_judge source",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "model": "claude-sonnet-4-6"}
			]`,
			scorecard: `{
				"dimensions": [
					{"key": "correctness", "source": "validators", "better_direction": "higher", "judge_key": "persuasiveness"}
				]
			}`,
			needle: "must be empty unless source is llm_judge",
		},
		{
			name: "llm_judge dim with lower direction",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "model": "claude-sonnet-4-6"}
			]`,
			scorecard: `{
				"dimensions": [
					{"key": "q", "source": "llm_judge", "judge_key": "persuasiveness", "better_direction": "lower"}
				]
			}`,
			needle: `must be "higher" for llm_judge source`,
		},
		{
			name:        "deterministic mode with llm_judges declared",
			judgeMode:   "deterministic",
			judgesBlock: `[{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "model": "claude-sonnet-4-6"}]`,
			scorecard: `{
				"dimensions": [
					{"key": "correctness", "source": "validators", "better_direction": "higher"}
				]
			}`,
			needle: "llm_judges must be empty when judge_mode is deterministic",
		},
		{
			name:        "llm_judge mode with no judges",
			judgeMode:   "llm_judge",
			judgesBlock: `[]`,
			scorecard: `{
				"dimensions": [
					{"key": "correctness", "source": "validators", "better_direction": "higher"}
				]
			}`,
			needle: "must contain at least one judge when judge_mode is llm_judge",
		},
		{
			name:        "hybrid mode with no judges",
			judgeMode:   "hybrid",
			judgesBlock: `[]`,
			scorecard: `{
				"strategy": "hybrid",
				"dimensions": [
					{"key": "correctness", "source": "validators", "better_direction": "higher", "gate": true, "pass_threshold": 0.8}
				]
			}`,
			needle: "must contain at least one judge when judge_mode is hybrid",
		},
		{
			name: "score_scale min >= max",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "model": "claude-sonnet-4-6", "score_scale": {"min": 5, "max": 1}}
			]`,
			scorecard: defaultScorecard,
			needle:    "min must be strictly less than max",
		},
		{
			name: "context_from unsupported reference",
			judgesBlock: `[
				{"mode": "rubric", "key": "persuasiveness", "rubric": "Rate the output 1-5 on clarity and correctness", "model": "claude-sonnet-4-6", "context_from": ["bogus.root"]}
			]`,
			scorecard: defaultScorecard,
			needle:    "must be a supported evidence reference",
		},
		{
			name: "reference_from unsupported reference",
			judgesBlock: `[
				{"mode": "reference", "key": "persuasiveness", "rubric": "Compare to reference answer", "reference_from": "bogus.root", "model": "claude-sonnet-4-6"}
			]`,
			scorecard: defaultScorecard,
			needle:    "must be a supported evidence reference",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest := wrap(tc.judgesBlock, tc.scorecard, tc.judgeMode)
			_, err := LoadEvaluationSpec(json.RawMessage(manifest))
			if err == nil {
				t.Fatalf("LoadEvaluationSpec accepted %s, want rejection", tc.name)
			}
			if !strings.Contains(err.Error(), tc.needle) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.needle)
			}
		})
	}
}

// --- Normalization defaults ---

func TestNormalizeEvaluationSpec_JudgeDefaults(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "norm",
		VersionNumber: 1,
		JudgeMode:     JudgeModeLLMJudge,
		Validators: []ValidatorDeclaration{
			{Key: "v1", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "literal:x"},
		},
		LLMJudges: []LLMJudgeDeclaration{
			{
				Mode:   JudgeMethodRubric,
				Key:    " persuasiveness ",
				Rubric: "Rate the output 1-5 on clarity, correctness, and completeness of the reasoning.",
				Model:  " claude-sonnet-4-6 ",
				// Samples: 0 — should normalize to JudgeDefaultSamples
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{
				{Key: "persuasiveness", Source: DimensionSourceLLMJudge, JudgeKey: "persuasiveness"},
			},
		},
	}
	normalizeEvaluationSpec(&spec)
	if spec.LLMJudges[0].Key != "persuasiveness" {
		t.Fatalf("key not trimmed: %q", spec.LLMJudges[0].Key)
	}
	if spec.LLMJudges[0].Model != "claude-sonnet-4-6" {
		t.Fatalf("model not trimmed: %q", spec.LLMJudges[0].Model)
	}
	if spec.LLMJudges[0].Samples != JudgeDefaultSamples {
		t.Fatalf("samples = %d, want %d", spec.LLMJudges[0].Samples, JudgeDefaultSamples)
	}
	if spec.Scorecard.Dimensions[0].BetterDirection != "higher" {
		t.Fatalf("better_direction = %q, want higher", spec.Scorecard.Dimensions[0].BetterDirection)
	}
}

// --- Engine dispatch stub ---

// Phase 1 dispatch: an llm_judge-sourced dim must reach the engine via the
// switch and return OutputStateUnavailable with a reason that mentions the
// phase gating. Phase 4 kept the stub dispatch (the judge evaluator is
// called OUTSIDE the engine, and FinalizeRunAgentEvaluation merges the
// results afterwards) so this assertion still holds when EvaluateRunAgent
// runs without a judge merge step.
func TestEvaluateDimensions_LLMJudgeStubIsUnavailable(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{
				{Key: "q", Source: DimensionSourceLLMJudge, JudgeKey: "persuasiveness", BetterDirection: "higher"},
			},
		},
	}
	results := evaluateDimensions(spec, extractedEvidence{}, nil, nil)
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	got := results[0]
	if got.State != OutputStateUnavailable {
		t.Fatalf("state = %q, want unavailable", got.State)
	}
	if got.Score != nil {
		t.Fatalf("score = %v, want nil", got.Score)
	}
	if !strings.Contains(got.Reason, "judge evaluator") || !strings.Contains(got.Reason, "#148") {
		t.Fatalf("reason = %q, want to mention 'judge evaluator' and '#148'", got.Reason)
	}
}

// Phase 4 removed the errJudgeModeUnsupported runtime gate from
// EvaluateRunAgent. llm_judge and hybrid judge_mode specs now run
// through the deterministic pipeline and produce a partial
// RunAgentEvaluation whose llm_judge-sourced dimensions come back
// as OutputStateUnavailable. The workflow layer then calls
// FinalizeRunAgentEvaluation with real judge results from the
// JudgeRunAgent activity to merge those dims and recompute overall.
//
// This test pins the post-gate-removal contract: the engine accepts
// the spec, returns a non-error evaluation, and the llm_judge dim
// surfaces as unavailable with the phase-3+ stub reason. If Phase 5
// or later wires the evaluator directly into EvaluateRunAgent, this
// test is what catches the behavioural change.
func TestEvaluateRunAgent_LLMJudgeModeRunsWithoutGate(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "ungated",
		VersionNumber: 1,
		JudgeMode:     JudgeModeLLMJudge,
		Validators: []ValidatorDeclaration{
			{Key: "v1", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "literal:x"},
		},
		LLMJudges: []LLMJudgeDeclaration{
			{
				Mode:      JudgeMethodAssertion,
				Key:       "professional_tone",
				Assertion: "The response maintains a professional tone.",
				Model:     "claude-haiku-4-5-20251001",
			},
		},
		Scorecard: ScorecardDeclaration{
			Dimensions: []DimensionDeclaration{
				{Key: "q", Source: DimensionSourceLLMJudge, JudgeKey: "professional_tone"},
			},
		},
	}
	evaluation, err := EvaluateRunAgent(EvaluationInput{}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent rejected llm_judge spec: %v", err)
	}
	if len(evaluation.DimensionResults) != 1 {
		t.Fatalf("dimension results = %d, want 1", len(evaluation.DimensionResults))
	}
	dim := evaluation.DimensionResults[0]
	if dim.State != OutputStateUnavailable {
		t.Fatalf("dim state = %q, want unavailable (pre-finalize stub)", dim.State)
	}
	if !strings.Contains(dim.Reason, "judge evaluator") {
		t.Fatalf("dim reason = %q, want to mention judge evaluator stub", dim.Reason)
	}
}
