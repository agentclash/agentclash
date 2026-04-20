package repository

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestBuildEvalSessionAggregatePayloadIsDeterministicAcrossRunOrder(t *testing.T) {
	firstRunID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	secondRunID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	first := evalSessionAggregateSource{
		RunID: firstRunID,
		Document: runScorecardDocument{
			Agents: []runScorecardAgentSummary{
				{
					RunAgentID:       uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
					LaneIndex:        0,
					Label:            "Primary",
					HasScorecard:     true,
					OverallScore:     float64Ptr(0.80),
					CorrectnessScore: float64Ptr(0.75),
					Dimensions: map[string]comparisonScorecardDimensionInfo{
						"custom": {Score: float64Ptr(0.60)},
					},
				},
			},
		},
	}
	second := evalSessionAggregateSource{
		RunID: secondRunID,
		Document: runScorecardDocument{
			Agents: []runScorecardAgentSummary{
				{
					RunAgentID:       uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
					LaneIndex:        0,
					Label:            "Primary",
					HasScorecard:     true,
					OverallScore:     float64Ptr(0.90),
					CorrectnessScore: float64Ptr(0.85),
					Dimensions: map[string]comparisonScorecardDimensionInfo{
						"custom": {Score: float64Ptr(0.70)},
					},
				},
			},
		},
	}

	leftAggregate, leftEvidence, _, err := buildEvalSessionAggregatePayload(2, []evalSessionAggregateSource{first, second}, nil)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}
	rightAggregate, rightEvidence, _, err := buildEvalSessionAggregatePayload(2, []evalSessionAggregateSource{second, first}, nil)
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}

	if string(leftAggregate) != string(rightAggregate) {
		t.Fatalf("aggregate payload differs by run order:\nleft=%s\nright=%s", leftAggregate, rightAggregate)
	}
	if string(leftEvidence) != string(rightEvidence) {
		t.Fatalf("evidence payload differs by run order:\nleft=%s\nright=%s", leftEvidence, rightEvidence)
	}
}

func TestBuildEvalSessionAggregatePayloadCapturesMissingScorecardsAndPartialCoverage(t *testing.T) {
	runID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	missingRunID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	aggregateJSON, evidenceJSON, scoredChildCount, err := buildEvalSessionAggregatePayload(2, []evalSessionAggregateSource{
		{
			RunID: runID,
			Document: runScorecardDocument{
				Agents: []runScorecardAgentSummary{
					{
						RunAgentID:       uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
						LaneIndex:        0,
						Label:            "Primary",
						HasScorecard:     true,
						OverallScore:     float64Ptr(0.88),
						CorrectnessScore: float64Ptr(0.81),
					},
				},
			},
		},
	}, []uuid.UUID{missingRunID})
	if err != nil {
		t.Fatalf("buildEvalSessionAggregatePayload returned error: %v", err)
	}
	if scoredChildCount != 1 {
		t.Fatalf("scored child count = %d, want 1", scoredChildCount)
	}

	var aggregate evalSessionAggregateDocument
	if err := json.Unmarshal(aggregateJSON, &aggregate); err != nil {
		t.Fatalf("unmarshal aggregate: %v", err)
	}
	if aggregate.ScoredChildCount != 1 {
		t.Fatalf("aggregate scored child count = %d, want 1", aggregate.ScoredChildCount)
	}
	if aggregate.Overall == nil || aggregate.Overall.Interval != nil {
		t.Fatalf("overall aggregate = %#v, want no interval for n=1", aggregate.Overall)
	}

	var evidence evalSessionAggregateEvidence
	if err := json.Unmarshal(evidenceJSON, &evidence); err != nil {
		t.Fatalf("unmarshal evidence: %v", err)
	}
	if !slices.Contains(evidence.MissingScorecardRunIDs, missingRunID) {
		t.Fatalf("missing scorecard ids = %v, want %s", evidence.MissingScorecardRunIDs, missingRunID)
	}
	if !slices.Contains(evidence.Warnings, "confidence intervals require at least 2 scored child runs") {
		t.Fatalf("warnings = %v, want insufficient evidence warning", evidence.Warnings)
	}
	if !slices.Contains(evidence.Warnings, "1 child run scorecards are missing from aggregation evidence") {
		t.Fatalf("warnings = %v, want missing scorecard warning", evidence.Warnings)
	}
}

func TestBuildEvalSessionAggregatePayloadRejectsMissingScoredChildren(t *testing.T) {
	_, _, _, err := buildEvalSessionAggregatePayload(2, nil, nil)
	if !errors.Is(err, ErrEvalSessionAggregateUnavailable) {
		t.Fatalf("error = %v, want ErrEvalSessionAggregateUnavailable", err)
	}
}
