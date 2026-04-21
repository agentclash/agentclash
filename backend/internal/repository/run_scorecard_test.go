package repository

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
)

func TestBuildRunScorecardDocumentSelectsWinnerByCorrectnessThenReliability(t *testing.T) {
	runID := uuid.New()
	evaluationSpecID := uuid.New()
	firstAgentID := uuid.New()
	secondAgentID := uuid.New()

	document, winningRunAgentID, err := buildRunScorecardDocument(runID, evaluationSpecID, []runScorecardParticipant{
		scorecardParticipantFixture(0, "baseline", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       firstAgentID,
			EvaluationSpecID: evaluationSpecID,
			CorrectnessScore: float64Ptr(0.9),
			ReliabilityScore: float64Ptr(0.7),
		}),
		scorecardParticipantFixture(1, "candidate", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       secondAgentID,
			EvaluationSpecID: evaluationSpecID,
			CorrectnessScore: float64Ptr(0.9),
			ReliabilityScore: float64Ptr(0.8),
		}),
	}, nil)
	if err != nil {
		t.Fatalf("buildRunScorecardDocument returned error: %v", err)
	}
	if winningRunAgentID == nil || *winningRunAgentID != secondAgentID {
		t.Fatalf("winning run agent id = %v, want %s", winningRunAgentID, secondAgentID)
	}

	var decoded runScorecardDocument
	if err := json.Unmarshal(document, &decoded); err != nil {
		t.Fatalf("unmarshal run scorecard document: %v", err)
	}
	if decoded.WinnerDetermination.ReasonCode != "reliability_tiebreaker" {
		t.Fatalf("winner reason code = %q, want reliability_tiebreaker", decoded.WinnerDetermination.ReasonCode)
	}
	if decoded.DimensionDeltas["correctness"].Delta == nil || *decoded.DimensionDeltas["correctness"].Delta != 0 {
		t.Fatalf("correctness delta = %v, want 0", decoded.DimensionDeltas["correctness"].Delta)
	}
	if decoded.DimensionDeltas["reliability"].Delta == nil || math.Abs(*decoded.DimensionDeltas["reliability"].Delta-0.1) > 1e-9 {
		t.Fatalf("reliability delta = %v, want 0.1", decoded.DimensionDeltas["reliability"].Delta)
	}
}

func TestBuildRunAgentScorecardDocumentIncludesRegressionCaseIDs(t *testing.T) {
	runAgentID := uuid.New()
	evaluationSpecID := uuid.New()
	regressionCaseID := uuid.New()

	document, err := buildRunAgentScorecardDocument(scoring.RunAgentEvaluation{
		RunAgentID:       runAgentID,
		EvaluationSpecID: evaluationSpecID,
		Status:           scoring.EvaluationStatusComplete,
		ValidatorResults: []scoring.ValidatorResult{
			{
				Key:              "exact",
				Type:             scoring.ValidatorTypeExactMatch,
				Verdict:          "pass",
				State:            scoring.OutputStateAvailable,
				RegressionCaseID: &regressionCaseID,
			},
		},
		MetricResults: []scoring.MetricResult{
			{
				Key:              "total_tokens",
				Collector:        "run_total_tokens",
				State:            scoring.OutputStateAvailable,
				RegressionCaseID: &regressionCaseID,
			},
		},
		DimensionResults: []scoring.DimensionResult{
			{
				Dimension: "correctness",
				State:     scoring.OutputStateAvailable,
				Score:     float64Ptr(1),
			},
		},
	})
	if err != nil {
		t.Fatalf("buildRunAgentScorecardDocument returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(document, &decoded); err != nil {
		t.Fatalf("unmarshal scorecard document: %v", err)
	}

	validatorDetails := decoded["validator_details"].([]any)
	if validatorDetails[0].(map[string]any)["regression_case_id"] != regressionCaseID.String() {
		t.Fatalf("validator regression_case_id = %#v, want %s", validatorDetails[0], regressionCaseID)
	}

	metricDetails := decoded["metric_details"].([]any)
	if metricDetails[0].(map[string]any)["regression_case_id"] != regressionCaseID.String() {
		t.Fatalf("metric regression_case_id = %#v, want %s", metricDetails[0], regressionCaseID)
	}
}

func TestBuildRunScorecardDocumentMarksSingleAgentAsTrivialWinner(t *testing.T) {
	runID := uuid.New()
	evaluationSpecID := uuid.New()
	runAgentID := uuid.New()

	_, winningRunAgentID, err := buildRunScorecardDocument(runID, evaluationSpecID, []runScorecardParticipant{
		scorecardParticipantFixture(0, "solo", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       runAgentID,
			EvaluationSpecID: evaluationSpecID,
			CorrectnessScore: float64Ptr(0.65),
		}),
	}, nil)
	if err != nil {
		t.Fatalf("buildRunScorecardDocument returned error: %v", err)
	}
	if winningRunAgentID == nil || *winningRunAgentID != runAgentID {
		t.Fatalf("winning run agent id = %v, want %s", winningRunAgentID, runAgentID)
	}
}

