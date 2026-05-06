package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const agentHarnessSuiteRankingSchemaVersion = "2026-05-06"

type BuildAgentHarnessSuiteRankingParams struct {
	WorkspaceID    uuid.UUID
	SuiteID        uuid.UUID
	SuiteVersionID *uuid.UUID
	K              int
}

type AgentHarnessSuiteRankingRecord struct {
	SuiteID        uuid.UUID
	SuiteVersionID uuid.UUID
	SuiteVersion   int32
	SchemaVersion  string
	Ranking        json.RawMessage
	ComputedAt     time.Time
}

type AgentHarnessSuiteVersionRef struct {
	ID            uuid.UUID
	SuiteID       uuid.UUID
	WorkspaceID   uuid.UUID
	VersionNumber int32
	Metadata      json.RawMessage
	CreatedAt     time.Time
}

type agentHarnessRankingAttemptRow struct {
	ExecutionID              uuid.UUID
	AgentHarnessID           uuid.UUID
	RunID                    *uuid.UUID
	RunAgentID               *uuid.UUID
	Status                   string
	HarnessSnapshot          json.RawMessage
	ExecutionConfigSnapshot  json.RawMessage
	EvaluationConfigSnapshot json.RawMessage
	ErrorMessage             *string
	StartedAt                *time.Time
	CompletedAt              *time.Time
	CreatedAt                time.Time
	OverallScore             *float64
	CorrectnessScore         *float64
	ReliabilityScore         *float64
	LatencyScore             *float64
	CostScore                *float64
	BehavioralScore          *float64
	ScorecardPassed          *bool
	Scorecard                json.RawMessage
	TotalCostUSD             *float64
	TotalInputTokens         *int64
	TotalOutputTokens        *int64
	FailureEventType         *string
}

type agentHarnessRankingDocument struct {
	SchemaVersion string                          `json:"schema_version"`
	SuiteID       uuid.UUID                       `json:"suite_id"`
	SuiteVersion  agentHarnessRankingSuiteVersion `json:"suite_version"`
	K             int                             `json:"k"`
	ComputedAt    time.Time                       `json:"computed_at"`
	Rankings      []agentHarnessRankingEntry      `json:"rankings"`
	Pairwise      []agentHarnessPairwiseRanking   `json:"pairwise"`
	Evidence      agentHarnessRankingEvidence     `json:"evidence"`
}

type agentHarnessRankingSuiteVersion struct {
	ID            uuid.UUID `json:"id"`
	VersionNumber int32     `json:"version_number"`
	Source        string    `json:"source"`
}

type agentHarnessRankingEntry struct {
	Rank           int                                   `json:"rank"`
	HarnessID      uuid.UUID                             `json:"harness_id"`
	HarnessName    string                                `json:"harness_name"`
	HarnessKind    string                                `json:"harness_kind"`
	CodexTemplate  string                                `json:"codex_template,omitempty"`
	CodexModel     string                                `json:"codex_model,omitempty"`
	AttemptCount   int                                   `json:"attempt_count"`
	ScoredCount    int                                   `json:"scored_count"`
	SuccessCount   int                                   `json:"success_count"`
	SuccessAt1     *agentHarnessRateMetric               `json:"success_at_1,omitempty"`
	PassAtK        *agentHarnessRateMetric               `json:"pass_at_k,omitempty"`
	PassPowK       *agentHarnessRateMetric               `json:"pass_pow_k,omitempty"`
	Score          *evalSessionMetricAggregate           `json:"score,omitempty"`
	Dimensions     map[string]evalSessionMetricAggregate `json:"dimensions,omitempty"`
	Cost           *agentHarnessCostMetric               `json:"cost,omitempty"`
	Latency        *agentHarnessLatencyMetric            `json:"latency,omitempty"`
	Budget         agentHarnessBudgetSnapshot            `json:"budget"`
	FailureModes   []agentHarnessFailureMode             `json:"failure_modes,omitempty"`
	FairnessGroups []string                              `json:"fairness_groups"`
	ExecutionIDs   []uuid.UUID                           `json:"execution_ids"`
}

type agentHarnessRateMetric struct {
	K                 int                           `json:"k"`
	N                 int                           `json:"n"`
	Successes         int                           `json:"successes"`
	Value             float64                       `json:"value"`
	Estimator         string                        `json:"estimator"`
	Interval          *evalSessionAggregateInterval `json:"interval,omitempty"`
	Available         bool                          `json:"available"`
	UnavailableReason string                        `json:"unavailable_reason,omitempty"`
}

