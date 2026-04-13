package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

const runComparisonSummarySchemaVersion = "2026-03-17"

type RunComparisonStatus string

const (
	RunComparisonStatusComparable    RunComparisonStatus = "comparable"
	RunComparisonStatusNotComparable RunComparisonStatus = "not_comparable"
)

type BuildRunComparisonParams struct {
	BaselineRunID       uuid.UUID
	CandidateRunID      uuid.UUID
	BaselineRunAgentID  *uuid.UUID
	CandidateRunAgentID *uuid.UUID
}

type RunComparison struct {
	ID                  uuid.UUID
	BaselineRunID       uuid.UUID
	CandidateRunID      uuid.UUID
	BaselineRunAgentID  *uuid.UUID
	CandidateRunAgentID *uuid.UUID
	Status              RunComparisonStatus
	ReasonCode          *string
	SourceFingerprint   string
	Summary             json.RawMessage
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type runComparisonSummaryDocument struct {
	SchemaVersion           string                         `json:"schema_version"`
	Status                  RunComparisonStatus            `json:"status"`
	ReasonCode              string                         `json:"reason_code,omitempty"`
	BaselineRefs            runComparisonRefs              `json:"baseline_refs"`
	CandidateRefs           runComparisonRefs              `json:"candidate_refs"`
	MatchedParticipants     *runComparisonMatchedPair      `json:"matched_participants,omitempty"`
	DimensionDeltas         map[string]runComparisonDelta  `json:"dimension_deltas,omitempty"`
	FailureDivergence       runComparisonFailureDivergence `json:"failure_divergence"`
	ReplaySummaryDivergence runComparisonReplayDivergence  `json:"replay_summary_divergence"`
	EvidenceQuality         runComparisonEvidenceQuality   `json:"evidence_quality"`
}

type runComparisonRefs struct {
	RunID                  uuid.UUID  `json:"run_id"`
	RunAgentID             *uuid.UUID `json:"run_agent_id,omitempty"`
	ChallengePackVersionID uuid.UUID  `json:"challenge_pack_version_id"`
	ChallengeInputSetID    *uuid.UUID `json:"challenge_input_set_id,omitempty"`
	EvaluationSpecID       *uuid.UUID `json:"evaluation_spec_id,omitempty"`
}

type runComparisonMatchedPair struct {
	BaselineRunAgentID  uuid.UUID `json:"baseline_run_agent_id"`
	CandidateRunAgentID uuid.UUID `json:"candidate_run_agent_id"`
}

type runComparisonDelta struct {
	BaselineValue   *float64 `json:"baseline_value,omitempty"`
	CandidateValue  *float64 `json:"candidate_value,omitempty"`
	Delta           *float64 `json:"delta,omitempty"`
	BetterDirection string   `json:"better_direction"`
	State           string   `json:"state"`
}

type runComparisonCountDelta struct {
	BaselineValue  *int64 `json:"baseline_value,omitempty"`
	CandidateValue *int64 `json:"candidate_value,omitempty"`
	Delta          *int64 `json:"delta,omitempty"`
	State          string `json:"state"`
}

type runComparisonFailureDivergence struct {
	BaselineRunAgentStatus        domain.RunAgentStatus `json:"baseline_run_agent_status"`
	CandidateRunAgentStatus       domain.RunAgentStatus `json:"candidate_run_agent_status"`
	BaselineTerminalReplayStatus  *string               `json:"baseline_terminal_replay_status,omitempty"`
	CandidateTerminalReplayStatus *string               `json:"candidate_terminal_replay_status,omitempty"`
	BaselineFailureReason         *string               `json:"baseline_failure_reason,omitempty"`
	CandidateFailureReason        *string               `json:"candidate_failure_reason,omitempty"`
	CandidateFailedBaselineOK     bool                  `json:"candidate_failed_baseline_succeeded"`
	CandidateOKBaselineFailed     bool                  `json:"candidate_succeeded_baseline_failed"`
	BothFailedDifferently         bool                  `json:"both_failed_differently"`
}

type runComparisonReplayDivergence struct {
	State                  string                             `json:"state"`
	BaselineStatus         *string                            `json:"baseline_status,omitempty"`
	CandidateStatus        *string                            `json:"candidate_status,omitempty"`
	BaselineHeadline       *string                            `json:"baseline_headline,omitempty"`
	CandidateHeadline      *string                            `json:"candidate_headline,omitempty"`
	BaselineTerminalEvent  *string                            `json:"baseline_terminal_event_type,omitempty"`
	CandidateTerminalEvent *string                            `json:"candidate_terminal_event_type,omitempty"`
	Counts                 map[string]runComparisonCountDelta `json:"counts,omitempty"`
}

type runComparisonEvidenceQuality struct {
	MissingFields []string `json:"missing_fields,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

type comparisonScorecardDocument struct {
	Status        string                                      `json:"status"`
	Strategy      string                                      `json:"strategy,omitempty"`
	OverallScore  *float64                                    `json:"overall_score,omitempty"`
	Passed        *bool                                       `json:"passed,omitempty"`
	OverallReason string                                      `json:"overall_reason,omitempty"`
	Dimensions    map[string]comparisonScorecardDimensionInfo `json:"dimensions"`
}

type comparisonScorecardDimensionInfo struct {
	State           string   `json:"state"`
	Score           *float64 `json:"score,omitempty"`
	BetterDirection string   `json:"better_direction,omitempty"`
}

type comparisonReplaySummaryDocument struct {
	Status        string                              `json:"status"`
	Headline      string                              `json:"headline"`
	Counts        comparisonReplaySummaryCounts       `json:"counts"`
	TerminalState *comparisonReplaySummaryTerminalRef `json:"terminal_state,omitempty"`
}

type comparisonReplaySummaryCounts struct {
	Events          int64 `json:"events"`
	ReplaySteps     int64 `json:"replay_steps"`
	ModelCalls      int64 `json:"model_calls"`
	ToolCalls       int64 `json:"tool_calls"`
	SandboxCommands int64 `json:"sandbox_commands"`
	Outputs         int64 `json:"outputs"`
	ScoringEvents   int64 `json:"scoring_events"`
}

type comparisonReplaySummaryTerminalRef struct {
	Status    string `json:"status"`
	EventType string `json:"event_type"`
}

type selectedRunComparisonParticipant struct {
	runAgent         domain.RunAgent
	executionContext RunAgentExecutionContext
	scorecard        RunAgentScorecard
	replay           *RunAgentReplay
}

func (r *Repository) GetRunComparisonByRunIDs(ctx context.Context, baselineRunID uuid.UUID, candidateRunID uuid.UUID) (RunComparison, error) {
	row, err := r.queries.GetRunComparisonByRunIDs(ctx, repositorysqlc.GetRunComparisonByRunIDsParams{
		BaselineRunID:  baselineRunID,
		CandidateRunID: candidateRunID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RunComparison{}, ErrRunComparisonNotFound
		}
		return RunComparison{}, fmt.Errorf("get run comparison by run ids: %w", err)
	}

	record, err := mapRunComparison(row)
	if err != nil {
		return RunComparison{}, fmt.Errorf("map run comparison: %w", err)
	}

	return record, nil
}

func (r *Repository) BuildRunComparison(ctx context.Context, params BuildRunComparisonParams) (RunComparison, error) {
	if params.BaselineRunID == params.CandidateRunID {
		return RunComparison{}, fmt.Errorf("baseline and candidate run ids must differ")
	}

	baselineRun, err := r.GetRunByID(ctx, params.BaselineRunID)
	if err != nil {
		return RunComparison{}, err
	}
	candidateRun, err := r.GetRunByID(ctx, params.CandidateRunID)
	if err != nil {
		return RunComparison{}, err
	}

	baselineRunAgents, err := r.ListRunAgentsByRunID(ctx, baselineRun.ID)
	if err != nil {
		return RunComparison{}, fmt.Errorf("list baseline run agents: %w", err)
	}
	candidateRunAgents, err := r.ListRunAgentsByRunID(ctx, candidateRun.ID)
	if err != nil {
		return RunComparison{}, fmt.Errorf("list candidate run agents: %w", err)
	}

	if len(baselineRunAgents) == 0 {
		return r.upsertNonComparableRunComparison(ctx, baselineRun, candidateRun, nil, nil, "baseline_run_agent_unresolved", nil)
	}
	if len(candidateRunAgents) == 0 {
		return r.upsertNonComparableRunComparison(ctx, baselineRun, candidateRun, nil, nil, "candidate_run_agent_unresolved", nil)
	}

	baselineSelected, candidateSelected, reasonCode, warnings, err := r.resolveRunComparisonParticipants(
		ctx,
		baselineRunAgents,
		candidateRunAgents,
		params,
	)
	if err != nil {
		return RunComparison{}, err
	}
	if reasonCode != "" {
		var baselineSelectedID *uuid.UUID
		var candidateSelectedID *uuid.UUID
		if baselineSelected != nil {
			baselineSelectedID = &baselineSelected.runAgent.ID
		}
		if candidateSelected != nil {
			candidateSelectedID = &candidateSelected.runAgent.ID
		}
		return r.upsertNonComparableRunComparison(ctx, baselineRun, candidateRun, baselineSelectedID, candidateSelectedID, reasonCode, warnings)
	}

	if baselineSelected.executionContext.Run.ChallengePackVersionID != candidateSelected.executionContext.Run.ChallengePackVersionID {
		return r.upsertNonComparableRunComparison(ctx, baselineRun, candidateRun, &baselineSelected.runAgent.ID, &candidateSelected.runAgent.ID, "challenge_pack_version_mismatch", warnings)
	}
	if !sameOptionalUUID(
		baselineSelected.executionContext.Run.ChallengeInputSetID,
		candidateSelected.executionContext.Run.ChallengeInputSetID,
	) {
		return r.upsertNonComparableRunComparison(ctx, baselineRun, candidateRun, &baselineSelected.runAgent.ID, &candidateSelected.runAgent.ID, "challenge_input_set_mismatch", warnings)
	}
	if baselineSelected.scorecard.EvaluationSpecID != candidateSelected.scorecard.EvaluationSpecID {
		return r.upsertNonComparableRunComparison(ctx, baselineRun, candidateRun, &baselineSelected.runAgent.ID, &candidateSelected.runAgent.ID, "evaluation_spec_mismatch", warnings)
	}

	baselineCoverage, err := r.listChallengeCoverageSet(ctx, baselineSelected.runAgent.ID, baselineSelected.scorecard.EvaluationSpecID)
	if err != nil {
		return RunComparison{}, err
	}
	candidateCoverage, err := r.listChallengeCoverageSet(ctx, candidateSelected.runAgent.ID, candidateSelected.scorecard.EvaluationSpecID)
	if err != nil {
		return RunComparison{}, err
	}
	if !equalStringSlices(baselineCoverage, candidateCoverage) {
		return r.upsertNonComparableRunComparison(ctx, baselineRun, candidateRun, &baselineSelected.runAgent.ID, &candidateSelected.runAgent.ID, "challenge_coverage_mismatch", warnings)
	}

	summary, fingerprint, err := buildComparableRunComparisonSummary(
		baselineRun,
		candidateRun,
		*baselineSelected,
		*candidateSelected,
		warnings,
	)
	if err != nil {
		return RunComparison{}, fmt.Errorf("build comparable run comparison summary: %w", err)
	}

	return r.upsertRunComparisonRecord(
		ctx,
		baselineRun.ID,
		candidateRun.ID,
		&baselineSelected.runAgent.ID,
		&candidateSelected.runAgent.ID,
		RunComparisonStatusComparable,
		nil,
		fingerprint,
		summary,
	)
}

func (r *Repository) resolveRunComparisonParticipants(
	ctx context.Context,
	baselineRunAgents []domain.RunAgent,
	candidateRunAgents []domain.RunAgent,
	params BuildRunComparisonParams,
) (*selectedRunComparisonParticipant, *selectedRunComparisonParticipant, string, []string, error) {
	if params.BaselineRunAgentID == nil && params.CandidateRunAgentID == nil {
		if len(baselineRunAgents) != len(candidateRunAgents) {
			return nil, nil, "participant_count_mismatch", nil, nil
		}
		if len(baselineRunAgents) != 1 {
			return nil, nil, "cross_run_participant_matching_unsupported", nil, nil
		}
		params.BaselineRunAgentID = &baselineRunAgents[0].ID
		params.CandidateRunAgentID = &candidateRunAgents[0].ID
	}
	if params.BaselineRunAgentID == nil || params.CandidateRunAgentID == nil {
		return nil, nil, "participant_match_not_resolved", nil, nil
	}

	baselineRunAgent, ok := findRunAgentByID(baselineRunAgents, *params.BaselineRunAgentID)
	if !ok {
		return nil, nil, "baseline_run_agent_unresolved", nil, nil
	}
	candidateRunAgent, ok := findRunAgentByID(candidateRunAgents, *params.CandidateRunAgentID)
	if !ok {
		return nil, nil, "candidate_run_agent_unresolved", nil, nil
	}

	baselineSelected, warnings, err := r.loadRunComparisonParticipant(ctx, baselineRunAgent)
	if err != nil {
		if errors.Is(err, ErrFrozenExecutionContext) {
			return nil, nil, "frozen_context_invalid", nil, nil
		}
		if errors.Is(err, ErrRunAgentScorecardNotFound) {
			return nil, nil, "missing_scorecard", nil, nil
		}
		return nil, nil, "", nil, fmt.Errorf("load baseline comparison participant: %w", err)
	}
	candidateSelected, candidateWarnings, err := r.loadRunComparisonParticipant(ctx, candidateRunAgent)
	if err != nil {
		if errors.Is(err, ErrFrozenExecutionContext) {
			return nil, nil, "frozen_context_invalid", nil, nil
		}
		if errors.Is(err, ErrRunAgentScorecardNotFound) {
			return nil, nil, "missing_scorecard", nil, nil
		}
		return nil, nil, "", nil, fmt.Errorf("load candidate comparison participant: %w", err)
	}
	warnings = append(warnings, candidateWarnings...)

	return baselineSelected, candidateSelected, "", uniqueSortedStrings(warnings), nil
}

func (r *Repository) loadRunComparisonParticipant(
	ctx context.Context,
	runAgent domain.RunAgent,
) (*selectedRunComparisonParticipant, []string, error) {
	executionContext, err := r.GetRunAgentExecutionContextByID(ctx, runAgent.ID)
	if err != nil {
		return nil, nil, err
	}
	scorecard, err := r.GetRunAgentScorecardByRunAgentID(ctx, runAgent.ID)
	if err != nil {
		return nil, nil, err
	}
	replay, err := r.GetRunAgentReplayByRunAgentID(ctx, runAgent.ID)
	if err != nil {
		if errors.Is(err, ErrRunAgentReplayNotFound) {
			return &selectedRunComparisonParticipant{
				runAgent:         runAgent,
				executionContext: executionContext,
				scorecard:        scorecard,
			}, []string{"replay summary unavailable"}, nil
		}
		return nil, nil, err
	}

	return &selectedRunComparisonParticipant{
		runAgent:         runAgent,
		executionContext: executionContext,
		scorecard:        scorecard,
		replay:           &replay,
	}, nil, nil
}

func (r *Repository) listChallengeCoverageSet(ctx context.Context, runAgentID uuid.UUID, evaluationSpecID uuid.UUID) ([]string, error) {
	judgeResults, err := r.ListJudgeResultsByRunAgentAndEvaluationSpec(ctx, runAgentID, evaluationSpecID)
	if err != nil {
		return nil, fmt.Errorf("list judge coverage: %w", err)
	}
	metricResults, err := r.ListMetricResultsByRunAgentAndEvaluationSpec(ctx, runAgentID, evaluationSpecID)
	if err != nil {
		return nil, fmt.Errorf("list metric coverage: %w", err)
	}

	set := make(map[string]struct{})
	for _, result := range judgeResults {
		key := challengeCoverageKey(result.ChallengeIdentityID)
		set[key] = struct{}{}
	}
	for _, result := range metricResults {
		key := challengeCoverageKey(result.ChallengeIdentityID)
		set[key] = struct{}{}
	}

	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

func challengeCoverageKey(challengeIdentityID *uuid.UUID) string {
	if challengeIdentityID == nil {
		return "nil"
	}
	return challengeIdentityID.String()
}

func buildComparableRunComparisonSummary(
	baselineRun domain.Run,
	candidateRun domain.Run,
	baseline selectedRunComparisonParticipant,
	candidate selectedRunComparisonParticipant,
	warnings []string,
) (json.RawMessage, string, error) {
	baselineScorecardDoc, err := decodeComparisonScorecard(baseline.scorecard.Scorecard)
	if err != nil {
		return nil, "", fmt.Errorf("decode baseline scorecard: %w", err)
	}
	candidateScorecardDoc, err := decodeComparisonScorecard(candidate.scorecard.Scorecard)
	if err != nil {
		return nil, "", fmt.Errorf("decode candidate scorecard: %w", err)
	}

	missingFields := make([]string, 0)
	dimensionDeltas := buildRunComparisonDimensionDeltas(
		&baseline.scorecard,
		&candidate.scorecard,
		baselineScorecardDoc.Dimensions,
		candidateScorecardDoc.Dimensions,
		&missingFields,
	)

	replayDivergence, replayWarnings, replayMissing, err := buildReplaySummaryDivergence(baseline.replay, candidate.replay)
	if err != nil {
		return nil, "", fmt.Errorf("build replay summary divergence: %w", err)
	}
	warnings = append(warnings, replayWarnings...)
	missingFields = append(missingFields, replayMissing...)

	summary := runComparisonSummaryDocument{
		SchemaVersion: runComparisonSummarySchemaVersion,
		Status:        RunComparisonStatusComparable,
		BaselineRefs: runComparisonRefs{
			RunID:                  baselineRun.ID,
			RunAgentID:             &baseline.runAgent.ID,
			ChallengePackVersionID: baseline.executionContext.Run.ChallengePackVersionID,
			ChallengeInputSetID:    cloneUUIDPtr(baseline.executionContext.Run.ChallengeInputSetID),
			EvaluationSpecID:       &baseline.scorecard.EvaluationSpecID,
		},
		CandidateRefs: runComparisonRefs{
			RunID:                  candidateRun.ID,
			RunAgentID:             &candidate.runAgent.ID,
			ChallengePackVersionID: candidate.executionContext.Run.ChallengePackVersionID,
			ChallengeInputSetID:    cloneUUIDPtr(candidate.executionContext.Run.ChallengeInputSetID),
			EvaluationSpecID:       &candidate.scorecard.EvaluationSpecID,
		},
		MatchedParticipants: &runComparisonMatchedPair{
			BaselineRunAgentID:  baseline.runAgent.ID,
			CandidateRunAgentID: candidate.runAgent.ID,
		},
		DimensionDeltas:         dimensionDeltas,
		ReplaySummaryDivergence: replayDivergence,
	}
	failureDivergence, failureWarnings := buildFailureDivergence(baseline.runAgent, candidate.runAgent, baseline.replay, candidate.replay)
	warnings = append(warnings, failureWarnings...)
	summary.FailureDivergence = failureDivergence
	summary.EvidenceQuality = runComparisonEvidenceQuality{
		MissingFields: uniqueSortedStrings(missingFields),
		Warnings:      uniqueSortedStrings(warnings),
	}

	encoded, err := json.Marshal(summary)
	if err != nil {
		return nil, "", err
	}

	fingerprint, err := buildRunComparisonFingerprint(summary, baseline, candidate)
	if err != nil {
		return nil, "", err
	}
	return encoded, fingerprint, nil
}

// buildRunComparisonDimensionDeltas walks the union of dimension keys that
// appear in either participant's scorecard JSONB and emits one delta per key.
// Built-in dims fall back to their typed scorecard columns so legacy
// comparisons keep working even if the JSONB lacks a score. Direction is
// sourced from the JSONB first, with legacyDimensionDirection as a backstop
// for pre-Phase-3 rows.
func buildRunComparisonDimensionDeltas(
	baselineScorecard *RunAgentScorecard,
	candidateScorecard *RunAgentScorecard,
	baselineDimensions map[string]comparisonScorecardDimensionInfo,
	candidateDimensions map[string]comparisonScorecardDimensionInfo,
	missingFields *[]string,
) map[string]runComparisonDelta {
	keys := make([]string, 0, len(baselineDimensions)+len(candidateDimensions))
	seen := make(map[string]struct{}, len(baselineDimensions)+len(candidateDimensions))
	for key := range baselineDimensions {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range candidateDimensions {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	deltas := make(map[string]runComparisonDelta, len(keys))
	for _, key := range keys {
		baselineInfo, baselinePresent := baselineDimensions[key]
		candidateInfo, candidatePresent := candidateDimensions[key]
		direction := baselineInfo.BetterDirection
		if direction == "" {
			direction = candidateInfo.BetterDirection
		}
		if direction == "" {
			direction = legacyDimensionDirection(key)
		}
		baselineValue := comparisonDimensionScore(key, baselineInfo, baselinePresent, baselineScorecard)
		candidateValue := comparisonDimensionScore(key, candidateInfo, candidatePresent, candidateScorecard)
		deltas[key] = buildDimensionDelta(
			direction,
			baselineValue,
			candidateValue,
			baselineInfo,
			candidateInfo,
			baselinePresent,
			candidatePresent,
			missingFields,
			"dimension_deltas."+key,
		)
	}
	return deltas
}

// comparisonDimensionScore prefers the typed scorecard column for built-in
// dimensions (so legacy rows without per-dim JSONB score still compare
// correctly) and falls through to the JSONB score for everything else.
func comparisonDimensionScore(
	key string,
	info comparisonScorecardDimensionInfo,
	present bool,
	scorecard *RunAgentScorecard,
) *float64 {
	if scorecard != nil {
		switch key {
		case "correctness":
			if scorecard.CorrectnessScore != nil {
				return scorecard.CorrectnessScore
			}
		case "reliability":
			if scorecard.ReliabilityScore != nil {
				return scorecard.ReliabilityScore
			}
		case "latency":
			if scorecard.LatencyScore != nil {
				return scorecard.LatencyScore
			}
		case "cost":
			if scorecard.CostScore != nil {
				return scorecard.CostScore
			}
		}
	}
	if present {
		return info.Score
	}
	return nil
}

func buildDimensionDelta(
	betterDirection string,
	baselineValue *float64,
	candidateValue *float64,
	baselineDimension comparisonScorecardDimensionInfo,
	candidateDimension comparisonScorecardDimensionInfo,
	baselinePresent bool,
	candidatePresent bool,
	missingFields *[]string,
	field string,
) runComparisonDelta {
	state := "available"
	switch {
	case baselineDimension.State == "error" || candidateDimension.State == "error":
		state = "error"
	case !baselinePresent && !candidatePresent:
		state = "unavailable"
	case !baselinePresent:
		state = "missing_baseline"
	case !candidatePresent:
		state = "missing_candidate"
	case baselineDimension.State == "" || candidateDimension.State == "" ||
		baselineDimension.State == "unavailable" || candidateDimension.State == "unavailable" ||
		baselineValue == nil || candidateValue == nil:
		state = "unavailable"
	}

	delta := runComparisonDelta{
		BaselineValue:   cloneFloat64Ptr(baselineValue),
		CandidateValue:  cloneFloat64Ptr(candidateValue),
		BetterDirection: betterDirection,
		State:           state,
	}
	if state != "available" {
		*missingFields = append(*missingFields, field)
		return delta
	}
	value := *candidateValue - *baselineValue
	delta.Delta = &value
	return delta
}

func buildFailureDivergence(
	baselineRunAgent domain.RunAgent,
	candidateRunAgent domain.RunAgent,
	baselineReplay *RunAgentReplay,
	candidateReplay *RunAgentReplay,
) (runComparisonFailureDivergence, []string) {
	result := runComparisonFailureDivergence{
		BaselineRunAgentStatus:  baselineRunAgent.Status,
		CandidateRunAgentStatus: candidateRunAgent.Status,
		BaselineFailureReason:   cloneStringPtr(baselineRunAgent.FailureReason),
		CandidateFailureReason:  cloneStringPtr(candidateRunAgent.FailureReason),
	}
	warnings := make([]string, 0, 2)

	if baselineReplay != nil {
		if status, _, ok, err := replayTerminalFields(baselineReplay.Summary); err != nil {
			warnings = append(warnings, fmt.Sprintf("baseline replay terminal state unavailable: %v", err))
		} else if ok {
			result.BaselineTerminalReplayStatus = &status
		}
	}
	if candidateReplay != nil {
		if status, _, ok, err := replayTerminalFields(candidateReplay.Summary); err != nil {
			warnings = append(warnings, fmt.Sprintf("candidate replay terminal state unavailable: %v", err))
		} else if ok {
			result.CandidateTerminalReplayStatus = &status
		}
	}

	baselineFailed := baselineRunAgent.Status == domain.RunAgentStatusFailed
	candidateFailed := candidateRunAgent.Status == domain.RunAgentStatusFailed
	result.CandidateFailedBaselineOK = candidateFailed && !baselineFailed
	result.CandidateOKBaselineFailed = !candidateFailed && baselineFailed
	result.BothFailedDifferently = baselineFailed && candidateFailed && (optionalStringValue(baselineRunAgent.FailureReason) != optionalStringValue(candidateRunAgent.FailureReason) ||
		optionalStringValue(result.BaselineTerminalReplayStatus) != optionalStringValue(result.CandidateTerminalReplayStatus))

	return result, warnings
}

func buildReplaySummaryDivergence(
	baselineReplay *RunAgentReplay,
	candidateReplay *RunAgentReplay,
) (runComparisonReplayDivergence, []string, []string, error) {
	if baselineReplay == nil || candidateReplay == nil {
		missing := []string{"replay_summary_divergence"}
		return runComparisonReplayDivergence{State: "unavailable"}, []string{"replay summary unavailable"}, missing, nil
	}

	baselineSummary, err := decodeComparisonReplaySummary(baselineReplay.Summary)
	if err != nil {
		return runComparisonReplayDivergence{}, nil, nil, fmt.Errorf("decode baseline replay summary: %w", err)
	}
	candidateSummary, err := decodeComparisonReplaySummary(candidateReplay.Summary)
	if err != nil {
		return runComparisonReplayDivergence{}, nil, nil, fmt.Errorf("decode candidate replay summary: %w", err)
	}

	result := runComparisonReplayDivergence{
		State:             "available",
		BaselineStatus:    stringPtr(baselineSummary.Status),
		CandidateStatus:   stringPtr(candidateSummary.Status),
		BaselineHeadline:  stringPtr(baselineSummary.Headline),
		CandidateHeadline: stringPtr(candidateSummary.Headline),
		Counts: map[string]runComparisonCountDelta{
			"events":           buildCountDelta(baselineSummary.Counts.Events, candidateSummary.Counts.Events),
			"replay_steps":     buildCountDelta(baselineSummary.Counts.ReplaySteps, candidateSummary.Counts.ReplaySteps),
			"model_calls":      buildCountDelta(baselineSummary.Counts.ModelCalls, candidateSummary.Counts.ModelCalls),
			"tool_calls":       buildCountDelta(baselineSummary.Counts.ToolCalls, candidateSummary.Counts.ToolCalls),
			"sandbox_commands": buildCountDelta(baselineSummary.Counts.SandboxCommands, candidateSummary.Counts.SandboxCommands),
			"outputs":          buildCountDelta(baselineSummary.Counts.Outputs, candidateSummary.Counts.Outputs),
			"scoring_events":   buildCountDelta(baselineSummary.Counts.ScoringEvents, candidateSummary.Counts.ScoringEvents),
		},
	}
	if baselineSummary.TerminalState != nil {
		result.BaselineTerminalEvent = stringPtr(baselineSummary.TerminalState.EventType)
	}
	if candidateSummary.TerminalState != nil {
		result.CandidateTerminalEvent = stringPtr(candidateSummary.TerminalState.EventType)
	}
	return result, nil, nil, nil
}

func buildCountDelta(baselineValue int64, candidateValue int64) runComparisonCountDelta {
	delta := candidateValue - baselineValue
	return runComparisonCountDelta{
		BaselineValue:  int64Ptr(baselineValue),
		CandidateValue: int64Ptr(candidateValue),
		Delta:          int64Ptr(delta),
		State:          "available",
	}
}

func buildRunComparisonFingerprint(
	summary runComparisonSummaryDocument,
	baseline selectedRunComparisonParticipant,
	candidate selectedRunComparisonParticipant,
) (string, error) {
	payload := struct {
		SchemaVersion               string     `json:"schema_version"`
		BaselineRunID               uuid.UUID  `json:"baseline_run_id"`
		CandidateRunID              uuid.UUID  `json:"candidate_run_id"`
		BaselineRunAgentID          uuid.UUID  `json:"baseline_run_agent_id"`
		CandidateRunAgentID         uuid.UUID  `json:"candidate_run_agent_id"`
		BaselineScorecardUpdatedAt  time.Time  `json:"baseline_scorecard_updated_at"`
		CandidateScorecardUpdatedAt time.Time  `json:"candidate_scorecard_updated_at"`
		BaselineReplayUpdatedAt     *time.Time `json:"baseline_replay_updated_at,omitempty"`
		CandidateReplayUpdatedAt    *time.Time `json:"candidate_replay_updated_at,omitempty"`
		EvaluationSpecID            uuid.UUID  `json:"evaluation_spec_id"`
		ChallengePackVersionID      uuid.UUID  `json:"challenge_pack_version_id"`
		BaselineInputChecksum       string     `json:"baseline_input_checksum,omitempty"`
		CandidateInputChecksum      string     `json:"candidate_input_checksum,omitempty"`
	}{
		SchemaVersion:               summary.SchemaVersion,
		BaselineRunID:               summary.BaselineRefs.RunID,
		CandidateRunID:              summary.CandidateRefs.RunID,
		BaselineRunAgentID:          baseline.runAgent.ID,
		CandidateRunAgentID:         candidate.runAgent.ID,
		BaselineScorecardUpdatedAt:  baseline.scorecard.UpdatedAt.UTC(),
		CandidateScorecardUpdatedAt: candidate.scorecard.UpdatedAt.UTC(),
		EvaluationSpecID:            baseline.scorecard.EvaluationSpecID,
		ChallengePackVersionID:      baseline.executionContext.Run.ChallengePackVersionID,
	}
	if baseline.replay != nil {
		value := baseline.replay.UpdatedAt.UTC()
		payload.BaselineReplayUpdatedAt = &value
	}
	if candidate.replay != nil {
		value := candidate.replay.UpdatedAt.UTC()
		payload.CandidateReplayUpdatedAt = &value
	}
	if baseline.executionContext.ChallengeInputSet != nil {
		payload.BaselineInputChecksum = baseline.executionContext.ChallengeInputSet.InputChecksum
	}
	if candidate.executionContext.ChallengeInputSet != nil {
		payload.CandidateInputChecksum = candidate.executionContext.ChallengeInputSet.InputChecksum
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func (r *Repository) upsertNonComparableRunComparison(
	ctx context.Context,
	baselineRun domain.Run,
	candidateRun domain.Run,
	baselineRunAgentID *uuid.UUID,
	candidateRunAgentID *uuid.UUID,
	reasonCode string,
	warnings []string,
) (RunComparison, error) {
	summary := runComparisonSummaryDocument{
		SchemaVersion: runComparisonSummarySchemaVersion,
		Status:        RunComparisonStatusNotComparable,
		ReasonCode:    reasonCode,
		BaselineRefs: runComparisonRefs{
			RunID:                  baselineRun.ID,
			RunAgentID:             cloneUUIDPtr(baselineRunAgentID),
			ChallengePackVersionID: baselineRun.ChallengePackVersionID,
			ChallengeInputSetID:    cloneUUIDPtr(baselineRun.ChallengeInputSetID),
		},
		CandidateRefs: runComparisonRefs{
			RunID:                  candidateRun.ID,
			RunAgentID:             cloneUUIDPtr(candidateRunAgentID),
			ChallengePackVersionID: candidateRun.ChallengePackVersionID,
			ChallengeInputSetID:    cloneUUIDPtr(candidateRun.ChallengeInputSetID),
		},
		FailureDivergence: runComparisonFailureDivergence{},
		ReplaySummaryDivergence: runComparisonReplayDivergence{
			State: "unavailable",
		},
		EvidenceQuality: runComparisonEvidenceQuality{
			Warnings: uniqueSortedStrings(warnings),
		},
	}
	encoded, err := json.Marshal(summary)
	if err != nil {
		return RunComparison{}, fmt.Errorf("marshal non-comparable run comparison summary: %w", err)
	}

	fingerprintPayload := fmt.Sprintf(
		"%s:%s:%s:%s:%s",
		baselineRun.ID,
		candidateRun.ID,
		optionalUUIDString(baselineRunAgentID),
		optionalUUIDString(candidateRunAgentID),
		reasonCode,
	)
	sum := sha256.Sum256([]byte(fingerprintPayload))
	fingerprint := hex.EncodeToString(sum[:])

	return r.upsertRunComparisonRecord(
		ctx,
		baselineRun.ID,
		candidateRun.ID,
		baselineRunAgentID,
		candidateRunAgentID,
		RunComparisonStatusNotComparable,
		&reasonCode,
		fingerprint,
		encoded,
	)
}

func (r *Repository) upsertRunComparisonRecord(
	ctx context.Context,
	baselineRunID uuid.UUID,
	candidateRunID uuid.UUID,
	baselineRunAgentID *uuid.UUID,
	candidateRunAgentID *uuid.UUID,
	status RunComparisonStatus,
	reasonCode *string,
	fingerprint string,
	summary json.RawMessage,
) (RunComparison, error) {
	row, err := r.queries.UpsertRunComparison(ctx, repositorysqlc.UpsertRunComparisonParams{
		BaselineRunID:       baselineRunID,
		CandidateRunID:      candidateRunID,
		BaselineRunAgentID:  cloneUUIDPtr(baselineRunAgentID),
		CandidateRunAgentID: cloneUUIDPtr(candidateRunAgentID),
		Status:              string(status),
		ReasonCode:          cloneStringPtr(reasonCode),
		SourceFingerprint:   fingerprint,
		Summary:             cloneJSON(summary),
	})
	if err != nil {
		return RunComparison{}, fmt.Errorf("upsert run comparison: %w", err)
	}

	record, err := mapRunComparison(row)
	if err != nil {
		return RunComparison{}, fmt.Errorf("map run comparison: %w", err)
	}
	return record, nil
}

func mapRunComparison(row repositorysqlc.RunComparison) (RunComparison, error) {
	createdAt, err := requiredTime("run_comparisons.created_at", row.CreatedAt)
	if err != nil {
		return RunComparison{}, err
	}
	updatedAt, err := requiredTime("run_comparisons.updated_at", row.UpdatedAt)
	if err != nil {
		return RunComparison{}, err
	}

	status := RunComparisonStatus(row.Status)
	if status != RunComparisonStatusComparable && status != RunComparisonStatusNotComparable {
		return RunComparison{}, fmt.Errorf("invalid run comparison status %q", row.Status)
	}

	return RunComparison{
		ID:                  row.ID,
		BaselineRunID:       row.BaselineRunID,
		CandidateRunID:      row.CandidateRunID,
		BaselineRunAgentID:  cloneUUIDPtr(row.BaselineRunAgentID),
		CandidateRunAgentID: cloneUUIDPtr(row.CandidateRunAgentID),
		Status:              status,
		ReasonCode:          cloneStringPtr(row.ReasonCode),
		SourceFingerprint:   row.SourceFingerprint,
		Summary:             cloneJSON(row.Summary),
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}, nil
}

func decodeComparisonScorecard(payload json.RawMessage) (comparisonScorecardDocument, error) {
	document := comparisonScorecardDocument{
		Dimensions: map[string]comparisonScorecardDimensionInfo{},
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		return comparisonScorecardDocument{}, err
	}
	if document.Dimensions == nil {
		document.Dimensions = map[string]comparisonScorecardDimensionInfo{}
	}
	return document, nil
}

func decodeComparisonReplaySummary(payload json.RawMessage) (comparisonReplaySummaryDocument, error) {
	var document comparisonReplaySummaryDocument
	if err := json.Unmarshal(payload, &document); err != nil {
		return comparisonReplaySummaryDocument{}, err
	}
	return document, nil
}

func replayTerminalFields(payload json.RawMessage) (string, string, bool, error) {
	document, err := decodeComparisonReplaySummary(payload)
	if err != nil {
		return "", "", false, err
	}
	if document.TerminalState == nil {
		return "", "", false, nil
	}
	return document.TerminalState.Status, document.TerminalState.EventType, true, nil
}

func findRunAgentByID(runAgents []domain.RunAgent, id uuid.UUID) (domain.RunAgent, bool) {
	for _, runAgent := range runAgents {
		if runAgent.ID == id {
			return runAgent, true
		}
	}
	return domain.RunAgent{}, false
}

func sameOptionalUUID(left *uuid.UUID, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func optionalUUIDString(value *uuid.UUID) string {
	if value == nil {
		return "<nil>"
	}
	return value.String()
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func equalStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func uniqueSortedStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		seen[item] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	values := make([]string, 0, len(seen))
	for item := range seen {
		values = append(values, item)
	}
	sort.Strings(values)
	return values
}
