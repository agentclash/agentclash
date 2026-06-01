package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	datasettraces "github.com/agentclash/agentclash/backend/internal/datasets/traces"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const datasetTraceArtifactType = "dataset_trace_raw"

type DatasetTraceRepository interface {
	ImportDatasetTraces(context.Context, repository.ImportDatasetTracesParams) (repository.ImportDatasetTracesResult, error)
	ListDatasetTraceCandidates(context.Context, repository.ListDatasetTraceCandidatesParams) (repository.ListDatasetTraceCandidatesResult, error)
	PromoteDatasetTraceCandidate(context.Context, repository.PromoteDatasetTraceCandidateParams) (repository.PromoteDatasetTraceCandidateResult, error)
	BuildDatasetTraceCandidatesFromRunAgent(context.Context, uuid.UUID) ([]datasettraces.Candidate, error)
	CreateArtifact(context.Context, repository.CreateArtifactParams) (repository.Artifact, error)
	GetOrganizationIDByWorkspaceID(context.Context, uuid.UUID) (uuid.UUID, error)
	GetRunAgentByID(context.Context, uuid.UUID) (domain.RunAgent, error)
}

type DatasetTraceService interface {
	ImportDatasetTraces(context.Context, Caller, ImportDatasetTracesInput) (repository.ImportDatasetTracesResult, error)
	ListDatasetTraceCandidates(context.Context, Caller, ListDatasetTraceCandidatesInput) (repository.ListDatasetTraceCandidatesResult, error)
	PromoteDatasetTraceCandidate(context.Context, Caller, PromoteDatasetTraceCandidateInput) (repository.PromoteDatasetTraceCandidateResult, error)
}

type traceArtifactDeps struct {
	store  storage.Store
	bucket string
}

func (m *DatasetManager) WithTraceArtifactStore(store storage.Store, bucket string) *DatasetManager {
	m.traceArtifacts = &traceArtifactDeps{store: store, bucket: bucket}
	return m
}

type ImportDatasetTracesInput struct {
	WorkspaceID    uuid.UUID
	DatasetID      uuid.UUID
	SourcePlatform string
	Payload        json.RawMessage
	RawData        []byte
	RunID          *uuid.UUID
	RunAgentID     *uuid.UUID
	ArtifactID     *uuid.UUID
	Redaction      datasettraces.RedactionConfig
}

type ListDatasetTraceCandidatesInput struct {
	WorkspaceID uuid.UUID
	DatasetID   uuid.UUID
	Status      *string
	Limit       int32
	Offset      int32
}

type PromoteDatasetTraceCandidateInput struct {
	WorkspaceID uuid.UUID
	DatasetID   uuid.UUID
	CandidateID uuid.UUID
	Expected    json.RawMessage
	Tags        []string
}