type agentHarnessCostMetric struct {
	TotalUSD     float64 `json:"total_usd"`
	MeanUSD      float64 `json:"mean_usd"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	ObservedRuns int     `json:"observed_runs"`
}

type agentHarnessLatencyMetric struct {
	MeanSeconds   float64 `json:"mean_seconds"`
	MedianSeconds float64 `json:"median_seconds"`
	MinSeconds    float64 `json:"min_seconds"`
	MaxSeconds    float64 `json:"max_seconds"`
	ObservedRuns  int     `json:"observed_runs"`
}

type agentHarnessBudgetSnapshot struct {
	Fingerprint        string   `json:"fingerprint"`
	TimeoutSeconds     *int     `json:"timeout_seconds,omitempty"`
	MaxWallTimeSeconds *int     `json:"max_wall_time_seconds,omitempty"`
	MaxTokenSpend      *int64   `json:"max_token_spend,omitempty"`
	MaxToolCalls       *int64   `json:"max_tool_calls,omitempty"`
	RetryPolicy        string   `json:"retry_policy,omitempty"`
	SetupCommands      []string `json:"setup_commands,omitempty"`
}

type agentHarnessFailureMode struct {
	Stage string `json:"stage"`
	Count int    `json:"count"`
}

type agentHarnessPairwiseRanking struct {
	LeftHarnessID    uuid.UUID  `json:"left_harness_id"`
	RightHarnessID   uuid.UUID  `json:"right_harness_id"`
	FairnessGroup    string     `json:"fairness_group"`
	ComparedAttempts int        `json:"compared_attempts"`
	LeftSuccessRate  float64    `json:"left_success_rate"`
	RightSuccessRate float64    `json:"right_success_rate"`
	Delta            float64    `json:"delta"`
	WinnerHarnessID  *uuid.UUID `json:"winner_harness_id,omitempty"`
	Status           string     `json:"status"`
	ReasonCode       string     `json:"reason_code,omitempty"`
}

type agentHarnessRankingEvidence struct {
	AttemptCount       int      `json:"attempt_count"`
	ScoredAttemptCount int      `json:"scored_attempt_count"`
	FairnessRule       string   `json:"fairness_rule"`
	SourceArtifacts    []string `json:"source_artifacts"`
	Warnings           []string `json:"warnings,omitempty"`
}

type agentHarnessRankingAttempt struct {
	row            agentHarnessRankingAttemptRow
	suiteID        uuid.UUID
	suiteVersionID uuid.UUID
	taskID         uuid.UUID
	taskPrompt     string
	repositoryURL  string
	baseBranch     string
	harnessName    string
	harnessKind    string
	codexTemplate  string
	codexModel     string
	budget         agentHarnessBudgetSnapshot
	fairnessGroup  string
	success        *bool
	latencySeconds *float64
}

func (r *Repository) BuildAgentHarnessSuiteRanking(ctx context.Context, p BuildAgentHarnessSuiteRankingParams) (AgentHarnessSuiteRankingRecord, error) {
	suite, err := r.GetAgentHarnessSuiteByID(ctx, p.SuiteID)
	if err != nil {
		return AgentHarnessSuiteRankingRecord{}, err
	}
	if suite.WorkspaceID != p.WorkspaceID || suite.Status != "active" {
		return AgentHarnessSuiteRankingRecord{}, ErrAgentHarnessSuiteNotFound
	}
	suiteVersion := AgentHarnessSuiteVersionRef{
		ID:            suite.CurrentVersionID,
		SuiteID:       suite.ID,
		WorkspaceID:   suite.WorkspaceID,
		VersionNumber: suite.CurrentVersionNumber,
		Metadata:      suite.Metadata,
	}
	if p.SuiteVersionID != nil {
		suiteVersion, err = r.GetAgentHarnessSuiteVersionByID(ctx, *p.SuiteVersionID)
		if err != nil {
			return AgentHarnessSuiteRankingRecord{}, err
		}
		if suiteVersion.WorkspaceID != p.WorkspaceID || suiteVersion.SuiteID != suite.ID {
			return AgentHarnessSuiteRankingRecord{}, ErrAgentHarnessSuiteNotFound
		}
	}
	k := p.K
	if k <= 0 {
		k = 1
	}
	rows, err := r.listAgentHarnessSuiteRankingAttemptRows(ctx, p.WorkspaceID, p.SuiteID, suiteVersion.ID)
	if err != nil {
		return AgentHarnessSuiteRankingRecord{}, err
	}
	document, err := buildAgentHarnessSuiteRankingDocument(suite, suiteVersion, rows, k, time.Now().UTC())
	if err != nil {
		return AgentHarnessSuiteRankingRecord{}, err
	}
	payload, err := json.Marshal(document)
	if err != nil {
		return AgentHarnessSuiteRankingRecord{}, fmt.Errorf("marshal agent harness suite ranking: %w", err)
	}
	return AgentHarnessSuiteRankingRecord{
		SuiteID:        suite.ID,
		SuiteVersionID: suiteVersion.ID,
		SuiteVersion:   suiteVersion.VersionNumber,
		SchemaVersion:  agentHarnessSuiteRankingSchemaVersion,
		Ranking:        payload,
		ComputedAt:     document.ComputedAt,
	}, nil
}

func (r *Repository) GetAgentHarnessSuiteVersionByID(ctx context.Context, id uuid.UUID) (AgentHarnessSuiteVersionRef, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, agent_harness_suite_id, workspace_id, version_number, metadata, created_at
FROM agent_harness_suite_versions
WHERE id = $1
	LIMIT 1`, id)
	var ref AgentHarnessSuiteVersionRef
	if err := row.Scan(&ref.ID, &ref.SuiteID, &ref.WorkspaceID, &ref.VersionNumber, &ref.Metadata, &ref.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentHarnessSuiteVersionRef{}, ErrAgentHarnessSuiteNotFound
		}
		return AgentHarnessSuiteVersionRef{}, fmt.Errorf("get agent harness suite version by id: %w", err)
	}
	return ref, nil
}