func TestBuildRunScorecardDocumentLeavesWinnerUnsetForTie(t *testing.T) {
	runID := uuid.New()
	evaluationSpecID := uuid.New()

	document, winningRunAgentID, err := buildRunScorecardDocument(runID, evaluationSpecID, []runScorecardParticipant{
		scorecardParticipantFixture(0, "left", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       uuid.New(),
			EvaluationSpecID: evaluationSpecID,
			CorrectnessScore: float64Ptr(0.9),
			ReliabilityScore: float64Ptr(0.8),
		}),
		scorecardParticipantFixture(1, "right", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       uuid.New(),
			EvaluationSpecID: evaluationSpecID,
			CorrectnessScore: float64Ptr(0.9),
			ReliabilityScore: float64Ptr(0.8),
		}),
	}, nil)
	if err != nil {
		t.Fatalf("buildRunScorecardDocument returned error: %v", err)
	}
	if winningRunAgentID != nil {
		t.Fatalf("winning run agent id = %v, want nil", *winningRunAgentID)
	}

	var decoded runScorecardDocument
	if err := json.Unmarshal(document, &decoded); err != nil {
		t.Fatalf("unmarshal run scorecard document: %v", err)
	}
	if decoded.WinnerDetermination.Status != "tie" {
		t.Fatalf("winner status = %q, want tie", decoded.WinnerDetermination.Status)
	}
}

