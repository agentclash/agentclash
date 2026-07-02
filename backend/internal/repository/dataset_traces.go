package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	datasettraces "github.com/agentclash/agentclash/runtime/datasets/traces"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrDatasetTraceCandidateNotFound = errors.New("dataset trace candidate not found")
	ErrDatasetTraceCandidatePromoted = errors.New("dataset trace candidate already promoted")
)

type DatasetTraceImport struct {
	ID             uuid.UUID  `json:"id"`
	DatasetID      uuid.UUID  `json:"dataset_id"`
	SourcePlatform string     `json:"source_platform"`
	ArtifactID     *uuid.UUID `json:"artifact_id,omitempty"`
	CandidateCount int32      `json:"candidate_count"`
	Status         string     `json:"status"`
	CreatedBy      uuid.UUID  `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
}

type DatasetTraceCandidate struct {
	ID                uuid.UUID       `json:"id"`
	DatasetID         uuid.UUID       `json:"dataset_id"`
	ImportID          uuid.UUID       `json:"import_id"`
	SourcePlatform    string          `json:"source_platform"`
	SourceTraceID     *string         `json:"source_trace_id,omitempty"`
	SourceRunID       *uuid.UUID      `json:"source_run_id,omitempty"`
	SourceRunAgentID  *uuid.UUID      `json:"source_run_agent_id,omitempty"`
	ExternalID        *string         `json:"external_id,omitempty"`
	Input             json.RawMessage `json:"input"`
	Output            json.RawMessage `json:"output,omitempty"`
	Expected          json.RawMessage `json:"expected,omitempty"`
	Metadata          json.RawMessage `json:"metadata"`
	Tags              []string        `json:"tags"`
	Status            string          `json:"status"`
	PromotedExampleID *uuid.UUID      `json:"promoted_example_id,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type ImportDatasetTracesParams struct {
	DatasetID      uuid.UUID
	SourcePlatform string
	ArtifactID     *uuid.UUID
	Candidates     []datasettraces.Candidate
	Actor          uuid.UUID
}

type ImportDatasetTracesResult struct {
	Import     DatasetTraceImport      `json:"import"`
	Candidates []DatasetTraceCandidate `json:"candidates"`
}

type ListDatasetTraceCandidatesParams struct {
	DatasetID uuid.UUID
	Status    *string
	Limit     int32
	Offset    int32
}

type ListDatasetTraceCandidatesResult struct {
	Candidates []DatasetTraceCandidate `json:"candidates"`
	Total      int64                   `json:"total"`
}

type PromoteDatasetTraceCandidateParams struct {
	CandidateID uuid.UUID
	DatasetID   uuid.UUID
	Expected    json.RawMessage
	Tags        []string
	Actor       uuid.UUID
}

type PromoteDatasetTraceCandidateResult struct {
	Candidate DatasetTraceCandidate `json:"candidate"`
	Example   DatasetExample        `json:"example"`
	Version   DatasetVersion        `json:"version"`
	Created   bool                  `json:"created"`
}

func (r *Repository) BuildDatasetTraceCandidatesFromRunAgent(ctx context.Context, runAgentID uuid.UUID) ([]datasettraces.Candidate, error) {
	runAgent, err := r.GetRunAgentByID(ctx, runAgentID)
	if err != nil {
		return nil, err
	}
	events, err := r.ListRunEventsByRunAgentID(ctx, runAgentID)
	if err != nil {
		return nil, err
	}
	envelopes := make([]runevents.Envelope, 0, len(events))
	for _, event := range events {
		envelopes = append(envelopes, runevents.Envelope{
			EventID:        fmt.Sprintf("persisted:%s:%d", event.RunAgentID.String(), event.SequenceNumber),
			SchemaVersion:  runevents.SchemaVersionV1,
			RunID:          event.RunID,
			RunAgentID:     event.RunAgentID,
			SequenceNumber: event.SequenceNumber,
			EventType:      event.EventType,
			Source:         event.Source,
			OccurredAt:     event.OccurredAt,
			Payload:        cloneJSON(event.Payload),
		})
	}
	return datasettraces.CandidatesFromRunEvents(runAgent.RunID, runAgent.ID, envelopes)
}