func (r *Repository) listAgentHarnessSuiteRankingAttemptRows(ctx context.Context, workspaceID uuid.UUID, suiteID uuid.UUID, suiteVersionID uuid.UUID) ([]agentHarnessRankingAttemptRow, error) {
	rows, err := r.db.Query(ctx, `
SELECT
    e.id, e.agent_harness_id, e.run_id, e.run_agent_id, e.status,
    e.harness_snapshot, e.execution_config_snapshot, e.evaluation_config_snapshot,
    e.error_message, e.started_at, e.completed_at, e.created_at,
    sc.overall_score, sc.correctness_score, sc.reliability_score, sc.latency_score, sc.cost_score, sc.behavioral_score,
	sc.scorecard_passed, COALESCE(sc.scorecard, '{}'::jsonb),
    cs.total_cost_usd, cs.total_input_tokens, cs.total_output_tokens,
    failed.event_type
FROM agent_harness_executions e
LEFT JOIN run_agent_scorecards sc ON sc.run_agent_id = e.run_agent_id
LEFT JOIN run_cost_summaries cs ON cs.run_id = e.run_id
LEFT JOIN LATERAL (
    SELECT event_type
    FROM agent_harness_execution_events ev
    WHERE ev.agent_harness_execution_id = e.id
      AND (ev.event_type = 'github.repository_access_revoked' OR ev.event_type LIKE '%.failed')
    ORDER BY ev.sequence_number DESC
    LIMIT 1
) failed ON true
WHERE e.workspace_id = $1
  AND e.evaluation_config_snapshot #>> '{suite,suite_id}' = $2
  AND e.evaluation_config_snapshot #>> '{suite,suite_version_id}' = $3
ORDER BY e.created_at ASC, e.id ASC`, workspaceID, suiteID.String(), suiteVersionID.String())
	if err != nil {
		return nil, fmt.Errorf("list agent harness suite ranking attempts: %w", err)
	}
	defer rows.Close()

	attempts := make([]agentHarnessRankingAttemptRow, 0)
	for rows.Next() {
		var row agentHarnessRankingAttemptRow
		var overall, correctness, reliability, latency, cost, behavioral pgtype.Numeric
		var totalCost pgtype.Numeric
		var inputTokens, outputTokens *int64
		if err := rows.Scan(
			&row.ExecutionID,
			&row.AgentHarnessID,
			&row.RunID,
			&row.RunAgentID,
			&row.Status,
			&row.HarnessSnapshot,
			&row.ExecutionConfigSnapshot,
			&row.EvaluationConfigSnapshot,
			&row.ErrorMessage,
			&row.StartedAt,
			&row.CompletedAt,
			&row.CreatedAt,
			&overall,
			&correctness,
			&reliability,
			&latency,
			&cost,
			&behavioral,
			&row.ScorecardPassed,
			&row.Scorecard,
			&totalCost,
			&inputTokens,
			&outputTokens,
			&row.FailureEventType,
		); err != nil {
			return nil, fmt.Errorf("scan agent harness suite ranking attempt: %w", err)
		}
		row.OverallScore = numericPtr(overall)
		row.CorrectnessScore = numericPtr(correctness)
		row.ReliabilityScore = numericPtr(reliability)
		row.LatencyScore = numericPtr(latency)
		row.CostScore = numericPtr(cost)
		row.BehavioralScore = numericPtr(behavioral)
		row.TotalCostUSD = numericPtr(totalCost)
		row.TotalInputTokens = inputTokens
		row.TotalOutputTokens = outputTokens
		attempts = append(attempts, row)
	}
	return attempts, rows.Err()
}

