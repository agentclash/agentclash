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
