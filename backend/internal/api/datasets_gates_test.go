package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	datasetgate "github.com/agentclash/agentclash/backend/internal/datasets/gate"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestEvaluateDatasetGateFailsOnRegression(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	baselineID := uuid.New()
	exampleID := uuid.New()
	pass := "pass"
	fail := "fail"
	repo := &datasetGateFakeRepo{
		dataset: repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
		baseline: repository.DatasetBaseline{
			ID: baselineID, DatasetID: datasetID,
			ExampleOutcomes: gateTestJSON([]datasetgate.ExampleOutcome{{DatasetExampleID: exampleID, Verdict: &pass}}),
		},
		candidateOutcomes: []datasetgate.ExampleOutcome{{DatasetExampleID: exampleID, Verdict: &fail}},
		run:               domain.Run{ID: uuid.New(), WorkspaceID: workspaceID, Status: domain.RunStatusCompleted},
		evalRun:           repository.DatasetEvalRun{DatasetID: datasetID},
	}
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	result, err := manager.EvaluateDatasetGate(context.Background(), datasetEvalCaller(workspaceID), EvaluateDatasetGateInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, BaselineID: baselineID, RunID: repo.run.ID,
		MaxRegressions: intPtrGate(0),
	})
	if err != nil {
		t.Fatalf("EvaluateDatasetGate() error = %v", err)
	}
	if result.Gate.Pass {
		t.Fatal("Gate.Pass = true, want false")
	}
}

func TestEvaluateDatasetGateRejectsIncompleteRun(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	baselineID := uuid.New()
	repo := &datasetGateFakeRepo{
		dataset: repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
		baseline: repository.DatasetBaseline{
			ID: baselineID, DatasetID: datasetID,
			ExampleOutcomes: gateTestJSON([]datasetgate.ExampleOutcome{}),
		},
		run:     domain.Run{ID: uuid.New(), WorkspaceID: workspaceID, Status: domain.RunStatusRunning},
		evalRun: repository.DatasetEvalRun{DatasetID: datasetID},
	}
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	_, err := manager.EvaluateDatasetGate(context.Background(), datasetEvalCaller(workspaceID), EvaluateDatasetGateInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, BaselineID: baselineID, RunID: repo.run.ID,
	})
	if !errors.Is(err, repository.ErrDatasetGateRunNotReady) {
		t.Fatalf("EvaluateDatasetGate() error = %v, want ErrDatasetGateRunNotReady", err)
	}
}

func TestEvaluateDatasetGateRejectsInputSetMismatch(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	baselineID := uuid.New()
	baselineInputSetID := uuid.New()
	candidateInputSetID := uuid.New()
	versionID := uuid.New()
	pass := "pass"
	exampleID := uuid.New()
	repo := &datasetGateFakeRepo{
		dataset: repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
		baseline: repository.DatasetBaseline{
			ID: baselineID, DatasetID: datasetID, DatasetVersionID: versionID,
			DatasetVersionInputSetID: &baselineInputSetID,
			ExampleOutcomes: gateTestJSON([]datasetgate.ExampleOutcome{{DatasetExampleID: exampleID, Verdict: &pass}}),
		},
		candidateOutcomes: []datasetgate.ExampleOutcome{{DatasetExampleID: exampleID, Verdict: &pass}},
		run:               domain.Run{ID: uuid.New(), WorkspaceID: workspaceID, Status: domain.RunStatusCompleted},
		evalRun: repository.DatasetEvalRun{
			DatasetID: datasetID, DatasetVersionID: versionID, DatasetVersionInputSetID: candidateInputSetID,
		},
	}
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	_, err := manager.EvaluateDatasetGate(context.Background(), datasetEvalCaller(workspaceID), EvaluateDatasetGateInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, BaselineID: baselineID, RunID: repo.run.ID,
	})
	if !errors.Is(err, repository.ErrDatasetGateInputSetMismatch) {
		t.Fatalf("EvaluateDatasetGate() error = %v, want ErrDatasetGateInputSetMismatch", err)
	}
}