func scorecardParticipantFixture(
	laneIndex int32,
	label string,
	status domain.RunAgentStatus,
	fixture runAgentScorecardFixture,
) runScorecardParticipant {
	runAgent := domain.RunAgent{
		ID:        fixture.RunAgentID,
		RunID:     uuid.New(),
		LaneIndex: laneIndex,
		Label:     label,
		Status:    status,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	scorecard := RunAgentScorecard{
		ID:               uuid.New(),
		RunAgentID:       fixture.RunAgentID,
		EvaluationSpecID: fixture.EvaluationSpecID,
		OverallScore:     fixture.OverallScore,
		CorrectnessScore: fixture.CorrectnessScore,
		ReliabilityScore: fixture.ReliabilityScore,
		LatencyScore:     fixture.LatencyScore,
		CostScore:        fixture.CostScore,
		BehavioralScore:  fixture.BehavioralScore,
		Scorecard:        fixture.document(),
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	document, err := decodeComparisonScorecard(scorecard.Scorecard)
	if err != nil {
		panic(err)
	}
	return runScorecardParticipant{
		runAgent:  runAgent,
		scorecard: &scorecard,
		document:  &document,
	}
}

type runAgentScorecardFixture struct {
	RunAgentID       uuid.UUID
	EvaluationSpecID uuid.UUID
	OverallScore     *float64
	CorrectnessScore *float64
	ReliabilityScore *float64
	LatencyScore     *float64
	CostScore        *float64
	BehavioralScore  *float64
}

func (f runAgentScorecardFixture) document() []byte {
	dimensions := map[string]any{
		"correctness": map[string]any{"state": scorecardState(f.CorrectnessScore), "score": f.CorrectnessScore},
		"reliability": map[string]any{"state": scorecardState(f.ReliabilityScore), "score": f.ReliabilityScore},
		"latency":     map[string]any{"state": scorecardState(f.LatencyScore), "score": f.LatencyScore},
		"cost":        map[string]any{"state": scorecardState(f.CostScore), "score": f.CostScore},
	}
	if f.BehavioralScore != nil {
		dimensions["behavioral"] = map[string]any{"state": scorecardState(f.BehavioralScore), "score": f.BehavioralScore}
	}
	payload, err := json.Marshal(map[string]any{
		"status":     "complete",
		"dimensions": dimensions,
	})
	if err != nil {
		panic(err)
	}
	return payload
}

func scorecardState(score *float64) string {
	if score == nil {
		return "unavailable"
	}
	return "available"
}

func float64Ptr(value float64) *float64 {
	return &value
}

func TestBuildRunScorecardDocumentSelectsWinnerByOverallScore(t *testing.T) {
	runID := uuid.New()
	evaluationSpecID := uuid.New()
	lowID := uuid.New()
	highID := uuid.New()

	_, winningRunAgentID, err := buildRunScorecardDocument(runID, evaluationSpecID, []runScorecardParticipant{
		scorecardParticipantFixture(0, "low", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       lowID,
			EvaluationSpecID: evaluationSpecID,
			OverallScore:     float64Ptr(0.6),
			CorrectnessScore: float64Ptr(0.9),
			ReliabilityScore: float64Ptr(1.0),
		}),
		scorecardParticipantFixture(1, "high", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       highID,
			EvaluationSpecID: evaluationSpecID,
			OverallScore:     float64Ptr(0.8),
			CorrectnessScore: float64Ptr(0.7),
			ReliabilityScore: float64Ptr(0.5),
		}),
	}, nil)
	if err != nil {
		t.Fatalf("buildRunScorecardDocument returned error: %v", err)
	}
	if winningRunAgentID == nil || *winningRunAgentID != highID {
		t.Fatalf("winning run agent id = %v, want %s", winningRunAgentID, highID)
	}
}

func TestBuildRunAgentScorecardDocumentIncludesTextCompareEvidence(t *testing.T) {
	evaluation := scoring.RunAgentEvaluation{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Status:           scoring.EvaluationStatusComplete,
		DimensionResults: []scoring.DimensionResult{
			{Dimension: "correctness", State: scoring.OutputStateAvailable, Score: float64Ptr(0)},
		},
		ValidatorResults: []scoring.ValidatorResult{
			{
				Key:             "exact",
				Type:            scoring.ValidatorTypeExactMatch,
				State:           scoring.OutputStateAvailable,
				Verdict:         "fail",
				NormalizedScore: float64Ptr(0),
				Target:          "final_output",
				ActualValue:     stringPtr("Paris, France"),
				ExpectedValue:   stringPtr("Paris"),
				RawOutput:       []byte(`{"state":"available","verdict":"fail","actual_value":"Paris, France","expected_value":"Paris"}`),
			},
		},
	}

	document, err := buildRunAgentScorecardDocument(evaluation)
	if err != nil {
		t.Fatalf("buildRunAgentScorecardDocument returned error: %v", err)
	}

	decoded := decodeRunScorecardJSON(t, document)
	validators := decoded["validator_details"].([]any)
	evidence := validators[0].(map[string]any)["evidence"].(map[string]any)
	if evidence["kind"] != "text_compare" {
		t.Fatalf("evidence kind = %#v, want %q", evidence["kind"], "text_compare")
	}
	if evidence["source_field"] != "final_output" {
		t.Fatalf("source_field = %#v, want %q", evidence["source_field"], "final_output")
	}
	if evidence["actual"] != "Paris, France" || evidence["expected"] != "Paris" {
		t.Fatalf("evidence = %#v, want actual/expected string pair", evidence)
	}
}

