package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	repositorysqlc "github.com/Atharva-Kanherkar/agentclash/backend/internal/repository/sqlc"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db      *pgxpool.Pool
	queries *repositorysqlc.Queries
}

type SetRunTemporalIDsParams struct {
	RunID              uuid.UUID
	TemporalWorkflowID string
	TemporalRunID      string
}

type TransitionRunStatusParams struct {
	RunID           uuid.UUID
	ToStatus        domain.RunStatus
	Reason          *string
	ChangedByUserID *uuid.UUID
}

type TransitionRunAgentStatusParams struct {
	RunAgentID    uuid.UUID
	ToStatus      domain.RunAgentStatus
	Reason        *string
	FailureReason *string
}

type InsertRunStatusHistoryParams struct {
	RunID           uuid.UUID
	FromStatus      *domain.RunStatus
	ToStatus        domain.RunStatus
	Reason          *string
	ChangedByUserID *uuid.UUID
}

type InsertRunAgentStatusHistoryParams struct {
	RunAgentID uuid.UUID
	FromStatus *domain.RunAgentStatus
	ToStatus   domain.RunAgentStatus
	Reason     *string
}

type RecordHostedRunEventParams struct {
	Event   runevents.Envelope
	Summary json.RawMessage
}

type RecordRunEventParams struct {
	Event runevents.Envelope
}

type RunnableChallengePackVersion struct {
	ID              uuid.UUID
	ChallengePackID uuid.UUID
	WorkspaceID     *uuid.UUID
}

type ChallengeInputSet struct {
	ID                     uuid.UUID
	ChallengePackVersionID uuid.UUID
}

type RunnableDeployment struct {
	ID                        uuid.UUID
	OrganizationID            uuid.UUID
	WorkspaceID               uuid.UUID
	Name                      string
	AgentDeploymentSnapshotID uuid.UUID
}

type CreateQueuedRunAgentParams struct {
	AgentDeploymentID         uuid.UUID
	AgentDeploymentSnapshotID uuid.UUID
	LaneIndex                 int32
	Label                     string
}

type CreateQueuedRunParams struct {
	OrganizationID         uuid.UUID
	WorkspaceID            uuid.UUID
	ChallengePackVersionID uuid.UUID
	ChallengeInputSetID    *uuid.UUID
	CreatedByUserID        *uuid.UUID
	Name                   string
	ExecutionMode          string
	ExecutionPlan          json.RawMessage
	RunAgents              []CreateQueuedRunAgentParams
}

type CreateQueuedRunResult struct {
	Run       domain.Run
	RunAgents []domain.RunAgent
}

type RunAgentReplay struct {
	ID                   uuid.UUID
	RunAgentID           uuid.UUID
	ArtifactID           *uuid.UUID
	Summary              json.RawMessage
	LatestSequenceNumber *int64
	EventCount           int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type RunAgentScorecard struct {
	ID               uuid.UUID
	RunAgentID       uuid.UUID
	EvaluationSpecID uuid.UUID
	OverallScore     *float64
	CorrectnessScore *float64
	ReliabilityScore *float64
	LatencyScore     *float64
	CostScore        *float64
	Scorecard        json.RawMessage
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type EvaluationSpecRecord struct {
	ID                     uuid.UUID
	ChallengePackVersionID uuid.UUID
	Name                   string
	VersionNumber          int32
	JudgeMode              string
	Definition             json.RawMessage
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type JudgeResultRecord struct {
	ID                  uuid.UUID
	RunAgentID          uuid.UUID
	EvaluationSpecID    uuid.UUID
	ChallengeIdentityID *uuid.UUID
	JudgeKey            string
	Verdict             *string
	NormalizedScore     *float64
	RawOutput           json.RawMessage
	CreatedAt           time.Time
}

type MetricResultRecord struct {
	ID                  uuid.UUID
	RunAgentID          uuid.UUID
	EvaluationSpecID    uuid.UUID
	ChallengeIdentityID *uuid.UUID
	MetricKey           string
	MetricType          string
	NumericValue        *float64
	TextValue           *string
	BooleanValue        *bool
	Unit                *string
	Metadata            json.RawMessage
	CreatedAt           time.Time
}

type CreateEvaluationSpecParams struct {
	ChallengePackVersionID uuid.UUID
	Name                   string
	VersionNumber          int32
	JudgeMode              string
	Definition             json.RawMessage
}

type RunEvent struct {
	ID             int64
	RunID          uuid.UUID
	RunAgentID     uuid.UUID
	SequenceNumber int64
	EventType      runevents.Type
	Source         runevents.Source
	OccurredAt     time.Time
	ArtifactID     *uuid.UUID
	Payload        json.RawMessage
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{
		db:      db,
		queries: repositorysqlc.New(db),
	}
}

func (r *Repository) GetRunByID(ctx context.Context, id uuid.UUID) (domain.Run, error) {
	row, err := r.queries.GetRunByID(ctx, repositorysqlc.GetRunByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Run{}, ErrRunNotFound
		}
		return domain.Run{}, fmt.Errorf("get run by id: %w", err)
	}

	run, err := mapRun(row)
	if err != nil {
		return domain.Run{}, fmt.Errorf("map run: %w", err)
	}

	return run, nil
}

func (r *Repository) GetRunnableChallengePackVersionByID(ctx context.Context, id uuid.UUID) (RunnableChallengePackVersion, error) {
	row := r.db.QueryRow(ctx, `
		SELECT cpv.id, cpv.challenge_pack_id, cp.workspace_id
		FROM challenge_pack_versions cpv
		JOIN challenge_packs cp ON cp.id = cpv.challenge_pack_id
		WHERE cpv.id = $1
		  AND cpv.lifecycle_status = 'runnable'
		  AND cpv.archived_at IS NULL
		  AND cp.archived_at IS NULL
		LIMIT 1
	`, id)

	var version RunnableChallengePackVersion
	if err := row.Scan(&version.ID, &version.ChallengePackID, &version.WorkspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RunnableChallengePackVersion{}, ErrChallengePackVersionNotFound
		}
		return RunnableChallengePackVersion{}, fmt.Errorf("get runnable challenge pack version by id: %w", err)
	}

	return version, nil
}

func (r *Repository) GetChallengeInputSetByID(ctx context.Context, id uuid.UUID) (ChallengeInputSet, error) {
	row, err := r.queries.GetChallengeInputSetByID(ctx, repositorysqlc.GetChallengeInputSetByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChallengeInputSet{}, ErrChallengeInputSetNotFound
		}
		return ChallengeInputSet{}, fmt.Errorf("get challenge input set by id: %w", err)
	}

	return ChallengeInputSet{
		ID:                     row.ID,
		ChallengePackVersionID: row.ChallengePackVersionID,
	}, nil
}

func (r *Repository) ListRunnableDeploymentsWithLatestSnapshot(
	ctx context.Context,
	workspaceID uuid.UUID,
	deploymentIDs []uuid.UUID,
) ([]RunnableDeployment, error) {
	rows, err := r.queries.ListRunnableDeploymentsWithLatestSnapshot(ctx, repositorysqlc.ListRunnableDeploymentsWithLatestSnapshotParams{
		WorkspaceID:   workspaceID,
		DeploymentIds: deploymentIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list runnable deployments with latest snapshot: %w", err)
	}

	deployments := make([]RunnableDeployment, 0, len(rows))
	for _, row := range rows {
		deployments = append(deployments, RunnableDeployment{
			ID:                        row.ID,
			OrganizationID:            row.OrganizationID,
			WorkspaceID:               row.WorkspaceID,
			Name:                      row.Name,
			AgentDeploymentSnapshotID: row.AgentDeploymentSnapshotID,
		})
	}

	return deployments, nil
}

func (r *Repository) CreateQueuedRun(ctx context.Context, params CreateQueuedRunParams) (CreateQueuedRunResult, error) {
	if params.Name == "" {
		return CreateQueuedRunResult{}, ErrRunNameRequired
	}
	if len(params.RunAgents) == 0 {
		return CreateQueuedRunResult{}, ErrRunParticipantsRequired
	}
	if params.ExecutionMode != "single_agent" && params.ExecutionMode != "comparison" {
		return CreateQueuedRunResult{}, ErrInvalidExecutionMode
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return CreateQueuedRunResult{}, fmt.Errorf("begin queued run creation transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)
	queuedAt := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	executionPlan := cloneJSON(params.ExecutionPlan)
	if len(executionPlan) == 0 {
		executionPlan = json.RawMessage(`{}`)
	}

	runRow, err := queries.CreateRun(ctx, repositorysqlc.CreateRunParams{
		OrganizationID:         params.OrganizationID,
		WorkspaceID:            params.WorkspaceID,
		ChallengePackVersionID: params.ChallengePackVersionID,
		ChallengeInputSetID:    cloneUUIDPtr(params.ChallengeInputSetID),
		CreatedByUserID:        cloneUUIDPtr(params.CreatedByUserID),
		Name:                   params.Name,
		Status:                 string(domain.RunStatusQueued),
		ExecutionMode:          params.ExecutionMode,
		ExecutionPlan:          executionPlan,
		QueuedAt:               queuedAt,
	})
	if err != nil {
		return CreateQueuedRunResult{}, fmt.Errorf("create run: %w", err)
	}

	_, err = queries.InsertRunStatusHistory(ctx, repositorysqlc.InsertRunStatusHistoryParams{
		RunID:           runRow.ID,
		FromStatus:      nil,
		ToStatus:        string(domain.RunStatusQueued),
		Reason:          stringPtr("run created by api"),
		ChangedByUserID: cloneUUIDPtr(params.CreatedByUserID),
	})
	if err != nil {
		return CreateQueuedRunResult{}, fmt.Errorf("insert initial run status history: %w", err)
	}

	runAgents := make([]domain.RunAgent, 0, len(params.RunAgents))
	for _, runAgent := range params.RunAgents {
		if runAgent.Label == "" {
			return CreateQueuedRunResult{}, ErrRunAgentLabelRequired
		}

		runAgentRow, createErr := queries.CreateRunAgent(ctx, repositorysqlc.CreateRunAgentParams{
			OrganizationID:            params.OrganizationID,
			WorkspaceID:               params.WorkspaceID,
			RunID:                     runRow.ID,
			AgentDeploymentID:         runAgent.AgentDeploymentID,
			AgentDeploymentSnapshotID: runAgent.AgentDeploymentSnapshotID,
			LaneIndex:                 runAgent.LaneIndex,
			Label:                     runAgent.Label,
			Status:                    string(domain.RunAgentStatusQueued),
			QueuedAt:                  queuedAt,
		})
		if createErr != nil {
			return CreateQueuedRunResult{}, fmt.Errorf("create run agent lane %d: %w", runAgent.LaneIndex, createErr)
		}

		_, createErr = queries.InsertRunAgentStatusHistory(ctx, repositorysqlc.InsertRunAgentStatusHistoryParams{
			RunAgentID: runAgentRow.ID,
			FromStatus: nil,
			ToStatus:   string(domain.RunAgentStatusQueued),
			Reason:     stringPtr("run agent created by api"),
		})
		if createErr != nil {
			return CreateQueuedRunResult{}, fmt.Errorf("insert initial run-agent status history lane %d: %w", runAgent.LaneIndex, createErr)
		}

		mappedRunAgent, mapErr := mapRunAgent(runAgentRow)
		if mapErr != nil {
			return CreateQueuedRunResult{}, fmt.Errorf("map run agent lane %d: %w", runAgent.LaneIndex, mapErr)
		}
		runAgents = append(runAgents, mappedRunAgent)
	}

	run, err := mapRun(runRow)
	if err != nil {
		return CreateQueuedRunResult{}, fmt.Errorf("map run: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateQueuedRunResult{}, fmt.Errorf("commit queued run creation: %w", err)
	}

	return CreateQueuedRunResult{
		Run:       run,
		RunAgents: runAgents,
	}, nil
}

func (r *Repository) ListRunAgentsByRunID(ctx context.Context, runID uuid.UUID) ([]domain.RunAgent, error) {
	rows, err := r.queries.ListRunAgentsByRunID(ctx, repositorysqlc.ListRunAgentsByRunIDParams{RunID: runID})
	if err != nil {
		return nil, fmt.Errorf("list run agents by run id: %w", err)
	}

	runAgents := make([]domain.RunAgent, 0, len(rows))
	for _, row := range rows {
		runAgent, mapErr := mapRunAgent(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map run agent %s: %w", row.ID, mapErr)
		}
		runAgents = append(runAgents, runAgent)
	}

	return runAgents, nil
}

func (r *Repository) GetRunAgentByID(ctx context.Context, id uuid.UUID) (domain.RunAgent, error) {
	row, err := r.queries.GetRunAgentByID(ctx, repositorysqlc.GetRunAgentByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RunAgent{}, ErrRunAgentNotFound
		}
		return domain.RunAgent{}, fmt.Errorf("get run agent by id: %w", err)
	}

	runAgent, err := mapRunAgent(row)
	if err != nil {
		return domain.RunAgent{}, fmt.Errorf("map run agent: %w", err)
	}

	return runAgent, nil
}

func (r *Repository) GetRunAgentReplayByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (RunAgentReplay, error) {
	row, err := r.queries.GetRunAgentReplayByRunAgentID(ctx, repositorysqlc.GetRunAgentReplayByRunAgentIDParams{RunAgentID: runAgentID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RunAgentReplay{}, ErrRunAgentReplayNotFound
		}
		return RunAgentReplay{}, fmt.Errorf("get run-agent replay by run-agent id: %w", err)
	}

	replay, err := mapRunAgentReplay(row)
	if err != nil {
		return RunAgentReplay{}, fmt.Errorf("map run-agent replay: %w", err)
	}

	return replay, nil
}

func (r *Repository) CreateEvaluationSpec(ctx context.Context, params CreateEvaluationSpecParams) (EvaluationSpecRecord, error) {
	normalizedDefinition, err := normalizeEvaluationSpecDefinition(params.Definition)
	if err != nil {
		return EvaluationSpecRecord{}, fmt.Errorf("create evaluation spec: %w", err)
	}

	row, err := r.queries.CreateEvaluationSpec(ctx, repositorysqlc.CreateEvaluationSpecParams{
		ChallengePackVersionID: &params.ChallengePackVersionID,
		Name:                   params.Name,
		VersionNumber:          params.VersionNumber,
		JudgeMode:              params.JudgeMode,
		Definition:             normalizedDefinition,
	})
	if err != nil {
		return EvaluationSpecRecord{}, fmt.Errorf("create evaluation spec: %w", err)
	}

	record, err := mapEvaluationSpecRecord(row)
	if err != nil {
		return EvaluationSpecRecord{}, fmt.Errorf("map evaluation spec: %w", err)
	}

	return record, nil
}

func (r *Repository) GetEvaluationSpecByID(ctx context.Context, id uuid.UUID) (EvaluationSpecRecord, error) {
	row, err := r.queries.GetEvaluationSpecByID(ctx, repositorysqlc.GetEvaluationSpecByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvaluationSpecRecord{}, ErrEvaluationSpecNotFound
		}
		return EvaluationSpecRecord{}, fmt.Errorf("get evaluation spec by id: %w", err)
	}

	record, err := mapEvaluationSpecRecord(row)
	if err != nil {
		return EvaluationSpecRecord{}, fmt.Errorf("map evaluation spec: %w", err)
	}

	return record, nil
}

func (r *Repository) GetEvaluationSpecByChallengePackVersionAndVersion(
	ctx context.Context,
	challengePackVersionID uuid.UUID,
	name string,
	versionNumber int32,
) (EvaluationSpecRecord, error) {
	row, err := r.queries.GetEvaluationSpecByChallengePackVersionAndVersion(ctx, repositorysqlc.GetEvaluationSpecByChallengePackVersionAndVersionParams{
		ChallengePackVersionID: &challengePackVersionID,
		Name:                   name,
		VersionNumber:          versionNumber,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvaluationSpecRecord{}, ErrEvaluationSpecNotFound
		}
		return EvaluationSpecRecord{}, fmt.Errorf("get evaluation spec by challenge pack version and version: %w", err)
	}

	record, err := mapEvaluationSpecRecord(row)
	if err != nil {
		return EvaluationSpecRecord{}, fmt.Errorf("map evaluation spec: %w", err)
	}

	return record, nil
}

func (r *Repository) GetRunAgentScorecardByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (RunAgentScorecard, error) {
	row, err := r.queries.GetRunAgentScorecardByRunAgentID(ctx, repositorysqlc.GetRunAgentScorecardByRunAgentIDParams{RunAgentID: runAgentID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RunAgentScorecard{}, ErrRunAgentScorecardNotFound
		}
		return RunAgentScorecard{}, fmt.Errorf("get run-agent scorecard by run-agent id: %w", err)
	}

	scorecard, err := mapRunAgentScorecard(row)
	if err != nil {
		return RunAgentScorecard{}, fmt.Errorf("map run-agent scorecard: %w", err)
	}

	return scorecard, nil
}

func (r *Repository) StoreRunAgentEvaluationResults(ctx context.Context, evaluation scoring.RunAgentEvaluation) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin scoring result transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)
	for _, result := range evaluation.ValidatorResults {
		numericScore, err := numericFromFloat(result.NormalizedScore)
		if err != nil {
			return fmt.Errorf("encode validator normalized score for %s: %w", result.Key, err)
		}

		rawOutput := cloneJSON(result.RawOutput)
		if len(rawOutput) == 0 {
			rawOutput = json.RawMessage(`{}`)
		}

		if _, err := queries.UpsertJudgeResult(ctx, repositorysqlc.UpsertJudgeResultParams{
			RunAgentID:          evaluation.RunAgentID,
			EvaluationSpecID:    evaluation.EvaluationSpecID,
			ChallengeIdentityID: cloneUUIDPtr(result.ChallengeIdentityID),
			JudgeKey:            result.Key,
			Verdict:             cloneStringPtr(optionalString(result.Verdict)),
			NormalizedScore:     numericScore,
			RawOutput:           rawOutput,
		}); err != nil {
			return fmt.Errorf("upsert judge result %s: %w", result.Key, err)
		}
	}

	for _, result := range evaluation.MetricResults {
		numericValue, err := numericFromFloat(result.NumericValue)
		if err != nil {
			return fmt.Errorf("encode metric numeric value for %s: %w", result.Key, err)
		}

		metadata := cloneJSON(result.Metadata)
		if len(metadata) == 0 {
			metadata = json.RawMessage(`{}`)
		}

		if _, err := queries.UpsertMetricResult(ctx, repositorysqlc.UpsertMetricResultParams{
			RunAgentID:          evaluation.RunAgentID,
			EvaluationSpecID:    evaluation.EvaluationSpecID,
			ChallengeIdentityID: cloneUUIDPtr(result.ChallengeIdentityID),
			MetricKey:           result.Key,
			MetricType:          string(result.Type),
			NumericValue:        numericValue,
			TextValue:           cloneStringPtr(result.TextValue),
			BooleanValue:        cloneBoolPtr(result.BooleanValue),
			Unit:                cloneStringPtr(optionalString(result.Unit)),
			Metadata:            metadata,
		}); err != nil {
			return fmt.Errorf("upsert metric result %s: %w", result.Key, err)
		}
	}

	scorecard, err := buildRunAgentScorecardDocument(evaluation)
	if err != nil {
		return fmt.Errorf("marshal run-agent scorecard: %w", err)
	}

	overallScore, err := numericFromFloat(nil)
	if err != nil {
		return fmt.Errorf("encode overall score: %w", err)
	}
	correctnessScore, err := numericFromFloat(evaluation.DimensionScores[string(scoring.ScorecardDimensionCorrectness)])
	if err != nil {
		return fmt.Errorf("encode correctness score: %w", err)
	}
	reliabilityScore, err := numericFromFloat(evaluation.DimensionScores[string(scoring.ScorecardDimensionReliability)])
	if err != nil {
		return fmt.Errorf("encode reliability score: %w", err)
	}
	latencyScore, err := numericFromFloat(evaluation.DimensionScores[string(scoring.ScorecardDimensionLatency)])
	if err != nil {
		return fmt.Errorf("encode latency score: %w", err)
	}
	costScore, err := numericFromFloat(evaluation.DimensionScores[string(scoring.ScorecardDimensionCost)])
	if err != nil {
		return fmt.Errorf("encode cost score: %w", err)
	}

	if _, err := queries.UpsertRunAgentScorecard(ctx, repositorysqlc.UpsertRunAgentScorecardParams{
		RunAgentID:       evaluation.RunAgentID,
		EvaluationSpecID: evaluation.EvaluationSpecID,
		OverallScore:     overallScore,
		CorrectnessScore: correctnessScore,
		ReliabilityScore: reliabilityScore,
		LatencyScore:     latencyScore,
		CostScore:        costScore,
		Scorecard:        scorecard,
	}); err != nil {
		return fmt.Errorf("upsert run-agent scorecard: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit scoring result transaction: %w", err)
	}
	return nil
}

