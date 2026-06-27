package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	datasetgeneration "github.com/agentclash/agentclash/backend/internal/datasets/generation"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestExecuteSyntheticDatasetGenerationAgenticAcceptsJudgeApprovedCandidate(t *testing.T) {
	fixture := newDatasetGenerationActivityFixture(t, []provider.Response{
		{OutputText: `{"input":{"q":"candidate"},"expected":{"a":"ok"}}`, Usage: provider.Usage{InputTokens: 10, OutputTokens: 5}},
		{OutputText: `{"verdict":"accept","quality_verdict":"high","weak_score":0.4,"strong_score":0.8,"gap":0.4,"capability_tags":["schema-following"],"gap_interpretation":"useful separation"}`, Usage: provider.Usage{InputTokens: 11, OutputTokens: 6}},
	})

	if err := fixture.activities.ExecuteSyntheticDatasetGeneration(context.Background(), ExecuteSyntheticDatasetGenerationInput{JobID: fixture.jobID}); err != nil {
		t.Fatalf("execute generation: %v", err)
	}
	if len(fixture.repo.upserts) != 1 {
		t.Fatalf("upserts = %d, want 1", len(fixture.repo.upserts))
	}
	upsert := fixture.repo.upserts[0]
	if !containsTag(upsert.Tags, "synthetic") || !containsTag(upsert.Tags, "agentic") {
		t.Fatalf("tags = %v, want synthetic and agentic", upsert.Tags)
	}
	var metadata map[string]any
	if err := json.Unmarshal(upsert.Metadata, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata["generator"] != datasetgeneration.StrategyAgenticSelfInstruct {
		t.Fatalf("generator = %v", metadata["generator"])
	}
	if metadata["judge_verdict"] != datasetgeneration.JudgeVerdictAccept {
		t.Fatalf("judge_verdict = %v", metadata["judge_verdict"])
	}
	if fixture.repo.progress.TotalInputTokens != 21 || fixture.repo.progress.TotalOutputTokens != 11 {
		t.Fatalf("usage = %d/%d", fixture.repo.progress.TotalInputTokens, fixture.repo.progress.TotalOutputTokens)
	}
}

func TestExecuteSyntheticDatasetGenerationAgenticRunsDirectProviderSolvers(t *testing.T) {
	fixture := newDatasetGenerationActivityFixture(t, []provider.Response{
		{OutputText: `{"input":{"q":"candidate"},"expected":{"a":"ok"}}`, Usage: provider.Usage{InputTokens: 10, OutputTokens: 5}},
		{OutputText: `weak answer`, Usage: provider.Usage{InputTokens: 2, OutputTokens: 3}},
		{OutputText: `strong answer`, Usage: provider.Usage{InputTokens: 4, OutputTokens: 5}},
		{OutputText: `{"verdict":"accept","quality_verdict":"high","weak_score":0.3,"strong_score":0.9,"gap":0.6,"capability_tags":["recovery"],"gap_interpretation":"clear separation"}`, Usage: provider.Usage{InputTokens: 11, OutputTokens: 6}},
	})
	configureDirectProviderSolvers(&fixture)

	if err := fixture.activities.ExecuteSyntheticDatasetGeneration(context.Background(), ExecuteSyntheticDatasetGenerationInput{JobID: fixture.jobID}); err != nil {
		t.Fatalf("execute generation: %v", err)
	}
	if fixture.client.calls != 4 {
		t.Fatalf("provider calls = %d, want 4", fixture.client.calls)
	}
	models := []string{
		fixture.client.requests[0].Model,
		fixture.client.requests[1].Model,
		fixture.client.requests[2].Model,
		fixture.client.requests[3].Model,
	}
	wantModels := []string{"gpt-4.1-mini", "gpt-4.1-nano", "gpt-4.1", "gpt-4.1-mini"}
	for i := range wantModels {
		if models[i] != wantModels[i] {
			t.Fatalf("call %d model = %q, want %q", i, models[i], wantModels[i])
		}
	}
	if strings.Contains(fixture.client.requests[1].Messages[0].Content, `"a":"ok"`) {
		t.Fatalf("weak solver prompt leaked expected answer: %q", fixture.client.requests[1].Messages[0].Content)
	}
	if !strings.Contains(fixture.client.requests[3].Messages[0].Content, "weak answer") || !strings.Contains(fixture.client.requests[3].Messages[0].Content, "strong answer") {
		t.Fatalf("judge prompt missing solver outputs: %q", fixture.client.requests[3].Messages[0].Content)
	}
	if len(fixture.repo.upserts) != 1 {
		t.Fatalf("upserts = %d, want 1", len(fixture.repo.upserts))
	}
	var metadata map[string]any
	if err := json.Unmarshal(fixture.repo.upserts[0].Metadata, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata["solver_mode"] != datasetgeneration.SolverModeDirectProvider {
		t.Fatalf("solver_mode = %v", metadata["solver_mode"])
	}
	if _, ok := metadata["weak_solver_attempts"].([]any); !ok {
		t.Fatalf("weak solver attempts missing from metadata: %+v", metadata)
	}
	var summary map[string]any
	if err := json.Unmarshal(fixture.repo.progress.Summary, &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary["avg_gap"] != 0.6 {
		t.Fatalf("avg_gap = %v, want 0.6", summary["avg_gap"])
	}
}

func TestExecuteSyntheticDatasetGenerationAgenticRecordsSolverFailure(t *testing.T) {
	fixture := newDatasetGenerationActivityFixture(t, []provider.Response{
		{OutputText: `{"input":{"q":"candidate"},"expected":{"a":"ok"}}`},
		{},
	})
	configureDirectProviderSolvers(&fixture)
	fixture.repo.context.Config.MaxRoundsPerExample = 1
	fixture.client.errors = []error{nil, errors.New("weak model down")}

	if err := fixture.activities.ExecuteSyntheticDatasetGeneration(context.Background(), ExecuteSyntheticDatasetGenerationInput{JobID: fixture.jobID}); err != nil {
		t.Fatalf("execute generation: %v", err)
	}
	if len(fixture.repo.upserts) != 0 {
		t.Fatalf("upserts = %d, want 0", len(fixture.repo.upserts))
	}
	if len(fixture.repo.rejections) == 0 {
		t.Fatal("expected at least one rejection")
	}
	rejection := fixture.repo.rejections[0]
	if rejection.ReasonCode != datasetgeneration.ReasonSolverError {
		t.Fatalf("reason = %q", rejection.ReasonCode)
	}
	if rejection.ReasonDetail == nil || !strings.Contains(*rejection.ReasonDetail, "weak model down") {
		t.Fatalf("reason detail = %v", rejection.ReasonDetail)
	}
}

func TestExecuteSyntheticDatasetGenerationAgenticRecordsJudgeRejection(t *testing.T) {
	fixture := newDatasetGenerationActivityFixture(t, []provider.Response{
		{OutputText: `{"input":{"q":"too easy"},"expected":{"a":"ok"}}`},
		{OutputText: `{"verdict":"reject","quality_verdict":"low","gap_interpretation":"too easy","suggestion_for_challenger":"make it require tool recovery"}`},
		{OutputText: `{"input":{"q":"accepted"},"expected":{"a":"ok"}}`},
		{OutputText: `{"verdict":"accept","quality_verdict":"high"}`},
	})

	if err := fixture.activities.ExecuteSyntheticDatasetGeneration(context.Background(), ExecuteSyntheticDatasetGenerationInput{JobID: fixture.jobID}); err != nil {
		t.Fatalf("execute generation: %v", err)
	}
	if len(fixture.repo.upserts) != 1 {
		t.Fatalf("upserts = %d, want 1", len(fixture.repo.upserts))
	}
	if len(fixture.repo.rejections) != 1 {
		t.Fatalf("rejections = %d, want 1", len(fixture.repo.rejections))
	}
	rejection := fixture.repo.rejections[0]
	if rejection.ReasonCode != datasetgeneration.ReasonQualityRejected {
		t.Fatalf("reason = %q", rejection.ReasonCode)
	}
	if rejection.ReasonDetail == nil || !strings.Contains(*rejection.ReasonDetail, "tool recovery") {
		t.Fatalf("reason detail = %v", rejection.ReasonDetail)
	}
}

func TestExecuteSyntheticDatasetGenerationAgenticRecordsMalformedJudgeOutput(t *testing.T) {
	fixture := newDatasetGenerationActivityFixture(t, []provider.Response{
		{OutputText: `{"input":{"q":"bad judge"},"expected":{"a":"ok"}}`},
		{OutputText: `not json`},
		{OutputText: `{"input":{"q":"accepted"},"expected":{"a":"ok"}}`},
		{OutputText: `{"verdict":"accept","quality_verdict":"high"}`},
	})

	if err := fixture.activities.ExecuteSyntheticDatasetGeneration(context.Background(), ExecuteSyntheticDatasetGenerationInput{JobID: fixture.jobID}); err != nil {
		t.Fatalf("execute generation: %v", err)
	}
	if len(fixture.repo.upserts) != 1 {
		t.Fatalf("upserts = %d, want 1", len(fixture.repo.upserts))
	}
	if len(fixture.repo.rejections) != 1 {
		t.Fatalf("rejections = %d, want 1", len(fixture.repo.rejections))
	}
	if fixture.repo.rejections[0].ReasonCode != datasetgeneration.ReasonJudgeParseError {
		t.Fatalf("reason = %q", fixture.repo.rejections[0].ReasonCode)
	}
	if fixture.repo.rejections[0].ReasonDetail == nil || !strings.Contains(*fixture.repo.rejections[0].ReasonDetail, "decode agentic judge response") {
		t.Fatalf("reason detail = %v", fixture.repo.rejections[0].ReasonDetail)
	}
}

type datasetGenerationActivityFixture struct {
	jobID      uuid.UUID
	repo       *fakeDatasetGenerationWorkflowRepo
	client     *scriptedDatasetGenerationProvider
	activities *DatasetGenerationActivities
}

func newDatasetGenerationActivityFixture(t *testing.T, responses []provider.Response) datasetGenerationActivityFixture {
	t.Helper()
	workspaceID := uuid.New()
	datasetID := uuid.New()
	jobID := uuid.New()
	providerAccountID := uuid.New()
	judgeProviderAccountID := uuid.New()
	createdBy := uuid.New()
	cfg := datasetgeneration.JobConfig{
		ProviderAccountID:      providerAccountID,
		Model:                  "gpt-4.1-mini",
		JudgeProviderAccountID: &judgeProviderAccountID,
		JudgeModel:             "gpt-4.1-mini",
		MaxRoundsPerExample:    3,
		AcceptanceMode:         datasetgeneration.AcceptanceModeJudge,
	}
	config, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	providerAccount := repository.ProviderAccountRow{
		ID:                  providerAccountID,
		WorkspaceID:         &workspaceID,
		ProviderKey:         "openai",
		CredentialReference: "secret://challenger",
	}
	judgeProviderAccount := repository.ProviderAccountRow{
		ID:                  judgeProviderAccountID,
		WorkspaceID:         &workspaceID,
		ProviderKey:         "openai",
		CredentialReference: "secret://judge",
	}
	repo := &fakeDatasetGenerationWorkflowRepo{
		context: repository.DatasetGenerationExecutionContext{
			Job: repository.DatasetGenerationJob{
				ID:             jobID,
				DatasetID:      datasetID,
				WorkspaceID:    workspaceID,
				Strategy:       datasetgeneration.StrategyAgenticSelfInstruct,
				Config:         config,
				TargetCount:    1,
				GeneratedCount: 0,
				AcceptedCount:  0,
				RejectedCount:  0,
				CreatedBy:      createdBy,
			},
			Dataset: repository.Dataset{
				ID:                  datasetID,
				WorkspaceID:         workspaceID,
				InputSchema:         json.RawMessage(`{"type":"object"}`),
				InputSchemaEnforced: false,
			},
			Config:               cfg,
			Seeds:                []datasetgeneration.SeedExample{{Input: json.RawMessage(`{"q":"seed"}`), Expected: json.RawMessage(`{"a":"seed"}`)}},
			ExistingInputs:       map[string]struct{}{},
			ProviderAccount:      providerAccount,
			JudgeProviderAccount: &judgeProviderAccount,
			Model:                cfg.Model,
		},
	}
	client := &scriptedDatasetGenerationProvider{responses: responses}
	return datasetGenerationActivityFixture{
		jobID:      jobID,
		repo:       repo,
		client:     client,
		activities: NewDatasetGenerationActivities(repo, client, nil),
	}
}

func configureDirectProviderSolvers(fixture *datasetGenerationActivityFixture) {
	workspaceID := fixture.repo.context.Job.WorkspaceID
	weakProviderAccountID := uuid.New()
	strongProviderAccountID := uuid.New()
	fixture.repo.context.Config.SolverMode = datasetgeneration.SolverModeDirectProvider
	fixture.repo.context.Config.WeakProviderAccountID = &weakProviderAccountID
	fixture.repo.context.Config.WeakModel = "gpt-4.1-nano"
	fixture.repo.context.Config.WeakRollouts = 1
	fixture.repo.context.Config.StrongProviderAccountID = &strongProviderAccountID
	fixture.repo.context.Config.StrongModel = "gpt-4.1"
	fixture.repo.context.Config.StrongRollouts = 1
	fixture.repo.context.WeakProviderAccount = &repository.ProviderAccountRow{
		ID:                  weakProviderAccountID,
		WorkspaceID:         &workspaceID,
		ProviderKey:         "openai",
		CredentialReference: "secret://weak",
	}
	fixture.repo.context.StrongProviderAccount = &repository.ProviderAccountRow{
		ID:                  strongProviderAccountID,
		WorkspaceID:         &workspaceID,
		ProviderKey:         "openai",
		CredentialReference: "secret://strong",
	}
}

type scriptedDatasetGenerationProvider struct {
	responses []provider.Response
	errors    []error
	requests  []provider.Request
	calls     int
}

func (c *scriptedDatasetGenerationProvider) InvokeModel(_ context.Context, request provider.Request) (provider.Response, error) {
	c.requests = append(c.requests, request)
	if c.calls >= len(c.responses) {
		c.calls++
		return provider.Response{}, nil
	}
	response := c.responses[c.calls]
	var err error
	if c.calls < len(c.errors) {
		err = c.errors[c.calls]
	}
	c.calls++
	return response, err
}

type fakeDatasetGenerationWorkflowRepo struct {
	context    repository.DatasetGenerationExecutionContext
	progress   repository.UpdateDatasetGenerationJobProgressParams
	rejections []repository.CreateDatasetGenerationRejectionParams
	upserts    []repository.UpsertDatasetExampleParams
}

func (r *fakeDatasetGenerationWorkflowRepo) GetDatasetGenerationExecutionContextByID(context.Context, uuid.UUID) (repository.DatasetGenerationExecutionContext, error) {
	return r.context, nil
}

func (r *fakeDatasetGenerationWorkflowRepo) SetDatasetGenerationJobTemporalIDs(context.Context, repository.SetDatasetGenerationJobTemporalIDsParams) (repository.DatasetGenerationJob, error) {
	return r.context.Job, nil
}

func (r *fakeDatasetGenerationWorkflowRepo) UpdateDatasetGenerationJobStatus(context.Context, repository.UpdateDatasetGenerationJobStatusParams) (repository.DatasetGenerationJob, error) {
	return r.context.Job, nil
}

func (r *fakeDatasetGenerationWorkflowRepo) UpdateDatasetGenerationJobProgress(_ context.Context, params repository.UpdateDatasetGenerationJobProgressParams) (repository.DatasetGenerationJob, error) {
	r.progress = params
	r.context.Job.GeneratedCount = params.GeneratedCount
	r.context.Job.AcceptedCount = params.AcceptedCount
	r.context.Job.RejectedCount = params.RejectedCount
	r.context.Job.TotalInputTokens = params.TotalInputTokens
	r.context.Job.TotalOutputTokens = params.TotalOutputTokens
	r.context.Job.TotalCostUSD = params.TotalCostUSD
	return r.context.Job, nil
}

func (r *fakeDatasetGenerationWorkflowRepo) CreateDatasetGenerationRejection(_ context.Context, params repository.CreateDatasetGenerationRejectionParams) (repository.DatasetGenerationRejection, error) {
	r.rejections = append(r.rejections, params)
	return repository.DatasetGenerationRejection{
		ID:             uuid.New(),
		JobID:          params.JobID,
		ReasonCode:     params.ReasonCode,
		ReasonDetail:   params.ReasonDetail,
		CandidateInput: params.CandidateInput,
		Metadata:       params.Metadata,
		CreatedAt:      time.Now(),
	}, nil
}

func (r *fakeDatasetGenerationWorkflowRepo) UpsertDatasetExample(_ context.Context, params repository.UpsertDatasetExampleParams) (repository.DatasetExample, error) {
	r.upserts = append(r.upserts, params)
	return repository.DatasetExample{
		ID:        uuid.New(),
		DatasetID: params.DatasetID,
		Input:     params.Input,
		Expected:  params.Expected,
		Metadata:  params.Metadata,
		Tags:      params.Tags,
		Status:    domain.DatasetExampleStatusActive,
		Source:    domain.DatasetExampleSourceSynthetic,
		CreatedBy: params.Actor,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (r *fakeDatasetGenerationWorkflowRepo) CreateDatasetVersion(context.Context, repository.CreateDatasetVersionParams) (repository.DatasetVersion, error) {
	return repository.DatasetVersion{ID: uuid.New(), DatasetID: r.context.Dataset.ID}, nil
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}
