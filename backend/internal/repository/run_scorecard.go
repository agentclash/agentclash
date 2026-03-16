package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const runScorecardSummarySchemaVersion = "2026-03-17"

type RunScorecard struct {
	ID                uuid.UUID
	RunID             uuid.UUID
	EvaluationSpecID  uuid.UUID
	WinningRunAgentID *uuid.UUID
	Scorecard         json.RawMessage
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type runScorecardParticipant struct {
	runAgent  domain.RunAgent
	scorecard *RunAgentScorecard
	document  *comparisonScorecardDocument
}

type runScorecardDocument struct {
	SchemaVersion       string                       `json:"schema_version"`
	RunID               uuid.UUID                    `json:"run_id"`
	EvaluationSpecID    uuid.UUID                    `json:"evaluation_spec_id"`
	WinningRunAgentID   *uuid.UUID                   `json:"winning_run_agent_id,omitempty"`
	WinnerDetermination runScorecardWinnerSummary    `json:"winner_determination"`
	Agents              []runScorecardAgentSummary   `json:"agents"`
	DimensionDeltas     map[string]runScorecardDelta `json:"dimension_deltas"`
	EvidenceQuality     runScorecardEvidenceQuality  `json:"evidence_quality"`
}

type runScorecardWinnerSummary struct {
	Strategy   string `json:"strategy"`
	Status     string `json:"status"`
	ReasonCode string `json:"reason_code"`
}

type runScorecardAgentSummary struct {
	RunAgentID       uuid.UUID                                   `json:"run_agent_id"`
	LaneIndex        int32                                       `json:"lane_index"`
	Label            string                                      `json:"label"`
	Status           domain.RunAgentStatus                       `json:"status"`
	HasScorecard     bool                                        `json:"has_scorecard"`
	EvaluationStatus string                                      `json:"evaluation_status,omitempty"`
	OverallScore     *float64                                    `json:"overall_score,omitempty"`
	CorrectnessScore *float64                                    `json:"correctness_score,omitempty"`
	ReliabilityScore *float64                                    `json:"reliability_score,omitempty"`
	LatencyScore     *float64                                    `json:"latency_score,omitempty"`
	CostScore        *float64                                    `json:"cost_score,omitempty"`
	Dimensions       map[string]comparisonScorecardDimensionInfo `json:"dimensions,omitempty"`
}

type runScorecardDelta struct {
	BetterDirection string                       `json:"better_direction"`
	State           string                       `json:"state"`
	WinnerValue     *float64                     `json:"winner_value,omitempty"`
	RunnerUpValue   *float64                     `json:"runner_up_value,omitempty"`
	Delta           *float64                     `json:"delta,omitempty"`
	Values          []runScorecardDimensionValue `json:"values"`
}

type runScorecardDimensionValue struct {
	RunAgentID uuid.UUID `json:"run_agent_id"`
	State      string    `json:"state"`
	Value      *float64  `json:"value,omitempty"`
}

type runScorecardEvidenceQuality struct {
	MissingFields []string `json:"missing_fields,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

type scoredRunAgent struct {
	runAgent         domain.RunAgent
	scorecard        RunAgentScorecard
	correctnessScore *float64
	reliabilityScore *float64
}

func (r *Repository) GetRunScorecardByRunID(ctx context.Context, runID uuid.UUID) (RunScorecard, error) {
	row, err := r.queries.GetRunScorecardByRunID(ctx, repositorysqlc.GetRunScorecardByRunIDParams{RunID: runID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RunScorecard{}, ErrRunScorecardNotFound
		}
		return RunScorecard{}, fmt.Errorf("get run scorecard by run id: %w", err)
	}

	scorecard, err := mapRunScorecard(row)
	if err != nil {
		return RunScorecard{}, fmt.Errorf("map run scorecard: %w", err)
	}

	return scorecard, nil
}

func (r *Repository) BuildRunScorecard(ctx context.Context, runID uuid.UUID) (RunScorecard, error) {
	runAgents, err := r.ListRunAgentsByRunID(ctx, runID)
	if err != nil {
		return RunScorecard{}, fmt.Errorf("list run agents: %w", err)
	}
	if len(runAgents) == 0 {
		return RunScorecard{}, fmt.Errorf("build run scorecard: run %s has no run agents", runID)
	}

	participants := make([]runScorecardParticipant, 0, len(runAgents))
	warnings := make([]string, 0)
	var evaluationSpecID *uuid.UUID

	for _, runAgent := range runAgents {
		participant := runScorecardParticipant{runAgent: runAgent}

		scorecard, err := r.GetRunAgentScorecardByRunAgentID(ctx, runAgent.ID)
		switch {
		case err == nil:
			document, decodeErr := decodeComparisonScorecard(scorecard.Scorecard)
			if decodeErr != nil {
				return RunScorecard{}, fmt.Errorf("decode run-agent scorecard %s: %w", runAgent.ID, decodeErr)
			}
			if evaluationSpecID == nil {
				value := scorecard.EvaluationSpecID
				evaluationSpecID = &value
			} else if *evaluationSpecID != scorecard.EvaluationSpecID {
				return RunScorecard{}, fmt.Errorf("build run scorecard: run %s has inconsistent evaluation specs", runID)
			}
			participant.scorecard = &scorecard
			participant.document = &document
		case errors.Is(err, ErrRunAgentScorecardNotFound):
			warnings = append(warnings, fmt.Sprintf("run-agent %s scorecard unavailable", runAgent.ID))
		default:
			return RunScorecard{}, fmt.Errorf("load run-agent scorecard %s: %w", runAgent.ID, err)
		}

		participants = append(participants, participant)
	}

	if evaluationSpecID == nil {
		return RunScorecard{}, fmt.Errorf("build run scorecard: run %s has no run-agent scorecards", runID)
	}

	document, winningRunAgentID, err := buildRunScorecardDocument(runID, *evaluationSpecID, participants, warnings)
	if err != nil {
		return RunScorecard{}, err
	}

	row, err := r.queries.UpsertRunScorecard(ctx, repositorysqlc.UpsertRunScorecardParams{
		RunID:             runID,
		EvaluationSpecID:  *evaluationSpecID,
		WinningRunAgentID: cloneUUIDPtr(winningRunAgentID),
		Scorecard:         document,
	})
	if err != nil {
		return RunScorecard{}, fmt.Errorf("upsert run scorecard: %w", err)
	}

	scorecard, err := mapRunScorecard(row)
	if err != nil {
		return RunScorecard{}, fmt.Errorf("map run scorecard: %w", err)
	}
	return scorecard, nil
}

func buildRunScorecardDocument(
	runID uuid.UUID,
	evaluationSpecID uuid.UUID,
	participants []runScorecardParticipant,
	warnings []string,
) (json.RawMessage, *uuid.UUID, error) {
	agents := make([]runScorecardAgentSummary, 0, len(participants))
	scoredAgents := make([]scoredRunAgent, 0, len(participants))
	missingFields := make([]string, 0)

	for _, participant := range participants {
		summary := runScorecardAgentSummary{
			RunAgentID:   participant.runAgent.ID,
			LaneIndex:    participant.runAgent.LaneIndex,
			Label:        participant.runAgent.Label,
			Status:       participant.runAgent.Status,
			HasScorecard: participant.scorecard != nil,
		}

		if participant.scorecard != nil && participant.document != nil {
			summary.EvaluationStatus = participant.document.Status
			summary.OverallScore = cloneFloat64Ptr(participant.scorecard.OverallScore)
			summary.CorrectnessScore = cloneFloat64Ptr(participant.scorecard.CorrectnessScore)
			summary.ReliabilityScore = cloneFloat64Ptr(participant.scorecard.ReliabilityScore)
			summary.LatencyScore = cloneFloat64Ptr(participant.scorecard.LatencyScore)
			summary.CostScore = cloneFloat64Ptr(participant.scorecard.CostScore)
			summary.Dimensions = cloneRunScorecardDimensions(participant.document.Dimensions)
			scoredAgents = append(scoredAgents, scoredRunAgent{
				runAgent:         participant.runAgent,
				scorecard:        *participant.scorecard,
				correctnessScore: availableDimensionScore(participant.scorecard.CorrectnessScore, participant.document.Dimensions["correctness"]),
				reliabilityScore: availableDimensionScore(participant.scorecard.ReliabilityScore, participant.document.Dimensions["reliability"]),
			})
		}

		agents = append(agents, summary)
	}

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].LaneIndex < agents[j].LaneIndex
	})

	dimensionDeltas := map[string]runScorecardDelta{
		"correctness": buildRunScorecardDelta(participants, "correctness", "higher", &missingFields),
		"reliability": buildRunScorecardDelta(participants, "reliability", "higher", &missingFields),
		"latency":     buildRunScorecardDelta(participants, "latency", "lower", &missingFields),
		"cost":        buildRunScorecardDelta(participants, "cost", "lower", &missingFields),
	}

	winningRunAgentID, winnerSummary := determineRunWinner(participants, scoredAgents)
	document := runScorecardDocument{
		SchemaVersion:       runScorecardSummarySchemaVersion,
		RunID:               runID,
		EvaluationSpecID:    evaluationSpecID,
		WinningRunAgentID:   cloneUUIDPtr(winningRunAgentID),
		WinnerDetermination: winnerSummary,
		Agents:              agents,
		DimensionDeltas:     dimensionDeltas,
		EvidenceQuality: runScorecardEvidenceQuality{
			MissingFields: uniqueSortedStrings(missingFields),
			Warnings:      uniqueSortedStrings(warnings),
		},
	}

	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal run scorecard: %w", err)
	}
	return encoded, winningRunAgentID, nil
}

func determineRunWinner(participants []runScorecardParticipant, scoredAgents []scoredRunAgent) (*uuid.UUID, runScorecardWinnerSummary) {
	summary := runScorecardWinnerSummary{
		Strategy: "correctness_then_reliability",
	}

	if len(participants) == 1 {
		winner := participants[0].runAgent.ID
		summary.Status = "winner"
		summary.ReasonCode = "single_agent_trivial_winner"
		return &winner, summary
	}
	if len(scoredAgents) == 0 {
		summary.Status = "inconclusive"
		summary.ReasonCode = "no_scored_agents"
		return nil, summary
	}

	bestCorrectness := highestAgentsByScore(scoredAgents, func(agent scoredRunAgent) *float64 { return agent.correctnessScore })
	if len(bestCorrectness) == 0 {
		summary.Status = "inconclusive"
		summary.ReasonCode = "missing_correctness"
		return nil, summary
	}
	if len(bestCorrectness) == 1 {
		winner := bestCorrectness[0].runAgent.ID
		summary.Status = "winner"
		summary.ReasonCode = "best_correctness"
		return &winner, summary
	}

	bestReliability := highestAgentsByScore(bestCorrectness, func(agent scoredRunAgent) *float64 { return agent.reliabilityScore })
	if len(bestReliability) == 0 {
		summary.Status = "inconclusive"
		summary.ReasonCode = "missing_reliability_tiebreaker"
		return nil, summary
	}
	if len(bestReliability) == 1 {
		winner := bestReliability[0].runAgent.ID
		summary.Status = "winner"
		summary.ReasonCode = "reliability_tiebreaker"
		return &winner, summary
	}

	summary.Status = "tie"
	summary.ReasonCode = "correctness_reliability_tie"
	return nil, summary
}

func highestAgentsByScore[T any](agents []T, scoreFn func(T) *float64) []T {
	var best *float64
	selected := make([]T, 0, len(agents))
	for _, agent := range agents {
		score := scoreFn(agent)
		if score == nil {
			continue
		}
		switch {
		case best == nil || *score > *best:
			value := *score
			best = &value
			selected = []T{agent}
		case *score == *best:
			selected = append(selected, agent)
		}
	}
	return selected
}

func buildRunScorecardDelta(
	participants []runScorecardParticipant,
	dimension string,
	betterDirection string,
	missingFields *[]string,
) runScorecardDelta {
	values := make([]runScorecardDimensionValue, 0, len(participants))
	available := make([]runScorecardDimensionValue, 0, len(participants))

	for _, participant := range participants {
		value := runScorecardDimensionValue{
			RunAgentID: participant.runAgent.ID,
			State:      "unavailable",
		}
		if participant.scorecard != nil && participant.document != nil {
			info := participant.document.Dimensions[dimension]
			score := scoreByDimension(*participant.scorecard, dimension)
			value.State = info.State
			value.Value = availableDimensionScore(score, info)
			if value.Value != nil {
				value.State = "available"
				available = append(available, value)
			} else if value.State == "" {
				value.State = "unavailable"
			}
		}
		values = append(values, value)
	}

	delta := runScorecardDelta{
		BetterDirection: betterDirection,
		State:           "unavailable",
		Values:          values,
	}
	if len(available) < 2 {
		*missingFields = append(*missingFields, "dimension_deltas."+dimension)
		return delta
	}

	sort.Slice(available, func(i, j int) bool {
		left := *available[i].Value
		right := *available[j].Value
		if betterDirection == "lower" {
			if left == right {
				return available[i].RunAgentID.String() < available[j].RunAgentID.String()
			}
			return left < right
		}
		if left == right {
			return available[i].RunAgentID.String() < available[j].RunAgentID.String()
		}
		return left > right
	})

	best := *available[0].Value
	runnerUp := *available[1].Value
	margin := best - runnerUp
	if betterDirection == "lower" {
		margin = runnerUp - best
	}

	delta.State = "available"
	delta.WinnerValue = cloneFloat64Ptr(&best)
	delta.RunnerUpValue = cloneFloat64Ptr(&runnerUp)
	delta.Delta = cloneFloat64Ptr(&margin)
	return delta
}

func availableDimensionScore(score *float64, dimension comparisonScorecardDimensionInfo) *float64 {
	if score == nil {
		return nil
	}
	if dimension.State != "available" {
		return nil
	}
	return cloneFloat64Ptr(score)
}

func scoreByDimension(scorecard RunAgentScorecard, dimension string) *float64 {
	switch dimension {
	case "correctness":
		return scorecard.CorrectnessScore
	case "reliability":
		return scorecard.ReliabilityScore
	case "latency":
		return scorecard.LatencyScore
	case "cost":
		return scorecard.CostScore
	default:
		return nil
	}
}

func cloneRunScorecardDimensions(input map[string]comparisonScorecardDimensionInfo) map[string]comparisonScorecardDimensionInfo {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]comparisonScorecardDimensionInfo, len(input))
	for key, value := range input {
		cloned[key] = comparisonScorecardDimensionInfo{
			State: value.State,
			Score: cloneFloat64Ptr(value.Score),
		}
	}
	return cloned
}

func mapRunScorecard(row repositorysqlc.RunScorecard) (RunScorecard, error) {
	createdAt, err := requiredTime("run_scorecards.created_at", row.CreatedAt)
	if err != nil {
		return RunScorecard{}, err
	}
	updatedAt, err := requiredTime("run_scorecards.updated_at", row.UpdatedAt)
	if err != nil {
		return RunScorecard{}, err
	}

	return RunScorecard{
		ID:                row.ID,
		RunID:             row.RunID,
		EvaluationSpecID:  row.EvaluationSpecID,
		WinningRunAgentID: cloneUUIDPtr(row.WinningRunAgentID),
		Scorecard:         cloneJSON(row.Scorecard),
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}