func buildAgentHarnessSuiteRankingDocument(suite AgentHarnessSuite, suiteVersion AgentHarnessSuiteVersionRef, rows []agentHarnessRankingAttemptRow, k int, computedAt time.Time) (agentHarnessRankingDocument, error) {
	attempts := make([]agentHarnessRankingAttempt, 0, len(rows))
	warnings := make([]string, 0)
	for _, row := range rows {
		attempt, attemptWarnings, err := buildAgentHarnessRankingAttempt(row)
		if err != nil {
			return agentHarnessRankingDocument{}, err
		}
		attempts = append(attempts, attempt)
		warnings = append(warnings, attemptWarnings...)
	}

	groups := map[uuid.UUID][]agentHarnessRankingAttempt{}
	for _, attempt := range attempts {
		groups[attempt.row.AgentHarnessID] = append(groups[attempt.row.AgentHarnessID], attempt)
	}
	harnessIDs := make([]uuid.UUID, 0, len(groups))
	for harnessID := range groups {
		harnessIDs = append(harnessIDs, harnessID)
	}
	sort.Slice(harnessIDs, func(i, j int) bool { return harnessIDs[i].String() < harnessIDs[j].String() })

	rankings := make([]agentHarnessRankingEntry, 0, len(harnessIDs))
	for _, harnessID := range harnessIDs {
		entry := buildAgentHarnessRankingEntry(harnessID, groups[harnessID], k)
		rankings = append(rankings, entry)
	}
	sort.Slice(rankings, func(i, j int) bool { return agentHarnessRankingLess(rankings[i], rankings[j]) })
	for i := range rankings {
		rankings[i].Rank = i + 1
	}

	scored := 0
	for _, attempt := range attempts {
		if attempt.success != nil || attempt.row.OverallScore != nil {
			scored++
		}
	}

	return agentHarnessRankingDocument{
		SchemaVersion: agentHarnessSuiteRankingSchemaVersion,
		SuiteID:       suite.ID,
		SuiteVersion: agentHarnessRankingSuiteVersion{
			ID:            suiteVersion.ID,
			VersionNumber: suiteVersion.VersionNumber,
			Source:        "agent_harness_execution_snapshot",
		},
		K:          k,
		ComputedAt: computedAt,
		Rankings:   rankings,
		Pairwise:   buildAgentHarnessPairwiseRankings(groups),
		Evidence: agentHarnessRankingEvidence{
			AttemptCount:       len(attempts),
			ScoredAttemptCount: scored,
			FairnessRule:       "suite_version_id + task_id + repository_url + base_branch + task_prompt + execution_config_snapshot fingerprint",
			SourceArtifacts: []string{
				"agent_harness_executions",
				"run_agent_scorecards",
				"run_cost_summaries",
				"agent_harness_execution_events",
			},
			Warnings: dedupeSortedStrings(warnings),
		},
	}, nil
}

