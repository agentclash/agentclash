package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	datasettraces "github.com/agentclash/agentclash/backend/internal/datasets/traces"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestImportDatasetTracesOTLP(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	repo := newDatasetTraceFakeRepo(workspaceID, datasetID)
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	payload := []byte(`{"resourceSpans":[{"scopeSpans":[{"spans":[{"traceId":"trace-1","spanId":"span-1","name":"chat","attributes":[{"key":"gen_ai.input.messages","value":{"stringValue":"[{\"role\":\"user\",\"content\":\"refund?\"}]"}},{"key":"gen_ai.output.messages","value":{"stringValue":"[{\"role\":\"assistant\",\"content\":\"approved\"}]"}}]}]}]}]}`)
	result, err := manager.ImportDatasetTraces(context.Background(), datasetImportCaller(workspaceID), ImportDatasetTracesInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, SourcePlatform: "otel", Payload: payload,
	})
	if err != nil {
		t.Fatalf("ImportDatasetTraces() error = %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(result.Candidates))
	}
	if result.Candidates[0].SourceTraceID == nil || *result.Candidates[0].SourceTraceID != "trace-1" {
		t.Fatalf("source_trace_id = %v", result.Candidates[0].SourceTraceID)
	}
}

func TestPromoteDatasetTraceCandidateCreatesExample(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	repo := newDatasetTraceFakeRepo(workspaceID, datasetID)
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	importResult, err := manager.ImportDatasetTraces(context.Background(), datasetImportCaller(workspaceID), ImportDatasetTracesInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, SourcePlatform: "braintrust",
		Payload: []byte(`{"input":{"q":"one"},"expected":{"a":"two"},"external_id":"trace-1"}` + "\n"),
	})
	if err != nil {
		t.Fatalf("ImportDatasetTraces() error = %v", err)
	}
	candidateID := importResult.Candidates[0].ID
	promoteResult, err := manager.PromoteDatasetTraceCandidate(context.Background(), datasetImportCaller(workspaceID), PromoteDatasetTraceCandidateInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, CandidateID: candidateID,
		Expected: json.RawMessage(`{"a":"edited"}`),
	})
	if err != nil {
		t.Fatalf("PromoteDatasetTraceCandidate() error = %v", err)
	}
	if !promoteResult.Created {
		t.Fatal("Created = false, want true")
	}
	if promoteResult.Example.Source != domain.DatasetExampleSourceTrace {
		t.Fatalf("source = %q, want trace", promoteResult.Example.Source)
	}
	if promoteResult.Example.SourceTraceID == nil || *promoteResult.Example.SourceTraceID != "trace-1" {
		t.Fatalf("source_trace_id = %v", promoteResult.Example.SourceTraceID)
	}
	if !jsonEqual(promoteResult.Example.Expected, json.RawMessage(`{"a":"edited"}`)) {
		t.Fatalf("expected = %s", string(promoteResult.Example.Expected))
	}
}

type datasetTraceFakeRepo struct {
	datasetImportFakeRepo
	imports    []repository.DatasetTraceImport
	candidates []repository.DatasetTraceCandidate
}

func newDatasetTraceFakeRepo(workspaceID, datasetID uuid.UUID) *datasetTraceFakeRepo {
	return &datasetTraceFakeRepo{datasetImportFakeRepo: *newDatasetImportFakeRepo(workspaceID, datasetID)}
}