func TestCreateDatasetBaselineRejectsIncompleteRun(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	repo := &datasetGateFakeRepo{
		dataset: repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
		run:     domain.Run{ID: uuid.New(), WorkspaceID: workspaceID, Status: domain.RunStatusRunning},
	}
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	_, err := manager.CreateDatasetBaseline(context.Background(), datasetEvalCaller(workspaceID), CreateDatasetBaselineInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, RunID: repo.run.ID,
	})
	if !errors.Is(err, repository.ErrDatasetGateRunNotReady) {
		t.Fatalf("CreateDatasetBaseline() error = %v, want ErrDatasetGateRunNotReady", err)
	}
}

func TestCreateDatasetBaselineRejectsForeignWorkspaceRun(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	repo := &datasetGateFakeRepo{
		dataset: repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
		run:     domain.Run{ID: uuid.New(), WorkspaceID: uuid.New()},
	}
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	_, err := manager.CreateDatasetBaseline(context.Background(), datasetEvalCaller(workspaceID), CreateDatasetBaselineInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, RunID: repo.run.ID,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateDatasetBaseline() error = %v, want ErrForbidden", err)
	}
}

func TestGetDatasetRegressionSuiteLink(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	suiteID := uuid.New()
	versionID := uuid.New()
	repo := &datasetGateFakeRepo{
		dataset: repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
		regressionLink: repository.DatasetRegressionSuiteLink{
			DatasetID:         datasetID,
			RegressionSuiteID: suiteID,
			SyncedVersionID:   &versionID,
		},
	}
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	link, err := manager.GetDatasetRegressionSuiteLink(context.Background(), datasetEvalCaller(workspaceID), GetDatasetInput{
		WorkspaceID: workspaceID,
		DatasetID:   datasetID,
	})
	if err != nil {
		t.Fatalf("GetDatasetRegressionSuiteLink() error = %v", err)
	}
	if link.RegressionSuiteID != suiteID {
		t.Fatalf("RegressionSuiteID = %s, want %s", link.RegressionSuiteID, suiteID)
	}
}

func TestGetDatasetRegressionSuiteLinkNotFound(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	repo := &datasetGateFakeRepo{
		dataset:           repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
		regressionLinkErr: repository.ErrDatasetRegressionSuiteLinkNotFound,
	}
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)

	_, err := manager.GetDatasetRegressionSuiteLink(context.Background(), datasetEvalCaller(workspaceID), GetDatasetInput{
		WorkspaceID: workspaceID,
		DatasetID:   datasetID,
	})
	if !errors.Is(err, repository.ErrDatasetRegressionSuiteLinkNotFound) {
		t.Fatalf("GetDatasetRegressionSuiteLink() error = %v, want ErrDatasetRegressionSuiteLinkNotFound", err)
	}
}

func TestSyncDatasetRegressionSuiteRequiresManageRegressions(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	repo := &datasetGateFakeRepo{
		dataset: repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
	}
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo)
	caller := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceViewer},
		},
	}

	_, err := manager.SyncDatasetRegressionSuite(context.Background(), caller, SyncDatasetRegressionSuiteInput{
		WorkspaceID:            workspaceID,
		DatasetID:              datasetID,
		VersionID:              uuid.New(),
		ChallengePackVersionID: uuid.New(),
		ChallengeKey:           "case-1",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("SyncDatasetRegressionSuite() error = %v, want ErrForbidden", err)
	}
}

type datasetGateFakeRepo struct {
	dataset           repository.Dataset
	baseline          repository.DatasetBaseline
	candidateOutcomes []datasetgate.ExampleOutcome
	run               domain.Run
	evalRun           repository.DatasetEvalRun
	regressionLink    repository.DatasetRegressionSuiteLink
	regressionLinkErr error
}