func buildRunAgentScorecardDocument(evaluation scoring.RunAgentEvaluation) (json.RawMessage, error) {
	type dimensionSummary struct {
		State  scoring.OutputState `json:"state"`
		Score  *float64            `json:"score,omitempty"`
		Reason string              `json:"reason,omitempty"`
	}

	type scorecardDocument struct {
		RunAgentID       uuid.UUID                   `json:"run_agent_id"`
		EvaluationSpecID uuid.UUID                   `json:"evaluation_spec_id"`
		Status           scoring.EvaluationStatus    `json:"status"`
		Warnings         []string                    `json:"warnings,omitempty"`
		Dimensions       map[string]dimensionSummary `json:"dimensions"`
		ValidatorSummary map[string]int              `json:"validator_summary"`
		MetricSummary    map[string]int              `json:"metric_summary"`
	}

	dimensions := make(map[string]dimensionSummary, len(evaluation.DimensionResults))
	for _, result := range evaluation.DimensionResults {
		dimensions[string(result.Dimension)] = dimensionSummary{
			State:  result.State,
			Score:  cloneFloat64Ptr(result.Score),
			Reason: result.Reason,
		}
	}

	validatorSummary := map[string]int{
		"total":       len(evaluation.ValidatorResults),
		"available":   0,
		"unavailable": 0,
		"error":       0,
		"pass":        0,
		"fail":        0,
	}
	for _, result := range evaluation.ValidatorResults {
		switch result.State {
		case scoring.OutputStateAvailable:
			validatorSummary["available"]++
		case scoring.OutputStateUnavailable:
			validatorSummary["unavailable"]++
		case scoring.OutputStateError:
			validatorSummary["error"]++
		}
		switch result.Verdict {
		case "pass":
			validatorSummary["pass"]++
		case "fail":
			validatorSummary["fail"]++
		}
	}

	metricSummary := map[string]int{
		"total":       len(evaluation.MetricResults),
		"available":   0,
		"unavailable": 0,
		"error":       0,
	}
	for _, result := range evaluation.MetricResults {
		switch result.State {
		case scoring.OutputStateAvailable:
			metricSummary["available"]++
		case scoring.OutputStateUnavailable:
			metricSummary["unavailable"]++
		case scoring.OutputStateError:
			metricSummary["error"]++
		}
	}

	document := scorecardDocument{
		RunAgentID:       evaluation.RunAgentID,
		EvaluationSpecID: evaluation.EvaluationSpecID,
		Status:           evaluation.Status,
		Warnings:         append([]string(nil), evaluation.Warnings...),
		Dimensions:       dimensions,
		ValidatorSummary: validatorSummary,
		MetricSummary:    metricSummary,
	}

	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func (r *Repository) ListJudgeResultsByRunAgentAndEvaluationSpec(ctx context.Context, runAgentID uuid.UUID, evaluationSpecID uuid.UUID) ([]JudgeResultRecord, error) {
	rows, err := r.queries.ListJudgeResultsByRunAgentAndEvaluationSpec(ctx, repositorysqlc.ListJudgeResultsByRunAgentAndEvaluationSpecParams{
		RunAgentID:       runAgentID,
		EvaluationSpecID: evaluationSpecID,
	})
	if err != nil {
		return nil, fmt.Errorf("list judge results by run-agent and evaluation spec: %w", err)
	}

	results := make([]JudgeResultRecord, 0, len(rows))
	for _, row := range rows {
		result, mapErr := mapJudgeResultRecord(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map judge result: %w", mapErr)
		}
		results = append(results, result)
	}
	return results, nil
}

func (r *Repository) ListMetricResultsByRunAgentAndEvaluationSpec(ctx context.Context, runAgentID uuid.UUID, evaluationSpecID uuid.UUID) ([]MetricResultRecord, error) {
	rows, err := r.queries.ListMetricResultsByRunAgentAndEvaluationSpec(ctx, repositorysqlc.ListMetricResultsByRunAgentAndEvaluationSpecParams{
		RunAgentID:       runAgentID,
		EvaluationSpecID: evaluationSpecID,
	})
	if err != nil {
		return nil, fmt.Errorf("list metric results by run-agent and evaluation spec: %w", err)
	}

	results := make([]MetricResultRecord, 0, len(rows))
	for _, row := range rows {
		result, mapErr := mapMetricResultRecord(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map metric result: %w", mapErr)
		}
		results = append(results, result)
	}
	return results, nil
}

func (r *Repository) RecordHostedRunEvent(ctx context.Context, params RecordHostedRunEventParams) (RunAgentReplay, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return RunAgentReplay{}, fmt.Errorf("begin hosted run event transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)
	if err := params.Event.ValidatePending(); err != nil {
		return RunAgentReplay{}, fmt.Errorf("validate hosted canonical event: %w", err)
	}

	insertedEvent, err := queries.InsertRunEvent(ctx, repositorysqlc.InsertRunEventParams{
		RunID:      params.Event.RunID,
		RunAgentID: params.Event.RunAgentID,
		EventType:  string(params.Event.EventType),
		ActorType:  string(params.Event.Source),
		OccurredAt: pgtype.Timestamptz{Time: params.Event.OccurredAt.UTC(), Valid: true},
		Payload:    cloneJSON(params.Event.Payload),
	})
	if err != nil {
		return RunAgentReplay{}, fmt.Errorf("insert hosted run event: %w", err)
	}

	summary := cloneJSON(params.Summary)
	if len(summary) == 0 {
		summary = json.RawMessage(`{}`)
	}

	replayRow, err := queries.UpsertRunAgentReplaySummary(ctx, repositorysqlc.UpsertRunAgentReplaySummaryParams{
		RunAgentID:           params.Event.RunAgentID,
		Summary:              summary,
		LatestSequenceNumber: int64Ptr(insertedEvent.SequenceNumber),
		EventCount:           insertedEvent.SequenceNumber,
	})
	if err != nil {
		return RunAgentReplay{}, fmt.Errorf("upsert run-agent replay summary: %w", err)
	}

	replay, err := mapRunAgentReplay(replayRow)
	if err != nil {
		return RunAgentReplay{}, fmt.Errorf("map run-agent replay: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return RunAgentReplay{}, fmt.Errorf("commit hosted run event: %w", err)
	}
	return replay, nil
}

func (r *Repository) RecordRunEvent(ctx context.Context, params RecordRunEventParams) (RunEvent, error) {
	if err := params.Event.ValidatePending(); err != nil {
		return RunEvent{}, fmt.Errorf("validate canonical run event: %w", err)
	}

	// Sequence assignment is append-only per run-agent via MAX(sequence_number)+1.
	// Callers must serialize writes for a given run_agent_id; concurrent inserts for
	// the same run-agent can race and one will fail on the unique sequence constraint.
	row, err := r.queries.InsertRunEvent(ctx, repositorysqlc.InsertRunEventParams{
		RunID:      params.Event.RunID,
		RunAgentID: params.Event.RunAgentID,
		EventType:  string(params.Event.EventType),
		ActorType:  string(params.Event.Source),
		OccurredAt: pgtype.Timestamptz{Time: params.Event.OccurredAt.UTC(), Valid: true},
		Payload:    cloneJSON(params.Event.Payload),
	})
	if err != nil {
		return RunEvent{}, fmt.Errorf("insert run event: %w", err)
	}

	event, err := mapRunEvent(row)
	if err != nil {
		return RunEvent{}, fmt.Errorf("map run event: %w", err)
	}
	return event, nil
}

func (r *Repository) ListRunEventsByRunAgentID(ctx context.Context, runAgentID uuid.UUID) ([]RunEvent, error) {
	rows, err := r.queries.ListRunEventsByRunAgentID(ctx, repositorysqlc.ListRunEventsByRunAgentIDParams{
		RunAgentID: runAgentID,
	})
	if err != nil {
		return nil, fmt.Errorf("list run events by run-agent id: %w", err)
	}

	events := make([]RunEvent, 0, len(rows))
	for _, row := range rows {
		event, mapErr := mapRunEvent(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map run event: %w", mapErr)
		}
		events = append(events, event)
	}
	return events, nil
}

func (r *Repository) SetRunTemporalIDs(ctx context.Context, params SetRunTemporalIDsParams) (domain.Run, error) {
	if params.TemporalWorkflowID == "" {
		return domain.Run{}, ErrTemporalWorkflowID
	}
	if params.TemporalRunID == "" {
		return domain.Run{}, ErrTemporalRunID
	}

	row, err := r.queries.SetRunTemporalIDs(ctx, repositorysqlc.SetRunTemporalIDsParams{
		ID:                 params.RunID,
		TemporalWorkflowID: stringPtr(params.TemporalWorkflowID),
		TemporalRunID:      stringPtr(params.TemporalRunID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			currentRow, getErr := r.queries.GetRunByID(ctx, repositorysqlc.GetRunByIDParams{ID: params.RunID})
			if getErr != nil {
				if errors.Is(getErr, pgx.ErrNoRows) {
					return domain.Run{}, ErrRunNotFound
				}
				return domain.Run{}, fmt.Errorf("load run after temporal id write miss: %w", getErr)
			}

			if temporalIDsMatch(currentRow, params) {
				run, mapErr := mapRun(currentRow)
				if mapErr != nil {
					return domain.Run{}, fmt.Errorf("map run: %w", mapErr)
				}
				return run, nil
			}

			return domain.Run{}, TemporalIDConflictError{
				RunID:                params.RunID,
				ExistingWorkflowID:   currentRow.TemporalWorkflowID,
				ExistingTemporalRun:  currentRow.TemporalRunID,
				RequestedWorkflowID:  params.TemporalWorkflowID,
				RequestedTemporalRun: params.TemporalRunID,
			}
		}
		return domain.Run{}, fmt.Errorf("set run temporal ids: %w", err)
	}

	run, err := mapRun(row)
	if err != nil {
		return domain.Run{}, fmt.Errorf("map run: %w", err)
	}

	return run, nil
}

func (r *Repository) TransitionRunStatus(ctx context.Context, params TransitionRunStatusParams) (domain.Run, error) {
	if !params.ToStatus.Valid() {
		return domain.Run{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunStatus, params.ToStatus)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Run{}, fmt.Errorf("begin run status transition transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)
	currentRow, err := queries.GetRunByID(ctx, repositorysqlc.GetRunByIDParams{ID: params.RunID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Run{}, ErrRunNotFound
		}
		return domain.Run{}, fmt.Errorf("load run for transition: %w", err)
	}

	currentStatus, err := domain.ParseRunStatus(currentRow.Status)
	if err != nil {
		return domain.Run{}, fmt.Errorf("load run status for transition: %w", err)
	}
	if !currentStatus.CanTransitionTo(params.ToStatus) {
		return domain.Run{}, InvalidTransitionError{
			Entity: "run",
			From:   string(currentStatus),
			To:     string(params.ToStatus),
		}
	}

	updatedRow, err := queries.UpdateRunStatus(ctx, repositorysqlc.UpdateRunStatusParams{
		ID:         params.RunID,
		FromStatus: string(currentStatus),
		ToStatus:   string(params.ToStatus),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Run{}, TransitionConflictError{
				Entity:   "run",
				ID:       params.RunID,
				Expected: string(currentStatus),
			}
		}
		return domain.Run{}, fmt.Errorf("update run status: %w", err)
	}

	_, err = queries.InsertRunStatusHistory(ctx, repositorysqlc.InsertRunStatusHistoryParams{
		RunID:           params.RunID,
		FromStatus:      stringPtr(string(currentStatus)),
		ToStatus:        string(params.ToStatus),
		Reason:          cloneStringPtr(params.Reason),
		ChangedByUserID: cloneUUIDPtr(params.ChangedByUserID),
	})
	if err != nil {
		return domain.Run{}, fmt.Errorf("insert run status history: %w", err)
	}

	run, err := mapRun(updatedRow)
	if err != nil {
		return domain.Run{}, fmt.Errorf("map run: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Run{}, fmt.Errorf("commit run status transition: %w", err)
	}

	return run, nil
}

func (r *Repository) TransitionRunAgentStatus(ctx context.Context, params TransitionRunAgentStatusParams) (domain.RunAgent, error) {
	if !params.ToStatus.Valid() {
		return domain.RunAgent{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunAgentStatus, params.ToStatus)
	}
	if params.ToStatus != domain.RunAgentStatusFailed && params.FailureReason != nil {
		return domain.RunAgent{}, ErrUnexpectedFailureCause
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.RunAgent{}, fmt.Errorf("begin run-agent status transition transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)
	currentRow, err := queries.GetRunAgentByID(ctx, repositorysqlc.GetRunAgentByIDParams{ID: params.RunAgentID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RunAgent{}, ErrRunAgentNotFound
		}
		return domain.RunAgent{}, fmt.Errorf("load run agent for transition: %w", err)
	}

	currentStatus, err := domain.ParseRunAgentStatus(currentRow.Status)
	if err != nil {
		return domain.RunAgent{}, fmt.Errorf("load run-agent status for transition: %w", err)
	}
	if !currentStatus.CanTransitionTo(params.ToStatus) {
		return domain.RunAgent{}, InvalidTransitionError{
			Entity: "run_agent",
			From:   string(currentStatus),
			To:     string(params.ToStatus),
		}
	}

	failureReason := cloneStringPtr(params.FailureReason)
	if params.ToStatus == domain.RunAgentStatusFailed && failureReason == nil {
		failureReason = cloneStringPtr(params.Reason)
	}
	historyReason := cloneStringPtr(params.Reason)
	if params.ToStatus == domain.RunAgentStatusFailed && historyReason == nil {
		historyReason = cloneStringPtr(failureReason)
	}

	updatedRow, err := queries.UpdateRunAgentStatus(ctx, repositorysqlc.UpdateRunAgentStatusParams{
		ID:            params.RunAgentID,
		FromStatus:    string(currentStatus),
		ToStatus:      string(params.ToStatus),
		FailureReason: failureReason,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RunAgent{}, TransitionConflictError{
				Entity:   "run_agent",
				ID:       params.RunAgentID,
				Expected: string(currentStatus),
			}
		}
		return domain.RunAgent{}, fmt.Errorf("update run-agent status: %w", err)
	}

	_, err = queries.InsertRunAgentStatusHistory(ctx, repositorysqlc.InsertRunAgentStatusHistoryParams{
		RunAgentID: params.RunAgentID,
		FromStatus: stringPtr(string(currentStatus)),
		ToStatus:   string(params.ToStatus),
		Reason:     historyReason,
	})
	if err != nil {
		return domain.RunAgent{}, fmt.Errorf("insert run-agent status history: %w", err)
	}

	runAgent, err := mapRunAgent(updatedRow)
	if err != nil {
		return domain.RunAgent{}, fmt.Errorf("map run agent: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RunAgent{}, fmt.Errorf("commit run-agent status transition: %w", err)
	}

	return runAgent, nil
}

func (r *Repository) InsertRunStatusHistory(ctx context.Context, params InsertRunStatusHistoryParams) (domain.RunStatusHistory, error) {
	if !params.ToStatus.Valid() {
		return domain.RunStatusHistory{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunStatus, params.ToStatus)
	}
	if params.FromStatus != nil && !params.FromStatus.Valid() {
		return domain.RunStatusHistory{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunStatus, *params.FromStatus)
	}
	if params.FromStatus != nil && !params.FromStatus.CanTransitionTo(params.ToStatus) {
		return domain.RunStatusHistory{}, InvalidTransitionError{
			Entity: "run",
			From:   string(*params.FromStatus),
			To:     string(params.ToStatus),
		}
	}

	row, err := r.queries.InsertRunStatusHistory(ctx, repositorysqlc.InsertRunStatusHistoryParams{
		RunID:           params.RunID,
		FromStatus:      runStatusPtr(params.FromStatus),
		ToStatus:        string(params.ToStatus),
		Reason:          cloneStringPtr(params.Reason),
		ChangedByUserID: cloneUUIDPtr(params.ChangedByUserID),
	})
	if err != nil {
		return domain.RunStatusHistory{}, fmt.Errorf("insert run status history: %w", err)
	}

	history, err := mapRunStatusHistory(row)
	if err != nil {
		return domain.RunStatusHistory{}, fmt.Errorf("map run status history: %w", err)
	}

	return history, nil
}

func (r *Repository) InsertRunAgentStatusHistory(ctx context.Context, params InsertRunAgentStatusHistoryParams) (domain.RunAgentStatusHistory, error) {
	if !params.ToStatus.Valid() {
		return domain.RunAgentStatusHistory{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunAgentStatus, params.ToStatus)
	}
	if params.FromStatus != nil && !params.FromStatus.Valid() {
		return domain.RunAgentStatusHistory{}, fmt.Errorf("%w: %q", domain.ErrInvalidRunAgentStatus, *params.FromStatus)
	}
	if params.FromStatus != nil && !params.FromStatus.CanTransitionTo(params.ToStatus) {
		return domain.RunAgentStatusHistory{}, InvalidTransitionError{
			Entity: "run_agent",
			From:   string(*params.FromStatus),
			To:     string(params.ToStatus),
		}
	}

	row, err := r.queries.InsertRunAgentStatusHistory(ctx, repositorysqlc.InsertRunAgentStatusHistoryParams{
		RunAgentID: params.RunAgentID,
		FromStatus: runAgentStatusPtr(params.FromStatus),
		ToStatus:   string(params.ToStatus),
		Reason:     cloneStringPtr(params.Reason),
	})
	if err != nil {
		return domain.RunAgentStatusHistory{}, fmt.Errorf("insert run-agent status history: %w", err)
	}

	history, err := mapRunAgentStatusHistory(row)
	if err != nil {
		return domain.RunAgentStatusHistory{}, fmt.Errorf("map run-agent status history: %w", err)
	}

	return history, nil
}

func mapRun(row repositorysqlc.Run) (domain.Run, error) {
	status, err := domain.ParseRunStatus(row.Status)
	if err != nil {
		return domain.Run{}, err
	}

	createdAt, err := requiredTime("runs.created_at", row.CreatedAt)
	if err != nil {
		return domain.Run{}, err
	}
	updatedAt, err := requiredTime("runs.updated_at", row.UpdatedAt)
	if err != nil {
		return domain.Run{}, err
	}

	return domain.Run{
		ID:                     row.ID,
		OrganizationID:         row.OrganizationID,
		WorkspaceID:            row.WorkspaceID,
		ChallengePackVersionID: row.ChallengePackVersionID,
		ChallengeInputSetID:    cloneUUIDPtr(row.ChallengeInputSetID),
		CreatedByUserID:        cloneUUIDPtr(row.CreatedByUserID),
		Name:                   row.Name,
		Status:                 status,
		ExecutionMode:          row.ExecutionMode,
		TemporalWorkflowID:     cloneStringPtr(row.TemporalWorkflowID),
		TemporalRunID:          cloneStringPtr(row.TemporalRunID),
		ExecutionPlan:          cloneJSON(row.ExecutionPlan),
		QueuedAt:               optionalTime(row.QueuedAt),
		StartedAt:              optionalTime(row.StartedAt),
		FinishedAt:             optionalTime(row.FinishedAt),
		CancelledAt:            optionalTime(row.CancelledAt),
		FailedAt:               optionalTime(row.FailedAt),
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
	}, nil
}

func mapRunAgent(row repositorysqlc.RunAgent) (domain.RunAgent, error) {
	status, err := domain.ParseRunAgentStatus(row.Status)
	if err != nil {
		return domain.RunAgent{}, err
	}

	createdAt, err := requiredTime("run_agents.created_at", row.CreatedAt)
	if err != nil {
		return domain.RunAgent{}, err
	}
	updatedAt, err := requiredTime("run_agents.updated_at", row.UpdatedAt)
	if err != nil {
		return domain.RunAgent{}, err
	}

	return domain.RunAgent{
		ID:                        row.ID,
		OrganizationID:            row.OrganizationID,
		WorkspaceID:               row.WorkspaceID,
		RunID:                     row.RunID,
		AgentDeploymentID:         row.AgentDeploymentID,
		AgentDeploymentSnapshotID: row.AgentDeploymentSnapshotID,
		LaneIndex:                 row.LaneIndex,
		Label:                     row.Label,
		Status:                    status,
		QueuedAt:                  optionalTime(row.QueuedAt),
		StartedAt:                 optionalTime(row.StartedAt),
		FinishedAt:                optionalTime(row.FinishedAt),
		FailureReason:             cloneStringPtr(row.FailureReason),
		CreatedAt:                 createdAt,
		UpdatedAt:                 updatedAt,
	}, nil
}

func mapRunAgentReplay(row repositorysqlc.RunAgentReplay) (RunAgentReplay, error) {
	createdAt, err := requiredTime("run_agent_replays.created_at", row.CreatedAt)
	if err != nil {
		return RunAgentReplay{}, err
	}
	updatedAt, err := requiredTime("run_agent_replays.updated_at", row.UpdatedAt)
	if err != nil {
		return RunAgentReplay{}, err
	}

	return RunAgentReplay{
		ID:                   row.ID,
		RunAgentID:           row.RunAgentID,
		ArtifactID:           cloneUUIDPtr(row.ArtifactID),
		Summary:              cloneJSON(row.Summary),
		LatestSequenceNumber: cloneInt64Ptr(row.LatestSequenceNumber),
		EventCount:           row.EventCount,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}, nil
}

func mapRunEvent(row repositorysqlc.RunEvent) (RunEvent, error) {
	occurredAt, err := requiredTime("run_events.occurred_at", row.OccurredAt)
	if err != nil {
		return RunEvent{}, err
	}

	event := RunEvent{
		ID:             row.ID,
		RunID:          row.RunID,
		RunAgentID:     row.RunAgentID,
		SequenceNumber: row.SequenceNumber,
		EventType:      runevents.Type(row.EventType),
		Source:         runevents.Source(row.ActorType),
		OccurredAt:     occurredAt,
		ArtifactID:     cloneUUIDPtr(row.ArtifactID),
		Payload:        cloneJSON(row.Payload),
	}
	if err := (runevents.Envelope{
		EventID:        fmt.Sprintf("persisted:%s:%d", event.RunAgentID.String(), event.SequenceNumber),
		SchemaVersion:  runevents.SchemaVersionV1,
		RunID:          event.RunID,
		RunAgentID:     event.RunAgentID,
		SequenceNumber: event.SequenceNumber,
		EventType:      event.EventType,
		Source:         event.Source,
		OccurredAt:     event.OccurredAt,
		Payload:        cloneJSON(event.Payload),
	}).ValidatePersisted(); err != nil {
		return RunEvent{}, fmt.Errorf("validate persisted run event: %w", err)
	}
	return event, nil
}

func mapRunAgentScorecard(row repositorysqlc.RunAgentScorecard) (RunAgentScorecard, error) {
	createdAt, err := requiredTime("run_agent_scorecards.created_at", row.CreatedAt)
	if err != nil {
		return RunAgentScorecard{}, err
	}
	updatedAt, err := requiredTime("run_agent_scorecards.updated_at", row.UpdatedAt)
	if err != nil {
		return RunAgentScorecard{}, err
	}

	return RunAgentScorecard{
		ID:               row.ID,
		RunAgentID:       row.RunAgentID,
		EvaluationSpecID: row.EvaluationSpecID,
		OverallScore:     numericPtr(row.OverallScore),
		CorrectnessScore: numericPtr(row.CorrectnessScore),
		ReliabilityScore: numericPtr(row.ReliabilityScore),
		LatencyScore:     numericPtr(row.LatencyScore),
		CostScore:        numericPtr(row.CostScore),
		Scorecard:        cloneJSON(row.Scorecard),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}, nil
}

func mapEvaluationSpecRecord(row repositorysqlc.EvaluationSpec) (EvaluationSpecRecord, error) {
	createdAt, err := requiredTime("evaluation_specs.created_at", row.CreatedAt)
	if err != nil {
		return EvaluationSpecRecord{}, err
	}
	updatedAt, err := requiredTime("evaluation_specs.updated_at", row.UpdatedAt)
	if err != nil {
		return EvaluationSpecRecord{}, err
	}
	if row.ChallengePackVersionID == nil {
		return EvaluationSpecRecord{}, fmt.Errorf("evaluation_specs.challenge_pack_version_id is required")
	}

	return EvaluationSpecRecord{
		ID:                     row.ID,
		ChallengePackVersionID: *row.ChallengePackVersionID,
		Name:                   row.Name,
		VersionNumber:          row.VersionNumber,
		JudgeMode:              row.JudgeMode,
		Definition:             cloneJSON(row.Definition),
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
	}, nil
}

func mapJudgeResultRecord(row repositorysqlc.JudgeResult) (JudgeResultRecord, error) {
	createdAt, err := requiredTime("judge_results.created_at", row.CreatedAt)
	if err != nil {
		return JudgeResultRecord{}, err
	}

	return JudgeResultRecord{
		ID:                  row.ID,
		RunAgentID:          row.RunAgentID,
		EvaluationSpecID:    row.EvaluationSpecID,
		ChallengeIdentityID: cloneUUIDPtr(row.ChallengeIdentityID),
		JudgeKey:            row.JudgeKey,
		Verdict:             cloneStringPtr(row.Verdict),
		NormalizedScore:     numericPtr(row.NormalizedScore),
		RawOutput:           cloneJSON(row.RawOutput),
		CreatedAt:           createdAt,
	}, nil
}

func mapMetricResultRecord(row repositorysqlc.MetricResult) (MetricResultRecord, error) {
	createdAt, err := requiredTime("metric_results.created_at", row.CreatedAt)
	if err != nil {
		return MetricResultRecord{}, err
	}

	return MetricResultRecord{
		ID:                  row.ID,
		RunAgentID:          row.RunAgentID,
		EvaluationSpecID:    row.EvaluationSpecID,
		ChallengeIdentityID: cloneUUIDPtr(row.ChallengeIdentityID),
		MetricKey:           row.MetricKey,
		MetricType:          row.MetricType,
		NumericValue:        numericPtr(row.NumericValue),
		TextValue:           cloneStringPtr(row.TextValue),
		BooleanValue:        cloneBoolPtr(row.BooleanValue),
		Unit:                cloneStringPtr(row.Unit),
		Metadata:            cloneJSON(row.Metadata),
		CreatedAt:           createdAt,
	}, nil
}

func normalizeEvaluationSpecDefinition(definition json.RawMessage) (json.RawMessage, error) {
	var spec scoring.EvaluationSpec
	if err := json.Unmarshal(definition, &spec); err != nil {
		return nil, fmt.Errorf("decode evaluation spec definition: %w", err)
	}

	normalized, err := scoring.MarshalDefinition(spec)
	if err != nil {
		return nil, fmt.Errorf("validate evaluation spec definition: %w", err)
	}

	return normalized, nil
}

func mapRunStatusHistory(row repositorysqlc.RunStatusHistory) (domain.RunStatusHistory, error) {
	toStatus, err := domain.ParseRunStatus(row.ToStatus)
	if err != nil {
		return domain.RunStatusHistory{}, err
	}

	fromStatus, err := parseOptionalRunStatus(row.FromStatus)
	if err != nil {
		return domain.RunStatusHistory{}, err
	}

	changedAt, err := requiredTime("run_status_history.changed_at", row.ChangedAt)
	if err != nil {
		return domain.RunStatusHistory{}, err
	}

	return domain.RunStatusHistory{
		ID:              row.ID,
		RunID:           row.RunID,
		FromStatus:      fromStatus,
		ToStatus:        toStatus,
		Reason:          cloneStringPtr(row.Reason),
		ChangedByUserID: cloneUUIDPtr(row.ChangedByUserID),
		ChangedAt:       changedAt,
	}, nil
}

func mapRunAgentStatusHistory(row repositorysqlc.RunAgentStatusHistory) (domain.RunAgentStatusHistory, error) {
	toStatus, err := domain.ParseRunAgentStatus(row.ToStatus)
	if err != nil {
		return domain.RunAgentStatusHistory{}, err
	}

	fromStatus, err := parseOptionalRunAgentStatus(row.FromStatus)
	if err != nil {
		return domain.RunAgentStatusHistory{}, err
	}

	changedAt, err := requiredTime("run_agent_status_history.changed_at", row.ChangedAt)
	if err != nil {
		return domain.RunAgentStatusHistory{}, err
	}

	return domain.RunAgentStatusHistory{
		ID:         row.ID,
		RunAgentID: row.RunAgentID,
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		Reason:     cloneStringPtr(row.Reason),
		ChangedAt:  changedAt,
	}, nil
}

func parseOptionalRunStatus(raw *string) (*domain.RunStatus, error) {
	if raw == nil {
		return nil, nil
	}

	status, err := domain.ParseRunStatus(*raw)
	if err != nil {
		return nil, err
	}

	return &status, nil
}

func parseOptionalRunAgentStatus(raw *string) (*domain.RunAgentStatus, error) {
	if raw == nil {
		return nil, nil
	}

	status, err := domain.ParseRunAgentStatus(*raw)
	if err != nil {
		return nil, err
	}

	return &status, nil
}

func requiredTime(field string, value pgtype.Timestamptz) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, fmt.Errorf("%s is null", field)
	}
	return value.Time, nil
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return timePtr(value.Time)
}

func optionalInt64(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	cloned := value.Int64
	return &cloned
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneJSON(value []byte) json.RawMessage {
	if value == nil {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}

func normalizeJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return cloneJSON(value)
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return stringPtr(*value)
}

func cloneUUIDPtr(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func numericPtr(value pgtype.Numeric) *float64 {
	if !value.Valid {
		return nil
	}

	float8, err := value.Float64Value()
	if err != nil || !float8.Valid {
		return nil
	}

	return cloneFloat64Ptr(&float8.Float64)
}

func numericFromFloat(value *float64) (pgtype.Numeric, error) {
	if value == nil {
		return pgtype.Numeric{}, nil
	}

	var numeric pgtype.Numeric
	if err := numeric.Scan(strconv.FormatFloat(*value, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}, err
	}
	return numeric, nil
}

func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func runStatusPtr(status *domain.RunStatus) *string {
	if status == nil {
		return nil
	}
	return stringPtr(string(*status))
}

func runAgentStatusPtr(status *domain.RunAgentStatus) *string {
	if status == nil {
		return nil
	}
	return stringPtr(string(*status))
}

func stringPtr(value string) *string {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}

func (r *Repository) ListRunsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit int32, offset int32) ([]domain.Run, error) {
	rows, err := r.queries.ListRunsByWorkspaceID(ctx, repositorysqlc.ListRunsByWorkspaceIDParams{
		WorkspaceID:  workspaceID,
		ResultLimit:  limit,
		ResultOffset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list runs by workspace id: %w", err)
	}

	runs := make([]domain.Run, 0, len(rows))
	for _, row := range rows {
		run, mapErr := mapRun(row)
		if mapErr != nil {
			return nil, fmt.Errorf("map run %s: %w", row.ID, mapErr)
		}
		runs = append(runs, run)
	}

	return runs, nil
}

func (r *Repository) CountRunsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (int64, error) {
	count, err := r.queries.CountRunsByWorkspaceID(ctx, repositorysqlc.CountRunsByWorkspaceIDParams{
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return 0, fmt.Errorf("count runs by workspace id: %w", err)
	}

	return count, nil
}

type AgentDeploymentSummary struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	WorkspaceID      uuid.UUID
	Name             string
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LatestSnapshotID *uuid.UUID
}

func (r *Repository) ListActiveAgentDeploymentsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]AgentDeploymentSummary, error) {
	rows, err := r.queries.ListActiveAgentDeploymentsByWorkspaceID(ctx, repositorysqlc.ListActiveAgentDeploymentsByWorkspaceIDParams{
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list active agent deployments by workspace id: %w", err)
	}

	deployments := make([]AgentDeploymentSummary, 0, len(rows))
	for _, row := range rows {
		createdAt, err := requiredTime("agent_deployments.created_at", row.CreatedAt)
		if err != nil {
			return nil, err
		}
		updatedAt, err := requiredTime("agent_deployments.updated_at", row.UpdatedAt)
		if err != nil {
			return nil, err
		}

		deployments = append(deployments, AgentDeploymentSummary{
			ID:               row.ID,
			OrganizationID:   row.OrganizationID,
			WorkspaceID:      row.WorkspaceID,
			Name:             row.Name,
			Status:           row.Status,
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
			LatestSnapshotID: cloneUUIDPtr(row.LatestSnapshotID),
		})
	}

	return deployments, nil
}

type ChallengePackSummary struct {
	ID          uuid.UUID
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ChallengePackVersionSummary struct {
	ID              uuid.UUID
	ChallengePackID uuid.UUID
	VersionNumber   int32
	LifecycleStatus string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (r *Repository) ListChallengePacks(ctx context.Context) ([]ChallengePackSummary, error) {
	rows, err := r.queries.ListChallengePacks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list challenge packs: %w", err)
	}

	packs := make([]ChallengePackSummary, 0, len(rows))
	for _, row := range rows {
		createdAt, err := requiredTime("challenge_packs.created_at", row.CreatedAt)
		if err != nil {
			return nil, err
		}
		updatedAt, err := requiredTime("challenge_packs.updated_at", row.UpdatedAt)
		if err != nil {
			return nil, err
		}

		packs = append(packs, ChallengePackSummary{
			ID:          row.ID,
			Name:        row.Name,
			Description: cloneStringPtr(row.Description),
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
	}

	return packs, nil
}

func (r *Repository) ListRunnableChallengePVersionsByPackID(ctx context.Context, challengePackID uuid.UUID) ([]ChallengePackVersionSummary, error) {
	rows, err := r.queries.ListRunnableChallengePVersionsByPackID(ctx, repositorysqlc.ListRunnableChallengePVersionsByPackIDParams{
		ChallengePackID: challengePackID,
	})
	if err != nil {
		return nil, fmt.Errorf("list runnable challenge pack versions by pack id: %w", err)
	}

	versions := make([]ChallengePackVersionSummary, 0, len(rows))
	for _, row := range rows {
		createdAt, err := requiredTime("challenge_pack_versions.created_at", row.CreatedAt)
		if err != nil {
			return nil, err
		}
		updatedAt, err := requiredTime("challenge_pack_versions.updated_at", row.UpdatedAt)
		if err != nil {
			return nil, err
		}

		versions = append(versions, ChallengePackVersionSummary{
			ID:              row.ID,
			ChallengePackID: row.ChallengePackID,
			VersionNumber:   row.VersionNumber,
			LifecycleStatus: row.LifecycleStatus,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
		})
	}

	return versions, nil
}

var (
	ErrAgentBuildNotFound        = errors.New("agent build not found")
	ErrAgentBuildVersionNotFound = errors.New("agent build version not found")
	ErrWorkspaceNotFound           = errors.New("workspace not found")
	ErrUserNotFound                = errors.New("user not found")
	ErrUserAlreadyExists           = errors.New("user already exists")
	ErrOrganizationNotFound        = errors.New("organization not found")
	ErrOrganizationLimitReached    = errors.New("organization limit reached")
	ErrSlugTaken                   = errors.New("slug taken")
	ErrMembershipNotFound          = errors.New("membership not found")
	ErrAlreadyMember               = errors.New("already a member")
	ErrLastOrgAdmin                = errors.New("cannot remove or demote the last org admin")
	ErrLastWorkspaceAdmin          = errors.New("cannot remove or demote the last workspace admin")
	ErrOrgMembershipRequired       = errors.New("user must be a member of the organization first")
	ErrInviteExpired               = errors.New("invite expired")
	ErrAlreadyOnboarded            = errors.New("already onboarded")
)

type User struct {
	ID           uuid.UUID
	WorkOSUserID string
	Email        string
	DisplayName  string
}

type WorkspaceMembershipRow struct {
	WorkspaceID uuid.UUID
	Role        string
}

type OrgMembershipRow struct {
	OrganizationID uuid.UUID
	Role           string
}

type CreateUserInput struct {
	WorkOSUserID string
	Email        string
	DisplayName  string
}

type UserMeOrgRow struct {
	ID   uuid.UUID
	Name string
	Slug string
	Role string
}

type UserMeWorkspaceRow struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	Role           string
}

type OrganizationRow struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateOrgWithAdminInput struct {
	Name   string
	Slug   string
	UserID uuid.UUID
}

type UpdateOrgInput struct {
	Name   *string
	Status *string
}

type WorkspaceRow struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateWorkspaceWithAdminInput struct {
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	UserID         uuid.UUID
}

type UpdateWorkspaceInput struct {
	Name   *string
	Status *string
}

type OrgMembershipFullRow struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	UserID           uuid.UUID
	Email            string
	DisplayName      string
	Role             string
	MembershipStatus string
	CreatedAt        time.Time
	UpdatedAt        time.Time // set by DB trigger on every UPDATE
}

type CreateOrgMembershipInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Role           string
}

type UpdateOrgMembershipInput struct {
	Role   *string
	Status *string
}

type WorkspaceMembershipFullRow struct {
	ID               uuid.UUID
	WorkspaceID      uuid.UUID
	OrganizationID   uuid.UUID
	UserID           uuid.UUID
	Email            string
	DisplayName      string
	Role             string
	MembershipStatus string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateWorkspaceMembershipInput struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	UserID         uuid.UUID
	Role           string
}

type UpdateWorkspaceMembershipInput struct {
	Role   *string
	Status *string
}

type OnboardInput struct {
	UserID            uuid.UUID
	OrganizationName  string
	OrganizationSlug  string
	WorkspaceName     string
	WorkspaceSlug     string
}

type OnboardResult struct {
	Organization OrganizationRow
	Workspace    WorkspaceRow
}

func (r *Repository) GetUserByWorkOSID(ctx context.Context, workosUserID string) (User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, workos_user_id, email, COALESCE(display_name, '')
		FROM users
		WHERE workos_user_id = $1 AND archived_at IS NULL
	`, workosUserID)

	var user User
	err := row.Scan(&user.ID, &user.WorkOSUserID, &user.Email, &user.DisplayName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("get user by workos id: %w", err)
	}
	return user, nil
}

func (r *Repository) GetActiveWorkspaceMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]WorkspaceMembershipRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT wm.workspace_id, wm.role
		FROM workspace_memberships wm
		WHERE wm.user_id = $1 AND wm.membership_status = 'active'
		ORDER BY wm.workspace_id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get active workspace memberships by user id: %w", err)
	}
	defer rows.Close()

	var memberships []WorkspaceMembershipRow
	for rows.Next() {
		var m WorkspaceMembershipRow
		if err := rows.Scan(&m.WorkspaceID, &m.Role); err != nil {
			return nil, fmt.Errorf("scan workspace membership: %w", err)
		}
		memberships = append(memberships, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace memberships: %w", err)
	}
	if memberships == nil {
		memberships = []WorkspaceMembershipRow{}
	}
	return memberships, nil
}

func (r *Repository) GetActiveOrganizationMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]OrgMembershipRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT om.organization_id, om.role
		FROM organization_memberships om
		WHERE om.user_id = $1 AND om.membership_status = 'active'
		ORDER BY om.organization_id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get active organization memberships by user id: %w", err)
	}
	defer rows.Close()

	var memberships []OrgMembershipRow
	for rows.Next() {
		var m OrgMembershipRow
		if err := rows.Scan(&m.OrganizationID, &m.Role); err != nil {
			return nil, fmt.Errorf("scan organization membership: %w", err)
		}
		memberships = append(memberships, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate organization memberships: %w", err)
	}
	if memberships == nil {
		memberships = []OrgMembershipRow{}
	}
	return memberships, nil
}

func (r *Repository) CreateUser(ctx context.Context, input CreateUserInput) (User, error) {
	var user User
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (id, workos_user_id, email, display_name)
		VALUES (gen_random_uuid(), $1, $2, $3)
		RETURNING id, workos_user_id, email, COALESCE(display_name, '')
	`, input.WorkOSUserID, input.Email, input.DisplayName).Scan(
		&user.ID, &user.WorkOSUserID, &user.Email, &user.DisplayName,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, fmt.Errorf("%w: user already exists", ErrUserAlreadyExists)
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (r *Repository) LinkWorkOSUser(ctx context.Context, userID uuid.UUID, workosUserID string) (User, error) {
	var user User
	err := r.db.QueryRow(ctx, `
		UPDATE users SET workos_user_id = $2
		WHERE id = $1 AND workos_user_id LIKE 'pending:%'
		RETURNING id, workos_user_id, email, COALESCE(display_name, '')
	`, userID, workosUserID).Scan(&user.ID, &user.WorkOSUserID, &user.Email, &user.DisplayName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("link workos user: %w", err)
	}
	return user, nil
}

func (r *Repository) GetOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]UserMeOrgRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT o.id, o.name, o.slug, om.role
		FROM organizations o
		JOIN organization_memberships om ON om.organization_id = o.id
		WHERE om.user_id = $1 AND om.membership_status = 'active'
		  AND o.status = 'active'
		ORDER BY o.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get organizations for user: %w", err)
	}
	defer rows.Close()

	var orgs []UserMeOrgRow
	for rows.Next() {
		var o UserMeOrgRow
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.Role); err != nil {
			return nil, fmt.Errorf("scan organization: %w", err)
		}
		orgs = append(orgs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate organizations: %w", err)
	}
	if orgs == nil {
		orgs = []UserMeOrgRow{}
	}
	return orgs, nil
}

func (r *Repository) GetWorkspacesForUser(ctx context.Context, userID uuid.UUID) ([]UserMeWorkspaceRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT w.id, w.organization_id, w.name, w.slug, wm.role
		FROM workspaces w
		JOIN workspace_memberships wm ON wm.workspace_id = w.id
		WHERE wm.user_id = $1 AND wm.membership_status = 'active'
		  AND w.status = 'active'
		ORDER BY w.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get workspaces for user: %w", err)
	}
	defer rows.Close()

	var workspaces []UserMeWorkspaceRow
	for rows.Next() {
		var w UserMeWorkspaceRow
		if err := rows.Scan(&w.ID, &w.OrganizationID, &w.Name, &w.Slug, &w.Role); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		workspaces = append(workspaces, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspaces: %w", err)
	}
	if workspaces == nil {
		workspaces = []UserMeWorkspaceRow{}
	}
	return workspaces, nil
}

func (r *Repository) GetAllWorkspacesForOrgs(ctx context.Context, orgIDs []uuid.UUID) ([]UserMeWorkspaceRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT w.id, w.organization_id, w.name, w.slug, '' AS role
		FROM workspaces w
		WHERE w.organization_id = ANY($1)
		  AND w.status = 'active'
		ORDER BY w.name
	`, orgIDs)
	if err != nil {
		return nil, fmt.Errorf("get all workspaces for orgs: %w", err)
	}
	defer rows.Close()

	var workspaces []UserMeWorkspaceRow
	for rows.Next() {
		var w UserMeWorkspaceRow
		if err := rows.Scan(&w.ID, &w.OrganizationID, &w.Name, &w.Slug, &w.Role); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		workspaces = append(workspaces, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspaces: %w", err)
	}
	if workspaces == nil {
		workspaces = []UserMeWorkspaceRow{}
	}
	return workspaces, nil
}

// --- Onboarding ---

func (r *Repository) Onboard(ctx context.Context, input OnboardInput) (OnboardResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return OnboardResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Insert organization.
	var org OrganizationRow
	err = tx.QueryRow(ctx, `
		INSERT INTO organizations (id, name, slug, status)
		VALUES (gen_random_uuid(), $1, $2, 'active')
		RETURNING id, name, slug, status, created_at, updated_at
	`, input.OrganizationName, input.OrganizationSlug).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Status, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "slug") {
			return OnboardResult{}, ErrSlugTaken
		}
		return OnboardResult{}, fmt.Errorf("insert organization: %w", err)
	}

	// 2. Insert workspace.
	var ws WorkspaceRow
	err = tx.QueryRow(ctx, `
		INSERT INTO workspaces (id, organization_id, name, slug, status)
		VALUES (gen_random_uuid(), $1, $2, $3, 'active')
		RETURNING id, organization_id, name, slug, status, created_at, updated_at
	`, org.ID, input.WorkspaceName, input.WorkspaceSlug).Scan(
		&ws.ID, &ws.OrganizationID, &ws.Name, &ws.Slug, &ws.Status, &ws.CreatedAt, &ws.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "slug") {
			return OnboardResult{}, ErrSlugTaken
		}
		return OnboardResult{}, fmt.Errorf("insert workspace: %w", err)
	}

	// 3. Insert org_admin membership.
	_, err = tx.Exec(ctx, `
		INSERT INTO organization_memberships (id, organization_id, user_id, role, membership_status)
		VALUES (gen_random_uuid(), $1, $2, 'org_admin', 'active')
	`, org.ID, input.UserID)
	if err != nil {
		return OnboardResult{}, fmt.Errorf("insert org admin membership: %w", err)
	}

	// 4. Insert workspace_admin membership.
	_, err = tx.Exec(ctx, `
		INSERT INTO workspace_memberships (id, organization_id, workspace_id, user_id, role, membership_status)
		VALUES (gen_random_uuid(), $1, $2, $3, 'workspace_admin', 'active')
	`, org.ID, ws.ID, input.UserID)
	if err != nil {
		return OnboardResult{}, fmt.Errorf("insert workspace admin membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return OnboardResult{}, fmt.Errorf("commit tx: %w", err)
	}

	return OnboardResult{
		Organization: org,
		Workspace:    ws,
	}, nil
}

// --- Workspace Membership CRUD ---

func (r *Repository) ListWorkspaceMemberships(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]WorkspaceMembershipFullRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT wm.id, wm.workspace_id, wm.organization_id, wm.user_id,
		       u.email, COALESCE(u.display_name, ''),
		       wm.role, wm.membership_status, wm.created_at, wm.updated_at
		FROM workspace_memberships wm
		JOIN users u ON u.id = wm.user_id
		WHERE wm.workspace_id = $1 AND wm.membership_status IN ('active', 'invited')
		ORDER BY wm.created_at
		LIMIT $2 OFFSET $3
	`, workspaceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list workspace memberships: %w", err)
	}
	defer rows.Close()

	var memberships []WorkspaceMembershipFullRow
	for rows.Next() {
		var m WorkspaceMembershipFullRow
		if err := rows.Scan(&m.ID, &m.WorkspaceID, &m.OrganizationID, &m.UserID,
			&m.Email, &m.DisplayName, &m.Role, &m.MembershipStatus, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workspace membership: %w", err)
		}
		memberships = append(memberships, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace memberships: %w", err)
	}
	if memberships == nil {
		memberships = []WorkspaceMembershipFullRow{}
	}
	return memberships, nil
}

func (r *Repository) CountWorkspaceMemberships(ctx context.Context, workspaceID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM workspace_memberships
		WHERE workspace_id = $1 AND membership_status IN ('active', 'invited')
	`, workspaceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count workspace memberships: %w", err)
	}
	return count, nil
}

func (r *Repository) GetWorkspaceMembershipByWorkspaceAndUser(ctx context.Context, workspaceID, userID uuid.UUID) (WorkspaceMembershipFullRow, error) {
	var m WorkspaceMembershipFullRow
	err := r.db.QueryRow(ctx, `
		SELECT wm.id, wm.workspace_id, wm.organization_id, wm.user_id,
		       u.email, COALESCE(u.display_name, ''),
		       wm.role, wm.membership_status, wm.created_at, wm.updated_at
		FROM workspace_memberships wm
		JOIN users u ON u.id = wm.user_id
		WHERE wm.workspace_id = $1 AND wm.user_id = $2
	`, workspaceID, userID).Scan(&m.ID, &m.WorkspaceID, &m.OrganizationID, &m.UserID,
		&m.Email, &m.DisplayName, &m.Role, &m.MembershipStatus, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkspaceMembershipFullRow{}, ErrMembershipNotFound
		}
		return WorkspaceMembershipFullRow{}, fmt.Errorf("get workspace membership by workspace and user: %w", err)
	}
	return m, nil
}

func (r *Repository) CreateWorkspaceMembership(ctx context.Context, input CreateWorkspaceMembershipInput) (WorkspaceMembershipFullRow, error) {
	var m WorkspaceMembershipFullRow
	err := r.db.QueryRow(ctx, `
		INSERT INTO workspace_memberships (id, organization_id, workspace_id, user_id, role, membership_status)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, 'invited')
		RETURNING id, organization_id, workspace_id, user_id, role, membership_status, created_at, updated_at
	`, input.OrganizationID, input.WorkspaceID, input.UserID, input.Role).Scan(
		&m.ID, &m.OrganizationID, &m.WorkspaceID, &m.UserID, &m.Role, &m.MembershipStatus, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return WorkspaceMembershipFullRow{}, ErrAlreadyMember
		}
		return WorkspaceMembershipFullRow{}, fmt.Errorf("create workspace membership: %w", err)
	}

	var user User
	if err = r.db.QueryRow(ctx, `SELECT email, COALESCE(display_name, '') FROM users WHERE id = $1`, input.UserID).Scan(&user.Email, &user.DisplayName); err != nil {
		return m, nil // membership created; user details are non-critical
	}
	m.Email = user.Email
	m.DisplayName = user.DisplayName

	return m, nil
}

func (r *Repository) GetWorkspaceMembershipByID(ctx context.Context, membershipID uuid.UUID) (WorkspaceMembershipFullRow, error) {
	var m WorkspaceMembershipFullRow
	err := r.db.QueryRow(ctx, `
		SELECT wm.id, wm.workspace_id, wm.organization_id, wm.user_id,
		       u.email, COALESCE(u.display_name, ''),
		       wm.role, wm.membership_status, wm.created_at, wm.updated_at
		FROM workspace_memberships wm
		JOIN users u ON u.id = wm.user_id
		WHERE wm.id = $1
	`, membershipID).Scan(&m.ID, &m.WorkspaceID, &m.OrganizationID, &m.UserID,
		&m.Email, &m.DisplayName, &m.Role, &m.MembershipStatus, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkspaceMembershipFullRow{}, ErrMembershipNotFound
		}
		return WorkspaceMembershipFullRow{}, fmt.Errorf("get workspace membership by id: %w", err)
	}
	return m, nil
}

func (r *Repository) UpdateWorkspaceMembership(ctx context.Context, membershipID uuid.UUID, input UpdateWorkspaceMembershipInput) (WorkspaceMembershipFullRow, error) {
	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if input.Role != nil {
		setClauses = append(setClauses, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *input.Role)
		argIdx++
	}
	if input.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("membership_status = $%d", argIdx))
		args = append(args, *input.Status)
		argIdx++
		if *input.Status == "archived" {
			setClauses = append(setClauses, fmt.Sprintf("archived_at = $%d", argIdx))
			args = append(args, time.Now())
			argIdx++
		} else {
			setClauses = append(setClauses, "archived_at = NULL")
		}
	}

	if len(setClauses) == 0 {
		return r.GetWorkspaceMembershipByID(ctx, membershipID)
	}

	query := fmt.Sprintf(`
		UPDATE workspace_memberships SET %s
		WHERE id = $%d
		RETURNING id, organization_id, workspace_id, user_id, role, membership_status, created_at, updated_at
	`, strings.Join(setClauses, ", "), argIdx)
	args = append(args, membershipID)

	var m WorkspaceMembershipFullRow
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&m.ID, &m.OrganizationID, &m.WorkspaceID, &m.UserID, &m.Role, &m.MembershipStatus, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkspaceMembershipFullRow{}, ErrMembershipNotFound
		}
		return WorkspaceMembershipFullRow{}, fmt.Errorf("update workspace membership: %w", err)
	}

	var user User
	if err = r.db.QueryRow(ctx, `SELECT email, COALESCE(display_name, '') FROM users WHERE id = $1`, m.UserID).Scan(&user.Email, &user.DisplayName); err != nil {
		return m, nil // membership updated; user details are non-critical
	}
	m.Email = user.Email
	m.DisplayName = user.DisplayName

	return m, nil
}

func (r *Repository) CountActiveWorkspaceAdmins(ctx context.Context, workspaceID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM workspace_memberships
		WHERE workspace_id = $1 AND role = 'workspace_admin' AND membership_status = 'active'
	`, workspaceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active workspace admins: %w", err)
	}
	return count, nil
}

// --- Organization Membership CRUD ---

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := r.db.QueryRow(ctx, `
		SELECT id, workos_user_id, email, COALESCE(display_name, '')
		FROM users WHERE email = $1 AND archived_at IS NULL
	`, email).Scan(&user.ID, &user.WorkOSUserID, &user.Email, &user.DisplayName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (r *Repository) ListOrgMemberships(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]OrgMembershipFullRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT om.id, om.organization_id, om.user_id, u.email, COALESCE(u.display_name, ''),
		       om.role, om.membership_status, om.created_at, om.updated_at
		FROM organization_memberships om
		JOIN users u ON u.id = om.user_id
		WHERE om.organization_id = $1 AND om.membership_status IN ('active', 'invited')
		ORDER BY om.created_at
		LIMIT $2 OFFSET $3
	`, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list org memberships: %w", err)
	}
	defer rows.Close()

	var memberships []OrgMembershipFullRow
	for rows.Next() {
		var m OrgMembershipFullRow
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Email, &m.DisplayName,
			&m.Role, &m.MembershipStatus, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan org membership: %w", err)
		}
		memberships = append(memberships, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate org memberships: %w", err)
	}
	if memberships == nil {
		memberships = []OrgMembershipFullRow{}
	}
	return memberships, nil
}

func (r *Repository) CountOrgMemberships(ctx context.Context, orgID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM organization_memberships
		WHERE organization_id = $1 AND membership_status IN ('active', 'invited')
	`, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count org memberships: %w", err)
	}
	return count, nil
}

func (r *Repository) GetOrgMembershipByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID) (OrgMembershipFullRow, error) {
	var m OrgMembershipFullRow
	err := r.db.QueryRow(ctx, `
		SELECT om.id, om.organization_id, om.user_id, u.email, COALESCE(u.display_name, ''),
		       om.role, om.membership_status, om.created_at, om.updated_at
		FROM organization_memberships om
		JOIN users u ON u.id = om.user_id
		WHERE om.organization_id = $1 AND om.user_id = $2
	`, orgID, userID).Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Email, &m.DisplayName,
		&m.Role, &m.MembershipStatus, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrgMembershipFullRow{}, ErrMembershipNotFound
		}
		return OrgMembershipFullRow{}, fmt.Errorf("get org membership by org and user: %w", err)
	}
	return m, nil
}

func (r *Repository) CreateOrgMembership(ctx context.Context, input CreateOrgMembershipInput) (OrgMembershipFullRow, error) {
	var m OrgMembershipFullRow
	err := r.db.QueryRow(ctx, `
		INSERT INTO organization_memberships (id, organization_id, user_id, role, membership_status)
		VALUES (gen_random_uuid(), $1, $2, $3, 'invited')
		RETURNING id, organization_id, user_id, role, membership_status, created_at, updated_at
	`, input.OrganizationID, input.UserID, input.Role).Scan(
		&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.MembershipStatus, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return OrgMembershipFullRow{}, ErrAlreadyMember
		}
		return OrgMembershipFullRow{}, fmt.Errorf("create org membership: %w", err)
	}

	// Fetch user details to fill the response.
	var user User
	if err = r.db.QueryRow(ctx, `SELECT email, COALESCE(display_name, '') FROM users WHERE id = $1`, input.UserID).Scan(&user.Email, &user.DisplayName); err != nil {
		return m, nil // membership created; user details are non-critical
	}
	m.Email = user.Email
	m.DisplayName = user.DisplayName

	return m, nil
}

