package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	evalSessionAggregateSchemaVersion     = 1
	evalSessionAggregateIntervalEstimator = "normal_95"
	evalSessionHighVarianceRule           = "stddev/abs(mean)>0.15_or_range>0.10"
	evalSessionDefaultSuccessThreshold    = 0.8
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
	TaskSuccess      []evalSessionTaskSuccess              `json:"task_success,omitempty"`
	PassAtK          *evalSessionPassMetricSeries          `json:"pass_at_k,omitempty"`
	PassPowK         *evalSessionPassMetricSeries          `json:"pass_pow_k,omitempty"`
	MetricRouting    *evalSessionMetricRouting             `json:"metric_routing,omitempty"`
	Participants     []evalSessionParticipantAggregate     `json:"participants,omitempty"`
	Comparison       *evalSessionRepeatedComparison        `json:"comparison,omitempty"`
}

type evalSessionParticipantAggregate struct {
	LaneIndex     int32                                 `json:"lane_index"`
	Label         string                                `json:"label"`
	Overall       *evalSessionMetricAggregate           `json:"overall,omitempty"`
	Dimensions    map[string]evalSessionMetricAggregate `json:"dimensions,omitempty"`
	TaskSuccess   []evalSessionTaskSuccess              `json:"task_success,omitempty"`
	PassAtK       *evalSessionPassMetricSeries          `json:"pass_at_k,omitempty"`
	PassPowK      *evalSessionPassMetricSeries          `json:"pass_pow_k,omitempty"`
	MetricRouting *evalSessionMetricRouting             `json:"metric_routing,omitempty"`
}

type evalSessionTaskSuccess struct {
	TaskKey             string             `json:"task_key"`
	ChallengeIdentityID *uuid.UUID         `json:"challenge_identity_id,omitempty"`
	ChallengeKey        string             `json:"challenge_key,omitempty"`
	Title               string             `json:"title,omitempty"`
	ObservedTrials      int                `json:"observed_trials"`
	SuccessfulTrials    int                `json:"successful_trials"`
	SuccessRate         float64            `json:"success_rate"`
	Source              string             `json:"source"`
	PassAtK             map[string]float64 `json:"pass_at_k,omitempty"`
	PassPowK            map[string]float64 `json:"pass_pow_k,omitempty"`
}

type evalSessionPassMetricSeries struct {
	EffectiveK int                                   `json:"effective_k"`
	ByK        map[string]evalSessionMetricAggregate `json:"by_k,omitempty"`
}

type evalSessionMetricRouting struct {
	Source              string                        `json:"source"`
	ReliabilityWeight   float64                       `json:"reliability_weight"`
	Reasoning           string                        `json:"reasoning"`
	PrimaryMetric       string                        `json:"primary_metric"`
	EffectiveK          int                           `json:"effective_k"`
	CompositeAgentScore float64                       `json:"composite_agent_score"`
	CompositeInterval   *evalSessionAggregateInterval `json:"composite_interval,omitempty"`
}

type evalSessionRepeatedComparison struct {
	Status            string                        `json:"status"`
	ReasonCode        string                        `json:"reason_code,omitempty"`
	ComparedMetric    string                        `json:"compared_metric,omitempty"`
	EffectiveK        int                           `json:"effective_k"`
	WinnerLaneIndex   *int32                        `json:"winner_lane_index,omitempty"`
	WinnerLabel       string                        `json:"winner_label,omitempty"`
	LeaderLaneIndex   *int32                        `json:"leader_lane_index,omitempty"`
	LeaderLabel       string                        `json:"leader_label,omitempty"`
	LeaderValue       *float64                      `json:"leader_value,omitempty"`
	LeaderInterval    *evalSessionAggregateInterval `json:"leader_interval,omitempty"`
	RunnerUpLaneIndex *int32                        `json:"runner_up_lane_index,omitempty"`
	RunnerUpLabel     string                        `json:"runner_up_label,omitempty"`
	RunnerUpValue     *float64                      `json:"runner_up_value,omitempty"`
	RunnerUpInterval  *evalSessionAggregateInterval `json:"runner_up_interval,omitempty"`
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
	RunID              uuid.UUID
	Document           runScorecardDocument
	ParticipantSources []evalSessionAggregateParticipantSource
}

type evalSessionParticipantKey struct {
	LaneIndex int32
	Label     string
}

type evalSessionAggregateParticipantSource struct {
	Key          evalSessionParticipantKey
	Agent        runScorecardAgentSummary
	TaskOutcomes []evalSessionAggregateTaskOutcome
}

type evalSessionAggregateTaskOutcome struct {
	TaskKey             string
	ChallengeIdentityID *uuid.UUID
	ChallengeKey        string
	Title               string
	Success             bool
	Source              string
}