func buildAgentHarnessRankingAttempt(row agentHarnessRankingAttemptRow) (agentHarnessRankingAttempt, []string, error) {
	warnings := make([]string, 0)
	var harness struct {
		Name          string  `json:"name"`
		HarnessKind   string  `json:"harness_kind"`
		CodexTemplate string  `json:"codex_template"`
		CodexModel    *string `json:"codex_model"`
		RepositoryURL *string `json:"repository_url"`
		BaseBranch    *string `json:"base_branch"`
		TaskPrompt    string  `json:"task_prompt"`
	}
	if len(row.HarnessSnapshot) > 0 {
		if err := json.Unmarshal(row.HarnessSnapshot, &harness); err != nil {
			return agentHarnessRankingAttempt{}, nil, fmt.Errorf("decode harness snapshot for execution %s: %w", row.ExecutionID, err)
		}
	}
	var evaluation struct {
		Suite struct {
			SuiteID        uuid.UUID `json:"suite_id"`
			SuiteVersionID uuid.UUID `json:"suite_version_id"`
			TaskID         uuid.UUID `json:"task_id"`
		} `json:"suite"`
	}
	if len(row.EvaluationConfigSnapshot) > 0 {
		if err := json.Unmarshal(row.EvaluationConfigSnapshot, &evaluation); err != nil {
			return agentHarnessRankingAttempt{}, nil, fmt.Errorf("decode evaluation snapshot for execution %s: %w", row.ExecutionID, err)
		}
	}
	budget := buildAgentHarnessBudgetSnapshot(row.ExecutionConfigSnapshot)
	attempt := agentHarnessRankingAttempt{
		row:            row,
		suiteID:        evaluation.Suite.SuiteID,
		suiteVersionID: evaluation.Suite.SuiteVersionID,
		taskID:         evaluation.Suite.TaskID,
		taskPrompt:     strings.TrimSpace(harness.TaskPrompt),
		repositoryURL:  derefString(harness.RepositoryURL),
		baseBranch:     derefString(harness.BaseBranch),
		harnessName:    strings.TrimSpace(harness.Name),
		harnessKind:    strings.TrimSpace(harness.HarnessKind),
		codexTemplate:  strings.TrimSpace(harness.CodexTemplate),
		codexModel:     derefString(harness.CodexModel),
		budget:         budget,
	}
	attempt.fairnessGroup = buildAgentHarnessFairnessGroup(attempt)
	attempt.success = resolveAgentHarnessAttemptSuccess(row)
	if row.CompletedAt != nil {
		started := row.CreatedAt
		if row.StartedAt != nil {
			started = *row.StartedAt
		}
		seconds := row.CompletedAt.Sub(started).Seconds()
		if seconds >= 0 {
			attempt.latencySeconds = &seconds
		}
	}
	if row.ScorecardPassed == nil && row.OverallScore == nil && row.Status == string(AgentHarnessExecutionStatusCompleted) {
		warnings = append(warnings, fmt.Sprintf("execution %s completed without scorecard evidence", row.ExecutionID))
	}
	return attempt, warnings, nil
}