func (r *Repository) ImportDatasetTraces(ctx context.Context, params ImportDatasetTracesParams) (ImportDatasetTracesResult, error) {
	if len(params.Candidates) == 0 {
		return ImportDatasetTracesResult{}, fmt.Errorf("at least one trace candidate is required")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ImportDatasetTracesResult{}, fmt.Errorf("begin dataset trace import transaction: %w", err)
	}
	defer rollback(ctx, tx)
	queries := r.queries.WithTx(tx)

	importRow, err := queries.CreateDatasetTraceImport(ctx, repositorysqlc.CreateDatasetTraceImportParams{
		DatasetID:      params.DatasetID,
		SourcePlatform: params.SourcePlatform,
		ArtifactID:     params.ArtifactID,
		CandidateCount: int32(len(params.Candidates)),
		Status:         "completed",
		CreatedBy:      params.Actor,
	})
	if err != nil {
		return ImportDatasetTracesResult{}, fmt.Errorf("create dataset trace import: %w", err)
	}

	candidates := make([]DatasetTraceCandidate, 0, len(params.Candidates))
	for _, candidate := range params.Candidates {
		row, insertErr := queries.CreateDatasetTraceCandidate(ctx, repositorysqlc.CreateDatasetTraceCandidateParams{
			DatasetID:        params.DatasetID,
			ImportID:         importRow.ID,
			SourcePlatform:   params.SourcePlatform,
			SourceTraceID:    stringPtr(candidate.SourceTraceID),
			SourceRunID:      candidate.SourceRunID,
			SourceRunAgentID: candidate.SourceRunAgentID,
			ExternalID:       candidate.ExternalID,
			Input:            candidate.Input,
			Output:           cloneJSON(candidate.Output),
			Expected:         cloneJSON(candidate.Expected),
			Metadata:         defaultJSON(candidate.Metadata),
			Tags:             candidate.Tags,
			Status:           "pending",
		})
		if insertErr != nil {
			return ImportDatasetTracesResult{}, fmt.Errorf("create dataset trace candidate: %w", insertErr)
		}
		candidate, mapErr := mapDatasetTraceCandidate(row)
		if mapErr != nil {
			return ImportDatasetTracesResult{}, mapErr
		}
		candidates = append(candidates, candidate)
	}

	if err := tx.Commit(ctx); err != nil {
		return ImportDatasetTracesResult{}, fmt.Errorf("commit dataset trace import transaction: %w", err)
	}
	importBatch, mapErr := mapDatasetTraceImport(importRow)
	if mapErr != nil {
		return ImportDatasetTracesResult{}, mapErr
	}
	return ImportDatasetTracesResult{
		Import:     importBatch,
		Candidates: candidates,
	}, nil
}

func (r *Repository) ListDatasetTraceCandidates(ctx context.Context, params ListDatasetTraceCandidatesParams) (ListDatasetTraceCandidatesResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	var status *string
	if params.Status != nil && strings.TrimSpace(*params.Status) != "" {
		trimmed := strings.TrimSpace(*params.Status)
		status = &trimmed
	}
	total, err := r.queries.CountDatasetTraceCandidates(ctx, repositorysqlc.CountDatasetTraceCandidatesParams{
		DatasetID: params.DatasetID,
		Status:    status,
	})
	if err != nil {
		return ListDatasetTraceCandidatesResult{}, fmt.Errorf("count dataset trace candidates: %w", err)
	}
	rows, err := r.queries.ListDatasetTraceCandidates(ctx, repositorysqlc.ListDatasetTraceCandidatesParams{
		DatasetID:   params.DatasetID,
		Status:      status,
		LimitCount:  limit,
		OffsetCount: params.Offset,
	})
	if err != nil {
		return ListDatasetTraceCandidatesResult{}, fmt.Errorf("list dataset trace candidates: %w", err)
	}
	candidates := make([]DatasetTraceCandidate, 0, len(rows))
	for _, row := range rows {
		candidate, mapErr := mapDatasetTraceCandidate(row)
		if mapErr != nil {
			return ListDatasetTraceCandidatesResult{}, mapErr
		}
		candidates = append(candidates, candidate)
	}
	return ListDatasetTraceCandidatesResult{Candidates: candidates, Total: total}, nil
}