func (r *datasetTraceFakeRepo) ImportDatasetTraces(_ context.Context, params repository.ImportDatasetTracesParams) (repository.ImportDatasetTracesResult, error) {
	importBatch := repository.DatasetTraceImport{
		ID: uuid.New(), DatasetID: params.DatasetID, SourcePlatform: params.SourcePlatform,
		ArtifactID: params.ArtifactID, CandidateCount: int32(len(params.Candidates)), Status: "completed",
		CreatedBy: params.Actor, CreatedAt: time.Now(),
	}
	r.imports = append(r.imports, importBatch)
	candidates := make([]repository.DatasetTraceCandidate, 0, len(params.Candidates))
	for _, candidate := range params.Candidates {
		row := repository.DatasetTraceCandidate{
			ID: uuid.New(), DatasetID: params.DatasetID, ImportID: importBatch.ID, SourcePlatform: params.SourcePlatform,
			SourceTraceID: stringPtrNonEmpty(candidate.SourceTraceID), SourceRunID: candidate.SourceRunID,
			SourceRunAgentID: candidate.SourceRunAgentID, ExternalID: candidate.ExternalID,
			Input: candidate.Input, Output: candidate.Output, Expected: candidate.Expected,
			Metadata: candidate.Metadata, Tags: candidate.Tags, Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		r.candidates = append(r.candidates, row)
		candidates = append(candidates, row)
	}
	return repository.ImportDatasetTracesResult{Import: importBatch, Candidates: candidates}, nil
}

func (r *datasetTraceFakeRepo) ListDatasetTraceCandidates(_ context.Context, params repository.ListDatasetTraceCandidatesParams) (repository.ListDatasetTraceCandidatesResult, error) {
	out := make([]repository.DatasetTraceCandidate, 0)
	for _, candidate := range r.candidates {
		if candidate.DatasetID != params.DatasetID {
			continue
		}
		if params.Status != nil && candidate.Status != *params.Status {
			continue
		}
		out = append(out, candidate)
	}
	return repository.ListDatasetTraceCandidatesResult{Candidates: out, Total: int64(len(out))}, nil
}

func (r *datasetTraceFakeRepo) PromoteDatasetTraceCandidate(_ context.Context, params repository.PromoteDatasetTraceCandidateParams) (repository.PromoteDatasetTraceCandidateResult, error) {
	for i, candidate := range r.candidates {
		if candidate.ID != params.CandidateID {
			continue
		}
		if candidate.Status == "promoted" && candidate.PromotedExampleID != nil {
			example, err := r.GetDatasetExampleByID(context.Background(), *candidate.PromotedExampleID)
			if err != nil {
				return repository.PromoteDatasetTraceCandidateResult{}, err
			}
			return repository.PromoteDatasetTraceCandidateResult{Candidate: candidate, Example: example, Created: false}, nil
		}
		expected := params.Expected
		if len(expected) == 0 {
			expected = candidate.Expected
		}
		if len(expected) == 0 {
			expected = candidate.Output
		}
		example, err := r.upsertTraceExample(context.Background(), repository.UpsertDatasetExampleParams{
			DatasetID: params.DatasetID, ExternalID: candidate.ExternalID, Input: candidate.Input, Expected: expected,
			Metadata: candidate.Metadata, Tags: candidate.Tags, Status: domain.DatasetExampleStatusActive,
			Source: domain.DatasetExampleSourceTrace, SourceRunID: candidate.SourceRunID, SourceTraceID: candidate.SourceTraceID,
			SourcePlatform: stringPtrNonEmpty(candidate.SourcePlatform), Actor: params.Actor,
		})
		if err != nil {
			return repository.PromoteDatasetTraceCandidateResult{}, err
		}
		r.candidates[i].Status = "promoted"
		r.candidates[i].PromotedExampleID = &example.ID
		r.candidates[i].Expected = expected
		version, err := r.CreateDatasetVersion(context.Background(), repository.CreateDatasetVersionParams{
			DatasetID: params.DatasetID, Label: stringPtrNonEmpty("promote:" + candidate.ID.String()), Actor: params.Actor,
		})
		if err != nil {
			return repository.PromoteDatasetTraceCandidateResult{}, err
		}
		return repository.PromoteDatasetTraceCandidateResult{Candidate: r.candidates[i], Example: example, Version: version, Created: true}, nil
	}
	return repository.PromoteDatasetTraceCandidateResult{}, repository.ErrDatasetTraceCandidateNotFound
}

func (r *datasetTraceFakeRepo) upsertTraceExample(_ context.Context, params repository.UpsertDatasetExampleParams) (repository.DatasetExample, error) {
	for i, example := range r.examples {
		if params.ExternalID != nil && example.ExternalID != nil && *params.ExternalID == *example.ExternalID {
			r.examples[i].Input = params.Input
			r.examples[i].Expected = params.Expected
			r.examples[i].Metadata = params.Metadata
			r.examples[i].Tags = params.Tags
			r.examples[i].Status = params.Status
			r.examples[i].Source = params.Source
			r.examples[i].SourceRunID = params.SourceRunID
			r.examples[i].SourceTraceID = params.SourceTraceID
			r.examples[i].SourcePlatform = params.SourcePlatform
			return r.examples[i], nil
		}
	}
	example := repository.DatasetExample{
		ID: uuid.New(), DatasetID: params.DatasetID, ExternalID: params.ExternalID, Input: params.Input,
		Expected: params.Expected, Metadata: params.Metadata, Tags: params.Tags, Status: params.Status,
		Source: params.Source, SourceRunID: params.SourceRunID, SourceTraceID: params.SourceTraceID,
		SourcePlatform: params.SourcePlatform, CreatedBy: params.Actor, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	r.examples = append(r.examples, example)
	return example, nil
}

func (r *datasetTraceFakeRepo) BuildDatasetTraceCandidatesFromRunAgent(context.Context, uuid.UUID) ([]datasettraces.Candidate, error) {
	return nil, nil
}
func (r *datasetTraceFakeRepo) CreateArtifact(context.Context, repository.CreateArtifactParams) (repository.Artifact, error) {
	return repository.Artifact{ID: uuid.New()}, nil
}
func (r *datasetTraceFakeRepo) GetOrganizationIDByWorkspaceID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (r *datasetTraceFakeRepo) GetRunAgentByID(context.Context, uuid.UUID) (domain.RunAgent, error) {
	return domain.RunAgent{}, nil
}

func stringPtrNonEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func jsonEqual(left, right json.RawMessage) bool {
	var a any
	var b any
	if err := json.Unmarshal(left, &a); err != nil {
		return false
	}
	if err := json.Unmarshal(right, &b); err != nil {
		return false
	}
	encodedA, _ := json.Marshal(a)
	encodedB, _ := json.Marshal(b)
	return string(encodedA) == string(encodedB)
}
