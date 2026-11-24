package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestStartDatasetEvalHandlerReturnsCreateRunResponse(t *testing.T) {
	workspaceID, datasetID, versionID := uuid.New(), uuid.New(), uuid.New()
	packVersionID, inputSetID, runID := uuid.New(), uuid.New(), uuid.New()
	materializedID := uuid.New()
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &datasetEvalFakeRepo{
		dataset:     repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
		version:     repository.DatasetVersion{ID: versionID, DatasetID: datasetID},
		packVersion: repository.RunnableChallengePackVersion{ID: packVersionID, WorkspaceID: &workspaceID},
		materialized: repository.DatasetVersionInputSet{
			ID: materializedID, DatasetID: datasetID, DatasetVersionID: versionID, ChallengeInputSetID: inputSetID,
		},
	}
	run := domain.Run{
		ID: runID, WorkspaceID: workspaceID, ChallengePackVersionID: packVersionID, ChallengeInputSetID: &inputSetID,
		Status: domain.RunStatusQueued, CreatedAt: createdAt,
	}
	manager := NewDatasetManager(datasetEvalAllowAuthorizer{}, repo).WithRunCreationService(&datasetEvalFakeRunCreator{run: run})
	router := chi.NewRouter()
	router.Post("/v1/workspaces/{workspaceID}/datasets/{datasetID}/evals", startDatasetEvalHandler(catalogTestLogger(), manager))

	body := fmt.Sprintf(`{"version_id":%q,"challenge_pack_version_id":%q,"challenge_id":"support","agent_deployment_ids":[%q]}`, versionID, packVersionID, uuid.New())
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/workspaces/%s/datasets/%s/evals", workspaceID, datasetID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, datasetEvalCaller(workspaceID)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var rawResponse map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rawResponse); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"Run", "DatasetVersionInputSet"} {
		if _, exists := rawResponse[key]; exists {
			t.Errorf("unexpected top-level key %q", key)
		}
	}
	rawRun, ok := rawResponse["run"].(map[string]any)
	if !ok {
		t.Fatalf("run = %v, want a JSON object", rawResponse["run"])
	}
	for key, want := range map[string]string{
		"id":           runID.String(),
		"workspace_id": workspaceID.String(),
		"status":       string(run.Status),
		"created_at":   createdAt.Format(time.RFC3339Nano),
	} {
		if got := rawRun[key]; got != want {
			t.Errorf("run.%s = %v, want %q", key, got, want)
		}
	}
	for _, key := range []string{"ID", "WorkspaceID", "Status", "CreatedAt"} {
		if _, exists := rawRun[key]; exists {
			t.Errorf("unexpected run key %q", key)
		}
	}
	rawInputSet, ok := rawResponse["dataset_version_input_set"].(map[string]any)
	if !ok {
		t.Fatalf("dataset_version_input_set = %v, want a JSON object", rawResponse["dataset_version_input_set"])
	}
	for key, want := range map[string]string{
		"id":                     materializedID.String(),
		"challenge_input_set_id": inputSetID.String(),
	} {
		if got := rawInputSet[key]; got != want {
			t.Errorf("dataset_version_input_set.%s = %v, want %q", key, got, want)
		}
	}
	for _, key := range []string{"ID", "ChallengeInputSetID"} {
		if _, exists := rawInputSet[key]; exists {
			t.Errorf("unexpected dataset_version_input_set key %q", key)
		}
	}

	var response struct {
		Run                    map[string]json.RawMessage        `json:"run"`
		DatasetVersionInputSet repository.DatasetVersionInputSet `json:"dataset_version_input_set"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	expectedRun, err := json.Marshal(buildCreateRunResponse(run))
	if err != nil {
		t.Fatal(err)
	}
	var expectedKeys map[string]json.RawMessage
	if err := json.Unmarshal(expectedRun, &expectedKeys); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(response.Run, expectedKeys) {
		t.Fatalf("run = %s, want %s", rec.Body.String(), expectedRun)
	}
	if response.DatasetVersionInputSet.ChallengeInputSetID != inputSetID {
		t.Fatalf("materialized input set = %s, want %s", response.DatasetVersionInputSet.ChallengeInputSetID, inputSetID)
	}
}

func TestDatasetManagerStartDatasetEvalMaterializesAndCreatesRun(t *testing.T) {
	workspaceID := uuid.New()
	datasetID := uuid.New()
	versionID := uuid.New()
	packVersionID := uuid.New()
	inputSetID := uuid.New()
	runID := uuid.New()
	caller := datasetEvalCaller(workspaceID)
	repo := &datasetEvalFakeRepo{
		dataset:     repository.Dataset{ID: datasetID, WorkspaceID: workspaceID, Name: "Golden", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		version:     repository.DatasetVersion{ID: versionID, DatasetID: datasetID, VersionNumber: 1, CreatedAt: time.Now()},
		packVersion: repository.RunnableChallengePackVersion{ID: packVersionID},
		publicPacks: true,
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
	if runCreator.input.DatasetEvalRun == nil {
		t.Fatal("dataset eval run linkage was not passed into run creation")
	}
	if runCreator.input.DatasetEvalRun.DatasetID != datasetID ||
		runCreator.input.DatasetEvalRun.DatasetVersionID != versionID ||
		runCreator.input.DatasetEvalRun.DatasetVersionInputSetID != repo.materialized.ID {
		t.Fatalf("dataset eval linkage = %+v", runCreator.input.DatasetEvalRun)
	}
}

func TestDatasetManagerStartDatasetEvalChecksPackVersionBeforeMaterializing(t *testing.T) {
	workspaceID, otherWorkspaceID := uuid.New(), uuid.New()
	datasetID, versionID, packVersionID := uuid.New(), uuid.New(), uuid.New()
	invisibleVersion := RunCreationValidationError{
		Code:    "invalid_challenge_pack_version_id",
		Message: "challenge_pack_version_id must be visible to the selected workspace",
	}
	for _, tt := range []struct {
		name        string
		packOwner   *uuid.UUID
		publicPacks bool
		wantErr     error
	}{
		{name: "workspace pack", packOwner: &workspaceID},
		{name: "another workspace's pack", packOwner: &otherWorkspaceID, publicPacks: true, wantErr: invisibleVersion},
		{name: "public packs disabled", wantErr: invisibleVersion},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &datasetEvalFakeRepo{
				dataset:     repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
				version:     repository.DatasetVersion{ID: versionID, DatasetID: datasetID},
				packVersion: repository.RunnableChallengePackVersion{ID: packVersionID, WorkspaceID: tt.packOwner},
				publicPacks: tt.publicPacks,
			}
			runCreator := &datasetEvalFakeRunCreator{}
			manager := NewDatasetManager(datasetEvalAllowAuthorizer{}, repo).WithRunCreationService(runCreator)
			_, err := manager.StartDatasetEval(context.Background(), datasetEvalCaller(workspaceID), StartDatasetEvalInput{
				WorkspaceID: workspaceID, DatasetID: datasetID, VersionID: versionID, ChallengePackVersionID: packVersionID,
				ChallengeID: "support", AgentDeploymentIDs: []uuid.UUID{uuid.New()},
			})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("StartDatasetEval error = %v, want %v", err, tt.wantErr)
			}
			wantCreated := tt.wantErr == nil
			if repo.materializeCalled != wantCreated || (runCreator.input.ChallengeInputSetID != nil) != wantCreated {
				t.Fatalf("materialized = %t, called run creation = %t, want %t for both",
					repo.materializeCalled, runCreator.input.ChallengeInputSetID != nil, wantCreated)
			}
		})
	}
}

func TestStartDatasetEvalHandlerRejectsUnknownPackVersion(t *testing.T) {
	workspaceID, datasetID, versionID, packVersionID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &datasetEvalFakeRepo{
		dataset: repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
		version: repository.DatasetVersion{ID: versionID, DatasetID: datasetID},
	}
	runCreator := &datasetEvalFakeRunCreator{}
	manager := NewDatasetManager(NewCallerWorkspaceAuthorizer(), repo).WithRunCreationService(runCreator)
	router := chi.NewRouter()
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/evals", startDatasetEvalHandler(slog.Default(), manager))
	body := fmt.Sprintf(`{"version_id":%q,"challenge_pack_version_id":%q,"challenge_id":"support","agent_deployment_ids":[%q]}`, versionID, packVersionID, uuid.New())
	path := fmt.Sprintf("/workspaces/%s/datasets/%s/evals", workspaceID, datasetID)
	ctx := context.WithValue(t.Context(), callerContextKey{}, datasetEvalCaller(workspaceID))
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	var result errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Error.Code != "invalid_challenge_pack_version_id" ||
		result.Error.Message != "challenge_pack_version_id must reference a runnable challenge pack version" {
		t.Errorf("unexpected response: %s", rec.Body.String())
	}
	if repo.materializeCalled || runCreator.input.ChallengeInputSetID != nil {
		t.Error("started a dataset eval for an unavailable pack version")
	}
}

func TestStartDatasetEvalHandlerRejectsViewerBeforeMaterializing(t *testing.T) {
	workspaceID, datasetID, versionID, packVersionID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &datasetEvalFakeRepo{
		dataset:     repository.Dataset{ID: datasetID, WorkspaceID: workspaceID},
		version:     repository.DatasetVersion{ID: versionID, DatasetID: datasetID},
		packVersion: repository.RunnableChallengePackVersion{ID: packVersionID, WorkspaceID: &workspaceID},
	}
	runCreator := &datasetEvalFakeRunCreator{}
	manager := NewDatasetManager(NewCallerWorkspaceAuthorizer(), repo).WithRunCreationService(runCreator)
	router := chi.NewRouter()
	router.Post("/workspaces/{workspaceID}/datasets/{datasetID}/evals", startDatasetEvalHandler(slog.Default(), manager))
	body := fmt.Sprintf(`{"version_id":%q,"challenge_pack_version_id":%q,"challenge_id":"support","agent_deployment_ids":[%q]}`, versionID, packVersionID, uuid.New())
	path := fmt.Sprintf("/workspaces/%s/datasets/%s/evals", workspaceID, datasetID)
	viewer := Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceViewer},
		},
	}
	ctx := context.WithValue(t.Context(), callerContextKey{}, viewer)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", rec.Code, rec.Body.String())
	}
	if repo.materializeCalled || runCreator.input.ChallengeInputSetID != nil {
		t.Error("a workspace viewer wrote a dataset input set")
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
		results: repository.ListDatasetEvalResultsResult{
			Items: []repository.DatasetEvalResult{{DatasetExampleID: uuid.New(), DatasetVersionID: versionID, RunID: &runID}},
			Total: 1,
			Limit: 50,
		},
	}
	manager := NewDatasetManager(datasetEvalAllowAuthorizer{}, repo)

	results, err := manager.ListDatasetResults(context.Background(), datasetEvalCaller(workspaceID), ListDatasetResultsInput{WorkspaceID: workspaceID, DatasetID: datasetID, VersionID: &versionID})
	if err != nil {
		t.Fatalf("ListDatasetResults() error = %v", err)
	}
	if len(results.Items) != 1 || results.Items[0].RunID == nil || *results.Items[0].RunID != runID {
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
	packVersion       repository.RunnableChallengePackVersion
	publicPacks       bool
	materialized      repository.DatasetVersionInputSet
	results           repository.ListDatasetEvalResultsResult
	materializeParams repository.MaterializeDatasetVersionInputSetParams
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
func (f *datasetEvalFakeRepo) GetRunnableChallengePackVersionByID(_ context.Context, id uuid.UUID) (repository.RunnableChallengePackVersion, error) {
	if f.packVersion.ID != id {
		return repository.RunnableChallengePackVersion{}, repository.ErrChallengePackVersionNotFound
	}
	return f.packVersion, nil
}
func (f *datasetEvalFakeRepo) WorkspacePublicPacksEnabled(_ context.Context, workspaceID uuid.UUID) (bool, error) {
	return f.publicPacks && workspaceID == f.dataset.WorkspaceID, nil
}
func (f *datasetEvalFakeRepo) MaterializeDatasetVersionInputSet(_ context.Context, params repository.MaterializeDatasetVersionInputSetParams) (repository.DatasetVersionInputSet, error) {
	f.materializeCalled = true
	f.materializeParams = params
	return f.materialized, nil
}
func (f *datasetEvalFakeRepo) ListDatasetEvalResults(context.Context, uuid.UUID, *uuid.UUID, int32, int32) (repository.ListDatasetEvalResultsResult, error) {
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