func (r *datasetGateFakeRepo) CreateDataset(context.Context, repository.CreateDatasetParams) (repository.Dataset, error) {
	return repository.Dataset{}, nil
}
func (r *datasetGateFakeRepo) GetDatasetByID(context.Context, uuid.UUID) (repository.Dataset, error) {
	return r.dataset, nil
}
func (r *datasetGateFakeRepo) ListDatasetsByWorkspaceID(context.Context, uuid.UUID, int32, int32) ([]repository.Dataset, error) {
	return nil, nil
}
func (r *datasetGateFakeRepo) CountDatasetsByWorkspaceID(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *datasetGateFakeRepo) PatchDataset(context.Context, repository.PatchDatasetParams) (repository.Dataset, error) {
	return repository.Dataset{}, nil
}
func (r *datasetGateFakeRepo) ArchiveDataset(context.Context, uuid.UUID) (repository.Dataset, error) {
	return repository.Dataset{}, nil
}
func (r *datasetGateFakeRepo) UpsertDatasetExample(context.Context, repository.UpsertDatasetExampleParams) (repository.DatasetExample, error) {
	return repository.DatasetExample{}, nil
}
func (r *datasetGateFakeRepo) GetDatasetExampleByID(context.Context, uuid.UUID) (repository.DatasetExample, error) {
	return repository.DatasetExample{}, nil
}
func (r *datasetGateFakeRepo) ListDatasetExamplesByDatasetID(context.Context, repository.ListDatasetExamplesParams) ([]repository.DatasetExample, error) {
	return nil, nil
}
func (r *datasetGateFakeRepo) CountDatasetExamplesByDatasetID(context.Context, uuid.UUID, *domain.DatasetExampleStatus) (int64, error) {
	return 0, nil
}
func (r *datasetGateFakeRepo) PatchDatasetExample(context.Context, repository.PatchDatasetExampleParams) (repository.DatasetExample, error) {
	return repository.DatasetExample{}, nil
}
func (r *datasetGateFakeRepo) CreateDatasetVersion(context.Context, repository.CreateDatasetVersionParams) (repository.DatasetVersion, error) {
	return repository.DatasetVersion{}, nil
}
func (r *datasetGateFakeRepo) ListDatasetVersionsByDatasetID(context.Context, uuid.UUID) ([]repository.DatasetVersion, error) {
	return nil, nil
}
func (r *datasetGateFakeRepo) GetDatasetVersionByID(context.Context, uuid.UUID) (repository.DatasetVersion, error) {
	return repository.DatasetVersion{}, nil
}
func (r *datasetGateFakeRepo) ListDatasetVersionExamples(context.Context, uuid.UUID) ([]repository.DatasetExample, error) {
	return nil, nil
}
func (r *datasetGateFakeRepo) MaterializeDatasetVersionInputSet(context.Context, repository.MaterializeDatasetVersionInputSetParams) (repository.DatasetVersionInputSet, error) {
	return repository.DatasetVersionInputSet{}, nil
}
func (r *datasetGateFakeRepo) ListDatasetEvalResults(context.Context, uuid.UUID, *uuid.UUID, int32, int32) (repository.ListDatasetEvalResultsResult, error) {
	return repository.ListDatasetEvalResultsResult{}, nil
}
func (r *datasetGateFakeRepo) CreateDatasetBaseline(context.Context, repository.CreateDatasetBaselineParams) (repository.DatasetBaseline, error) {
	return r.baseline, nil
}
func (r *datasetGateFakeRepo) ListDatasetBaselines(context.Context, repository.ListDatasetBaselinesParams) (repository.ListDatasetBaselinesResult, error) {
	return repository.ListDatasetBaselinesResult{}, nil
}
func (r *datasetGateFakeRepo) GetDatasetBaselineByID(context.Context, uuid.UUID) (repository.DatasetBaseline, error) {
	return r.baseline, nil
}
func (r *datasetGateFakeRepo) ListDatasetEvalOutcomesForRun(context.Context, uuid.UUID, *uuid.UUID) ([]datasetgate.ExampleOutcome, error) {
	return r.candidateOutcomes, nil
}
func (r *datasetGateFakeRepo) GetRunByID(context.Context, uuid.UUID) (domain.Run, error) {
	return r.run, nil
}
func (r *datasetGateFakeRepo) GetDatasetEvalRunByRunID(context.Context, uuid.UUID) (repository.DatasetEvalRun, error) {
	return r.evalRun, nil
}
func (r *datasetGateFakeRepo) GetDatasetRegressionSuiteLink(context.Context, uuid.UUID) (repository.DatasetRegressionSuiteLink, error) {
	if r.regressionLinkErr != nil {
		return repository.DatasetRegressionSuiteLink{}, r.regressionLinkErr
	}
	return r.regressionLink, nil
}
func (r *datasetGateFakeRepo) SyncDatasetRegressionSuite(context.Context, repository.SyncDatasetRegressionSuiteParams) (repository.SyncDatasetRegressionSuiteResult, error) {
	return repository.SyncDatasetRegressionSuiteResult{}, nil
}

func gateTestJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func intPtrGate(value int) *int { return &value }
