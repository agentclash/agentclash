package repository

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
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
}

func (f runAgentScorecardFixture) document() []byte {
	payload, err := json.Marshal(map[string]any{
		"status": "complete",
		"dimensions": map[string]any{
			"correctness": map[string]any{"state": scorecardState(f.CorrectnessScore), "score": f.CorrectnessScore},
			"reliability": map[string]any{"state": scorecardState(f.ReliabilityScore), "score": f.ReliabilityScore},
			"latency":     map[string]any{"state": scorecardState(f.LatencyScore), "score": f.LatencyScore},
			"cost":        map[string]any{"state": scorecardState(f.CostScore), "score": f.CostScore},
		},
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