func TestBuildRunAgentScorecardDocumentFallsBackToCustomEvidence(t *testing.T) {
	evaluation := scoring.RunAgentEvaluation{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Status:           scoring.EvaluationStatusComplete,
		DimensionResults: []scoring.DimensionResult{
			{Dimension: "correctness", State: scoring.OutputStateAvailable, Score: float64Ptr(1)},
		},
		ValidatorResults: []scoring.ValidatorResult{
			{
				Key:             "code",
				Type:            scoring.ValidatorTypeCodeExecution,
				State:           scoring.OutputStateAvailable,
				Verdict:         "pass",
				NormalizedScore: float64Ptr(1),
				Target:          "file:solution",
				RawOutput:       []byte(`{"stdout":"ok","stderr":"","passed_tests":3}`),
			},
		},
	}

	document, err := buildRunAgentScorecardDocument(evaluation)
	if err != nil {
		t.Fatalf("buildRunAgentScorecardDocument returned error: %v", err)
	}

	decoded := decodeRunScorecardJSON(t, document)
	validators := decoded["validator_details"].([]any)
	evidence := validators[0].(map[string]any)["evidence"].(map[string]any)
	if evidence["kind"] != "custom" {
		t.Fatalf("evidence kind = %#v, want %q", evidence["kind"], "custom")
	}
	raw := evidence["raw"].(map[string]any)
	if raw["stdout"] != "ok" {
		t.Fatalf("raw evidence = %#v, want stdout", raw)
	}
}

func TestBuildRunAgentScorecardDocumentIncludesRegexEvidence(t *testing.T) {
	evaluation := scoring.RunAgentEvaluation{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Status:           scoring.EvaluationStatusComplete,
		DimensionResults: []scoring.DimensionResult{
			{Dimension: "correctness", State: scoring.OutputStateAvailable, Score: float64Ptr(1)},
		},
		ValidatorResults: []scoring.ValidatorResult{
			{
				Key:             "pattern",
				Type:            scoring.ValidatorTypeRegexMatch,
				State:           scoring.OutputStateAvailable,
				Verdict:         "pass",
				NormalizedScore: float64Ptr(1),
				Target:          "final_output",
				ActualValue:     stringPtr("Paris, France"),
				ExpectedValue:   stringPtr("Par[a-z]+"),
				RawOutput:       []byte(`{"state":"available","verdict":"pass","actual_value":"Paris, France","expected_value":"Par[a-z]+"}`),
			},
		},
	}

	document, err := buildRunAgentScorecardDocument(evaluation)
	if err != nil {
		t.Fatalf("buildRunAgentScorecardDocument returned error: %v", err)
	}

	decoded := decodeRunScorecardJSON(t, document)
	validators := decoded["validator_details"].([]any)
	evidence := validators[0].(map[string]any)["evidence"].(map[string]any)
	if evidence["kind"] != "regex_match" {
		t.Fatalf("evidence kind = %#v, want %q", evidence["kind"], "regex_match")
	}
	if evidence["pattern"] != "Par[a-z]+" || evidence["actual"] != "Paris, France" {
		t.Fatalf("regex evidence = %#v, want pattern + actual", evidence)
	}
	if evidence["matched"] != true {
		t.Fatalf("matched = %#v, want true", evidence["matched"])
	}
}

func TestBuildValidatorJSONSchemaEvidenceFallsBackToCustomWhenNoStructuredFieldsExist(t *testing.T) {
	result := scoring.ValidatorResult{
		Type:      scoring.ValidatorTypeJSONSchema,
		RawOutput: []byte(`{"opaque":"value"}`),
	}

	evidence := buildValidatorJSONSchemaEvidence(result, decodeRawJSONObject(result.RawOutput))
	if evidence == nil {
		t.Fatal("evidence is nil, want custom fallback")
	}
	decoded := evidence.(map[string]any)
	if decoded["kind"] != "custom" {
		t.Fatalf("evidence kind = %#v, want %q", decoded["kind"], "custom")
	}
	raw := decoded["raw"].(map[string]any)
	if raw["opaque"] != "value" {
		t.Fatalf("raw evidence = %#v, want opaque field", raw)
	}
}

