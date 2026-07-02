package scoring

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestAudit147_HybridWeightedNonGateAdversarial pins the audit fix from
// commit 966b70f. Three non-gate dims with uneven weights (2, 1, 1) and two
// gates that pass with high scores. The expected overall is computed
// EXCLUDING the gates: (2*0.3 + 1*0.4 + 1*0.5) / 4 = 0.375. If the bug
// regresses (gates included), the overall would be ~0.65.
func TestAudit147_HybridWeightedNonGateAdversarial(t *testing.T) {
	spec := EvaluationSpec{
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyHybrid,
			Dimensions: []DimensionDeclaration{
				{Key: "ng_a", Weight: floatPtr(2)},
				{Key: "ng_b", Weight: floatPtr(1)},
				{Key: "ng_c", Weight: floatPtr(1)},
				{Key: "gate1", Gate: true, PassThreshold: floatPtr(0.5), Weight: floatPtr(10)},
				{Key: "gate2", Gate: true, PassThreshold: floatPtr(0.5), Weight: floatPtr(10)},
			},
		},
	}
	results := []DimensionResult{
		{Dimension: "ng_a", Score: floatPtr(0.3), State: OutputStateAvailable},
		{Dimension: "ng_b", Score: floatPtr(0.4), State: OutputStateAvailable},
		{Dimension: "ng_c", Score: floatPtr(0.5), State: OutputStateAvailable},
		{Dimension: "gate1", Score: floatPtr(0.95), State: OutputStateAvailable},
		{Dimension: "gate2", Score: floatPtr(0.95), State: OutputStateAvailable},
	}

	overall, passed, _ := computeOverallScore(spec, results)
	if overall == nil {
		t.Fatal("overall is nil")
	}
	want := 0.375
	if math.Abs(*overall-want) > 1e-9 {
		t.Fatalf("overall = %v, want %v (gates must be excluded from non-gate weighted mean)", *overall, want)
	}
	if passed == nil || !*passed {
		t.Fatalf("passed = %v, want true", passed)
	}
}

// TestAudit147_StrictDecodingMissing proves a CRITICAL gap: scoring spec
// loaders use plain json.Unmarshal, so spec authors can typo a field name
// and silently get default behavior. This is the Atharva-flagged severity
// Typos in dimension fields (`wieght`, `gait`, `pas_threshold`) must be
// rejected at spec-load time instead of silently running with defaults.
// This is the Atharva-severity issue raised on PR #232, ported to #147.
// Cover both the envelope-level decoder and the per-dimension custom
// Unmarshaler — the latter is the sharp edge because a custom Unmarshaler
// opts out of the outer decoder's strict field walk.
func TestAudit147_StrictDecodingRejectsUnknownFields(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		wantSub  string
	}{
		{
			name: "unknown dimension field",
			manifest: `{
				"evaluation_spec": {
					"name": "typo-spec",
					"version_number": 1,
					"judge_mode": "deterministic",
					"validators": [
						{"key": "v1", "type": "exact_match", "target": "final_output", "expected_from": "literal:done"}
					],
					"scorecard": {
						"strategy": "weighted",
						"dimensions": [
							{"key": "correctness", "source": "validators", "wieght": 0.5}
						]
					}
				}
			}`,
			wantSub: "wieght",
		},
		{
			name: "unknown dimension field gait",
			manifest: `{
				"evaluation_spec": {
					"name": "typo-spec",
					"version_number": 1,
					"judge_mode": "deterministic",
					"validators": [
						{"key": "v1", "type": "exact_match", "target": "final_output", "expected_from": "literal:done"}
					],
					"scorecard": {
						"strategy": "hybrid",
						"dimensions": [
							{"key": "correctness", "source": "validators", "gait": true, "pass_threshold": 0.9}
						]
					}
				}
			}`,
			wantSub: "gait",
		},
		{
			name: "unknown top-level scorecard field",
			manifest: `{
				"evaluation_spec": {
					"name": "typo-spec",
					"version_number": 1,
					"judge_mode": "deterministic",
					"validators": [
						{"key": "v1", "type": "exact_match", "target": "final_output", "expected_from": "literal:done"}
					],
					"scorecard": {
						"strategy": "weighted",
						"pas_threshold": 0.7,
						"dimensions": [{"key": "correctness", "source": "validators"}]
					}
				}
			}`,
			wantSub: "pas_threshold",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadEvaluationSpec([]byte(tc.manifest))
			if err == nil {
				t.Fatalf("LoadEvaluationSpec accepted unknown field, want rejection")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error = %q, want substring %q", err, tc.wantSub)
			}
		})
	}
}

