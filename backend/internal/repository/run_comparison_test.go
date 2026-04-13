package repository

import (
	"math"
	"testing"
)

// Phase 4: the legacy 4-dim hardcoded comparison map is replaced by a walk
// over the union of baseline + candidate scorecard dimension keys. A custom
// "safety" dim declared in both scorecards must surface in the deltas using
// the direction it carries on the scorecard itself.
func TestBuildRunComparisonDimensionDeltasEmitsUnionOfKeys(t *testing.T) {
	baselineDims := map[string]comparisonScorecardDimensionInfo{
		"correctness": {State: "available", Score: float64Ptr(0.80), BetterDirection: "higher"},
		"safety":      {State: "available", Score: float64Ptr(0.40), BetterDirection: "higher"},
	}
	candidateDims := map[string]comparisonScorecardDimensionInfo{
		"correctness": {State: "available", Score: float64Ptr(0.90), BetterDirection: "higher"},
		"safety":      {State: "available", Score: float64Ptr(0.70), BetterDirection: "higher"},
	}
	baselineSc := &RunAgentScorecard{CorrectnessScore: float64Ptr(0.80)}
	candidateSc := &RunAgentScorecard{CorrectnessScore: float64Ptr(0.90)}

	missingFields := make([]string, 0)
	deltas := buildRunComparisonDimensionDeltas(baselineSc, candidateSc, baselineDims, candidateDims, &missingFields)

	safety, ok := deltas["safety"]
	if !ok {
		t.Fatalf("safety delta missing; keys = %v", deltaKeys(deltas))
	}
	if safety.State != "available" {
		t.Fatalf("safety.state = %q, want available", safety.State)
	}
	if safety.BetterDirection != "higher" {
		t.Fatalf("safety.better_direction = %q, want higher", safety.BetterDirection)
	}
	if safety.Delta == nil || math.Abs(*safety.Delta-0.30) > 1e-9 {
		t.Fatalf("safety.delta = %v, want 0.30", safety.Delta)
	}
	if _, ok := deltas["correctness"]; !ok {
		t.Fatalf("correctness delta missing; keys = %v", deltaKeys(deltas))
	}
	if len(missingFields) != 0 {
		t.Fatalf("missing_fields = %v, want empty", missingFields)
	}
}

// When a dim exists only on one side, the delta must surface a distinct
// "missing_baseline" or "missing_candidate" state so operators can tell the
// difference between "both sides skipped it" and "the other side added a new
// dim".
func TestBuildRunComparisonDimensionDeltasFlagsSideMissingDimensions(t *testing.T) {
	baselineDims := map[string]comparisonScorecardDimensionInfo{
		"safety": {State: "available", Score: float64Ptr(0.50), BetterDirection: "higher"},
	}
	candidateDims := map[string]comparisonScorecardDimensionInfo{
		"robustness": {State: "available", Score: float64Ptr(0.80), BetterDirection: "higher"},
	}
	baselineSc := &RunAgentScorecard{}
	candidateSc := &RunAgentScorecard{}

	missingFields := make([]string, 0)
	deltas := buildRunComparisonDimensionDeltas(baselineSc, candidateSc, baselineDims, candidateDims, &missingFields)

	if deltas["safety"].State != "missing_candidate" {
		t.Fatalf("safety.state = %q, want missing_candidate", deltas["safety"].State)
	}
	if deltas["robustness"].State != "missing_baseline" {
		t.Fatalf("robustness.state = %q, want missing_baseline", deltas["robustness"].State)
	}
	if deltas["safety"].Delta != nil {
		t.Fatalf("safety.delta = %v, want nil", deltas["safety"].Delta)
	}
	if deltas["robustness"].Delta != nil {
		t.Fatalf("robustness.delta = %v, want nil", deltas["robustness"].Delta)
	}
	if len(missingFields) != 2 {
		t.Fatalf("missing_fields = %v, want 2 entries", missingFields)
	}
}

// Built-in dims still read from the typed scorecard column when the JSONB
// lacks a score — this is the safety net for pre-Phase-3 rows that persisted
// dim values only in the typed columns.
func TestBuildRunComparisonDimensionDeltasFallsBackToTypedColumns(t *testing.T) {
	baselineDims := map[string]comparisonScorecardDimensionInfo{
		"correctness": {State: "available", BetterDirection: "higher"},
	}
	candidateDims := map[string]comparisonScorecardDimensionInfo{
		"correctness": {State: "available", BetterDirection: "higher"},
	}
	baselineSc := &RunAgentScorecard{CorrectnessScore: float64Ptr(0.40)}
	candidateSc := &RunAgentScorecard{CorrectnessScore: float64Ptr(0.80)}

	missingFields := make([]string, 0)
	deltas := buildRunComparisonDimensionDeltas(baselineSc, candidateSc, baselineDims, candidateDims, &missingFields)

	correctness := deltas["correctness"]
	if correctness.State != "available" {
		t.Fatalf("correctness.state = %q, want available", correctness.State)
	}
	if correctness.Delta == nil || math.Abs(*correctness.Delta-0.40) > 1e-9 {
		t.Fatalf("correctness.delta = %v, want 0.40", correctness.Delta)
	}
}

func deltaKeys(m map[string]runComparisonDelta) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