func (r *Repository) GetOrgMembershipByID(ctx context.Context, membershipID uuid.UUID) (OrgMembershipFullRow, error) {
	var m OrgMembershipFullRow
	err := r.db.QueryRow(ctx, `
		SELECT om.id, om.organization_id, om.user_id, u.email, COALESCE(u.display_name, ''),
		       om.role, om.membership_status, om.created_at, om.updated_at
		FROM organization_memberships om
		JOIN users u ON u.id = om.user_id
		WHERE om.id = $1
	`, membershipID).Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Email, &m.DisplayName,
		&m.Role, &m.MembershipStatus, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrgMembershipFullRow{}, ErrMembershipNotFound
		}
		return OrgMembershipFullRow{}, fmt.Errorf("get org membership by id: %w", err)
	}
	return m, nil
}

func (r *Repository) UpdateOrgMembership(ctx context.Context, membershipID uuid.UUID, input UpdateOrgMembershipInput) (OrgMembershipFullRow, error) {
	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if input.Role != nil {
		setClauses = append(setClauses, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *input.Role)
		argIdx++
	}
	if input.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("membership_status = $%d", argIdx))
		args = append(args, *input.Status)
		argIdx++
		if *input.Status == "archived" {
			setClauses = append(setClauses, fmt.Sprintf("archived_at = $%d", argIdx))
			args = append(args, time.Now())
			argIdx++
		} else {
			setClauses = append(setClauses, "archived_at = NULL")
		}
	}

	if len(setClauses) == 0 {
		return r.GetOrgMembershipByID(ctx, membershipID)
	}

	query := fmt.Sprintf(`
		UPDATE organization_memberships SET %s
		WHERE id = $%d
		RETURNING id, organization_id, user_id, role, membership_status, created_at, updated_at
	`, strings.Join(setClauses, ", "), argIdx)
	args = append(args, membershipID)

	var m OrgMembershipFullRow
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.MembershipStatus, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrgMembershipFullRow{}, ErrMembershipNotFound
		}
		return OrgMembershipFullRow{}, fmt.Errorf("update org membership: %w", err)
	}

	// Fetch user details.
	var user User
	if err = r.db.QueryRow(ctx, `SELECT email, COALESCE(display_name, '') FROM users WHERE id = $1`, m.UserID).Scan(&user.Email, &user.DisplayName); err != nil {
		return m, nil // membership updated; user details are non-critical
	}
	m.Email = user.Email
	m.DisplayName = user.DisplayName

	return m, nil
}