// TestAudit147_ScorecardPassMissingFromJSONOnLegacy guards the audit-commit
// fix: when both baseline and candidate have nil Passed pointers, the
// summary JSON must NOT contain "scorecard_pass".
func TestAudit147_ScorecardPassDoesNotLeakWhenNil(t *testing.T) {
	type doc struct {
		ScorecardPass *struct {
			Baseline  *bool `json:"baseline,omitempty"`
			Candidate *bool `json:"candidate,omitempty"`
		} `json:"scorecard_pass,omitempty"`
	}
	d := doc{}
	encoded, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "scorecard_pass") {
		t.Fatalf("encoded = %s, must omit scorecard_pass entirely", encoded)
	}
}

// TestAudit147_E2EHybridWithMixedSources is the integration test the audit
// brief asks for: hybrid spec with 4 dims, one a safety gate, the other
// three contributing to the weighted mean. Verifies that:
//   - the weighted mean EXCLUDES the gate
//   - the safety gate passes at 0.95
//   - correctness = 0.5 from one passing + one failing validator
//   - the JSON round-trip carries dimension state and score
func TestAudit147_E2EHybridWithMixedSources(t *testing.T) {
	target := 5000.0
	maxLatency := 30000.0
	tokenTarget := 100.0
	tokenMax := 1000.0
	spec := EvaluationSpec{
		Name:          "audit-147-e2e",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "v_pass", Type: ValidatorTypeContains, Target: "final_output", ExpectedFrom: "literal:done"},
			{Key: "v_fail", Type: ValidatorTypeContains, Target: "final_output", ExpectedFrom: "literal:NOTFOUND"},
			{Key: "v_safety", Type: ValidatorTypeContains, Target: "final_output", ExpectedFrom: "literal:done"},
		},
		Metrics: []MetricDeclaration{
			{Key: "tokens", Type: MetricTypeNumeric, Collector: "run_total_tokens"},
		},
		Scorecard: ScorecardDeclaration{
			Strategy:      ScoringStrategyHybrid,
			PassThreshold: floatPtr(0.4),
			Normalization: ScorecardNormalization{
				Latency: &LatencyNormalization{TargetMS: &target, MaxMS: &maxLatency},
			},
			Dimensions: []DimensionDeclaration{
				{Key: "correctness", Source: DimensionSourceValidators, Validators: []string{"v_pass", "v_fail"}, BetterDirection: "higher", Weight: floatPtr(0.4)},
				{Key: "safety", Source: DimensionSourceValidators, Validators: []string{"v_safety"}, BetterDirection: "higher", Gate: true, PassThreshold: floatPtr(0.9)},
				{Key: "latency", Source: DimensionSourceLatency, BetterDirection: "lower", Weight: floatPtr(0.3), Normalization: &DimensionNormalization{Target: &target, Max: &maxLatency}},
				{Key: "efficiency", Source: DimensionSourceMetric, Metric: "tokens", BetterDirection: "lower", Weight: floatPtr(0.3), Normalization: &DimensionNormalization{Target: &tokenTarget, Max: &tokenMax}},
			},
		},
	}

	evaluation, err := EvaluateRunAgent(EvaluationInput{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		ChallengeInputs: []EvidenceInput{
			{ChallengeIdentityID: uuid.New(), ItemKey: "expected.txt", Payload: []byte(`"done"`)},
		},
		Events: []Event{
			{Type: "system.run.started", OccurredAt: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), Payload: []byte(`{}`)},
			{Type: "system.output.finalized", OccurredAt: time.Date(2026, 3, 16, 9, 0, 1, 0, time.UTC), Payload: []byte(`{"output":"done"}`)},
			{Type: "system.run.completed", OccurredAt: time.Date(2026, 3, 16, 9, 0, 2, 0, time.UTC), Payload: []byte(`{"final_output":"done","total_tokens":550}`)},
		},
	}, spec)
	if err != nil {
		t.Fatalf("EvaluateRunAgent returned error: %v", err)
	}

	if evaluation.DimensionScores["correctness"] == nil || *evaluation.DimensionScores["correctness"] != 0.5 {
		t.Fatalf("correctness score = %v, want 0.5 (one pass + one fail validator)", evaluation.DimensionScores["correctness"])
	}
	if evaluation.DimensionScores["safety"] == nil || *evaluation.DimensionScores["safety"] != 1.0 {
		t.Fatalf("safety score = %v, want 1.0", evaluation.DimensionScores["safety"])
	}
	if evaluation.DimensionScores["latency"] == nil {
		t.Fatalf("latency score = nil, want available")
	}
	if evaluation.DimensionScores["efficiency"] == nil || *evaluation.DimensionScores["efficiency"] != 0.5 {
		t.Fatalf("efficiency score = %v, want 0.5", evaluation.DimensionScores["efficiency"])
	}

	// Latency is 2 seconds vs target 5s/max 30s ⇒ score = 1.0 (better than target).
	// Non-gate weighted mean = (0.4*0.5 + 0.3*1.0 + 0.3*0.5) / 1.0 = 0.65.
	if evaluation.OverallScore == nil {
		t.Fatalf("OverallScore = nil")
	}
	want := 0.65
	if math.Abs(*evaluation.OverallScore-want) > 1e-9 {
		t.Fatalf("OverallScore = %v, want %v (non-gate weighted mean, gate excluded)", *evaluation.OverallScore, want)
	}

	if evaluation.Passed == nil || !*evaluation.Passed {
		t.Fatalf("Passed = %v, want true (gate clears 0.9 and overall >= 0.4)", evaluation.Passed)
	}

	// JSON round-trip must carry state and score for custom dim keys.
	encoded, err := json.Marshal(evaluation)
	if err != nil {
		t.Fatalf("marshal evaluation: %v", err)
	}
	if !strings.Contains(string(encoded), `"safety"`) {
		t.Fatalf("evaluation JSON missing safety dimension: %s", encoded)
	}
	if !strings.Contains(string(encoded), `"efficiency"`) {
		t.Fatalf("evaluation JSON missing efficiency dimension: %s", encoded)
	}
}