func (r *Repository) PromoteDatasetTraceCandidate(ctx context.Context, params PromoteDatasetTraceCandidateParams) (PromoteDatasetTraceCandidateResult, error) {
	candidateRow, err := r.queries.GetDatasetTraceCandidateByID(ctx, repositorysqlc.GetDatasetTraceCandidateByIDParams{ID: params.CandidateID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PromoteDatasetTraceCandidateResult{}, ErrDatasetTraceCandidateNotFound
		}
		return PromoteDatasetTraceCandidateResult{}, fmt.Errorf("get dataset trace candidate: %w", err)
	}
	candidate, err := mapDatasetTraceCandidate(candidateRow)
	if err != nil {
		return PromoteDatasetTraceCandidateResult{}, err
	}
	if candidate.DatasetID != params.DatasetID {
		return PromoteDatasetTraceCandidateResult{}, ErrDatasetTraceCandidateNotFound
	}
	dataset, err := r.GetDatasetByID(ctx, params.DatasetID)
	if err != nil {
		return PromoteDatasetTraceCandidateResult{}, err
	}
	if dataset.InputSchemaEnforced {
		if err := domain.ValidateDatasetInputAgainstSchema(dataset.InputSchema, candidate.Input); err != nil {
			return PromoteDatasetTraceCandidateResult{}, err
		}
	}
	if candidate.Status == "promoted" && candidate.PromotedExampleID != nil {
		example, getErr := r.GetDatasetExampleByID(ctx, *candidate.PromotedExampleID)
		if getErr != nil {
			return PromoteDatasetTraceCandidateResult{}, getErr
		}
		return PromoteDatasetTraceCandidateResult{
			Candidate: candidate,
			Example:   example,
			Created:   false,
		}, nil
	}
	if candidate.Status != "pending" {
		return PromoteDatasetTraceCandidateResult{}, ErrDatasetTraceCandidatePromoted
	}

	expected := cloneJSON(params.Expected)
	if len(expected) == 0 {
		expected = cloneJSON(candidate.Expected)
	}
	if len(expected) == 0 {
		expected = cloneJSON(candidate.Output)
	}
	tags := params.Tags
	if len(tags) == 0 {
		tags = candidate.Tags
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return PromoteDatasetTraceCandidateResult{}, fmt.Errorf("begin dataset trace promote transaction: %w", err)
	}
	defer rollback(ctx, tx)
	q := r.queries.WithTx(tx)

	exampleRow, err := upsertDatasetExampleWithQueries(ctx, q, UpsertDatasetExampleParams{
		DatasetID:      params.DatasetID,
		ExternalID:     candidate.ExternalID,
		Input:          candidate.Input,
		Expected:       expected,
		Metadata:       candidate.Metadata,
		Tags:           tags,
		Status:         domain.DatasetExampleStatusActive,
		Source:         domain.DatasetExampleSourceTrace,
		SourceRunID:    candidate.SourceRunID,
		SourceTraceID:  candidate.SourceTraceID,
		SourcePlatform: stringPtr(candidate.SourcePlatform),
		Actor:          params.Actor,
	})
	if err != nil {
		return PromoteDatasetTraceCandidateResult{}, err
	}

	updatedRow, err := q.UpdateDatasetTraceCandidatePromotion(ctx, repositorysqlc.UpdateDatasetTraceCandidatePromotionParams{
		ID:                params.CandidateID,
		PromotedExampleID: &exampleRow.ID,
		Expected:          expected,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PromoteDatasetTraceCandidateResult{}, ErrDatasetTraceCandidatePromoted
		}
		return PromoteDatasetTraceCandidateResult{}, fmt.Errorf("mark dataset trace candidate promoted: %w", err)
	}

	versionRow, err := createDatasetVersionWithQueries(ctx, q, CreateDatasetVersionParams{
		DatasetID: params.DatasetID,
		Label:     stringPtr("promote:" + candidate.ID.String()),
		Actor:     params.Actor,
	})
	if err != nil {
		return PromoteDatasetTraceCandidateResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PromoteDatasetTraceCandidateResult{}, fmt.Errorf("commit dataset trace promote transaction: %w", err)
	}

	example, err := mapDatasetExample(exampleRow)
	if err != nil {
		return PromoteDatasetTraceCandidateResult{}, err
	}
	version, err := mapDatasetVersion(versionRow)
	if err != nil {
		return PromoteDatasetTraceCandidateResult{}, err
	}
	promotedCandidate, err := mapDatasetTraceCandidate(updatedRow)
	if err != nil {
		return PromoteDatasetTraceCandidateResult{}, err
	}

	return PromoteDatasetTraceCandidateResult{
		Candidate: promotedCandidate,
		Example:   example,
		Version:   version,
		Created:   true,
	}, nil
}

func mapDatasetTraceImport(row repositorysqlc.DatasetTraceImport) (DatasetTraceImport, error) {
	createdAt, err := requiredTime("dataset_trace_imports.created_at", row.CreatedAt)
	if err != nil {
		return DatasetTraceImport{}, err
	}
	return DatasetTraceImport{
		ID:             row.ID,
		DatasetID:      row.DatasetID,
		SourcePlatform: row.SourcePlatform,
		ArtifactID:     cloneUUIDPtr(row.ArtifactID),
		CandidateCount: row.CandidateCount,
		Status:         row.Status,
		CreatedBy:      row.CreatedBy,
		CreatedAt:      createdAt,
	}, nil
}

func mapDatasetTraceCandidate(row repositorysqlc.DatasetTraceCandidate) (DatasetTraceCandidate, error) {
	createdAt, err := requiredTime("dataset_trace_candidates.created_at", row.CreatedAt)
	if err != nil {
		return DatasetTraceCandidate{}, err
	}
	updatedAt, err := requiredTime("dataset_trace_candidates.updated_at", row.UpdatedAt)
	if err != nil {
		return DatasetTraceCandidate{}, err
	}
	return DatasetTraceCandidate{
		ID:                row.ID,
		DatasetID:         row.DatasetID,
		ImportID:          row.ImportID,
		SourcePlatform:    row.SourcePlatform,
		SourceTraceID:     cloneStringPtr(row.SourceTraceID),
		SourceRunID:       cloneUUIDPtr(row.SourceRunID),
		SourceRunAgentID:  cloneUUIDPtr(row.SourceRunAgentID),
		ExternalID:        cloneStringPtr(row.ExternalID),
		Input:             cloneJSON(row.Input),
		Output:            cloneJSON(row.Output),
		Expected:          cloneJSON(row.Expected),
		Metadata:          defaultJSON(row.Metadata),
		Tags:              append([]string(nil), row.Tags...),
		Status:            row.Status,
		PromotedExampleID: cloneUUIDPtr(row.PromotedExampleID),
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}

func defaultJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return cloneJSON(raw)
}