func (r *Repository) CascadeOrgMembershipStatusToWorkspaces(ctx context.Context, orgID, userID uuid.UUID, status string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx, `
		UPDATE workspace_memberships
		SET membership_status = $3, archived_at = CASE WHEN $3 = 'archived' THEN $4 ELSE archived_at END
		WHERE organization_id = $1 AND user_id = $2 AND membership_status NOT IN ('archived')
	`, orgID, userID, status, now)
	if err != nil {
		return fmt.Errorf("cascade org membership status to workspaces: %w", err)
	}
	return nil
}

func (r *Repository) CountActiveOrgAdmins(ctx context.Context, orgID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM organization_memberships
		WHERE organization_id = $1 AND role = 'org_admin' AND membership_status = 'active'
	`, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active org admins: %w", err)
	}
	return count, nil
}

// --- Workspace CRUD ---

func (r *Repository) CreateWorkspaceWithAdmin(ctx context.Context, input CreateWorkspaceWithAdminInput) (WorkspaceRow, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return WorkspaceRow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var ws WorkspaceRow
	err = tx.QueryRow(ctx, `
		INSERT INTO workspaces (id, organization_id, name, slug, status)
		VALUES (gen_random_uuid(), $1, $2, $3, 'active')
		RETURNING id, organization_id, name, slug, status, created_at, updated_at
	`, input.OrganizationID, input.Name, input.Slug).Scan(
		&ws.ID, &ws.OrganizationID, &ws.Name, &ws.Slug, &ws.Status, &ws.CreatedAt, &ws.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "slug") {
			return WorkspaceRow{}, ErrSlugTaken
		}
		return WorkspaceRow{}, fmt.Errorf("insert workspace: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO workspace_memberships (id, organization_id, workspace_id, user_id, role, membership_status)
		VALUES (gen_random_uuid(), $1, $2, $3, 'workspace_admin', 'active')
	`, input.OrganizationID, ws.ID, input.UserID)
	if err != nil {
		return WorkspaceRow{}, fmt.Errorf("insert workspace admin membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return WorkspaceRow{}, fmt.Errorf("commit tx: %w", err)
	}
	return ws, nil
}

