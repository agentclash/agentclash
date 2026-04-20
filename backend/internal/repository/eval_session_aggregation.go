package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	evalSessionAggregateSchemaVersion     = 1
	evalSessionAggregateIntervalEstimator = "normal_95"
	evalSessionHighVarianceRule           = "stddev/abs(mean)>0.15_or_range>0.10"
)

type EvalSessionAggregateRecord struct {
	ID               uuid.UUID
	EvalSessionID    uuid.UUID
	SchemaVersion    int32
	ChildRunCount    int32
	ScoredChildCount int32
	Aggregate        json.RawMessage
	Evidence         json.RawMessage
	ComputedAt       time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type UpsertEvalSessionAggregateParams struct {
	EvalSessionID    uuid.UUID
	SchemaVersion    int32
	ChildRunCount    int32
	ScoredChildCount int32
	Aggregate        json.RawMessage
	Evidence         json.RawMessage
}

type evalSessionAggregateDocument struct {
	SchemaVersion    int32                                 `json:"schema_version"`
	ChildRunCount    int                                   `json:"child_run_count"`
	ScoredChildCount int                                   `json:"scored_child_count"`
	TopLevelSource   string                                `json:"top_level_source,omitempty"`
	Overall          *evalSessionMetricAggregate           `json:"overall,omitempty"`
	Dimensions       map[string]evalSessionMetricAggregate `json:"dimensions,omitempty"`
	Participants     []evalSessionParticipantAggregate     `json:"participants,omitempty"`
}

type evalSessionParticipantAggregate struct {
	LaneIndex  int32                                 `json:"lane_index"`
	Label      string                                `json:"label"`
	Overall    *evalSessionMetricAggregate           `json:"overall,omitempty"`
	Dimensions map[string]evalSessionMetricAggregate `json:"dimensions,omitempty"`
}

type evalSessionMetricAggregate struct {
	N                int                           `json:"n"`
	Mean             float64                       `json:"mean"`
	Median           float64                       `json:"median"`
	StdDev           float64                       `json:"std_dev"`
	Min              float64                       `json:"min"`
	Max              float64                       `json:"max"`
	Interval         *evalSessionAggregateInterval `json:"interval,omitempty"`
	HighVariance     bool                          `json:"high_variance"`
	HighVarianceRule string                        `json:"high_variance_rule"`
}

type evalSessionAggregateInterval struct {
	Estimator string  `json:"estimator"`
	Lower     float64 `json:"lower"`
	Upper     float64 `json:"upper"`
}

type evalSessionAggregateEvidence struct {
	Warnings               []string    `json:"warnings,omitempty"`
	MissingScorecardRunIDs []uuid.UUID `json:"missing_scorecard_run_ids,omitempty"`
}

type evalSessionAggregateSource struct {
	RunID    uuid.UUID
	Document runScorecardDocument
}

type evalSessionParticipantKey struct {
	LaneIndex int32
	Label     string
}

type evalSessionParticipantAccumulator struct {
	Key        evalSessionParticipantKey
	Overall    []float64
	Dimensions map[string][]float64
}

func (r *Repository) GetEvalSessionResultBySessionID(ctx context.Context, evalSessionID uuid.UUID) (EvalSessionAggregateRecord, error) {
	row, err := r.queries.GetEvalSessionResultBySessionID(ctx, repositorysqlc.GetEvalSessionResultBySessionIDParams{
		EvalSessionID: evalSessionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvalSessionAggregateRecord{}, ErrEvalSessionResultNotFound
		}
		return EvalSessionAggregateRecord{}, fmt.Errorf("get eval session result by session id: %w", err)
	}

	record, err := mapEvalSessionAggregateRecord(row)
	if err != nil {
		return EvalSessionAggregateRecord{}, fmt.Errorf("map eval session result: %w", err)
	}
	return record, nil
}

func (r *Repository) UpsertEvalSessionAggregate(ctx context.Context, params UpsertEvalSessionAggregateParams) (EvalSessionAggregateRecord, error) {
	row, err := r.queries.UpsertEvalSessionResult(ctx, repositorysqlc.UpsertEvalSessionResultParams{
		EvalSessionID:    params.EvalSessionID,
		SchemaVersion:    params.SchemaVersion,
		ChildRunCount:    params.ChildRunCount,
		ScoredChildCount: params.ScoredChildCount,
		Aggregate:        normalizeJSON(params.Aggregate),
		Evidence:         normalizeJSON(params.Evidence),
	})
	if err != nil {
		return EvalSessionAggregateRecord{}, fmt.Errorf("upsert eval session result: %w", err)
	}

	record, err := mapEvalSessionAggregateRecord(row)
	if err != nil {
		return EvalSessionAggregateRecord{}, fmt.Errorf("map eval session result: %w", err)
	}
	return record, nil
}

func (r *Repository) AggregateEvalSession(ctx context.Context, evalSessionID uuid.UUID) (EvalSessionAggregateRecord, error) {
	if _, err := r.GetEvalSessionByID(ctx, evalSessionID); err != nil {
		return EvalSessionAggregateRecord{}, err
	}

	runs, err := r.ListRunsByEvalSessionID(ctx, evalSessionID)
	if err != nil {
		return EvalSessionAggregateRecord{}, err
	}

	missingScorecardRunIDs := make([]uuid.UUID, 0)
	sources := make([]evalSessionAggregateSource, 0, len(runs))
	for _, run := range runs {
		scorecard, scorecardErr := r.GetRunScorecardByRunID(ctx, run.ID)
		switch {
		case scorecardErr == nil:
			var document runScorecardDocument
			if err := json.Unmarshal(scorecard.Scorecard, &document); err != nil {
				return EvalSessionAggregateRecord{}, fmt.Errorf("decode run scorecard %s: %w", run.ID, err)
			}
			sources = append(sources, evalSessionAggregateSource{
				RunID:    run.ID,
				Document: document,
			})
		case errors.Is(scorecardErr, ErrRunScorecardNotFound):
			missingScorecardRunIDs = append(missingScorecardRunIDs, run.ID)
		default:
			return EvalSessionAggregateRecord{}, fmt.Errorf("load run scorecard %s: %w", run.ID, scorecardErr)
		}
	}

	aggregate, evidence, scoredChildCount, err := buildEvalSessionAggregatePayload(len(runs), sources, missingScorecardRunIDs)
	if err != nil {
		return EvalSessionAggregateRecord{}, err
	}

	return r.UpsertEvalSessionAggregate(ctx, UpsertEvalSessionAggregateParams{
		EvalSessionID:    evalSessionID,
		SchemaVersion:    evalSessionAggregateSchemaVersion,
		ChildRunCount:    int32(len(runs)),
		ScoredChildCount: int32(scoredChildCount),
		Aggregate:        aggregate,
		Evidence:         evidence,
	})
}

func buildEvalSessionAggregatePayload(childRunCount int, sources []evalSessionAggregateSource, missingScorecardRunIDs []uuid.UUID) (json.RawMessage, json.RawMessage, int, error) {
	if len(sources) == 0 {
		return nil, nil, 0, ErrEvalSessionAggregateUnavailable
	}

	sort.Slice(sources, func(i, j int) bool {
		return sources[i].RunID.String() < sources[j].RunID.String()
	})
	sort.Slice(missingScorecardRunIDs, func(i, j int) bool {
		return missingScorecardRunIDs[i].String() < missingScorecardRunIDs[j].String()
	})

	warnings := make([]string, 0)
	topLevelOverall := make([]float64, 0, len(sources))
	topLevelDimensions := map[string][]float64{}
	participantAccumulators := map[evalSessionParticipantKey]*evalSessionParticipantAccumulator{}
	insufficientEvidence := false

	for _, source := range sources {
		selectedAgent, ok := selectEvalSessionAggregateAgent(source.Document)
		if ok {
			if selectedAgent.OverallScore != nil {
				topLevelOverall = append(topLevelOverall, *selectedAgent.OverallScore)
			}
			for key, value := range evalSessionAggregateDimensions(selectedAgent) {
				topLevelDimensions[key] = append(topLevelDimensions[key], value)
			}
		} else {
			warnings = append(warnings, fmt.Sprintf("run %s did not expose a sole or winning agent summary for top-level aggregation", source.RunID))
		}

		for _, agent := range source.Document.Agents {
			if !agent.HasScorecard {
				warnings = append(warnings, fmt.Sprintf("run %s participant %q (lane %d) has no scorecard", source.RunID, agent.Label, agent.LaneIndex))
				continue
			}
			key := evalSessionParticipantKey{LaneIndex: agent.LaneIndex, Label: agent.Label}
			accumulator, ok := participantAccumulators[key]
			if !ok {
				accumulator = &evalSessionParticipantAccumulator{
					Key:        key,
					Dimensions: map[string][]float64{},
				}
				participantAccumulators[key] = accumulator
			}
			if agent.OverallScore != nil {
				accumulator.Overall = append(accumulator.Overall, *agent.OverallScore)
			}
			for dimKey, value := range evalSessionAggregateDimensions(agent) {
				accumulator.Dimensions[dimKey] = append(accumulator.Dimensions[dimKey], value)
			}
		}
	}

	evidence := evalSessionAggregateEvidence{
		MissingScorecardRunIDs: append([]uuid.UUID(nil), missingScorecardRunIDs...),
	}
	if len(missingScorecardRunIDs) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d child run scorecards are missing from aggregation evidence", len(missingScorecardRunIDs)))
	}

	document := evalSessionAggregateDocument{
		SchemaVersion:    evalSessionAggregateSchemaVersion,
		ChildRunCount:    childRunCount,
		ScoredChildCount: len(sources),
		TopLevelSource:   "sole_agent_or_winner",
	}

	if len(topLevelOverall) > 0 {
		overall := buildEvalSessionMetricAggregate(topLevelOverall)
		document.Overall = &overall
		if len(topLevelOverall) < len(sources) {
			warnings = append(warnings, fmt.Sprintf("overall aggregate uses %d of %d scored child runs", len(topLevelOverall), len(sources)))
		}
		if len(topLevelOverall) < 2 {
			insufficientEvidence = true
		}
	}
	if len(topLevelDimensions) > 0 {
		document.Dimensions = map[string]evalSessionMetricAggregate{}
		keys := sortedMetricKeys(topLevelDimensions)
		for _, key := range keys {
			values := topLevelDimensions[key]
			document.Dimensions[key] = buildEvalSessionMetricAggregate(values)
			if len(values) < len(sources) {
				warnings = append(warnings, fmt.Sprintf("dimension %q aggregate uses %d of %d scored child runs", key, len(values), len(sources)))
			}
			if len(values) < 2 {
				insufficientEvidence = true
			}
		}
	}

	participantKeys := make([]evalSessionParticipantKey, 0, len(participantAccumulators))
	for key := range participantAccumulators {
		participantKeys = append(participantKeys, key)
	}
	sort.Slice(participantKeys, func(i, j int) bool {
		if participantKeys[i].LaneIndex != participantKeys[j].LaneIndex {
			return participantKeys[i].LaneIndex < participantKeys[j].LaneIndex
		}
		return participantKeys[i].Label < participantKeys[j].Label
	})
	document.Participants = make([]evalSessionParticipantAggregate, 0, len(participantKeys))
	for _, key := range participantKeys {
		accumulator := participantAccumulators[key]
		participant := evalSessionParticipantAggregate{
			LaneIndex: key.LaneIndex,
			Label:     key.Label,
		}
		if len(accumulator.Overall) > 0 {
			overall := buildEvalSessionMetricAggregate(accumulator.Overall)
			participant.Overall = &overall
			if len(accumulator.Overall) < len(sources) {
				warnings = append(warnings, fmt.Sprintf("participant %q (lane %d) overall aggregate uses %d of %d scored child runs", key.Label, key.LaneIndex, len(accumulator.Overall), len(sources)))
			}
			if len(accumulator.Overall) < 2 {
				insufficientEvidence = true
			}
		}
		if len(accumulator.Dimensions) > 0 {
			participant.Dimensions = map[string]evalSessionMetricAggregate{}
			for _, dimKey := range sortedMetricKeys(accumulator.Dimensions) {
				values := accumulator.Dimensions[dimKey]
				participant.Dimensions[dimKey] = buildEvalSessionMetricAggregate(values)
				if len(values) < len(sources) {
					warnings = append(warnings, fmt.Sprintf("participant %q (lane %d) dimension %q aggregate uses %d of %d scored child runs", key.Label, key.LaneIndex, dimKey, len(values), len(sources)))
				}
				if len(values) < 2 {
					insufficientEvidence = true
				}
			}
		}
		document.Participants = append(document.Participants, participant)
	}

	if insufficientEvidence {
		warnings = append(warnings, "confidence intervals require at least 2 scored child runs")
	}

	sort.Strings(warnings)
	evidence.Warnings = dedupeSortedStrings(warnings)

	aggregateJSON, err := json.Marshal(document)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("marshal eval session aggregate: %w", err)
	}
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("marshal eval session aggregate evidence: %w", err)
	}
	return aggregateJSON, evidenceJSON, len(sources), nil
}

