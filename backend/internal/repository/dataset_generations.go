package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	datasetgeneration "github.com/agentclash/agentclash/backend/internal/datasets/generation"
	"github.com/agentclash/agentclash/backend/internal/domain"
	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrDatasetGenerationJobNotFound = errors.New("dataset generation job not found")
)

type DatasetGenerationStatus string

const (
	DatasetGenerationStatusQueued    DatasetGenerationStatus = "queued"
	DatasetGenerationStatusRunning   DatasetGenerationStatus = "running"
	DatasetGenerationStatusCompleted DatasetGenerationStatus = "completed"
	DatasetGenerationStatusFailed    DatasetGenerationStatus = "failed"
)

type DatasetGenerationJob struct {
	ID                 uuid.UUID               `json:"id"`
	DatasetID          uuid.UUID               `json:"dataset_id"`
	WorkspaceID        uuid.UUID               `json:"workspace_id"`
	Strategy           string                  `json:"strategy"`
	Status             DatasetGenerationStatus `json:"status"`
	Config             json.RawMessage         `json:"config"`
	Summary            json.RawMessage         `json:"summary"`
	TargetCount        int32                   `json:"target_count"`
	GeneratedCount     int32                   `json:"generated_count"`
	AcceptedCount      int32                   `json:"accepted_count"`
	RejectedCount      int32                   `json:"rejected_count"`
	TotalInputTokens   int64                   `json:"total_input_tokens"`
	TotalOutputTokens  int64                   `json:"total_output_tokens"`
	TotalCostUSD       float64                 `json:"total_cost_usd"`
	VersionID          *uuid.UUID              `json:"version_id,omitempty"`
	TemporalWorkflowID *string                 `json:"temporal_workflow_id,omitempty"`
	TemporalRunID      *string                 `json:"temporal_run_id,omitempty"`
	ErrorMessage       *string                 `json:"error_message,omitempty"`
	CreatedBy          uuid.UUID               `json:"created_by"`
	QueuedAt           time.Time               `json:"queued_at"`
	StartedAt          *time.Time              `json:"started_at,omitempty"`
	FinishedAt         *time.Time              `json:"finished_at,omitempty"`
	FailedAt           *time.Time              `json:"failed_at,omitempty"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
}

type DatasetGenerationRejection struct {
	ID                uuid.UUID       `json:"id"`
	JobID             uuid.UUID       `json:"job_id"`
	ReasonCode        string          `json:"reason_code"`
	ReasonDetail      *string         `json:"reason_detail,omitempty"`
	CandidateInput    json.RawMessage `json:"candidate_input,omitempty"`
	CandidateExpected json.RawMessage `json:"candidate_expected,omitempty"`
	Metadata          json.RawMessage `json:"metadata"`
	CreatedAt         time.Time       `json:"created_at"`
}

type CreateDatasetGenerationJobParams struct {
	DatasetID   uuid.UUID
	WorkspaceID uuid.UUID
	Strategy    string
	Config      json.RawMessage
	TargetCount int32
	Actor       uuid.UUID
	QueuedAt    time.Time
}

type SetDatasetGenerationJobTemporalIDsParams struct {
	ID                 uuid.UUID
	TemporalWorkflowID string
	TemporalRunID      string
}

type UpdateDatasetGenerationJobStatusParams struct {
	ID           uuid.UUID
	Status       DatasetGenerationStatus
	Summary      json.RawMessage
	VersionID    *uuid.UUID
	ErrorMessage *string
	StartedAt    *time.Time
	FinishedAt   *time.Time
	FailedAt     *time.Time
}

type UpdateDatasetGenerationJobProgressParams struct {
	ID                uuid.UUID
	GeneratedCount    int32
	AcceptedCount     int32
	RejectedCount     int32
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalCostUSD      float64
	Summary           json.RawMessage
}

type CreateDatasetGenerationRejectionParams struct {
	JobID             uuid.UUID
	ReasonCode        string
	ReasonDetail      *string
	CandidateInput    json.RawMessage
	CandidateExpected json.RawMessage
	Metadata          json.RawMessage
}

type DatasetGenerationExecutionContext struct {
	Job             DatasetGenerationJob
	Dataset         Dataset
	Config          datasetgeneration.JobConfig
	Seeds           []datasetgeneration.SeedExample
	ExistingInputs  map[string]struct{}
	ProviderAccount ProviderAccountRow
	ModelAlias      ModelAliasRow
	ModelCatalog    ModelCatalogEntryRow
}

func (r *Repository) CreateDatasetGenerationJob(ctx context.Context, params CreateDatasetGenerationJobParams) (DatasetGenerationJob, error) {
	row, err := r.queries.CreateDatasetGenerationJob(ctx, repositorysqlc.CreateDatasetGenerationJobParams{
		DatasetID:   params.DatasetID,
		WorkspaceID: params.WorkspaceID,
		Strategy:    params.Strategy,
		Status:      string(DatasetGenerationStatusQueued),
		Config:      datasetDefaultJSONObject(params.Config),
		Summary:     datasetDefaultJSONObject(nil),
		TargetCount: params.TargetCount,
		CreatedBy:   params.Actor,
		QueuedAt:    pgtype.Timestamptz{Time: params.QueuedAt.UTC(), Valid: true},
	})
	if err != nil {
		return DatasetGenerationJob{}, fmt.Errorf("create dataset generation job: %w", err)
	}
	return mapDatasetGenerationJob(row)
}

func (r *Repository) GetDatasetGenerationJobByID(ctx context.Context, id uuid.UUID) (DatasetGenerationJob, error) {
	row, err := r.queries.GetDatasetGenerationJobByID(ctx, repositorysqlc.GetDatasetGenerationJobByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetGenerationJob{}, ErrDatasetGenerationJobNotFound
		}
		return DatasetGenerationJob{}, fmt.Errorf("get dataset generation job: %w", err)
	}
	return mapDatasetGenerationJob(row)
}

func (r *Repository) SetDatasetGenerationJobTemporalIDs(ctx context.Context, params SetDatasetGenerationJobTemporalIDsParams) (DatasetGenerationJob, error) {
	row, err := r.queries.SetDatasetGenerationJobTemporalIDs(ctx, repositorysqlc.SetDatasetGenerationJobTemporalIDsParams{
		ID:                 params.ID,
		TemporalWorkflowID: &params.TemporalWorkflowID,
		TemporalRunID:      &params.TemporalRunID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetGenerationJob{}, ErrDatasetGenerationJobNotFound
		}
		return DatasetGenerationJob{}, fmt.Errorf("set dataset generation job temporal ids: %w", err)
	}
	return mapDatasetGenerationJob(row)
}

func (r *Repository) UpdateDatasetGenerationJobStatus(ctx context.Context, params UpdateDatasetGenerationJobStatusParams) (DatasetGenerationJob, error) {
	row, err := r.queries.UpdateDatasetGenerationJobStatus(ctx, repositorysqlc.UpdateDatasetGenerationJobStatusParams{
		ID:           params.ID,
		Status:       string(params.Status),
		Summary:      nullableJSON(params.Summary),
		VersionID:    params.VersionID,
		ErrorMessage: params.ErrorMessage,
		StartedAt:    timePtrToPg(params.StartedAt),
		FinishedAt:   timePtrToPg(params.FinishedAt),
		FailedAt:     timePtrToPg(params.FailedAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetGenerationJob{}, ErrDatasetGenerationJobNotFound
		}
		return DatasetGenerationJob{}, fmt.Errorf("update dataset generation job status: %w", err)
	}
	return mapDatasetGenerationJob(row)
}

func (r *Repository) UpdateDatasetGenerationJobProgress(ctx context.Context, params UpdateDatasetGenerationJobProgressParams) (DatasetGenerationJob, error) {
	row, err := r.queries.UpdateDatasetGenerationJobProgress(ctx, repositorysqlc.UpdateDatasetGenerationJobProgressParams{
		ID:                params.ID,
		GeneratedCount:    params.GeneratedCount,
		AcceptedCount:     params.AcceptedCount,
		RejectedCount:     params.RejectedCount,
		TotalInputTokens:  params.TotalInputTokens,
		TotalOutputTokens: params.TotalOutputTokens,
		TotalCostUsd:      pgtypeNumericFromFloat(params.TotalCostUSD),
		Summary:           nullableJSON(params.Summary),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetGenerationJob{}, ErrDatasetGenerationJobNotFound
		}
		return DatasetGenerationJob{}, fmt.Errorf("update dataset generation job progress: %w", err)
	}
	return mapDatasetGenerationJob(row)
}

func (r *Repository) CreateDatasetGenerationRejection(ctx context.Context, params CreateDatasetGenerationRejectionParams) (DatasetGenerationRejection, error) {
	row, err := r.queries.CreateDatasetGenerationRejection(ctx, repositorysqlc.CreateDatasetGenerationRejectionParams{
		JobID:             params.JobID,
		ReasonCode:        params.ReasonCode,
		ReasonDetail:      params.ReasonDetail,
		CandidateInput:    nullableJSON(params.CandidateInput),
		CandidateExpected: nullableJSON(params.CandidateExpected),
		Metadata:          datasetDefaultJSONObject(params.Metadata),
	})
	if err != nil {
		return DatasetGenerationRejection{}, fmt.Errorf("create dataset generation rejection: %w", err)
	}
	return mapDatasetGenerationRejection(row)
}

func (r *Repository) GetDatasetGenerationExecutionContextByID(ctx context.Context, jobID uuid.UUID) (DatasetGenerationExecutionContext, error) {
	job, err := r.GetDatasetGenerationJobByID(ctx, jobID)
	if err != nil {
		return DatasetGenerationExecutionContext{}, err
	}
	dataset, err := r.GetDatasetByID(ctx, job.DatasetID)
	if err != nil {
		return DatasetGenerationExecutionContext{}, err
	}
	cfg, err := datasetgeneration.DecodeJobConfig(job.Config)
	if err != nil {
		return DatasetGenerationExecutionContext{}, err
	}
	providerAccount, err := r.GetProviderAccountByID(ctx, cfg.ProviderAccountID)
	if err != nil {
		return DatasetGenerationExecutionContext{}, err
	}
	modelAlias, err := r.GetModelAliasByID(ctx, cfg.ModelAliasID)
	if err != nil {
		return DatasetGenerationExecutionContext{}, err
	}
	modelCatalog, err := r.GetModelCatalogEntryByID(ctx, modelAlias.ModelCatalogEntryID)
	if err != nil {
		return DatasetGenerationExecutionContext{}, err
	}

	active := domain.DatasetExampleStatusActive
	examples, err := r.ListDatasetExamplesByDatasetID(ctx, ListDatasetExamplesParams{
		DatasetID: job.DatasetID,
		Status:    &active,
		Limit:     10_000,
		Offset:    0,
	})
	if err != nil {
		return DatasetGenerationExecutionContext{}, err
	}

	seeds := make([]datasetgeneration.SeedExample, 0)
	existing := make(map[string]struct{})
	for _, example := range examples {
		hash, hashErr := datasetgeneration.CanonicalInputHash(example.Input)
		if hashErr == nil {
			existing[hash] = struct{}{}
		}
		if cfg.SeedsTag != "" && !containsTag(example.Tags, cfg.SeedsTag) {
			continue
		}
		seeds = append(seeds, datasetgeneration.SeedExample{Input: example.Input, Expected: example.Expected})
	}

	return DatasetGenerationExecutionContext{
		Job:             job,
		Dataset:         dataset,
		Config:          cfg,
		Seeds:           seeds,
		ExistingInputs:  existing,
		ProviderAccount: providerAccount,
		ModelAlias:      modelAlias,
		ModelCatalog:    modelCatalog,
	}, nil
}

func mapDatasetGenerationJob(row repositorysqlc.DatasetGenerationJob) (DatasetGenerationJob, error) {
	return DatasetGenerationJob{
		ID:                 row.ID,
		DatasetID:          row.DatasetID,
		WorkspaceID:        row.WorkspaceID,
		Strategy:           row.Strategy,
		Status:             DatasetGenerationStatus(row.Status),
		Config:             cloneDatasetGenerationJSON(row.Config),
		Summary:            cloneDatasetGenerationJSON(row.Summary),
		TargetCount:        row.TargetCount,
		GeneratedCount:     row.GeneratedCount,
		AcceptedCount:      row.AcceptedCount,
		RejectedCount:      row.RejectedCount,
		TotalInputTokens:   row.TotalInputTokens,
		TotalOutputTokens:  row.TotalOutputTokens,
		TotalCostUSD:       numericToFloat64(row.TotalCostUsd),
		VersionID:          row.VersionID,
		TemporalWorkflowID: row.TemporalWorkflowID,
		TemporalRunID:      row.TemporalRunID,
		ErrorMessage:       row.ErrorMessage,
		CreatedBy:          row.CreatedBy,
		QueuedAt:           row.QueuedAt.Time,
		StartedAt:          pgTimePtr(row.StartedAt),
		FinishedAt:         pgTimePtr(row.FinishedAt),
		FailedAt:           pgTimePtr(row.FailedAt),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}, nil
}

func mapDatasetGenerationRejection(row repositorysqlc.DatasetGenerationRejection) (DatasetGenerationRejection, error) {
	return DatasetGenerationRejection{
		ID:                row.ID,
		JobID:             row.JobID,
		ReasonCode:        row.ReasonCode,
		ReasonDetail:      row.ReasonDetail,
		CandidateInput:    cloneDatasetGenerationJSON(row.CandidateInput),
		CandidateExpected: cloneDatasetGenerationJSON(row.CandidateExpected),
		Metadata:          cloneDatasetGenerationJSON(row.Metadata),
		CreatedAt:         row.CreatedAt.Time,
	}, nil
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

func timePtrToPg(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func pgTimePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

func cloneDatasetGenerationJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	cloned := make([]byte, len(raw))
	copy(cloned, raw)
	return json.RawMessage(cloned)
}
