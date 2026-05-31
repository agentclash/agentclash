package api

import (
	"context"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestDatasetManagerStartDatasetEvalMaterializesAndCreatesRun(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	versionID := uuid.New()
	packVersionID := uuid.New()
	inputSetID := uuid.New()
	runID := uuid.New()
	caller := datasetEvalCaller(workspaceID)
	repo := &datasetEvalFakeRepo{
		dataset: repository.Dataset{ID: datasetID, WorkspaceID: workspaceID, Name: "Golden", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		version: repository.DatasetVersion{ID: versionID, DatasetID: datasetID, VersionNumber: 1, CreatedAt: time.Now()},
		materialized: repository.DatasetVersionInputSet{
			ID: versionID, DatasetID: datasetID, DatasetVersionID: versionID, ChallengePackVersionID: packVersionID,
			ChallengeInputSetID: inputSetID, ChallengeKey: "support", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	runCreator := &datasetEvalFakeRunCreator{run: domain.Run{ID: runID, WorkspaceID: workspaceID, ChallengePackVersionID: packVersionID, ChallengeInputSetID: &inputSetID, Status: domain.RunStatusQueued}}
	manager := NewDatasetManager(datasetEvalAllowAuthorizer{}, repo).WithRunCreationService(runCreator)

	deploymentID := uuid.New()
	result, err := manager.StartDatasetEval(context.Background(), caller, StartDatasetEvalInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, VersionID: versionID, ChallengePackVersionID: packVersionID,
		ChallengeID: "support", AgentDeploymentIDs: []uuid.UUID{deploymentID}, Name: "Dataset eval",
	})
	if err != nil {
		t.Fatalf("StartDatasetEval() error = %v", err)
	}
	if result.Run.ID != runID {
		t.Fatalf("run id = %s, want %s", result.Run.ID, runID)
	}
	if repo.materializeParams.DatasetVersionID != versionID || repo.materializeParams.ChallengeKey != "support" {
		t.Fatalf("materialize params = %+v", repo.materializeParams)
	}
	if runCreator.input.ChallengeInputSetID == nil || *runCreator.input.ChallengeInputSetID != inputSetID {
		t.Fatalf("run challenge_input_set_id = %v, want %s", runCreator.input.ChallengeInputSetID, inputSetID)
	}
	if len(runCreator.input.AgentDeploymentIDs) != 1 || runCreator.input.AgentDeploymentIDs[0] != deploymentID {
		t.Fatalf("run deployments = %+v, want %s", runCreator.input.AgentDeploymentIDs, deploymentID)
	}
	if repo.recordedRun.RunID != runID {
		t.Fatalf("recorded run = %+v, want run %s", repo.recordedRun, runID)
	}
}

func TestDatasetManagerStartDatasetEvalRejectsVersionFromOtherDataset(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	repo := &datasetEvalFakeRepo{
		dataset: repository.Dataset{ID: datasetID, WorkspaceID: workspaceID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		version: repository.DatasetVersion{ID: uuid.New(), DatasetID: uuid.New(), VersionNumber: 1, CreatedAt: time.Now()},
	}
	manager := NewDatasetManager(datasetEvalAllowAuthorizer{}, repo).WithRunCreationService(&datasetEvalFakeRunCreator{})

	_, err := manager.StartDatasetEval(context.Background(), datasetEvalCaller(workspaceID), StartDatasetEvalInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, VersionID: repo.version.ID, ChallengePackVersionID: uuid.New(),
		ChallengeID: "support", AgentDeploymentIDs: []uuid.UUID{uuid.New()},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if repo.materializeCalled {
		t.Fatal("materialization ran for wrong dataset version")
	}
}

func TestDatasetManagerListDatasetResults(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	versionID := uuid.New()
	runID := uuid.New()
	repo := &datasetEvalFakeRepo{
		dataset: repository.Dataset{ID: datasetID, WorkspaceID: workspaceID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		version: repository.DatasetVersion{ID: versionID, DatasetID: datasetID, VersionNumber: 1, CreatedAt: time.Now()},
		results: []repository.DatasetEvalResult{{DatasetExampleID: uuid.New(), DatasetVersionID: versionID, RunID: &runID}},
	}
	manager := NewDatasetManager(datasetEvalAllowAuthorizer{}, repo)

	results, err := manager.ListDatasetResults(context.Background(), datasetEvalCaller(workspaceID), ListDatasetResultsInput{WorkspaceID: workspaceID, DatasetID: datasetID, VersionID: &versionID})
	if err != nil {
		t.Fatalf("ListDatasetResults() error = %v", err)
	}
	if len(results) != 1 || results[0].RunID == nil || *results[0].RunID != runID {
		t.Fatalf("results = %+v, want linked run %s", results, runID)
	}
}

type datasetEvalAllowAuthorizer struct{}

func (datasetEvalAllowAuthorizer) AuthorizeWorkspace(context.Context, Caller, uuid.UUID) error {
	return nil
}

type datasetEvalFakeRunCreator struct {
	input CreateRunInput
	run   domain.Run
}

func (f *datasetEvalFakeRunCreator) CreateRun(_ context.Context, _ Caller, input CreateRunInput) (CreateRunResult, error) {
	f.input = input
	if f.run.ID == uuid.Nil {
		f.run = domain.Run{ID: uuid.New(), WorkspaceID: input.WorkspaceID, ChallengePackVersionID: input.ChallengePackVersionID, ChallengeInputSetID: input.ChallengeInputSetID, Status: domain.RunStatusQueued}
	}
	return CreateRunResult{Run: f.run}, nil
}

func (f *datasetEvalFakeRunCreator) CreateEvalSession(context.Context, Caller, CreateEvalSessionInput) (CreateEvalSessionResult, error) {
	return CreateEvalSessionResult{}, nil
}

type datasetEvalFakeRepo struct {
	dataset           repository.Dataset
	version           repository.DatasetVersion
	materialized      repository.DatasetVersionInputSet
	results           []repository.DatasetEvalResult
	materializeParams repository.MaterializeDatasetVersionInputSetParams
	recordedRun       repository.RecordDatasetEvalRunParams
	materializeCalled bool
}

func (f *datasetEvalFakeRepo) CreateDataset(context.Context, repository.CreateDatasetParams) (repository.Dataset, error) {
	return repository.Dataset{}, nil
}
func (f *datasetEvalFakeRepo) GetDatasetByID(context.Context, uuid.UUID) (repository.Dataset, error) {
	return f.dataset, nil
}
func (f *datasetEvalFakeRepo) ListDatasetsByWorkspaceID(context.Context, uuid.UUID, int32, int32) ([]repository.Dataset, error) {
	return nil, nil
}
func (f *datasetEvalFakeRepo) CountDatasetsByWorkspaceID(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}
func (f *datasetEvalFakeRepo) PatchDataset(context.Context, repository.PatchDatasetParams) (repository.Dataset, error) {
	return repository.Dataset{}, nil
}
func (f *datasetEvalFakeRepo) ArchiveDataset(context.Context, uuid.UUID) (repository.Dataset, error) {
	return repository.Dataset{}, nil
}
func (f *datasetEvalFakeRepo) UpsertDatasetExample(context.Context, repository.UpsertDatasetExampleParams) (repository.DatasetExample, error) {
	return repository.DatasetExample{}, nil
}
func (f *datasetEvalFakeRepo) GetDatasetExampleByID(context.Context, uuid.UUID) (repository.DatasetExample, error) {
	return repository.DatasetExample{}, nil
}
func (f *datasetEvalFakeRepo) ListDatasetExamplesByDatasetID(context.Context, repository.ListDatasetExamplesParams) ([]repository.DatasetExample, error) {
	return nil, nil
}
func (f *datasetEvalFakeRepo) CountDatasetExamplesByDatasetID(context.Context, uuid.UUID, *domain.DatasetExampleStatus) (int64, error) {
	return 0, nil
}
func (f *datasetEvalFakeRepo) PatchDatasetExample(context.Context, repository.PatchDatasetExampleParams) (repository.DatasetExample, error) {
	return repository.DatasetExample{}, nil
}
func (f *datasetEvalFakeRepo) CreateDatasetVersion(context.Context, repository.CreateDatasetVersionParams) (repository.DatasetVersion, error) {
	return repository.DatasetVersion{}, nil
}
func (f *datasetEvalFakeRepo) ListDatasetVersionsByDatasetID(context.Context, uuid.UUID) ([]repository.DatasetVersion, error) {
	return nil, nil
}
func (f *datasetEvalFakeRepo) GetDatasetVersionByID(context.Context, uuid.UUID) (repository.DatasetVersion, error) {
	return f.version, nil
}
func (f *datasetEvalFakeRepo) ListDatasetVersionExamples(context.Context, uuid.UUID) ([]repository.DatasetExample, error) {
	return nil, nil
}
func (f *datasetEvalFakeRepo) MaterializeDatasetVersionInputSet(_ context.Context, params repository.MaterializeDatasetVersionInputSetParams) (repository.DatasetVersionInputSet, error) {
	f.materializeCalled = true
	f.materializeParams = params
	return f.materialized, nil
}
func (f *datasetEvalFakeRepo) RecordDatasetEvalRun(_ context.Context, params repository.RecordDatasetEvalRunParams) (repository.DatasetEvalRun, error) {
	f.recordedRun = params
	return repository.DatasetEvalRun{ID: uuid.New(), DatasetID: params.DatasetID, DatasetVersionID: params.DatasetVersionID, DatasetVersionInputSetID: params.DatasetVersionInputSetID, RunID: params.RunID, CreatedAt: time.Now()}, nil
}
func (f *datasetEvalFakeRepo) ListDatasetEvalResults(context.Context, uuid.UUID, *uuid.UUID) ([]repository.DatasetEvalResult, error) {
	return f.results, nil
}

func datasetEvalCaller(workspaceID uuid.UUID) Caller {
	return Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceAdmin},
		},
	}
}