func (r *Repository) GetWorkspaceByID(ctx context.Context, workspaceID uuid.UUID) (WorkspaceRow, error) {
	var ws WorkspaceRow
	err := r.db.QueryRow(ctx, `
		SELECT id, organization_id, name, slug, status, created_at, updated_at
		FROM workspaces WHERE id = $1
	`, workspaceID).Scan(&ws.ID, &ws.OrganizationID, &ws.Name, &ws.Slug, &ws.Status, &ws.CreatedAt, &ws.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkspaceRow{}, ErrWorkspaceNotFound
		}
		return WorkspaceRow{}, fmt.Errorf("get workspace by id: %w", err)
	}
	return ws, nil
}

func (r *Repository) ListWorkspacesByOrgID(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]WorkspaceRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, name, slug, status, created_at, updated_at
		FROM workspaces
		WHERE organization_id = $1 AND status = 'active'
		ORDER BY name
		LIMIT $2 OFFSET $3
	`, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list workspaces by org id: %w", err)
	}
	defer rows.Close()

	return scanWorkspaceRows(rows)
}

func (r *Repository) CountWorkspacesByOrgID(ctx context.Context, orgID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM workspaces WHERE organization_id = $1 AND status = 'active'
	`, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count workspaces by org id: %w", err)
	}
	return count, nil
}