func buildAgentHarnessRankingEntry(harnessID uuid.UUID, attempts []agentHarnessRankingAttempt, k int) agentHarnessRankingEntry {
	first := attempts[0]
	entry := agentHarnessRankingEntry{
		HarnessID:      harnessID,
		HarnessName:    first.harnessName,
		HarnessKind:    first.harnessKind,
		CodexTemplate:  first.codexTemplate,
		CodexModel:     first.codexModel,
		AttemptCount:   len(attempts),
		Budget:         first.budget,
		FairnessGroups: uniqueSortedAgentHarnessFairnessGroups(attempts),
		ExecutionIDs:   make([]uuid.UUID, 0, len(attempts)),
	}
	successes := 0
	outcomes := make([]bool, 0, len(attempts))
	overallValues := make([]float64, 0, len(attempts))
	dimensions := map[string][]float64{}
	latencies := make([]float64, 0, len(attempts))
	failures := map[string]int{}
	var totalCost float64
	var inputTokens, outputTokens int64
	costRuns := 0
	for _, attempt := range attempts {
		entry.ExecutionIDs = append(entry.ExecutionIDs, attempt.row.ExecutionID)
		if attempt.success != nil {
			entry.ScoredCount++
			outcomes = append(outcomes, *attempt.success)
			if *attempt.success {
				successes++
			}
		}
		if attempt.row.OverallScore != nil {
			overallValues = append(overallValues, *attempt.row.OverallScore)
		}
		addAgentHarnessDimension(dimensions, "correctness", attempt.row.CorrectnessScore)
		addAgentHarnessDimension(dimensions, "reliability", attempt.row.ReliabilityScore)
		addAgentHarnessDimension(dimensions, "latency", attempt.row.LatencyScore)
		addAgentHarnessDimension(dimensions, "cost", attempt.row.CostScore)
		addAgentHarnessDimension(dimensions, "behavioral", attempt.row.BehavioralScore)
		if attempt.row.TotalCostUSD != nil {
			totalCost += *attempt.row.TotalCostUSD
			costRuns++
		}
		if attempt.row.TotalInputTokens != nil {
			inputTokens += *attempt.row.TotalInputTokens
		}
		if attempt.row.TotalOutputTokens != nil {
			outputTokens += *attempt.row.TotalOutputTokens
		}
		if attempt.latencySeconds != nil {
			latencies = append(latencies, *attempt.latencySeconds)
		}
		if stage := agentHarnessFailureStageFromEvent(attempt.row.Status, attempt.row.FailureEventType); stage != "" {
			failures[stage]++
		}
	}
	sort.Slice(entry.ExecutionIDs, func(i, j int) bool { return entry.ExecutionIDs[i].String() < entry.ExecutionIDs[j].String() })
	entry.SuccessCount = successes
	if entry.ScoredCount > 0 {
		entry.SuccessAt1 = &agentHarnessRateMetric{
			K:         1,
			N:         entry.ScoredCount,
			Successes: successes,
			Value:     float64(successes) / float64(entry.ScoredCount),
			Estimator: "observed_rate",
			Interval:  wilsonInterval(successes, entry.ScoredCount),
			Available: true,
		}
		entry.PassAtK = agentHarnessPassAtKMetric(successes, entry.ScoredCount, k)
		entry.PassPowK = agentHarnessPassPowKMetric(successes, entry.ScoredCount, k)
	}
	if len(overallValues) > 0 {
		score := buildEvalSessionMetricAggregate(overallValues)
		entry.Score = &score
	}
	if len(dimensions) > 0 {
		entry.Dimensions = map[string]evalSessionMetricAggregate{}
		for _, key := range sortedMetricKeys(dimensions) {
			entry.Dimensions[key] = buildEvalSessionMetricAggregate(dimensions[key])
		}
	}
	if costRuns > 0 {
		entry.Cost = &agentHarnessCostMetric{
			TotalUSD:     totalCost,
			MeanUSD:      totalCost / float64(costRuns),
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			ObservedRuns: costRuns,
		}
	}
	if len(latencies) > 0 {
		sort.Float64s(latencies)
		entry.Latency = &agentHarnessLatencyMetric{
			MeanSeconds:   kahanMean(latencies),
			MedianSeconds: median(latencies),
			MinSeconds:    latencies[0],
			MaxSeconds:    latencies[len(latencies)-1],
			ObservedRuns:  len(latencies),
		}
	}
	entry.FailureModes = buildAgentHarnessFailureModes(failures)
	return entry
}

func agentHarnessRankingLess(left, right agentHarnessRankingEntry) bool {
	leftScore := rankingPrimaryScore(left)
	rightScore := rankingPrimaryScore(right)
	if leftScore != rightScore {
		return leftScore > rightScore
	}
	leftSuccessLower := rankingSuccessLowerBound(left)
	rightSuccessLower := rankingSuccessLowerBound(right)
	if leftSuccessLower != rightSuccessLower {
		return leftSuccessLower > rightSuccessLower
	}
	if left.Cost != nil && right.Cost != nil && left.Cost.MeanUSD != right.Cost.MeanUSD {
		return left.Cost.MeanUSD < right.Cost.MeanUSD
	}
	if left.Latency != nil && right.Latency != nil && left.Latency.MeanSeconds != right.Latency.MeanSeconds {
		return left.Latency.MeanSeconds < right.Latency.MeanSeconds
	}
	return left.HarnessID.String() < right.HarnessID.String()
}

func rankingPrimaryScore(entry agentHarnessRankingEntry) float64 {
	if entry.Score != nil {
		return entry.Score.Mean
	}
	if entry.SuccessAt1 != nil && entry.SuccessAt1.Available {
		return entry.SuccessAt1.Value
	}
	return -1
}

func rankingSuccessLowerBound(entry agentHarnessRankingEntry) float64 {
	if entry.SuccessAt1 == nil || entry.SuccessAt1.Interval == nil {
		return -1
	}
	return entry.SuccessAt1.Interval.Lower
}

func agentHarnessPassAtKMetric(successes, n, k int) *agentHarnessRateMetric {
	metric := &agentHarnessRateMetric{K: k, N: n, Successes: successes, Estimator: "observed_without_replacement", Available: false}
	if k <= 0 {
		metric.UnavailableReason = "k_must_be_positive"
		return metric
	}
	if n < k {
		metric.UnavailableReason = "observed_trials_less_than_k"
		return metric
	}
	metric.Value = passAtKObserved(successes, n, k)
	metric.Available = true
	return metric
}