func TestBuildValidatorJSONPathEvidenceFallsBackToCustomWhenNoStructuredFieldsExist(t *testing.T) {
	result := scoring.ValidatorResult{
		Type:      scoring.ValidatorTypeJSONPathMatch,
		RawOutput: []byte(`{"opaque":"value"}`),
	}

	evidence := buildValidatorJSONPathEvidence(result, decodeRawJSONObject(result.RawOutput))
	if evidence == nil {
		t.Fatal("evidence is nil, want custom fallback")
	}
	decoded := evidence.(map[string]any)
	if decoded["kind"] != "custom" {
		t.Fatalf("evidence kind = %#v, want %q", decoded["kind"], "custom")
	}
	raw := decoded["raw"].(map[string]any)
	if raw["opaque"] != "value" {
		t.Fatalf("raw evidence = %#v, want opaque field", raw)
	}
}

func decodeRunScorecardJSON(t *testing.T, payload []byte) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal scorecard JSON: %v", err)
	}
	return decoded
}

func TestBuildRunScorecardDocumentSurfacesBehavioralScore(t *testing.T) {
	runID := uuid.New()
	evaluationSpecID := uuid.New()
	firstAgentID := uuid.New()
	secondAgentID := uuid.New()

	document, _, err := buildRunScorecardDocument(runID, evaluationSpecID, []runScorecardParticipant{
		scorecardParticipantFixture(0, "baseline", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       firstAgentID,
			EvaluationSpecID: evaluationSpecID,
			BehavioralScore:  float64Ptr(0.4),
		}),
		scorecardParticipantFixture(1, "candidate", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       secondAgentID,
			EvaluationSpecID: evaluationSpecID,
			BehavioralScore:  float64Ptr(0.8),
		}),
	}, nil)
	if err != nil {
		t.Fatalf("buildRunScorecardDocument returned error: %v", err)
	}

	var decoded runScorecardDocument
	if err := json.Unmarshal(document, &decoded); err != nil {
		t.Fatalf("unmarshal run scorecard document: %v", err)
	}
	if decoded.Agents[1].BehavioralScore == nil || *decoded.Agents[1].BehavioralScore != 0.8 {
		t.Fatalf("agent behavioral score = %v, want 0.8", decoded.Agents[1].BehavioralScore)
	}
	if decoded.DimensionDeltas["behavioral"].Delta == nil || *decoded.DimensionDeltas["behavioral"].Delta != 0.4 {
		t.Fatalf("behavioral delta = %v, want 0.4", decoded.DimensionDeltas["behavioral"].Delta)
	}
}

func TestBuildRunScorecardDocumentOverallScoreTieFallsBackToCorrectnessThenReliability(t *testing.T) {
	runID := uuid.New()
	evaluationSpecID := uuid.New()
	correctnessWinnerID := uuid.New()
	reliabilityOnlyID := uuid.New()

	document, winningRunAgentID, err := buildRunScorecardDocument(runID, evaluationSpecID, []runScorecardParticipant{
		scorecardParticipantFixture(0, "correctness-winner", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       correctnessWinnerID,
			EvaluationSpecID: evaluationSpecID,
			OverallScore:     float64Ptr(0.8),
			CorrectnessScore: float64Ptr(0.9),
			ReliabilityScore: float64Ptr(0.2),
		}),
		scorecardParticipantFixture(1, "reliability-only", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       reliabilityOnlyID,
			EvaluationSpecID: evaluationSpecID,
			OverallScore:     float64Ptr(0.8),
			CorrectnessScore: float64Ptr(0.7),
			ReliabilityScore: float64Ptr(0.95),
		}),
	}, nil)
	if err != nil {
		t.Fatalf("buildRunScorecardDocument returned error: %v", err)
	}
	if winningRunAgentID == nil || *winningRunAgentID != correctnessWinnerID {
		t.Fatalf("winning run agent id = %v, want %s", winningRunAgentID, correctnessWinnerID)
	}

	var decoded runScorecardDocument
	if err := json.Unmarshal(document, &decoded); err != nil {
		t.Fatalf("unmarshal run scorecard document: %v", err)
	}
	if decoded.WinnerDetermination.ReasonCode != "overall_score_correctness_tiebreaker" {
		t.Fatalf("winner reason code = %q, want overall_score_correctness_tiebreaker", decoded.WinnerDetermination.ReasonCode)
	}
}