func (r *Repository) ListWorkspacesByOrgIDForMember(ctx context.Context, orgID, userID uuid.UUID, limit, offset int32) ([]WorkspaceRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT w.id, w.organization_id, w.name, w.slug, w.status, w.created_at, w.updated_at
		FROM workspaces w
		JOIN workspace_memberships wm ON wm.workspace_id = w.id
		WHERE w.organization_id = $1 AND wm.user_id = $2 AND wm.membership_status = 'active'
		  AND w.status = 'active'
		ORDER BY w.name
		LIMIT $3 OFFSET $4
	`, orgID, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list workspaces by org id for member: %w", err)
	}
	defer rows.Close()

	return scanWorkspaceRows(rows)
}

func (r *Repository) CountWorkspacesByOrgIDForMember(ctx context.Context, orgID, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM workspaces w
		JOIN workspace_memberships wm ON wm.workspace_id = w.id
		WHERE w.organization_id = $1 AND wm.user_id = $2 AND wm.membership_status = 'active'
		  AND w.status = 'active'
	`, orgID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count workspaces for member: %w", err)
	}
	return count, nil
}

func (r *Repository) UpdateWorkspace(ctx context.Context, workspaceID uuid.UUID, input UpdateWorkspaceInput) (WorkspaceRow, error) {
	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if input.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *input.Name)
		argIdx++
	}
	if input.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *input.Status)
		argIdx++
		if *input.Status == "archived" {
			setClauses = append(setClauses, fmt.Sprintf("archived_at = $%d", argIdx))
			args = append(args, time.Now())
			argIdx++
		} else if *input.Status == "active" {
			setClauses = append(setClauses, "archived_at = NULL")
		}
	}

	if len(setClauses) == 0 {
		return r.GetWorkspaceByID(ctx, workspaceID)
	}

	query := fmt.Sprintf(`
		UPDATE workspaces SET %s
		WHERE id = $%d
		RETURNING id, organization_id, name, slug, status, created_at, updated_at
	`, strings.Join(setClauses, ", "), argIdx)
	args = append(args, workspaceID)

	var ws WorkspaceRow
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&ws.ID, &ws.OrganizationID, &ws.Name, &ws.Slug, &ws.Status, &ws.CreatedAt, &ws.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkspaceRow{}, ErrWorkspaceNotFound
		}
		return WorkspaceRow{}, fmt.Errorf("update workspace: %w", err)
	}
	return ws, nil
}