func agentHarnessPassPowKMetric(successes, n, k int) *agentHarnessRateMetric {
	metric := &agentHarnessRateMetric{K: k, N: n, Successes: successes, Estimator: "observed_without_replacement", Available: false}
	if k <= 0 {
		metric.UnavailableReason = "k_must_be_positive"
		return metric
	}
	if n < k {
		metric.UnavailableReason = "observed_trials_less_than_k"
		return metric
	}
	metric.Value = passPowKObserved(successes, n, k)
	metric.Available = true
	return metric
}

func passAtKObserved(successes, n, k int) float64 {
	failures := n - successes
	if successes <= 0 {
		return 0
	}
	if failures < k {
		return 1
	}
	return 1 - combinationRatio(failures, n, k)
}

func passPowKObserved(successes, n, k int) float64 {
	if successes < k {
		return 0
	}
	return combinationRatio(successes, n, k)
}

func combinationRatio(selected, total, k int) float64 {
	if k <= 0 {
		return 1
	}
	result := 1.0
	for i := 0; i < k; i++ {
		result *= float64(selected-i) / float64(total-i)
	}
	return result
}

func wilsonInterval(successes, n int) *evalSessionAggregateInterval {
	if n <= 0 {
		return nil
	}
	z := 1.959963984540054
	phat := float64(successes) / float64(n)
	denominator := 1 + z*z/float64(n)
	center := phat + z*z/(2*float64(n))
	margin := z * math.Sqrt((phat*(1-phat)+z*z/(4*float64(n)))/float64(n))
	return &evalSessionAggregateInterval{
		Estimator: "wilson_95",
		Lower:     clamp01((center - margin) / denominator),
		Upper:     clamp01((center + margin) / denominator),
	}
}

