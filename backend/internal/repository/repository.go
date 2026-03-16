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
	row, err := r.queries.GetRunnableChallengePackVersionByID(ctx, repositorysqlc.GetRunnableChallengePackVersionByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RunnableChallengePackVersion{}, ErrChallengePackVersionNotFound
		}
		return RunnableChallengePackVersion{}, fmt.Errorf("get runnable challenge pack version by id: %w", err)
	}

	return RunnableChallengePackVersion{
		ID:              row.ID,
		ChallengePackID: row.ChallengePackID,
	}, nil
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

func temporalIDsMatch(row repositorysqlc.Run, params SetRunTemporalIDsParams) bool {
	if row.TemporalWorkflowID == nil || row.TemporalRunID == nil {
		return false
	}
	return *row.TemporalWorkflowID == params.TemporalWorkflowID &&
		*row.TemporalRunID == params.TemporalRunID
}