func selectEvalSessionAggregateAgent(document runScorecardDocument) (runScorecardAgentSummary, bool) {
	if len(document.Agents) == 1 {
		return document.Agents[0], true
	}
	if document.WinningRunAgentID == nil {
		return runScorecardAgentSummary{}, false
	}
	for _, agent := range document.Agents {
		if agent.RunAgentID == *document.WinningRunAgentID {
			return agent, true
		}
	}
	return runScorecardAgentSummary{}, false
}

func evalSessionAggregateDimensions(agent runScorecardAgentSummary) map[string]float64 {
	dimensions := make(map[string]float64, len(agent.Dimensions)+5)
	for key, dimension := range agent.Dimensions {
		if dimension.Score == nil {
			continue
		}
		dimensions[key] = *dimension.Score
	}
	appendBuiltInEvalSessionDimension(dimensions, "correctness", agent.CorrectnessScore)
	appendBuiltInEvalSessionDimension(dimensions, "reliability", agent.ReliabilityScore)
	appendBuiltInEvalSessionDimension(dimensions, "latency", agent.LatencyScore)
	appendBuiltInEvalSessionDimension(dimensions, "cost", agent.CostScore)
	appendBuiltInEvalSessionDimension(dimensions, "behavioral", agent.BehavioralScore)
	return dimensions
}