func (m *DatasetManager) ImportDatasetTraces(ctx context.Context, caller Caller, input ImportDatasetTracesInput) (repository.ImportDatasetTracesResult, error) {
	dataset, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID})
	if err != nil {
		return repository.ImportDatasetTracesResult{}, err
	}
	platform, err := datasettraces.NormalizePlatform(strings.TrimSpace(input.SourcePlatform))
	if err != nil {
		return repository.ImportDatasetTracesResult{}, err
	}

	traceRepo, ok := m.repo.(DatasetTraceRepository)
	if !ok {
		return repository.ImportDatasetTracesResult{}, fmt.Errorf("dataset trace repository not configured")
	}

	rawData := append([]byte(nil), input.RawData...)
	if len(rawData) == 0 && len(input.Payload) > 0 {
		rawData = input.Payload
	}

	var artifactID = input.ArtifactID
	if artifactID == nil && len(rawData) > 0 && m.traceArtifacts != nil {
		storedID, storeErr := m.storeDatasetTraceArtifact(ctx, traceRepo, caller, input, rawData)
		if storeErr != nil {
			return repository.ImportDatasetTracesResult{}, storeErr
		}
		artifactID = &storedID
	}

	var candidates []datasettraces.Candidate
	switch platform {
	case datasettraces.SourceAgentClash:
		runAgentID := input.RunAgentID
		if runAgentID == nil {
			return repository.ImportDatasetTracesResult{}, fmt.Errorf("run_agent_id is required for agentclash trace import")
		}
		runAgent, getErr := traceRepo.GetRunAgentByID(ctx, *runAgentID)
		if getErr != nil {
			return repository.ImportDatasetTracesResult{}, getErr
		}
		if input.RunID != nil && runAgent.RunID != *input.RunID {
			return repository.ImportDatasetTracesResult{}, ErrForbidden
		}
		candidates, err = traceRepo.BuildDatasetTraceCandidatesFromRunAgent(ctx, *runAgentID)
	default:
		if len(rawData) == 0 {
			return repository.ImportDatasetTracesResult{}, fmt.Errorf("payload or raw trace data is required")
		}
		parsed, parseErr := datasettraces.ImportFromPayload(platform, rawData)
		if parseErr != nil {
			return repository.ImportDatasetTracesResult{}, parseErr
		}
		if len(parsed.Errors) > 0 {
			return repository.ImportDatasetTracesResult{}, fmt.Errorf("trace parse errors: %s", parsed.Errors[0].Message)
		}
		candidates = parsed.Candidates
	}
	if err != nil {
		return repository.ImportDatasetTracesResult{}, err
	}

	redacted := make([]datasettraces.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		item, redactErr := datasettraces.RedactCandidate(candidate, input.Redaction)
		if redactErr != nil {
			return repository.ImportDatasetTracesResult{}, redactErr
		}
		if dataset.InputSchemaEnforced {
			if err := domain.ValidateDatasetInputAgainstSchema(dataset.InputSchema, item.Input); err != nil {
				return repository.ImportDatasetTracesResult{}, err
			}
		}
		redacted = append(redacted, item)
	}

	return traceRepo.ImportDatasetTraces(ctx, repository.ImportDatasetTracesParams{
		DatasetID:      input.DatasetID,
		SourcePlatform: string(platform),
		ArtifactID:     artifactID,
		Candidates:     redacted,
		Actor:          caller.UserID,
	})
}

func (m *DatasetManager) ListDatasetTraceCandidates(ctx context.Context, caller Caller, input ListDatasetTraceCandidatesInput) (repository.ListDatasetTraceCandidatesResult, error) {
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return repository.ListDatasetTraceCandidatesResult{}, err
	}
	traceRepo, ok := m.repo.(DatasetTraceRepository)
	if !ok {
		return repository.ListDatasetTraceCandidatesResult{}, fmt.Errorf("dataset trace repository not configured")
	}
	return traceRepo.ListDatasetTraceCandidates(ctx, repository.ListDatasetTraceCandidatesParams{
		DatasetID: input.DatasetID,
		Status:    input.Status,
		Limit:     input.Limit,
		Offset:    input.Offset,
	})
}

func (m *DatasetManager) PromoteDatasetTraceCandidate(ctx context.Context, caller Caller, input PromoteDatasetTraceCandidateInput) (repository.PromoteDatasetTraceCandidateResult, error) {
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return repository.PromoteDatasetTraceCandidateResult{}, err
	}
	traceRepo, ok := m.repo.(DatasetTraceRepository)
	if !ok {
		return repository.PromoteDatasetTraceCandidateResult{}, fmt.Errorf("dataset trace repository not configured")
	}
	return traceRepo.PromoteDatasetTraceCandidate(ctx, repository.PromoteDatasetTraceCandidateParams{
		CandidateID: input.CandidateID,
		DatasetID:   input.DatasetID,
		Expected:    input.Expected,
		Tags:        input.Tags,
		Actor:       caller.UserID,
	})
}