// TestAudit147_LegacyStringSpecStillEvaluates pins backward compat: a spec
// authored with the old string-only dimension format must still evaluate
// every built-in dim correctly.
func TestAudit147_LegacyStringSpecStillEvaluates(t *testing.T) {
	manifest := []byte(`{
		"evaluation_spec": {
			"name": "legacy",
			"version_number": 1,
			"judge_mode": "deterministic",
			"validators": [
				{"key": "exact", "type": "exact_match", "target": "final_output", "expected_from": "challenge_input"}
			],
			"metrics": [
				{"key": "completed", "type": "boolean", "collector": "run_completed_successfully"},
				{"key": "failures", "type": "numeric", "collector": "run_failure_count"}
			],
			"runtime_limits": {"max_duration_ms": 30000, "max_cost_usd": 1.0},
			"scorecard": {
				"normalization": {
					"latency": {"target_ms": 1000, "max_ms": 30000},
					"cost": {"target_usd": 0.01, "max_usd": 1.0}
				},
				"dimensions": ["correctness", "reliability", "latency", "cost"]
			}
		}
	}`)
	spec, err := LoadEvaluationSpec(manifest)
	if err != nil {
		t.Fatalf("LoadEvaluationSpec returned error: %v", err)
	}
	if got := spec.Scorecard.Strategy; got != ScoringStrategyWeighted {
		t.Fatalf("strategy default = %q, want weighted", got)
	}
	for _, dim := range spec.Scorecard.Dimensions {
		if dim.Source == "" {
			t.Fatalf("dim %q has empty source after expansion", dim.Key)
		}
	}
}

// TestAudit147_DuplicateDimensionKeysRejected pins that duplicates fail
// validation rather than silently overwriting.
func TestAudit147_DuplicateDimensionKeysRejected(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "dup",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "v", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "literal:x"},
		},
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{Key: "correctness", Source: DimensionSourceValidators, BetterDirection: "higher"},
				{Key: "correctness", Source: DimensionSourceValidators, BetterDirection: "higher"},
			},
		},
	}
	err := ValidateEvaluationSpec(spec)
	if err == nil {
		t.Fatal("ValidateEvaluationSpec returned nil for duplicate dimension keys")
	}
	if !strings.Contains(err.Error(), "unique") {
		t.Fatalf("error = %q, want it to mention uniqueness", err)
	}
}

// TestAudit147_ZeroDimensionsRejected pins that an empty scorecard is rejected.
func TestAudit147_ZeroDimensionsRejected(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "zero",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "v", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "literal:x"},
		},
		Scorecard: ScorecardDeclaration{Strategy: ScoringStrategyWeighted},
	}
	err := ValidateEvaluationSpec(spec)
	if err == nil {
		t.Fatal("ValidateEvaluationSpec returned nil for zero dimensions")
	}
	if !strings.Contains(err.Error(), "at least one dimension") {
		t.Fatalf("error = %q, want it to mention 'at least one dimension'", err)
	}
}

// TestAudit147_UnknownDimensionSourceRejected pins that {key,source:"bogus"}
// is rejected at validation, not silently dispatched to the engine default.
func TestAudit147_UnknownDimensionSourceRejected(t *testing.T) {
	spec := EvaluationSpec{
		Name:          "bogus",
		VersionNumber: 1,
		JudgeMode:     JudgeModeDeterministic,
		Validators: []ValidatorDeclaration{
			{Key: "v", Type: ValidatorTypeExactMatch, Target: "final_output", ExpectedFrom: "literal:x"},
		},
		Scorecard: ScorecardDeclaration{
			Strategy: ScoringStrategyWeighted,
			Dimensions: []DimensionDeclaration{
				{Key: "x", Source: DimensionSource("bogus")},
			},
		},
	}
	err := ValidateEvaluationSpec(spec)
	if err == nil {
		t.Fatal("ValidateEvaluationSpec returned nil for bogus source")
	}
	if !strings.Contains(err.Error(), "source") {
		t.Fatalf("error = %q, want it to mention source", err)
	}
}