type evalSessionParticipantAccumulator struct {
	Key          evalSessionParticipantKey
	Overall      []float64
	Dimensions   map[string][]float64
	TaskOutcomes map[string]*evalSessionTaskOutcomeAccumulator
}

type evalSessionTaskOutcomeAccumulator struct {
	TaskKey             string
	ChallengeIdentityID *uuid.UUID
	ChallengeKey        string
	Title               string
	Source              string
	Outcomes            []bool
}

type evalSessionAggregateBehavior struct {
	KValues                 []int
	EffectiveK              int
	SuccessThreshold        float64
	RequireAllDimensions    []string
	ManualReliabilityWeight *float64
	TaskProperties          evalSessionRoutingTaskProperties
}

type evalSessionAggregateConfigSnapshot struct {
	Method             string   `json:"method"`
	ReportVariance     bool     `json:"report_variance"`
	ConfidenceInterval float64  `json:"confidence_interval"`
	ReliabilityWeight  *float64 `json:"reliability_weight,omitempty"`
}

type evalSessionSuccessThresholdSnapshot struct {
	MinPassRate          *float64 `json:"min_pass_rate,omitempty"`
	RequireAllDimensions []string `json:"require_all_dimensions,omitempty"`
}

type evalSessionRoutingTaskDocument struct {
	Task json.RawMessage `json:"task"`
}

type evalSessionRoutingTaskProperties struct {
	HasSideEffects bool
	Autonomy       string
	StepCount      int
	OutputType     string
}

type evalSessionComparableParticipant struct {
	Index     int
	LaneIndex int32
	Label     string
	PassAtK   evalSessionMetricAggregate
	PassPowK  evalSessionMetricAggregate
	Routing   evalSessionMetricRouting
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
	session, err := r.GetEvalSessionByID(ctx, evalSessionID)
	if err != nil {
		return EvalSessionAggregateRecord{}, err
	}
	behavior, err := buildEvalSessionAggregateBehavior(session)
	if err != nil {
		return EvalSessionAggregateRecord{}, fmt.Errorf("build eval session aggregate behavior: %w", err)
	}

	runs, err := r.ListRunsByEvalSessionID(ctx, evalSessionID)
	if err != nil {
		return EvalSessionAggregateRecord{}, err
	}

	missingScorecardRunIDs := make([]uuid.UUID, 0)
	sources := make([]evalSessionAggregateSource, 0, len(runs))
	warnings := make([]string, 0)
	for _, run := range runs {
		scorecard, scorecardErr := r.GetRunScorecardByRunID(ctx, run.ID)
		switch {
		case scorecardErr == nil:
			var document runScorecardDocument
			if err := json.Unmarshal(scorecard.Scorecard, &document); err != nil {
				return EvalSessionAggregateRecord{}, fmt.Errorf("decode run scorecard %s: %w", run.ID, err)
			}
			participantSources, participantWarnings, err := r.buildEvalSessionAggregateParticipantSources(ctx, run.ID, document, behavior)
			if err != nil {
				return EvalSessionAggregateRecord{}, fmt.Errorf("build eval session participant sources for run %s: %w", run.ID, err)
			}
			warnings = append(warnings, participantWarnings...)
			sources = append(sources, evalSessionAggregateSource{
				RunID:              run.ID,
				Document:           document,
				ParticipantSources: participantSources,
			})
		case errors.Is(scorecardErr, ErrRunScorecardNotFound):
			missingScorecardRunIDs = append(missingScorecardRunIDs, run.ID)
		default:
			return EvalSessionAggregateRecord{}, fmt.Errorf("load run scorecard %s: %w", run.ID, scorecardErr)
		}
	}

	aggregate, evidence, scoredChildCount, err := buildEvalSessionAggregatePayload(len(runs), sources, missingScorecardRunIDs, behavior, warnings)
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

func buildEvalSessionAggregateBehavior(session domain.EvalSession) (evalSessionAggregateBehavior, error) {
	behavior := evalSessionAggregateBehavior{
		KValues:          evalSessionKValues(int(session.Repetitions)),
		EffectiveK:       max(int(session.Repetitions), 1),
		SuccessThreshold: evalSessionDefaultSuccessThreshold,
	}

	if len(strings.TrimSpace(string(session.AggregationConfig.Document))) > 0 {
		var aggregation evalSessionAggregateConfigSnapshot
		if err := json.Unmarshal(session.AggregationConfig.Document, &aggregation); err != nil {
			return evalSessionAggregateBehavior{}, fmt.Errorf("decode aggregation config: %w", err)
		}
		behavior.ManualReliabilityWeight = cloneFloat64Ptr(aggregation.ReliabilityWeight)
	}

	if len(strings.TrimSpace(string(session.SuccessThresholdConfig.Document))) > 0 {
		var threshold evalSessionSuccessThresholdSnapshot
		if err := json.Unmarshal(session.SuccessThresholdConfig.Document, &threshold); err != nil {
			return evalSessionAggregateBehavior{}, fmt.Errorf("decode success threshold config: %w", err)
		}
		if threshold.MinPassRate != nil {
			behavior.SuccessThreshold = *threshold.MinPassRate
		}
		behavior.RequireAllDimensions = uniqueSortedStrings(append([]string(nil), threshold.RequireAllDimensions...))
	}

	taskProperties, err := buildEvalSessionRoutingTaskProperties(session.RoutingTaskSnapshot.Document)
	if err != nil {
		return evalSessionAggregateBehavior{}, err
	}
	behavior.TaskProperties = taskProperties

	return behavior, nil
}

func buildEvalSessionRoutingTaskProperties(payload json.RawMessage) (evalSessionRoutingTaskProperties, error) {
	if len(strings.TrimSpace(string(payload))) == 0 || string(payload) == "{}" {
		return evalSessionRoutingTaskProperties{}, nil
	}

	var document map[string]any
	if err := json.Unmarshal(payload, &document); err != nil {
		return evalSessionRoutingTaskProperties{}, fmt.Errorf("decode routing task snapshot: %w", err)
	}
	taskBody, ok := mapValue(document["task"])
	if !ok {
		return evalSessionRoutingTaskProperties{}, nil
	}
	properties, ok := mapValue(taskBody["task_properties"])
	if !ok {
		return evalSessionRoutingTaskProperties{}, nil
	}

	return evalSessionRoutingTaskProperties{
		HasSideEffects: boolValue(properties["has_side_effects"]),
		Autonomy:       strings.ToLower(stringValue(properties["autonomy"])),
		StepCount:      intValue(properties["step_count"]),
		OutputType:     strings.ToLower(stringValue(properties["output_type"])),
	}, nil
}

func evalSessionKValues(repetitions int) []int {
	set := map[int]struct{}{
		1:  {},
		3:  {},
		5:  {},
		10: {},
	}
	if repetitions > 0 {
		set[repetitions] = struct{}{}
	}
	values := make([]int, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Ints(values)
	return values
}

func mapValue(value any) (map[string]any, bool) {
	result, ok := value.(map[string]any)
	return result, ok
}

func boolValue(value any) bool {
	result, ok := value.(bool)
	return ok && result
}

func stringValue(value any) string {
	result, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(result)
}

func intValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	default:
		return 0
	}
}

