package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestDatasetImportDryRunDoesNotMutate(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	caller := datasetImportCaller(workspaceID)
	repo := newDatasetImportFakeRepo(workspaceID, datasetID)
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	result, err := manager.ImportDataset(context.Background(), caller, DatasetImportInput{
		WorkspaceID: workspaceID,
		DatasetID:   datasetID,
		Format:      "braintrust",
		Mode:        DatasetImportModeAdd,
		DryRun:      true,
		Data:        []byte(`{"input":{"q":"one"},"expected":{"a":"two"},"external_id":"case-1"}` + "\n"),
	})
	if err != nil {
		t.Fatalf("ImportDataset() error = %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("ImportDataset() errors = %+v", result.Errors)
	}
	if len(result.Preview) != 1 {
		t.Fatalf("len(preview) = %d, want 1", len(result.Preview))
	}
	if len(repo.examples) != 0 {
		t.Fatalf("dry run mutated examples: %+v", repo.examples)
	}
	if repo.versionCreated {
		t.Fatal("dry run created a version")
	}
}

func TestDatasetImportAdditiveCreatesVersion(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	caller := datasetImportCaller(workspaceID)
	repo := newDatasetImportFakeRepo(workspaceID, datasetID)
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	result, err := manager.ImportDataset(context.Background(), caller, DatasetImportInput{
		WorkspaceID: workspaceID,
		DatasetID:   datasetID,
		Format:      "openai",
		Mode:        DatasetImportModeAdd,
		Data:        []byte(`{"input":{"q":"one"},"ideal":{"a":"two"},"external_id":"case-1"}` + "\n"),
	})
	if err != nil {
		t.Fatalf("ImportDataset() error = %v", err)
	}
	if result.ImportedCount != 1 {
		t.Fatalf("ImportedCount = %d, want 1", result.ImportedCount)
	}
	if result.Version == nil {
		t.Fatal("Version is nil")
	}
	if len(repo.examples) != 1 {
		t.Fatalf("len(examples) = %d, want 1", len(repo.examples))
	}
	example := repo.examples[0]
	if example.ExternalID == nil || *example.ExternalID != "case-1" {
		t.Fatalf("external_id = %v, want case-1", example.ExternalID)
	}
	if example.Source != domain.DatasetExampleSourceImport {
		t.Fatalf("source = %q, want import", example.Source)
	}
}

func TestDatasetImportReplaceArchivesMissingActiveExamples(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	caller := datasetImportCaller(workspaceID)
	repo := newDatasetImportFakeRepo(workspaceID, datasetID)
	oldID := "old"
	keepID := "keep"
	repo.examples = []repository.DatasetExample{
		datasetImportExample(datasetID, &oldID, domain.DatasetExampleStatusActive),
		datasetImportExample(datasetID, &keepID, domain.DatasetExampleStatusActive),
	}
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	_, err := manager.ImportDataset(context.Background(), caller, DatasetImportInput{
		WorkspaceID: workspaceID,
		DatasetID:   datasetID,
		Format:      "braintrust",
		Mode:        DatasetImportModeReplace,
		Data:        []byte(`{"input":{"q":"new"},"expected":{"a":"new"},"external_id":"keep"}` + "\n"),
	})
	if err != nil {
		t.Fatalf("ImportDataset() error = %v", err)
	}
	for _, example := range repo.examples {
		if example.ExternalID != nil && *example.ExternalID == "old" && example.Status != domain.DatasetExampleStatusArchived {
			t.Fatalf("old example status = %q, want archived", example.Status)
		}
		if example.ExternalID != nil && *example.ExternalID == "keep" && example.Status != domain.DatasetExampleStatusActive {
			t.Fatalf("keep example status = %q, want active", example.Status)
		}
	}
}

func TestDatasetImportEnforcedSchemaReportsRowErrors(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	caller := datasetImportCaller(workspaceID)
	repo := newDatasetImportFakeRepo(workspaceID, datasetID)
	repo.dataset.InputSchemaEnforced = true
	repo.dataset.InputSchema = json.RawMessage(`{"type":"object","required":["q"]}`)
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	result, err := manager.ImportDataset(context.Background(), caller, DatasetImportInput{
		WorkspaceID: workspaceID,
		DatasetID:   datasetID,
		Format:      "braintrust",
		Mode:        DatasetImportModeAdd,
		Data:        []byte(`{"input":{"missing":"q"},"expected":"nope","external_id":"bad"}` + "\n"),
	})
	if err != nil {
		t.Fatalf("ImportDataset() error = %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("len(errors) = %d, want 1: %+v", len(result.Errors), result.Errors)
	}
	if len(repo.examples) != 0 {
		t.Fatalf("schema failure mutated examples: %+v", repo.examples)
	}
}

type allowWorkspaceAuthorizer struct{}

func (allowWorkspaceAuthorizer) AuthorizeWorkspace(context.Context, Caller, uuid.UUID) error {
	return nil
}

type datasetImportFakeRepo struct {
	dataset        repository.Dataset
	examples       []repository.DatasetExample
	versionCreated bool
}

func newDatasetImportFakeRepo(workspaceID, datasetID uuid.UUID) *datasetImportFakeRepo {
	return &datasetImportFakeRepo{
		dataset: repository.Dataset{
			ID:          datasetID,
			WorkspaceID: workspaceID,
			Slug:        "dataset",
			Name:        "Dataset",
			CreatedBy:   uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

func (r *datasetImportFakeRepo) CreateDataset(context.Context, repository.CreateDatasetParams) (repository.Dataset, error) {
	return repository.Dataset{}, nil
}
func (r *datasetImportFakeRepo) GetDatasetByID(context.Context, uuid.UUID) (repository.Dataset, error) {
	return r.dataset, nil
}
func (r *datasetImportFakeRepo) ListDatasetsByWorkspaceID(context.Context, uuid.UUID, int32, int32) ([]repository.Dataset, error) {
	return nil, nil
}
func (r *datasetImportFakeRepo) CountDatasetsByWorkspaceID(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *datasetImportFakeRepo) PatchDataset(context.Context, repository.PatchDatasetParams) (repository.Dataset, error) {
	return repository.Dataset{}, nil
}
func (r *datasetImportFakeRepo) ArchiveDataset(context.Context, uuid.UUID) (repository.Dataset, error) {
	return repository.Dataset{}, nil
}
func (r *datasetImportFakeRepo) UpsertDatasetExample(_ context.Context, params repository.UpsertDatasetExampleParams) (repository.DatasetExample, error) {
	for i, example := range r.examples {
		if params.ExternalID != nil && example.ExternalID != nil && *params.ExternalID == *example.ExternalID {
			r.examples[i].Input = params.Input
			r.examples[i].Expected = params.Expected
			r.examples[i].Metadata = params.Metadata
			r.examples[i].Tags = params.Tags
			r.examples[i].Status = params.Status
			r.examples[i].Source = params.Source
			return r.examples[i], nil
		}
	}
	example := repository.DatasetExample{
		ID:         uuid.New(),
		DatasetID:  params.DatasetID,
		ExternalID: params.ExternalID,
		Input:      params.Input,
		Expected:   params.Expected,
		Metadata:   params.Metadata,
		Tags:       params.Tags,
		Status:     params.Status,
		Source:     params.Source,
		CreatedBy:  params.Actor,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	r.examples = append(r.examples, example)
	return example, nil
}
func (r *datasetImportFakeRepo) GetDatasetExampleByID(_ context.Context, id uuid.UUID) (repository.DatasetExample, error) {
	for _, example := range r.examples {
		if example.ID == id {
			return example, nil
		}
	}
	return repository.DatasetExample{}, repository.ErrDatasetExampleNotFound
}
func (r *datasetImportFakeRepo) ListDatasetExamplesByDatasetID(_ context.Context, params repository.ListDatasetExamplesParams) ([]repository.DatasetExample, error) {
	out := make([]repository.DatasetExample, 0, len(r.examples))
	for _, example := range r.examples {
		if params.Status != nil && example.Status != *params.Status {
			continue
		}
		out = append(out, example)
	}
	return out, nil
}
func (r *datasetImportFakeRepo) CountDatasetExamplesByDatasetID(context.Context, uuid.UUID, *domain.DatasetExampleStatus) (int64, error) {
	return int64(len(r.examples)), nil
}
func (r *datasetImportFakeRepo) PatchDatasetExample(_ context.Context, params repository.PatchDatasetExampleParams) (repository.DatasetExample, error) {
	for i, example := range r.examples {
		if example.ID == params.ID {
			if params.Status != nil {
				r.examples[i].Status = *params.Status
			}
			return r.examples[i], nil
		}
	}
	return repository.DatasetExample{}, repository.ErrDatasetExampleNotFound
}
func (r *datasetImportFakeRepo) CreateDatasetVersion(_ context.Context, params repository.CreateDatasetVersionParams) (repository.DatasetVersion, error) {
	r.versionCreated = true
	return repository.DatasetVersion{ID: uuid.New(), DatasetID: params.DatasetID, VersionNumber: 1, ExampleCount: int32(len(r.examples)), CreatedBy: params.Actor, CreatedAt: time.Now()}, nil
}
func (r *datasetImportFakeRepo) ListDatasetVersionsByDatasetID(context.Context, uuid.UUID) ([]repository.DatasetVersion, error) {
	return nil, nil
}
func (r *datasetImportFakeRepo) GetDatasetVersionByID(context.Context, uuid.UUID) (repository.DatasetVersion, error) {
	return repository.DatasetVersion{}, nil
}
func (r *datasetImportFakeRepo) ListDatasetVersionExamples(context.Context, uuid.UUID) ([]repository.DatasetExample, error) {
	return nil, nil
}

func datasetImportCaller(workspaceID uuid.UUID) Caller {
	return Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceAdmin},
		},
	}
}

func datasetImportExample(datasetID uuid.UUID, externalID *string, status domain.DatasetExampleStatus) repository.DatasetExample {
	return repository.DatasetExample{
		ID:         uuid.New(),
		DatasetID:  datasetID,
		ExternalID: externalID,
		Input:      json.RawMessage(`{"q":"old"}`),
		Metadata:   json.RawMessage(`{}`),
		Status:     status,
		Source:     domain.DatasetExampleSourceManual,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}