// Phase 3: custom user-declared dims must surface in dimension_deltas using
// the direction persisted on the per-agent scorecard, not a hardcoded map of
// legacy keys. A "safety" dim with better_direction=higher should rank the
// agent with the higher score as the delta winner.
func TestBuildRunScorecardDocumentEmitsCustomDimensionDelta(t *testing.T) {
	runID := uuid.New()
	evaluationSpecID := uuid.New()
	winnerID := uuid.New()
	loserID := uuid.New()

	document, _, err := buildRunScorecardDocument(runID, evaluationSpecID, []runScorecardParticipant{
		customDimScorecardParticipantFixture(0, "loser", loserID, evaluationSpecID, map[string]customDimFixture{
			"safety": {score: 0.40, direction: "higher"},
		}),
		customDimScorecardParticipantFixture(1, "winner", winnerID, evaluationSpecID, map[string]customDimFixture{
			"safety": {score: 0.90, direction: "higher"},
		}),
	}, nil)
	if err != nil {
		t.Fatalf("buildRunScorecardDocument returned error: %v", err)
	}

	var decoded runScorecardDocument
	if err := json.Unmarshal(document, &decoded); err != nil {
		t.Fatalf("unmarshal run scorecard document: %v", err)
	}
	delta, ok := decoded.DimensionDeltas["safety"]
	if !ok {
		t.Fatalf("dimension_deltas missing safety key; got keys = %v", decoded.DimensionDeltas)
	}
	if delta.BetterDirection != "higher" {
		t.Fatalf("safety better_direction = %q, want higher", delta.BetterDirection)
	}
	if delta.WinnerValue == nil || math.Abs(*delta.WinnerValue-0.90) > 1e-9 {
		t.Fatalf("safety winner_value = %v, want 0.90", delta.WinnerValue)
	}
	if delta.Delta == nil || math.Abs(*delta.Delta-0.50) > 1e-9 {
		t.Fatalf("safety delta = %v, want 0.50", delta.Delta)
	}
}

type customDimFixture struct {
	score     float64
	direction string
}