func appendBuiltInEvalSessionDimension(target map[string]float64, key string, value *float64) {
	if value == nil {
		return
	}
	if _, ok := target[key]; ok {
		return
	}
	target[key] = *value
}

func buildEvalSessionMetricAggregate(values []float64) evalSessionMetricAggregate {
	sortedValues := append([]float64(nil), values...)
	sort.Float64s(sortedValues)

	meanValue := kahanMean(sortedValues)
	stdDev := sampleStdDev(sortedValues, meanValue)
	minValue := sortedValues[0]
	maxValue := sortedValues[len(sortedValues)-1]
	highVariance := false
	if len(sortedValues) > 1 {
		rangeValue := maxValue - minValue
		if math.Abs(meanValue) > 1e-6 && (stdDev/math.Abs(meanValue)) > 0.15 {
			highVariance = true
		}
		if rangeValue > 0.10 {
			highVariance = true
		}
	}

	aggregate := evalSessionMetricAggregate{
		N:                len(sortedValues),
		Mean:             meanValue,
		Median:           median(sortedValues),
		StdDev:           stdDev,
		Min:              minValue,
		Max:              maxValue,
		HighVariance:     highVariance,
		HighVarianceRule: evalSessionHighVarianceRule,
	}
	if len(sortedValues) > 1 {
		margin := 1.96 * stdDev / math.Sqrt(float64(len(sortedValues)))
		aggregate.Interval = &evalSessionAggregateInterval{
			Estimator: evalSessionAggregateIntervalEstimator,
			Lower:     meanValue - margin,
			Upper:     meanValue + margin,
		}
	}
	return aggregate
}

