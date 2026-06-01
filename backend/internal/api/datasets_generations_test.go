package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type datasetGenerationFakeStarter struct{}

func (datasetGenerationFakeStarter) StartSyntheticDatasetGenerationWorkflow(context.Context, uuid.UUID) error {
	return nil
}

type denyWorkspaceAccessAuthorizer struct{}

func (denyWorkspaceAccessAuthorizer) AuthorizeWorkspace(context.Context, Caller, uuid.UUID) error {
	return ErrForbidden
}

type datasetGenerationFakeRepo struct {
	*datasetImportFakeRepo
	providerAccount repository.ProviderAccountRow
	modelAlias      repository.ModelAliasRow
	job             repository.DatasetGenerationJob
}

func (r *datasetGenerationFakeRepo) CreateDatasetGenerationJob(_ context.Context, params repository.CreateDatasetGenerationJobParams) (repository.DatasetGenerationJob, error) {
	r.job = repository.DatasetGenerationJob{
		ID:          uuid.New(),
		DatasetID:   params.DatasetID,
		WorkspaceID: params.WorkspaceID,
		Strategy:    params.Strategy,
		Status:      repository.DatasetGenerationStatusQueued,
		Config:      params.Config,
		TargetCount: params.TargetCount,
		CreatedBy:   params.Actor,
	}
	return r.job, nil
}

func (r *datasetGenerationFakeRepo) GetDatasetGenerationJobByID(_ context.Context, id uuid.UUID) (repository.DatasetGenerationJob, error) {
	if r.job.ID == id {
		return r.job, nil
	}
	return repository.DatasetGenerationJob{}, repository.ErrDatasetGenerationJobNotFound
}

func (r *datasetGenerationFakeRepo) GetProviderAccountByID(_ context.Context, id uuid.UUID) (repository.ProviderAccountRow, error) {
	if r.providerAccount.ID == id {
		return r.providerAccount, nil
	}
	return repository.ProviderAccountRow{}, repository.ErrProviderAccountNotFound
}

func (r *datasetGenerationFakeRepo) GetModelAliasByID(_ context.Context, id uuid.UUID) (repository.ModelAliasRow, error) {
	if r.modelAlias.ID == id {
		return r.modelAlias, nil
	}
	return repository.ModelAliasRow{}, repository.ErrModelAliasNotFound
}

func TestStartDatasetGenerationRequiresManageDatasets(t *testing.T) {
	wsID := uuid.New()
	datasetID := uuid.New()
	repo := &datasetGenerationFakeRepo{
		datasetImportFakeRepo: newDatasetImportFakeRepo(wsID, datasetID),
	}
	manager := NewDatasetManager(denyWorkspaceAccessAuthorizer{}, repo).WithGenerationWorkflowStarter(datasetGenerationFakeStarter{})
	_, err := manager.StartDatasetGeneration(context.Background(), Caller{UserID: uuid.New()}, StartDatasetGenerationInput{
		WorkspaceID:       wsID,
		DatasetID:         datasetID,
		Strategy:          "self_instruct",
		TargetCount:       3,
		ProviderAccountID: uuid.New(),
		ModelAliasID:      uuid.New(),
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestStartDatasetGenerationCreatesJob(t *testing.T) {
	wsID := uuid.New()
	datasetID := uuid.New()
	providerID := uuid.New()
	modelAliasID := uuid.New()
	repo := &datasetGenerationFakeRepo{
		datasetImportFakeRepo: newDatasetImportFakeRepo(wsID, datasetID),
		providerAccount:       repository.ProviderAccountRow{ID: providerID, WorkspaceID: &wsID, ProviderKey: "openai"},
		modelAlias:            repository.ModelAliasRow{ID: modelAliasID, WorkspaceID: &wsID},
	}
	repo.examples = []repository.DatasetExample{{
		ID:        uuid.New(),
		DatasetID: datasetID,
		Input:     json.RawMessage(`{"q":"seed"}`),
		Status:    domain.DatasetExampleStatusActive,
	}}
	manager := NewDatasetManager(allowWorkspaceAuthorizer{}, repo).WithGenerationWorkflowStarter(datasetGenerationFakeStarter{})
	job, err := manager.StartDatasetGeneration(context.Background(), Caller{UserID: uuid.New()}, StartDatasetGenerationInput{
		WorkspaceID:       wsID,
		DatasetID:         datasetID,
		Strategy:          "self-instruct",
		TargetCount:       2,
		ProviderAccountID: providerID,
		ModelAliasID:      modelAliasID,
	})
	if err != nil {
		t.Fatalf("start generation: %v", err)
	}
	if job.TargetCount != 2 {
		t.Fatalf("expected target_count 2, got %d", job.TargetCount)
	}
}