func customDimScorecardParticipantFixture(
	laneIndex int32,
	label string,
	runAgentID uuid.UUID,
	evaluationSpecID uuid.UUID,
	dims map[string]customDimFixture,
) runScorecardParticipant {
	runAgent := domain.RunAgent{
		ID:        runAgentID,
		RunID:     uuid.New(),
		LaneIndex: laneIndex,
		Label:     label,
		Status:    domain.RunAgentStatusCompleted,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	dimensionMap := make(map[string]any, len(dims))
	for key, fixture := range dims {
		dimensionMap[key] = map[string]any{
			"state":            "available",
			"score":            fixture.score,
			"better_direction": fixture.direction,
		}
	}
	payload, err := json.Marshal(map[string]any{
		"status":     "complete",
		"dimensions": dimensionMap,
	})
	if err != nil {
		panic(err)
	}
	scorecard := RunAgentScorecard{
		ID:               uuid.New(),
		RunAgentID:       runAgentID,
		EvaluationSpecID: evaluationSpecID,
		Scorecard:        payload,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	doc, err := decodeComparisonScorecard(scorecard.Scorecard)
	if err != nil {
		panic(err)
	}
	return runScorecardParticipant{
		runAgent:  runAgent,
		scorecard: &scorecard,
		document:  &doc,
	}
}

func TestBuildRunScorecardDocumentFallsBackToLegacyWhenOverallScoreMissing(t *testing.T) {
	runID := uuid.New()
	evaluationSpecID := uuid.New()
	betterID := uuid.New()
	worseID := uuid.New()

	// Legacy-only scorecards (no overall_score) should still rank by the
	// correctness/reliability tiebreak chain.
	_, winningRunAgentID, err := buildRunScorecardDocument(runID, evaluationSpecID, []runScorecardParticipant{
		scorecardParticipantFixture(0, "worse", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       worseID,
			EvaluationSpecID: evaluationSpecID,
			CorrectnessScore: float64Ptr(0.5),
			ReliabilityScore: float64Ptr(1.0),
		}),
		scorecardParticipantFixture(1, "better", domain.RunAgentStatusCompleted, runAgentScorecardFixture{
			RunAgentID:       betterID,
			EvaluationSpecID: evaluationSpecID,
			CorrectnessScore: float64Ptr(0.9),
			ReliabilityScore: float64Ptr(0.1),
		}),
	}, nil)
	if err != nil {
		t.Fatalf("buildRunScorecardDocument returned error: %v", err)
	}
	if winningRunAgentID == nil || *winningRunAgentID != betterID {
		t.Fatalf("winning run agent id = %v, want %s (legacy correctness winner)", winningRunAgentID, betterID)
	}
}

func TestBuildRunAgentScorecardDocumentIncludesSourcePointer(t *testing.T) {
	seq := int64(42)
	evaluation := scoring.RunAgentEvaluation{
		RunAgentID:       uuid.New(),
		EvaluationSpecID: uuid.New(),
		Status:           scoring.EvaluationStatusComplete,
		DimensionResults: []scoring.DimensionResult{
			{Dimension: "correctness", State: scoring.OutputStateAvailable, Score: float64Ptr(1)},
		},
		ValidatorResults: []scoring.ValidatorResult{
			{
				Key:             "exact",
				Type:            scoring.ValidatorTypeExactMatch,
				State:           scoring.OutputStateAvailable,
				Verdict:         "pass",
				NormalizedScore: float64Ptr(1),
				Target:          "final_output",
				ActualValue:     stringPtr("done"),
				ExpectedValue:   stringPtr("done"),
				Source: &scoring.Source{
					Kind:      scoring.SourceKindFinalOutput,
					Sequence:  &seq,
					EventType: "system.run.completed",
					FieldPath: "final_output",
				},
				RawOutput: []byte(`{"state":"available","verdict":"pass","actual_value":"done","expected_value":"done"}`),
			},
		},
		MetricResults: []scoring.MetricResult{
			{
				Key:          "token_usage",
				State:        scoring.OutputStateAvailable,
				Collector:    "usage_aggregator",
				NumericValue: float64Ptr(1234),
			},
		},
	}

	document, err := buildRunAgentScorecardDocument(evaluation)
	if err != nil {
		t.Fatalf("buildRunAgentScorecardDocument returned error: %v", err)
	}

	decoded := decodeRunScorecardJSON(t, document)
	validators := decoded["validator_details"].([]any)
	source, ok := validators[0].(map[string]any)["source"].(map[string]any)
	if !ok {
		t.Fatalf("validator_details[0].source missing or wrong type: %#v", validators[0])
	}
	if source["kind"] != "final_output" {
		t.Fatalf("source.kind = %#v, want final_output", source["kind"])
	}
	if source["sequence"].(float64) != 42 {
		t.Fatalf("source.sequence = %#v, want 42", source["sequence"])
	}
	if source["event_type"] != "system.run.completed" {
		t.Fatalf("source.event_type = %#v, want system.run.completed", source["event_type"])
	}

	metrics := decoded["metric_details"].([]any)
	if _, present := metrics[0].(map[string]any)["source"]; present {
		t.Fatalf("metric_details[0] should not carry a source field (none is populated by any collector today), got %#v", metrics[0])
	}
}