func sampleStdDev(values []float64, meanValue float64) float64 {
	if len(values) < 2 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		diff := value - meanValue
		total += diff * diff
	}
	return math.Sqrt(total / float64(len(values)-1))
}

func kahanMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	compensation := 0.0
	for _, value := range values {
		y := value - compensation
		t := sum + y
		compensation = (t - sum) - y
		sum = t
	}
	return sum / float64(len(values))
}

func median(values []float64) float64 {
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid]
	}
	return (values[mid-1] + values[mid]) / 2
}

func sortedMetricKeys(values map[string][]float64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func dedupeSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	var previous string
	for idx, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if idx == 0 || value != previous {
			result = append(result, value)
			previous = value
		}
	}
	return result
}

func mapEvalSessionAggregateRecord(row repositorysqlc.EvalSessionResult) (EvalSessionAggregateRecord, error) {
	computedAt, err := requiredTime("eval_session_results.computed_at", row.ComputedAt)
	if err != nil {
		return EvalSessionAggregateRecord{}, err
	}
	createdAt, err := requiredTime("eval_session_results.created_at", row.CreatedAt)
	if err != nil {
		return EvalSessionAggregateRecord{}, err
	}
	updatedAt, err := requiredTime("eval_session_results.updated_at", row.UpdatedAt)
	if err != nil {
		return EvalSessionAggregateRecord{}, err
	}

	return EvalSessionAggregateRecord{
		ID:               row.ID,
		EvalSessionID:    row.EvalSessionID,
		SchemaVersion:    row.SchemaVersion,
		ChildRunCount:    row.ChildRunCount,
		ScoredChildCount: row.ScoredChildCount,
		Aggregate:        cloneJSON(row.Aggregate),
		Evidence:         cloneJSON(row.Evidence),
		ComputedAt:       computedAt,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}, nil
}