func (r *Repository) buildEvalSessionAggregateParticipantSources(
	ctx context.Context,
	runID uuid.UUID,
	document runScorecardDocument,
	behavior evalSessionAggregateBehavior,
) ([]evalSessionAggregateParticipantSource, []string, error) {
	participantSources := make([]evalSessionAggregateParticipantSource, 0, len(document.Agents))
	warnings := make([]string, 0)

	for _, agent := range document.Agents {
		participantSource := evalSessionAggregateParticipantSource{
			Key: evalSessionParticipantKey{
				LaneIndex: agent.LaneIndex,
				Label:     agent.Label,
			},
			Agent: agent,
		}

		if agent.HasScorecard {
			taskOutcomes, taskWarnings, err := r.loadEvalSessionParticipantTaskOutcomes(ctx, document.EvaluationSpecID, agent, behavior)
			if err != nil {
				return nil, nil, fmt.Errorf("resolve participant %q (lane %d) task outcomes: %w", agent.Label, agent.LaneIndex, err)
			}
			participantSource.TaskOutcomes = taskOutcomes
			for _, warning := range taskWarnings {
				warnings = append(warnings, fmt.Sprintf("run %s participant %q (lane %d): %s", runID, agent.Label, agent.LaneIndex, warning))
			}
		}

		participantSources = append(participantSources, participantSource)
	}

	return participantSources, warnings, nil
}

func (r *Repository) loadEvalSessionParticipantTaskOutcomes(
	ctx context.Context,
	evaluationSpecID uuid.UUID,
	agent runScorecardAgentSummary,
	behavior evalSessionAggregateBehavior,
) ([]evalSessionAggregateTaskOutcome, []string, error) {
	warnings := make([]string, 0)
	challengeMetadata := map[uuid.UUID]ChallengeDefinitionExecutionContext{}

	executionContext, err := r.GetRunAgentExecutionContextByID(ctx, agent.RunAgentID)
	if err == nil {
		challengeMetadata = buildEvalSessionChallengeMetadata(executionContext)
	} else {
		warnings = append(warnings, fmt.Sprintf("challenge metadata unavailable (%v); task summaries will fall back to challenge ids when possible", err))
	}

	if evaluationSpecID != uuid.Nil {
		judgeResults, err := r.ListJudgeResultsByRunAgentAndEvaluationSpec(ctx, agent.RunAgentID, evaluationSpecID)
		if err != nil {
			return nil, nil, fmt.Errorf("list judge results: %w", err)
		}
		taskOutcomes := deriveEvalSessionChallengeTaskOutcomes(judgeResults, challengeMetadata, behavior.SuccessThreshold)
		if len(taskOutcomes) > 0 {
			return taskOutcomes, warnings, nil
		}
	}

	fallback, ok := deriveEvalSessionSuiteFallbackOutcome(agent, behavior)
	if ok {
		warnings = append(warnings, "challenge-level task outcomes unavailable; using suite-level scorecard fallback")
		return []evalSessionAggregateTaskOutcome{fallback}, warnings, nil
	}

	warnings = append(warnings, "task outcomes unavailable; neither challenge-level judge results nor suite-level scorecard fallback could be resolved")
	return nil, warnings, nil
}