func (r *Repository) ArchiveWorkspaceCascade(ctx context.Context, workspaceID uuid.UUID) (WorkspaceRow, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return WorkspaceRow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()

	_, err = tx.Exec(ctx, `
		UPDATE workspace_memberships
		SET membership_status = 'archived', archived_at = $2
		WHERE workspace_id = $1 AND membership_status != 'archived'
	`, workspaceID, now)
	if err != nil {
		return WorkspaceRow{}, fmt.Errorf("archive workspace memberships: %w", err)
	}

	var ws WorkspaceRow
	err = tx.QueryRow(ctx, `
		UPDATE workspaces
		SET status = 'archived', archived_at = $2
		WHERE id = $1
		RETURNING id, organization_id, name, slug, status, created_at, updated_at
	`, workspaceID, now).Scan(
		&ws.ID, &ws.OrganizationID, &ws.Name, &ws.Slug, &ws.Status, &ws.CreatedAt, &ws.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WorkspaceRow{}, ErrWorkspaceNotFound
		}
		return WorkspaceRow{}, fmt.Errorf("archive workspace: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return WorkspaceRow{}, fmt.Errorf("commit tx: %w", err)
	}
	return ws, nil
}

func scanWorkspaceRows(rows pgx.Rows) ([]WorkspaceRow, error) {
	var workspaces []WorkspaceRow
	for rows.Next() {
		var ws WorkspaceRow
		if err := rows.Scan(&ws.ID, &ws.OrganizationID, &ws.Name, &ws.Slug, &ws.Status, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		workspaces = append(workspaces, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspaces: %w", err)
	}
	if workspaces == nil {
		workspaces = []WorkspaceRow{}
	}
	return workspaces, nil
}

// --- Organization CRUD ---

func (r *Repository) CountActiveOrgAdminMemberships(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM organization_memberships
		WHERE user_id = $1 AND role = 'org_admin' AND membership_status = 'active'
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active org admin memberships: %w", err)
	}
	return count, nil
}

func (r *Repository) CreateOrganizationWithAdmin(ctx context.Context, input CreateOrgWithAdminInput) (OrganizationRow, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return OrganizationRow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var org OrganizationRow
	err = tx.QueryRow(ctx, `
		INSERT INTO organizations (id, name, slug, status)
		VALUES (gen_random_uuid(), $1, $2, 'active')
		RETURNING id, name, slug, status, created_at, updated_at
	`, input.Name, input.Slug).Scan(&org.ID, &org.Name, &org.Slug, &org.Status, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "slug") {
			return OrganizationRow{}, ErrSlugTaken
		}
		return OrganizationRow{}, fmt.Errorf("insert organization: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO organization_memberships (id, organization_id, user_id, role, membership_status)
		VALUES (gen_random_uuid(), $1, $2, 'org_admin', 'active')
	`, org.ID, input.UserID)
	if err != nil {
		return OrganizationRow{}, fmt.Errorf("insert org admin membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return OrganizationRow{}, fmt.Errorf("commit tx: %w", err)
	}
	return org, nil
}

func (r *Repository) GetOrganizationByID(ctx context.Context, orgID uuid.UUID) (OrganizationRow, error) {
	var org OrganizationRow
	err := r.db.QueryRow(ctx, `
		SELECT id, name, slug, status, created_at, updated_at
		FROM organizations WHERE id = $1
	`, orgID).Scan(&org.ID, &org.Name, &org.Slug, &org.Status, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrganizationRow{}, ErrOrganizationNotFound
		}
		return OrganizationRow{}, fmt.Errorf("get organization by id: %w", err)
	}
	return org, nil
}

func (r *Repository) ListOrganizationsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]OrganizationRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT o.id, o.name, o.slug, o.status, o.created_at, o.updated_at
		FROM organizations o
		JOIN organization_memberships om ON om.organization_id = o.id
		WHERE om.user_id = $1 AND om.membership_status = 'active'
		ORDER BY o.name
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list organizations by user id: %w", err)
	}
	defer rows.Close()

	var orgs []OrganizationRow
	for rows.Next() {
		var o OrganizationRow
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan organization: %w", err)
		}
		orgs = append(orgs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate organizations: %w", err)
	}
	if orgs == nil {
		orgs = []OrganizationRow{}
	}
	return orgs, nil
}

func (r *Repository) CountOrganizationsByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM organizations o
		JOIN organization_memberships om ON om.organization_id = o.id
		WHERE om.user_id = $1 AND om.membership_status = 'active'
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count organizations by user id: %w", err)
	}
	return count, nil
}

func (r *Repository) UpdateOrganization(ctx context.Context, orgID uuid.UUID, input UpdateOrgInput) (OrganizationRow, error) {
	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if input.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *input.Name)
		argIdx++
	}
	if input.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *input.Status)
		argIdx++
		if *input.Status == "archived" {
			setClauses = append(setClauses, fmt.Sprintf("archived_at = $%d", argIdx))
			args = append(args, time.Now())
			argIdx++
		} else if *input.Status == "active" {
			setClauses = append(setClauses, "archived_at = NULL")
		}
	}

	if len(setClauses) == 0 {
		return r.GetOrganizationByID(ctx, orgID)
	}

	query := fmt.Sprintf(`
		UPDATE organizations SET %s
		WHERE id = $%d
		RETURNING id, name, slug, status, created_at, updated_at
	`, strings.Join(setClauses, ", "), argIdx)
	args = append(args, orgID)

	var org OrganizationRow
	err := r.db.QueryRow(ctx, query, args...).Scan(&org.ID, &org.Name, &org.Slug, &org.Status, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrganizationRow{}, ErrOrganizationNotFound
		}
		return OrganizationRow{}, fmt.Errorf("update organization: %w", err)
	}
	return org, nil
}

func (r *Repository) ArchiveOrganizationCascade(ctx context.Context, orgID uuid.UUID) (OrganizationRow, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return OrganizationRow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()

	// Archive all workspace memberships in this org.
	_, err = tx.Exec(ctx, `
		UPDATE workspace_memberships
		SET membership_status = 'archived', archived_at = $2
		WHERE organization_id = $1 AND membership_status != 'archived'
	`, orgID, now)
	if err != nil {
		return OrganizationRow{}, fmt.Errorf("archive workspace memberships: %w", err)
	}

	// Archive all workspaces in this org.
	_, err = tx.Exec(ctx, `
		UPDATE workspaces
		SET status = 'archived', archived_at = $2
		WHERE organization_id = $1 AND status != 'archived'
	`, orgID, now)
	if err != nil {
		return OrganizationRow{}, fmt.Errorf("archive workspaces: %w", err)
	}

	// Archive all org memberships.
	_, err = tx.Exec(ctx, `
		UPDATE organization_memberships
		SET membership_status = 'archived', archived_at = $2
		WHERE organization_id = $1 AND membership_status != 'archived'
	`, orgID, now)
	if err != nil {
		return OrganizationRow{}, fmt.Errorf("archive org memberships: %w", err)
	}

	// Archive the org itself.
	var org OrganizationRow
	err = tx.QueryRow(ctx, `
		UPDATE organizations
		SET status = 'archived', archived_at = $2
		WHERE id = $1
		RETURNING id, name, slug, status, created_at, updated_at
	`, orgID, now).Scan(&org.ID, &org.Name, &org.Slug, &org.Status, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrganizationRow{}, ErrOrganizationNotFound
		}
		return OrganizationRow{}, fmt.Errorf("archive organization: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return OrganizationRow{}, fmt.Errorf("commit tx: %w", err)
	}
	return org, nil
}

func (r *Repository) GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error) {
	var orgID uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT organization_id FROM workspaces WHERE id = $1`, workspaceID).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrWorkspaceNotFound
		}
		return uuid.Nil, fmt.Errorf("get organization id by workspace id: %w", err)
	}
	return orgID, nil
}

type AgentBuild struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	Name            string
	Slug            string
	Description     string
	LifecycleStatus string
	CreatedByUserID *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AgentBuildVersion struct {
	ID               uuid.UUID
	AgentBuildID     uuid.UUID
	VersionNumber    int32
	VersionStatus    string
	AgentKind        string
	InterfaceSpec    json.RawMessage
	PolicySpec       json.RawMessage
	ReasoningSpec    json.RawMessage
	MemorySpec       json.RawMessage
	WorkflowSpec     json.RawMessage
	GuardrailSpec    json.RawMessage
	ModelSpec        json.RawMessage
	OutputSchema     json.RawMessage
	TraceContract    json.RawMessage
	PublicationSpec  json.RawMessage
	Tools            []AgentBuildVersionToolBinding
	KnowledgeSources []AgentBuildVersionKnowledgeSourceBinding
	CreatedByUserID  *uuid.UUID
	CreatedAt        time.Time
}

type AgentBuildVersionToolBinding struct {
	ToolID        uuid.UUID
	BindingRole   string
	BindingConfig json.RawMessage
}

type AgentBuildVersionKnowledgeSourceBinding struct {
	KnowledgeSourceID uuid.UUID
	BindingRole       string
	BindingConfig     json.RawMessage
}

type AgentDeploymentRow struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	WorkspaceID           uuid.UUID
	AgentBuildID          uuid.UUID
	CurrentBuildVersionID uuid.UUID
	Name                  string
	Slug                  string
	DeploymentType        string
	Status                string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type CreateAgentBuildParams struct {
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	Name            string
	Slug            string
	Description     string
	CreatedByUserID *uuid.UUID
}

type CreateAgentBuildVersionParams struct {
	AgentBuildID     uuid.UUID
	VersionNumber    int32
	AgentKind        string
	InterfaceSpec    json.RawMessage
	PolicySpec       json.RawMessage
	ReasoningSpec    json.RawMessage
	MemorySpec       json.RawMessage
	WorkflowSpec     json.RawMessage
	GuardrailSpec    json.RawMessage
	ModelSpec        json.RawMessage
	OutputSchema     json.RawMessage
	TraceContract    json.RawMessage
	PublicationSpec  json.RawMessage
	Tools            []AgentBuildVersionToolBinding
	KnowledgeSources []AgentBuildVersionKnowledgeSourceBinding
	CreatedByUserID  *uuid.UUID
}

type UpdateAgentBuildVersionDraftParams struct {
	ID               uuid.UUID
	AgentKind        string
	InterfaceSpec    json.RawMessage
	PolicySpec       json.RawMessage
	ReasoningSpec    json.RawMessage
	MemorySpec       json.RawMessage
	WorkflowSpec     json.RawMessage
	GuardrailSpec    json.RawMessage
	ModelSpec        json.RawMessage
	OutputSchema     json.RawMessage
	TraceContract    json.RawMessage
	PublicationSpec  json.RawMessage
	Tools            []AgentBuildVersionToolBinding
	KnowledgeSources []AgentBuildVersionKnowledgeSourceBinding
}

type CreateAgentDeploymentParams struct {
	OrganizationID        uuid.UUID
	WorkspaceID           uuid.UUID
	AgentBuildID          uuid.UUID
	CurrentBuildVersionID uuid.UUID
	RuntimeProfileID      uuid.UUID
	ProviderAccountID     *uuid.UUID
	ModelAliasID          *uuid.UUID
	Name                  string
	Slug                  string
	DeploymentConfig      json.RawMessage
}

func (r *Repository) CreateAgentBuild(ctx context.Context, params CreateAgentBuildParams) (AgentBuild, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO agent_builds (organization_id, workspace_id, name, slug, description, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, organization_id, workspace_id, name, slug, description, lifecycle_status, created_by_user_id, created_at, updated_at
	`, params.OrganizationID, params.WorkspaceID, params.Name, params.Slug, params.Description, params.CreatedByUserID)

	var build AgentBuild
	var createdAt, updatedAt pgtype.Timestamptz
	err := row.Scan(
		&build.ID, &build.OrganizationID, &build.WorkspaceID,
		&build.Name, &build.Slug, &build.Description, &build.LifecycleStatus,
		&build.CreatedByUserID, &createdAt, &updatedAt,
	)
	if err != nil {
		return AgentBuild{}, fmt.Errorf("create agent build: %w", err)
	}

	build.CreatedAt = createdAt.Time
	build.UpdatedAt = updatedAt.Time
	return build, nil
}

func (r *Repository) GetAgentBuildByID(ctx context.Context, id uuid.UUID) (AgentBuild, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, organization_id, workspace_id, name, slug, description, lifecycle_status, created_by_user_id, created_at, updated_at
		FROM agent_builds WHERE id = $1 AND archived_at IS NULL
	`, id)

	var build AgentBuild
	var createdAt, updatedAt pgtype.Timestamptz
	err := row.Scan(
		&build.ID, &build.OrganizationID, &build.WorkspaceID,
		&build.Name, &build.Slug, &build.Description, &build.LifecycleStatus,
		&build.CreatedByUserID, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentBuild{}, ErrAgentBuildNotFound
		}
		return AgentBuild{}, fmt.Errorf("get agent build by id: %w", err)
	}

	build.CreatedAt = createdAt.Time
	build.UpdatedAt = updatedAt.Time
	return build, nil
}