func (m *DatasetManager) storeDatasetTraceArtifact(ctx context.Context, repo DatasetTraceRepository, caller Caller, input ImportDatasetTracesInput, rawData []byte) (uuid.UUID, error) {
	if m.traceArtifacts == nil || m.traceArtifacts.store == nil {
		return uuid.Nil, fmt.Errorf("trace artifact store is not configured")
	}
	orgID, err := repo.GetOrganizationIDByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return uuid.Nil, err
	}
	sum := sha256.Sum256(rawData)
	checksum := hex.EncodeToString(sum[:])
	objectKey := buildArtifactStorageKey(input.WorkspaceID, datasetTraceArtifactType, checksum)
	meta, err := m.traceArtifacts.store.PutObject(ctx, storage.PutObjectInput{
		Key:         objectKey,
		Body:        bytes.NewReader(rawData),
		SizeBytes:   int64(len(rawData)),
		ContentType: "application/json",
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("store dataset trace artifact: %w", err)
	}
	artifact, err := repo.CreateArtifact(ctx, repository.CreateArtifactParams{
		OrganizationID:  orgID,
		WorkspaceID:     input.WorkspaceID,
		RunID:           input.RunID,
		RunAgentID:      input.RunAgentID,
		ArtifactType:    datasetTraceArtifactType,
		StorageBucket:   meta.Bucket,
		StorageKey:      meta.Key,
		ContentType:     datasetStringPtr("application/json"),
		SizeBytes:       datasetInt64Ptr(int64(len(rawData))),
		ChecksumSHA256:  datasetStringPtr(checksum),
		Visibility:      defaultArtifactVisibility,
		RetentionStatus: defaultArtifactRetentionStatus,
		Metadata:        json.RawMessage(`{"source_platform":` + jsonString(input.SourcePlatform) + `}`),
	})
	if err != nil {
		return uuid.Nil, err
	}
	return artifact.ID, nil
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func datasetInt64Ptr(value int64) *int64 {
	return &value
}

func importDatasetTracesHandler(logger *slog.Logger, service DatasetTraceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxDatasetImportBytes))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "failed to read request body")
			return
		}
		var req struct {
			SourcePlatform string                        `json:"source_platform"`
			Payload        json.RawMessage               `json:"payload"`
			RunID          *uuid.UUID                    `json:"run_id"`
			RunAgentID     *uuid.UUID                    `json:"run_agent_id"`
			ArtifactID     *uuid.UUID                    `json:"artifact_id"`
			Redaction      datasettraces.RedactionConfig `json:"redaction"`
		}
		rawData := []byte(nil)
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				rawData = body
				req.SourcePlatform = r.URL.Query().Get("source_platform")
			} else if len(req.Payload) == 0 && req.RunAgentID == nil && req.ArtifactID == nil && strings.TrimSpace(string(body)) != "" && strings.TrimSpace(string(body))[0] != '{' {
				rawData = body
			}
		}
		if strings.TrimSpace(req.SourcePlatform) == "" {
			req.SourcePlatform = r.URL.Query().Get("source_platform")
		}
		result, err := service.ImportDatasetTraces(r.Context(), caller, ImportDatasetTracesInput{
			WorkspaceID: workspaceID, DatasetID: datasetID, SourcePlatform: req.SourcePlatform,
			Payload: req.Payload, RawData: rawData, RunID: req.RunID, RunAgentID: req.RunAgentID,
			ArtifactID: req.ArtifactID, Redaction: req.Redaction,
		})
		if err != nil {
			handleDatasetTraceError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	}
}

func listDatasetTraceCandidatesHandler(logger *slog.Logger, service DatasetTraceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		limit, offset, ok := paginationFromRequest(w, r)
		if !ok {
			return
		}
		var status *string
		if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
			status = &raw
		}
		result, err := service.ListDatasetTraceCandidates(r.Context(), caller, ListDatasetTraceCandidatesInput{
			WorkspaceID: workspaceID, DatasetID: datasetID, Status: status, Limit: limit, Offset: offset,
		})
		if err != nil {
			handleDatasetTraceError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func promoteDatasetTraceCandidateHandler(logger *slog.Logger, service DatasetTraceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		candidateID, err := uuid.Parse(chi.URLParam(r, "candidateID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_candidate_id", "candidate ID is malformed")
			return
		}
		var req struct {
			Expected json.RawMessage `json:"expected"`
			Tags     []string        `json:"tags"`
		}
		if r.Body != nil {
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
				return
			}
		}
		result, err := service.PromoteDatasetTraceCandidate(r.Context(), caller, PromoteDatasetTraceCandidateInput{
			WorkspaceID: workspaceID, DatasetID: datasetID, CandidateID: candidateID, Expected: req.Expected, Tags: req.Tags,
		})
		if err != nil {
			handleDatasetTraceError(w, logger, err)
			return
		}
		status := http.StatusCreated
		if !result.Created {
			status = http.StatusOK
		}
		writeJSON(w, status, result)
	}
}

func handleDatasetTraceError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, repository.ErrDatasetTraceCandidateNotFound):
		writeError(w, http.StatusNotFound, "not_found", "trace candidate not found")
	case errors.Is(err, repository.ErrDatasetTraceCandidatePromoted):
		writeError(w, http.StatusConflict, "already_promoted", "trace candidate is not pending")
	default:
		handleDatasetError(w, logger, err)
	}
}