func buildEvalSessionChallengeMetadata(executionContext RunAgentExecutionContext) map[uuid.UUID]ChallengeDefinitionExecutionContext {
	metadata := make(map[uuid.UUID]ChallengeDefinitionExecutionContext, len(executionContext.ChallengePackVersion.Challenges))
	for _, challenge := range executionContext.ChallengePackVersion.Challenges {
		metadata[challenge.ChallengeIdentityID] = challenge
	}
	return metadata
}

func deriveEvalSessionChallengeTaskOutcomes(
	results []JudgeResultRecord,
	challengeMetadata map[uuid.UUID]ChallengeDefinitionExecutionContext,
	threshold float64,
) []evalSessionAggregateTaskOutcome {
	grouped := map[uuid.UUID][]JudgeResultRecord{}
	for _, result := range results {
		if result.ChallengeIdentityID == nil {
			continue
		}
		grouped[*result.ChallengeIdentityID] = append(grouped[*result.ChallengeIdentityID], result)
	}

	challengeIDs := make([]uuid.UUID, 0, len(grouped))
	for challengeID := range grouped {
		challengeIDs = append(challengeIDs, challengeID)
	}
	sort.Slice(challengeIDs, func(i, j int) bool {
		left, leftOK := challengeMetadata[challengeIDs[i]]
		right, rightOK := challengeMetadata[challengeIDs[j]]
		switch {
		case leftOK && rightOK && left.ExecutionOrder != right.ExecutionOrder:
			return left.ExecutionOrder < right.ExecutionOrder
		case leftOK && rightOK && left.ChallengeKey != right.ChallengeKey:
			return left.ChallengeKey < right.ChallengeKey
		default:
			return challengeIDs[i].String() < challengeIDs[j].String()
		}
	})

	outcomes := make([]evalSessionAggregateTaskOutcome, 0, len(challengeIDs))
	for _, challengeID := range challengeIDs {
		success, source, ok := deriveEvalSessionChallengeSuccess(grouped[challengeID], threshold)
		if !ok {
			continue
		}
		taskKey := challengeID.String()
		challengeKey := ""
		title := ""
		if challenge, ok := challengeMetadata[challengeID]; ok {
			if strings.TrimSpace(challenge.ChallengeKey) != "" {
				taskKey = challenge.ChallengeKey
				challengeKey = challenge.ChallengeKey
			}
			title = strings.TrimSpace(challenge.Title)
		}
		challengeIDCopy := challengeID
		outcomes = append(outcomes, evalSessionAggregateTaskOutcome{
			TaskKey:             taskKey,
			ChallengeIdentityID: &challengeIDCopy,
			ChallengeKey:        challengeKey,
			Title:               title,
			Success:             success,
			Source:              source,
		})
	}

	return outcomes
}

func deriveEvalSessionChallengeSuccess(results []JudgeResultRecord, threshold float64) (bool, string, bool) {
	verdictSeen := false
	scores := make([]float64, 0, len(results))

	for _, result := range results {
		if result.Verdict != nil {
			verdict := strings.ToLower(strings.TrimSpace(*result.Verdict))
			if verdict != "" {
				verdictSeen = true
				if verdict != "pass" {
					return false, "judge_results_verdict", true
				}
			}
		}
		if result.NormalizedScore != nil && !verdictSeen {
			scores = append(scores, *result.NormalizedScore)
		}
	}

	if verdictSeen {
		return true, "judge_results_verdict", true
	}
	if len(scores) > 0 {
		return kahanMean(scores) >= threshold, "judge_results_threshold", true
	}
	return false, "", false
}