func (r *Repository) ListAgentBuildsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]AgentBuild, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, workspace_id, name, slug, description, lifecycle_status, created_by_user_id, created_at, updated_at
		FROM agent_builds
		WHERE workspace_id = $1 AND lifecycle_status = 'active' AND archived_at IS NULL
		ORDER BY updated_at DESC
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list agent builds by workspace id: %w", err)
	}
	defer rows.Close()

	var builds []AgentBuild
	for rows.Next() {
		var build AgentBuild
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(
			&build.ID, &build.OrganizationID, &build.WorkspaceID,
			&build.Name, &build.Slug, &build.Description, &build.LifecycleStatus,
			&build.CreatedByUserID, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent build: %w", err)
		}
		build.CreatedAt = createdAt.Time
		build.UpdatedAt = updatedAt.Time
		builds = append(builds, build)
	}

	if builds == nil {
		builds = []AgentBuild{}
	}
	return builds, nil
}

func (r *Repository) CreateAgentBuildVersion(ctx context.Context, params CreateAgentBuildVersionParams) (AgentBuildVersion, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return AgentBuildVersion{}, fmt.Errorf("begin create agent build version tx: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		INSERT INTO agent_build_versions (
			agent_build_id, version_number, version_status,
			agent_kind, interface_spec, policy_spec, reasoning_spec,
			memory_spec, workflow_spec, guardrail_spec, model_spec,
			output_schema, trace_contract, publication_spec, created_by_user_id
		) VALUES (
			$1, $2, 'draft',
			$3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14
		) RETURNING id, agent_build_id, version_number, version_status,
			agent_kind, interface_spec, policy_spec, reasoning_spec,
			memory_spec, workflow_spec, guardrail_spec, model_spec,
			output_schema, trace_contract, publication_spec, created_by_user_id, created_at
	`, params.AgentBuildID, params.VersionNumber,
		params.AgentKind, params.InterfaceSpec, params.PolicySpec, params.ReasoningSpec,
		params.MemorySpec, params.WorkflowSpec, params.GuardrailSpec, params.ModelSpec,
		params.OutputSchema, params.TraceContract, params.PublicationSpec, params.CreatedByUserID,
	)

	version, err := scanAgentBuildVersion(row)
	if err != nil {
		return AgentBuildVersion{}, err
	}

	if err := replaceAgentBuildVersionBindings(ctx, tx, version.ID, params.Tools, params.KnowledgeSources); err != nil {
		return AgentBuildVersion{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AgentBuildVersion{}, fmt.Errorf("commit create agent build version tx: %w", err)
	}

	version.Tools = normalizeToolBindings(params.Tools)
	version.KnowledgeSources = normalizeKnowledgeSourceBindings(params.KnowledgeSources)
	return version, nil
}

func (r *Repository) GetAgentBuildVersionByID(ctx context.Context, id uuid.UUID) (AgentBuildVersion, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, agent_build_id, version_number, version_status,
			agent_kind, interface_spec, policy_spec, reasoning_spec,
			memory_spec, workflow_spec, guardrail_spec, model_spec,
			output_schema, trace_contract, publication_spec, created_by_user_id, created_at
		FROM agent_build_versions WHERE id = $1
	`, id)

	version, err := scanAgentBuildVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentBuildVersion{}, ErrAgentBuildVersionNotFound
		}
		return AgentBuildVersion{}, err
	}
	if err := r.loadAgentBuildVersionBindings(ctx, &version); err != nil {
		return AgentBuildVersion{}, err
	}
	return version, nil
}

func (r *Repository) GetLatestVersionNumberForBuild(ctx context.Context, agentBuildID uuid.UUID) (int32, error) {
	var maxVersion int32
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(version_number), 0)::integer
		FROM agent_build_versions WHERE agent_build_id = $1
	`, agentBuildID).Scan(&maxVersion)
	if err != nil {
		return 0, fmt.Errorf("get latest version number for build: %w", err)
	}
	return maxVersion, nil
}

func (r *Repository) ListAgentBuildVersionsByBuildID(ctx context.Context, agentBuildID uuid.UUID) ([]AgentBuildVersion, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, agent_build_id, version_number, version_status,
			agent_kind, interface_spec, policy_spec, reasoning_spec,
			memory_spec, workflow_spec, guardrail_spec, model_spec,
			output_schema, trace_contract, publication_spec, created_by_user_id, created_at
		FROM agent_build_versions
		WHERE agent_build_id = $1
		ORDER BY version_number DESC
	`, agentBuildID)
	if err != nil {
		return nil, fmt.Errorf("list agent build versions by build id: %w", err)
	}
	defer rows.Close()

	var versions []AgentBuildVersion
	for rows.Next() {
		var v AgentBuildVersion
		var createdAt pgtype.Timestamptz
		if err := rows.Scan(
			&v.ID, &v.AgentBuildID, &v.VersionNumber, &v.VersionStatus,
			&v.AgentKind, &v.InterfaceSpec, &v.PolicySpec, &v.ReasoningSpec,
			&v.MemorySpec, &v.WorkflowSpec, &v.GuardrailSpec, &v.ModelSpec,
			&v.OutputSchema, &v.TraceContract, &v.PublicationSpec, &v.CreatedByUserID, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent build version: %w", err)
		}
		v.CreatedAt = createdAt.Time
		if err := r.loadAgentBuildVersionBindings(ctx, &v); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}

	if versions == nil {
		versions = []AgentBuildVersion{}
	}
	return versions, nil
}

func (r *Repository) UpdateAgentBuildVersionDraft(ctx context.Context, params UpdateAgentBuildVersionDraftParams) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update agent build version tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE agent_build_versions SET
			agent_kind = $2,
			interface_spec = $3,
			policy_spec = $4,
			reasoning_spec = $5,
			memory_spec = $6,
			workflow_spec = $7,
			guardrail_spec = $8,
			model_spec = $9,
			output_schema = $10,
			trace_contract = $11,
			publication_spec = $12
		WHERE id = $1 AND version_status = 'draft'
	`, params.ID,
		params.AgentKind, params.InterfaceSpec, params.PolicySpec, params.ReasoningSpec,
		params.MemorySpec, params.WorkflowSpec, params.GuardrailSpec, params.ModelSpec,
		params.OutputSchema, params.TraceContract, params.PublicationSpec,
	)
	if err != nil {
		return fmt.Errorf("update agent build version draft: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAgentBuildVersionNotFound
	}
	if err := replaceAgentBuildVersionBindings(ctx, tx, params.ID, params.Tools, params.KnowledgeSources); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit update agent build version tx: %w", err)
	}
	return nil
}

func (r *Repository) MarkAgentBuildVersionReady(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE agent_build_versions SET version_status = 'ready'
		WHERE id = $1 AND version_status = 'draft'
	`, id)
	if err != nil {
		return fmt.Errorf("mark agent build version ready: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAgentBuildVersionNotFound
	}
	return nil
}

func (r *Repository) CreateAgentDeployment(ctx context.Context, params CreateAgentDeploymentParams) (AgentDeploymentRow, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO agent_deployments (
			organization_id, workspace_id, agent_build_id, current_build_version_id,
			runtime_profile_id, provider_account_id, model_alias_id,
			name, slug, deployment_type, deployment_config
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, 'native', $10
		) RETURNING id, organization_id, workspace_id, agent_build_id, current_build_version_id,
			name, slug, deployment_type, status, created_at, updated_at
	`, params.OrganizationID, params.WorkspaceID, params.AgentBuildID, params.CurrentBuildVersionID,
		params.RuntimeProfileID, params.ProviderAccountID, params.ModelAliasID,
		params.Name, params.Slug, params.DeploymentConfig,
	)

	var dep AgentDeploymentRow
	var createdAt, updatedAt pgtype.Timestamptz
	err := row.Scan(
		&dep.ID, &dep.OrganizationID, &dep.WorkspaceID, &dep.AgentBuildID, &dep.CurrentBuildVersionID,
		&dep.Name, &dep.Slug, &dep.DeploymentType, &dep.Status, &createdAt, &updatedAt,
	)
	if err != nil {
		return AgentDeploymentRow{}, fmt.Errorf("create agent deployment: %w", err)
	}

	dep.CreatedAt = createdAt.Time
	dep.UpdatedAt = updatedAt.Time
	return dep, nil
}

func scanAgentBuildVersion(row pgx.Row) (AgentBuildVersion, error) {
	var v AgentBuildVersion
	var createdAt pgtype.Timestamptz
	err := row.Scan(
		&v.ID, &v.AgentBuildID, &v.VersionNumber, &v.VersionStatus,
		&v.AgentKind, &v.InterfaceSpec, &v.PolicySpec, &v.ReasoningSpec,
		&v.MemorySpec, &v.WorkflowSpec, &v.GuardrailSpec, &v.ModelSpec,
		&v.OutputSchema, &v.TraceContract, &v.PublicationSpec, &v.CreatedByUserID, &createdAt,
	)
	if err != nil {
		return AgentBuildVersion{}, fmt.Errorf("scan agent build version: %w", err)
	}
	v.CreatedAt = createdAt.Time
	return v, nil
}

type agentBuildVersionBindingQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func replaceAgentBuildVersionBindings(
	ctx context.Context,
	q agentBuildVersionBindingQuerier,
	versionID uuid.UUID,
	tools []AgentBuildVersionToolBinding,
	knowledgeSources []AgentBuildVersionKnowledgeSourceBinding,
) error {
	if _, err := q.Exec(ctx, `DELETE FROM agent_build_version_tools WHERE agent_build_version_id = $1`, versionID); err != nil {
		return fmt.Errorf("delete agent build version tools: %w", err)
	}
	for _, binding := range normalizeToolBindings(tools) {
		if _, err := q.Exec(ctx, `
			INSERT INTO agent_build_version_tools (agent_build_version_id, tool_id, binding_role, binding_config)
			VALUES ($1, $2, $3, $4)
		`, versionID, binding.ToolID, binding.BindingRole, binding.BindingConfig); err != nil {
			return fmt.Errorf("insert agent build version tool binding: %w", err)
		}
	}

	if _, err := q.Exec(ctx, `DELETE FROM agent_build_version_knowledge_sources WHERE agent_build_version_id = $1`, versionID); err != nil {
		return fmt.Errorf("delete agent build version knowledge sources: %w", err)
	}
	for _, binding := range normalizeKnowledgeSourceBindings(knowledgeSources) {
		if _, err := q.Exec(ctx, `
			INSERT INTO agent_build_version_knowledge_sources (agent_build_version_id, knowledge_source_id, binding_role, binding_config)
			VALUES ($1, $2, $3, $4)
		`, versionID, binding.KnowledgeSourceID, binding.BindingRole, binding.BindingConfig); err != nil {
			return fmt.Errorf("insert agent build version knowledge source binding: %w", err)
		}
	}

	return nil
}

func (r *Repository) loadAgentBuildVersionBindings(ctx context.Context, version *AgentBuildVersion) error {
	toolRows, err := r.db.Query(ctx, `
		SELECT tool_id, binding_role, binding_config
		FROM agent_build_version_tools
		WHERE agent_build_version_id = $1
		ORDER BY tool_id
	`, version.ID)
	if err != nil {
		return fmt.Errorf("list agent build version tools: %w", err)
	}
	defer toolRows.Close()

	var tools []AgentBuildVersionToolBinding
	for toolRows.Next() {
		var binding AgentBuildVersionToolBinding
		if err := toolRows.Scan(&binding.ToolID, &binding.BindingRole, &binding.BindingConfig); err != nil {
			return fmt.Errorf("scan agent build version tool binding: %w", err)
		}
		tools = append(tools, binding)
	}
	if err := toolRows.Err(); err != nil {
		return fmt.Errorf("iterate agent build version tools: %w", err)
	}

	knowledgeRows, err := r.db.Query(ctx, `
		SELECT knowledge_source_id, binding_role, binding_config
		FROM agent_build_version_knowledge_sources
		WHERE agent_build_version_id = $1
		ORDER BY knowledge_source_id
	`, version.ID)
	if err != nil {
		return fmt.Errorf("list agent build version knowledge sources: %w", err)
	}
	defer knowledgeRows.Close()

	var knowledgeSources []AgentBuildVersionKnowledgeSourceBinding
	for knowledgeRows.Next() {
		var binding AgentBuildVersionKnowledgeSourceBinding
		if err := knowledgeRows.Scan(&binding.KnowledgeSourceID, &binding.BindingRole, &binding.BindingConfig); err != nil {
			return fmt.Errorf("scan agent build version knowledge source binding: %w", err)
		}
		knowledgeSources = append(knowledgeSources, binding)
	}
	if err := knowledgeRows.Err(); err != nil {
		return fmt.Errorf("iterate agent build version knowledge sources: %w", err)
	}

	version.Tools = normalizeToolBindings(tools)
	version.KnowledgeSources = normalizeKnowledgeSourceBindings(knowledgeSources)
	return nil
}

func normalizeToolBindings(bindings []AgentBuildVersionToolBinding) []AgentBuildVersionToolBinding {
	if bindings == nil {
		return []AgentBuildVersionToolBinding{}
	}
	out := make([]AgentBuildVersionToolBinding, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, AgentBuildVersionToolBinding{
			ToolID:        binding.ToolID,
			BindingRole:   defaultStringOrFallback(binding.BindingRole, "default"),
			BindingConfig: defaultJSONObject(binding.BindingConfig),
		})
	}
	return out
}

func normalizeKnowledgeSourceBindings(bindings []AgentBuildVersionKnowledgeSourceBinding) []AgentBuildVersionKnowledgeSourceBinding {
	if bindings == nil {
		return []AgentBuildVersionKnowledgeSourceBinding{}
	}
	out := make([]AgentBuildVersionKnowledgeSourceBinding, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, AgentBuildVersionKnowledgeSourceBinding{
			KnowledgeSourceID: binding.KnowledgeSourceID,
			BindingRole:       defaultStringOrFallback(binding.BindingRole, "default"),
			BindingConfig:     defaultJSONObject(binding.BindingConfig),
		})
	}
	return out
}

func defaultStringOrFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultJSONObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func temporalIDsMatch(row repositorysqlc.Run, params SetRunTemporalIDsParams) bool {
	if row.TemporalWorkflowID == nil || row.TemporalRunID == nil {
		return false
	}
	return *row.TemporalWorkflowID == params.TemporalWorkflowID &&
		*row.TemporalRunID == params.TemporalRunID
}