func buildAgentHarnessPairwiseRankings(groups map[uuid.UUID][]agentHarnessRankingAttempt) []agentHarnessPairwiseRanking {
	fairness := map[string]map[uuid.UUID][]agentHarnessRankingAttempt{}
	for harnessID, attempts := range groups {
		for _, attempt := range attempts {
			if attempt.success == nil {
				continue
			}
			if _, ok := fairness[attempt.fairnessGroup]; !ok {
				fairness[attempt.fairnessGroup] = map[uuid.UUID][]agentHarnessRankingAttempt{}
			}
			fairness[attempt.fairnessGroup][harnessID] = append(fairness[attempt.fairnessGroup][harnessID], attempt)
		}
	}
	groupKeys := make([]string, 0, len(fairness))
	for key := range fairness {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	results := make([]agentHarnessPairwiseRanking, 0)
	for _, groupKey := range groupKeys {
		harnessAttempts := fairness[groupKey]
		harnessIDs := make([]uuid.UUID, 0, len(harnessAttempts))
		for harnessID := range harnessAttempts {
			harnessIDs = append(harnessIDs, harnessID)
		}
		sort.Slice(harnessIDs, func(i, j int) bool { return harnessIDs[i].String() < harnessIDs[j].String() })
		for i := 0; i < len(harnessIDs); i++ {
			for j := i + 1; j < len(harnessIDs); j++ {
				leftID, rightID := harnessIDs[i], harnessIDs[j]
				leftSuccess, leftN := agentHarnessSuccessRate(harnessAttempts[leftID])
				rightSuccess, rightN := agentHarnessSuccessRate(harnessAttempts[rightID])
				compared := min(leftN, rightN)
				pair := agentHarnessPairwiseRanking{
					LeftHarnessID:    leftID,
					RightHarnessID:   rightID,
					FairnessGroup:    groupKey,
					ComparedAttempts: compared,
					LeftSuccessRate:  leftSuccess,
					RightSuccessRate: rightSuccess,
					Delta:            leftSuccess - rightSuccess,
					Status:           "comparable",
				}
				if compared == 0 {
					pair.Status = "not_comparable"
					pair.ReasonCode = "no_scored_attempts"
				} else if leftSuccess > rightSuccess {
					winner := leftID
					pair.WinnerHarnessID = &winner
				} else if rightSuccess > leftSuccess {
					winner := rightID
					pair.WinnerHarnessID = &winner
				} else {
					pair.ReasonCode = "tie"
				}
				results = append(results, pair)
			}
		}
	}
	return results
}

func agentHarnessSuccessRate(attempts []agentHarnessRankingAttempt) (float64, int) {
	n := 0
	successes := 0
	for _, attempt := range attempts {
		if attempt.success == nil {
			continue
		}
		n++
		if *attempt.success {
			successes++
		}
	}
	if n == 0 {
		return 0, 0
	}
	return float64(successes) / float64(n), n
}

func resolveAgentHarnessAttemptSuccess(row agentHarnessRankingAttemptRow) *bool {
	if row.ScorecardPassed != nil {
		return cloneBoolPtr(row.ScorecardPassed)
	}
	if row.OverallScore != nil {
		success := *row.OverallScore >= evalSessionDefaultSuccessThreshold
		return &success
	}
	return nil
}

func buildAgentHarnessBudgetSnapshot(raw json.RawMessage) agentHarnessBudgetSnapshot {
	var config struct {
		TimeoutSeconds     *int     `json:"timeout_seconds"`
		MaxWallTimeSeconds *int     `json:"max_wall_time_seconds"`
		MaxTokenSpend      *int64   `json:"max_token_spend"`
		MaxToolCalls       *int64   `json:"max_tool_calls"`
		RetryPolicy        string   `json:"retry_policy"`
		SetupCommands      []string `json:"setup_commands"`
	}
	_ = json.Unmarshal(raw, &config)
	return agentHarnessBudgetSnapshot{
		Fingerprint:        fingerprintJSON(raw),
		TimeoutSeconds:     config.TimeoutSeconds,
		MaxWallTimeSeconds: config.MaxWallTimeSeconds,
		MaxTokenSpend:      config.MaxTokenSpend,
		MaxToolCalls:       config.MaxToolCalls,
		RetryPolicy:        strings.TrimSpace(config.RetryPolicy),
		SetupCommands:      append([]string(nil), config.SetupCommands...),
	}
}

func buildAgentHarnessFairnessGroup(attempt agentHarnessRankingAttempt) string {
	parts := []string{
		attempt.suiteVersionID.String(),
		attempt.taskID.String(),
		strings.TrimSpace(attempt.repositoryURL),
		strings.TrimSpace(attempt.baseBranch),
		strings.TrimSpace(attempt.taskPrompt),
		attempt.budget.Fingerprint,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func fingerprintJSON(raw json.RawMessage) string {
	var value any
	if len(raw) > 0 && json.Unmarshal(raw, &value) == nil {
		normalized, err := json.Marshal(value)
		if err == nil {
			sum := sha256.Sum256(normalized)
			return hex.EncodeToString(sum[:])
		}
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func addAgentHarnessDimension(dimensions map[string][]float64, key string, value *float64) {
	if value != nil {
		dimensions[key] = append(dimensions[key], *value)
	}
}

func uniqueSortedAgentHarnessFairnessGroups(attempts []agentHarnessRankingAttempt) []string {
	set := map[string]struct{}{}
	for _, attempt := range attempts {
		set[attempt.fairnessGroup] = struct{}{}
	}
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func buildAgentHarnessFailureModes(counts map[string]int) []agentHarnessFailureMode {
	if len(counts) == 0 {
		return nil
	}
	stages := make([]string, 0, len(counts))
	for stage := range counts {
		stages = append(stages, stage)
	}
	sort.Strings(stages)
	modes := make([]agentHarnessFailureMode, 0, len(stages))
	for _, stage := range stages {
		modes = append(modes, agentHarnessFailureMode{Stage: stage, Count: counts[stage]})
	}
	return modes
}

func agentHarnessFailureStageFromEvent(status string, eventType *string) string {
	if status != string(AgentHarnessExecutionStatusFailed) {
		return ""
	}
	if eventType != nil {
		switch {
		case *eventType == "github.repository_access_revoked":
			return "repository"
		case strings.HasPrefix(*eventType, "setup."):
			return "setup"
		case strings.HasPrefix(*eventType, "codex.") || strings.HasPrefix(*eventType, "claude."):
			return "agent"
		case strings.HasPrefix(*eventType, "validator.") || strings.HasPrefix(*eventType, "scoring.") || strings.HasPrefix(*eventType, "llm_judges."):
			return "validator"
		case strings.HasPrefix(*eventType, "repository.") || strings.HasPrefix(*eventType, "github.git_auth"):
			return "repository"
		}
	}
	return "infrastructure"
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