func deriveEvalSessionSuiteFallbackOutcome(agent runScorecardAgentSummary, behavior evalSessionAggregateBehavior) (evalSessionAggregateTaskOutcome, bool) {
	if agent.Passed != nil {
		return evalSessionAggregateTaskOutcome{
			TaskKey: "suite",
			Title:   "Suite-level fallback",
			Success: *agent.Passed,
			Source:  "scorecard_passed",
		}, true
	}
	if agent.OverallScore == nil {
		return evalSessionAggregateTaskOutcome{}, false
	}

	success := *agent.OverallScore >= behavior.SuccessThreshold
	if success && len(behavior.RequireAllDimensions) > 0 {
		dimensions := evalSessionAggregateDimensions(agent)
		for _, dimension := range behavior.RequireAllDimensions {
			score, ok := dimensions[dimension]
			if !ok || score < behavior.SuccessThreshold {
				success = false
				break
			}
		}
	}

	return evalSessionAggregateTaskOutcome{
		TaskKey: "suite",
		Title:   "Suite-level fallback",
		Success: success,
		Source:  "scorecard_threshold",
	}, true
}

func buildEvalSessionAggregatePayload(
	childRunCount int,
	sources []evalSessionAggregateSource,
	missingScorecardRunIDs []uuid.UUID,
	behavior evalSessionAggregateBehavior,
	extraWarnings []string,
) (json.RawMessage, json.RawMessage, int, error) {
	if len(sources) == 0 {
		return nil, nil, 0, ErrEvalSessionAggregateUnavailable
	}

	sort.Slice(sources, func(i, j int) bool {
		return sources[i].RunID.String() < sources[j].RunID.String()
	})
	sort.Slice(missingScorecardRunIDs, func(i, j int) bool {
		return missingScorecardRunIDs[i].String() < missingScorecardRunIDs[j].String()
	})

	warnings := append([]string(nil), extraWarnings...)
	participantAccumulators := map[evalSessionParticipantKey]*evalSessionParticipantAccumulator{}
	insufficientEvidence := false

	for _, source := range sources {
		for _, participantSource := range defaultEvalSessionParticipantSources(source) {
			agent := participantSource.Agent
			if !agent.HasScorecard {
				warnings = append(warnings, fmt.Sprintf("run %s participant %q (lane %d) has no scorecard", source.RunID, agent.Label, agent.LaneIndex))
				continue
			}
			key := participantSource.Key
			accumulator, ok := participantAccumulators[key]
			if !ok {
				accumulator = &evalSessionParticipantAccumulator{
					Key:          key,
					Dimensions:   map[string][]float64{},
					TaskOutcomes: map[string]*evalSessionTaskOutcomeAccumulator{},
				}
				participantAccumulators[key] = accumulator
			}
			if agent.OverallScore != nil {
				accumulator.Overall = append(accumulator.Overall, *agent.OverallScore)
			}
			for dimKey, value := range evalSessionAggregateDimensions(agent) {
				accumulator.Dimensions[dimKey] = append(accumulator.Dimensions[dimKey], value)
			}
			for _, taskOutcome := range participantSource.TaskOutcomes {
				taskKey := strings.TrimSpace(taskOutcome.TaskKey)
				if taskKey == "" {
					if taskOutcome.ChallengeIdentityID != nil {
						taskKey = taskOutcome.ChallengeIdentityID.String()
					} else {
						taskKey = "suite"
					}
				}
				taskAccumulator, ok := accumulator.TaskOutcomes[taskKey]
				if !ok {
					taskAccumulator = &evalSessionTaskOutcomeAccumulator{
						TaskKey:             taskKey,
						ChallengeIdentityID: cloneUUIDPtr(taskOutcome.ChallengeIdentityID),
						ChallengeKey:        taskOutcome.ChallengeKey,
						Title:               taskOutcome.Title,
						Source:              taskOutcome.Source,
					}
					accumulator.TaskOutcomes[taskKey] = taskAccumulator
				}
				if taskAccumulator.ChallengeIdentityID == nil {
					taskAccumulator.ChallengeIdentityID = cloneUUIDPtr(taskOutcome.ChallengeIdentityID)
				}
				if strings.TrimSpace(taskAccumulator.ChallengeKey) == "" {
					taskAccumulator.ChallengeKey = taskOutcome.ChallengeKey
				}
				if strings.TrimSpace(taskAccumulator.Title) == "" {
					taskAccumulator.Title = taskOutcome.Title
				}
				if taskAccumulator.Source == "" {
					taskAccumulator.Source = taskOutcome.Source
				} else if taskOutcome.Source != "" && taskAccumulator.Source != taskOutcome.Source {
					taskAccumulator.Source = "mixed"
				}
				taskAccumulator.Outcomes = append(taskAccumulator.Outcomes, taskOutcome.Success)
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
		if len(accumulator.TaskOutcomes) > 0 {
			participant.TaskSuccess = buildEvalSessionTaskSuccess(accumulator.TaskOutcomes, behavior.KValues)
			participant.PassAtK = buildEvalSessionPassMetricSeries(participant.TaskSuccess, behavior.KValues, behavior.EffectiveK, "pass_at_k")
			participant.PassPowK = buildEvalSessionPassMetricSeries(participant.TaskSuccess, behavior.KValues, behavior.EffectiveK, "pass_pow_k")
			participant.MetricRouting = buildEvalSessionMetricRouting(behavior, participant.PassAtK, participant.PassPowK)
			for _, task := range participant.TaskSuccess {
				if task.ObservedTrials < len(sources) {
					warnings = append(warnings, fmt.Sprintf("participant %q (lane %d) task %q uses %d of %d scored child runs", key.Label, key.LaneIndex, task.TaskKey, task.ObservedTrials, len(sources)))
				}
			}
		}
		document.Participants = append(document.Participants, participant)
	}

	if len(document.Participants) == 1 {
		participant := document.Participants[0]
		document.TopLevelSource = "sole_participant"
		document.Overall = participant.Overall
		document.Dimensions = participant.Dimensions
		document.TaskSuccess = participant.TaskSuccess
		document.PassAtK = participant.PassAtK
		document.PassPowK = participant.PassPowK
		document.MetricRouting = participant.MetricRouting
	} else if len(document.Participants) > 1 {
		document.Comparison = buildEvalSessionRepeatedComparison(document.Participants, behavior.EffectiveK, len(sources))
		if document.Comparison != nil && document.Comparison.Status == "clear_winner" && document.Comparison.WinnerLaneIndex != nil {
			if winner, ok := evalSessionParticipantByLane(document.Participants, *document.Comparison.WinnerLaneIndex); ok {
				document.TopLevelSource = "repeated_clear_winner"
				document.Overall = winner.Overall
				document.Dimensions = winner.Dimensions
			}
		} else {
			warnings = append(warnings, buildEvalSessionComparisonOmissionWarning(document.Comparison))
		}
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

func defaultEvalSessionParticipantSources(source evalSessionAggregateSource) []evalSessionAggregateParticipantSource {
	if len(source.ParticipantSources) > 0 {
		return append([]evalSessionAggregateParticipantSource(nil), source.ParticipantSources...)
	}
	participants := make([]evalSessionAggregateParticipantSource, 0, len(source.Document.Agents))
	for _, agent := range source.Document.Agents {
		participants = append(participants, evalSessionAggregateParticipantSource{
			Key: evalSessionParticipantKey{
				LaneIndex: agent.LaneIndex,
				Label:     agent.Label,
			},
			Agent: agent,
		})
	}
	return participants
}

func buildEvalSessionTaskSuccess(
	taskOutcomes map[string]*evalSessionTaskOutcomeAccumulator,
	kValues []int,
) []evalSessionTaskSuccess {
	taskKeys := make([]string, 0, len(taskOutcomes))
	for taskKey := range taskOutcomes {
		taskKeys = append(taskKeys, taskKey)
	}
	sort.Strings(taskKeys)

	results := make([]evalSessionTaskSuccess, 0, len(taskKeys))
	for _, taskKey := range taskKeys {
		taskOutcome := taskOutcomes[taskKey]
		if len(taskOutcome.Outcomes) == 0 {
			continue
		}
		successfulTrials := 0
		for _, outcome := range taskOutcome.Outcomes {
			if outcome {
				successfulTrials++
			}
		}
		observedTrials := len(taskOutcome.Outcomes)
		successRate := float64(successfulTrials) / float64(observedTrials)
		taskResult := evalSessionTaskSuccess{
			TaskKey:             taskOutcome.TaskKey,
			ChallengeIdentityID: cloneUUIDPtr(taskOutcome.ChallengeIdentityID),
			ChallengeKey:        taskOutcome.ChallengeKey,
			Title:               taskOutcome.Title,
			ObservedTrials:      observedTrials,
			SuccessfulTrials:    successfulTrials,
			SuccessRate:         successRate,
			Source:              taskOutcome.Source,
			PassAtK:             map[string]float64{},
			PassPowK:            map[string]float64{},
		}
		for _, k := range kValues {
			key := strconv.Itoa(k)
			taskResult.PassAtK[key] = 1 - math.Pow(1-successRate, float64(k))
			taskResult.PassPowK[key] = math.Pow(successRate, float64(k))
		}
		results = append(results, taskResult)
	}

	return results
}

func buildEvalSessionPassMetricSeries(
	taskSuccess []evalSessionTaskSuccess,
	kValues []int,
	effectiveK int,
	metric string,
) *evalSessionPassMetricSeries {
	if len(taskSuccess) == 0 {
		return nil
	}

	series := &evalSessionPassMetricSeries{
		EffectiveK: effectiveK,
		ByK:        map[string]evalSessionMetricAggregate{},
	}
	for _, k := range kValues {
		key := strconv.Itoa(k)
		values := make([]float64, 0, len(taskSuccess))
		for _, task := range taskSuccess {
			switch metric {
			case "pass_pow_k":
				if value, ok := task.PassPowK[key]; ok {
					values = append(values, value)
				}
			default:
				if value, ok := task.PassAtK[key]; ok {
					values = append(values, value)
				}
			}
		}
		if len(values) == 0 {
			continue
		}
		series.ByK[key] = buildEvalSessionMetricAggregate(values)
	}

	if len(series.ByK) == 0 {
		return nil
	}
	return series
}

func buildEvalSessionMetricRouting(
	behavior evalSessionAggregateBehavior,
	passAtK *evalSessionPassMetricSeries,
	passPowK *evalSessionPassMetricSeries,
) *evalSessionMetricRouting {
	passAtAggregate, ok := evalSessionPassMetricAggregateForK(passAtK, behavior.EffectiveK)
	if !ok {
		return nil
	}
	passPowAggregate, ok := evalSessionPassMetricAggregateForK(passPowK, behavior.EffectiveK)
	if !ok {
		return nil
	}

	source, weight, reasoning := resolveEvalSessionReliabilityWeight(behavior)
	primaryMetric := "pass_at_k"
	if weight >= 0.5 {
		primaryMetric = "pass_pow_k"
	}

	routing := &evalSessionMetricRouting{
		Source:              source,
		ReliabilityWeight:   weight,
		Reasoning:           reasoning,
		PrimaryMetric:       primaryMetric,
		EffectiveK:          behavior.EffectiveK,
		CompositeAgentScore: ((1 - weight) * passAtAggregate.Mean) + (weight * passPowAggregate.Mean),
	}
	if passAtAggregate.Interval != nil && passPowAggregate.Interval != nil {
		routing.CompositeInterval = &evalSessionAggregateInterval{
			Estimator: evalSessionAggregateIntervalEstimator,
			Lower:     ((1 - weight) * passAtAggregate.Interval.Lower) + (weight * passPowAggregate.Interval.Lower),
			Upper:     ((1 - weight) * passAtAggregate.Interval.Upper) + (weight * passPowAggregate.Interval.Upper),
		}
	}
	return routing
}

func resolveEvalSessionReliabilityWeight(behavior evalSessionAggregateBehavior) (string, float64, string) {
	if behavior.ManualReliabilityWeight != nil {
		weight := clamp01(*behavior.ManualReliabilityWeight)
		return "manual_override", weight, fmt.Sprintf("manual override from eval_session.aggregation.reliability_weight=%s", strconv.FormatFloat(weight, 'f', -1, 64))
	}

	weight := 0.0
	reasons := make([]string, 0, 4)
	props := behavior.TaskProperties

	if props.HasSideEffects {
		weight += 0.35
		reasons = append(reasons, "task has side effects")
	}
	switch props.Autonomy {
	case "full":
		weight += 0.30
		reasons = append(reasons, "task is fully autonomous")
	case "semi":
		weight += 0.15
		reasons = append(reasons, "task is semi-autonomous")
	}
	switch {
	case props.StepCount > 3:
		weight += 0.25
		reasons = append(reasons, fmt.Sprintf("task compounds reliability across %d steps", props.StepCount))
	case props.StepCount > 1:
		weight += 0.10
		reasons = append(reasons, fmt.Sprintf("task spans %d sequential steps", props.StepCount))
	}
	if props.OutputType == "action" {
		weight += 0.10
		reasons = append(reasons, "task output is an action, not a reviewable artifact")
	}

	weight = clamp01(weight)
	if len(reasons) == 0 {
		return "default", weight, "no reliability-sensitive task_properties were provided; defaulting toward capability-first pass@k"
	}
	return "task_properties", weight, strings.Join(reasons, "; ")
}

func evalSessionPassMetricAggregateForK(series *evalSessionPassMetricSeries, k int) (evalSessionMetricAggregate, bool) {
	if series == nil {
		return evalSessionMetricAggregate{}, false
	}
	aggregate, ok := series.ByK[strconv.Itoa(k)]
	return aggregate, ok
}

func buildEvalSessionRepeatedComparison(
	participants []evalSessionParticipantAggregate,
	effectiveK int,
	scoredChildCount int,
) *evalSessionRepeatedComparison {
	if len(participants) < 2 {
		return nil
	}

	comparables := make([]evalSessionComparableParticipant, 0, len(participants))
	for idx, participant := range participants {
		if participant.MetricRouting == nil {
			continue
		}
		passAtAggregate, passAtOK := evalSessionPassMetricAggregateForK(participant.PassAtK, effectiveK)
		passPowAggregate, passPowOK := evalSessionPassMetricAggregateForK(participant.PassPowK, effectiveK)
		if !passAtOK || !passPowOK {
			continue
		}
		comparables = append(comparables, evalSessionComparableParticipant{
			Index:     idx,
			LaneIndex: participant.LaneIndex,
			Label:     participant.Label,
			PassAtK:   passAtAggregate,
			PassPowK:  passPowAggregate,
			Routing:   *participant.MetricRouting,
		})
	}

	comparison := &evalSessionRepeatedComparison{
		Status:     "insufficient_evidence",
		ReasonCode: "participant_metrics_unavailable",
		EffectiveK: effectiveK,
	}
	if len(comparables) < 2 {
		return comparison
	}

	sort.Slice(comparables, func(i, j int) bool {
		if comparables[i].Routing.CompositeAgentScore != comparables[j].Routing.CompositeAgentScore {
			return comparables[i].Routing.CompositeAgentScore > comparables[j].Routing.CompositeAgentScore
		}
		if comparables[i].LaneIndex != comparables[j].LaneIndex {
			return comparables[i].LaneIndex < comparables[j].LaneIndex
		}
		return comparables[i].Label < comparables[j].Label
	})

	leader := comparables[0]
	runnerUp := comparables[1]
	comparison.ComparedMetric = leader.Routing.PrimaryMetric
	comparison.LeaderLaneIndex = &leader.LaneIndex
	comparison.LeaderLabel = leader.Label
	comparison.RunnerUpLaneIndex = &runnerUp.LaneIndex
	comparison.RunnerUpLabel = runnerUp.Label

	if leader.Routing.PrimaryMetric != runnerUp.Routing.PrimaryMetric {
		// Participants in one eval session currently share a single aggregate behavior,
		// so routing mismatches should be unreachable unless per-participant routing is added later.
		comparison.ReasonCode = "metric_routing_mismatch"
		return comparison
	}

	leaderAggregate, leaderValue := evalSessionComparisonMetric(leader, leader.Routing.PrimaryMetric)
	runnerUpAggregate, runnerUpValue := evalSessionComparisonMetric(runnerUp, runnerUp.Routing.PrimaryMetric)
	comparison.LeaderValue = &leaderValue
	comparison.RunnerUpValue = &runnerUpValue
	comparison.LeaderInterval = cloneEvalSessionAggregateInterval(leaderAggregate.Interval)
	comparison.RunnerUpInterval = cloneEvalSessionAggregateInterval(runnerUpAggregate.Interval)

	if scoredChildCount < 2 {
		comparison.ReasonCode = "scored_child_runs_insufficient"
		return comparison
	}
	if leaderAggregate.N < 2 || runnerUpAggregate.N < 2 || leaderAggregate.Interval == nil || runnerUpAggregate.Interval == nil {
		comparison.ReasonCode = "task_evidence_insufficient"
		return comparison
	}
	if intervalsOverlap(leaderAggregate.Interval, runnerUpAggregate.Interval) {
		comparison.Status = "no_clear_winner"
		comparison.ReasonCode = "interval_overlap"
		return comparison
	}

	comparison.Status = "clear_winner"
	comparison.ReasonCode = "non_overlapping_intervals"
	comparison.WinnerLaneIndex = &leader.LaneIndex
	comparison.WinnerLabel = leader.Label
	return comparison
}

func evalSessionComparisonMetric(
	participant evalSessionComparableParticipant,
	metric string,
) (evalSessionMetricAggregate, float64) {
	if metric == "pass_pow_k" {
		return participant.PassPowK, participant.PassPowK.Mean
	}
	return participant.PassAtK, participant.PassAtK.Mean
}

func evalSessionParticipantByLane(
	participants []evalSessionParticipantAggregate,
	laneIndex int32,
) (evalSessionParticipantAggregate, bool) {
	for _, participant := range participants {
		if participant.LaneIndex == laneIndex {
			return participant, true
		}
	}
	return evalSessionParticipantAggregate{}, false
}

func buildEvalSessionComparisonOmissionWarning(comparison *evalSessionRepeatedComparison) string {
	if comparison == nil {
		return "comparison session top-level winner aggregate omitted because repeated-session comparison evidence is unavailable"
	}
	switch comparison.Status {
	case "no_clear_winner":
		return "comparison session top-level winner aggregate omitted because repeated-session evidence overlaps and no clear winner exists"
	case "insufficient_evidence":
		return "comparison session top-level winner aggregate omitted because repeated-session evidence is insufficient"
	default:
		return "comparison session top-level winner aggregate omitted because repeated-session comparison is not definitive"
	}
}

func cloneEvalSessionAggregateInterval(interval *evalSessionAggregateInterval) *evalSessionAggregateInterval {
	if interval == nil {
		return nil
	}
	cloned := *interval
	return &cloned
}

func intervalsOverlap(left, right *evalSessionAggregateInterval) bool {
	if left == nil || right == nil {
		return true
	}
	return left.Lower <= right.Upper && right.Lower <= left.Upper
}

func clamp01(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
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
	if len(values) == 0 {
		return evalSessionMetricAggregate{}
	}

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
