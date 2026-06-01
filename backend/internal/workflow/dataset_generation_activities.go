package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	datasetgeneration "github.com/agentclash/agentclash/backend/internal/datasets/generation"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

const (
	loadDatasetGenerationExecutionContextActivityName = "workflow.load_dataset_generation_execution_context"
	setDatasetGenerationJobTemporalIDsActivityName    = "workflow.set_dataset_generation_job_temporal_ids"
	updateDatasetGenerationJobStatusActivityName    = "workflow.update_dataset_generation_job_status"
	executeSyntheticDatasetGenerationActivityName     = "workflow.execute_synthetic_dataset_generation"
)

type DatasetGenerationWorkflowRepository interface {
	GetDatasetGenerationExecutionContextByID(ctx context.Context, jobID uuid.UUID) (repository.DatasetGenerationExecutionContext, error)
	SetDatasetGenerationJobTemporalIDs(ctx context.Context, params repository.SetDatasetGenerationJobTemporalIDsParams) (repository.DatasetGenerationJob, error)
	UpdateDatasetGenerationJobStatus(ctx context.Context, params repository.UpdateDatasetGenerationJobStatusParams) (repository.DatasetGenerationJob, error)
	UpdateDatasetGenerationJobProgress(ctx context.Context, params repository.UpdateDatasetGenerationJobProgressParams) (repository.DatasetGenerationJob, error)
	CreateDatasetGenerationRejection(ctx context.Context, params repository.CreateDatasetGenerationRejectionParams) (repository.DatasetGenerationRejection, error)
	UpsertDatasetExample(ctx context.Context, params repository.UpsertDatasetExampleParams) (repository.DatasetExample, error)
	CreateDatasetVersion(ctx context.Context, params repository.CreateDatasetVersionParams) (repository.DatasetVersion, error)
}

type DatasetGenerationSecretsLookup interface {
	LoadWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error)
}

type DatasetGenerationActivities struct {
	repo          DatasetGenerationWorkflowRepository
	client        provider.Client
	secretsLookup DatasetGenerationSecretsLookup
}

type LoadDatasetGenerationExecutionContextInput struct {
	JobID uuid.UUID `json:"job_id"`
}

type SetDatasetGenerationJobTemporalIDsInput struct {
	JobID              uuid.UUID `json:"job_id"`
	TemporalWorkflowID string    `json:"temporal_workflow_id"`
	TemporalRunID      string    `json:"temporal_run_id"`
}

type UpdateDatasetGenerationJobStatusInput struct {
	JobID        uuid.UUID                          `json:"job_id"`
	Status       repository.DatasetGenerationStatus `json:"status"`
	Summary      json.RawMessage                    `json:"summary,omitempty"`
	VersionID    *uuid.UUID                         `json:"version_id,omitempty"`
	ErrorMessage *string                            `json:"error_message,omitempty"`
	StartedAt    *time.Time                         `json:"started_at,omitempty"`
	FinishedAt   *time.Time                         `json:"finished_at,omitempty"`
	FailedAt     *time.Time                         `json:"failed_at,omitempty"`
}

type ExecuteSyntheticDatasetGenerationInput struct {
	JobID uuid.UUID `json:"job_id"`
}

func NewDatasetGenerationActivities(repo DatasetGenerationWorkflowRepository, client provider.Client, secretsLookup DatasetGenerationSecretsLookup) *DatasetGenerationActivities {
	return &DatasetGenerationActivities{repo: repo, client: client, secretsLookup: secretsLookup}
}

func (a *DatasetGenerationActivities) LoadDatasetGenerationExecutionContext(ctx context.Context, input LoadDatasetGenerationExecutionContextInput) (repository.DatasetGenerationExecutionContext, error) {
	executionContext, err := a.repo.GetDatasetGenerationExecutionContextByID(ctx, input.JobID)
	return executionContext, wrapActivityError(err)
}

func (a *DatasetGenerationActivities) SetDatasetGenerationJobTemporalIDs(ctx context.Context, input SetDatasetGenerationJobTemporalIDsInput) (repository.DatasetGenerationJob, error) {
	job, err := a.repo.SetDatasetGenerationJobTemporalIDs(ctx, repository.SetDatasetGenerationJobTemporalIDsParams{
		ID:                 input.JobID,
		TemporalWorkflowID: input.TemporalWorkflowID,
		TemporalRunID:      input.TemporalRunID,
	})
	return job, wrapActivityError(err)
}

func (a *DatasetGenerationActivities) UpdateDatasetGenerationJobStatus(ctx context.Context, input UpdateDatasetGenerationJobStatusInput) (repository.DatasetGenerationJob, error) {
	job, err := a.repo.UpdateDatasetGenerationJobStatus(ctx, repository.UpdateDatasetGenerationJobStatusParams{
		ID:           input.JobID,
		Status:       input.Status,
		Summary:      cloneRawJSON(input.Summary),
		VersionID:    input.VersionID,
		ErrorMessage: input.ErrorMessage,
		StartedAt:    cloneTimePtr(input.StartedAt),
		FinishedAt:   cloneTimePtr(input.FinishedAt),
		FailedAt:     cloneTimePtr(input.FailedAt),
	})
	return job, wrapActivityError(err)
}

func (a *DatasetGenerationActivities) ExecuteSyntheticDatasetGeneration(ctx context.Context, input ExecuteSyntheticDatasetGenerationInput) error {
	executionContext, err := a.repo.GetDatasetGenerationExecutionContextByID(ctx, input.JobID)
	if err != nil {
		return wrapActivityError(err)
	}
	if len(executionContext.Seeds) == 0 {
		return wrapActivityError(fmt.Errorf("dataset must have at least one seed example for generation"))
	}

	if a.secretsLookup != nil {
		secrets, secretErr := a.secretsLookup.LoadWorkspaceSecrets(ctx, executionContext.Job.WorkspaceID)
		if secretErr != nil {
			return wrapActivityError(fmt.Errorf("load workspace secrets: %w", secretErr))
		}
		ctx = provider.WithWorkspaceSecrets(ctx, secrets)
	}

	rng := rand.New(rand.NewSource(int64(executionContext.Job.ID[0])<<56 | int64(executionContext.Job.ID[15])))
	acceptedHashes := make(map[string]struct{}, len(executionContext.ExistingInputs)+int(executionContext.Job.TargetCount))
	for hash := range executionContext.ExistingInputs {
		acceptedHashes[hash] = struct{}{}
	}

	var generatedCount int32
	var acceptedCount int32
	var rejectedCount int32
	var totalInputTokens int64
	var totalOutputTokens int64
	var totalCostUSD float64
	maxAttempts := int(executionContext.Job.TargetCount) * 5
	if maxAttempts < 10 {
		maxAttempts = 10
	}

	for attempt := 0; attempt < maxAttempts && acceptedCount < executionContext.Job.TargetCount; attempt++ {
		seedBatch := pickSeedBatch(executionContext.Seeds, rng, 3)
		prompt := datasetgeneration.BuildSelfInstructPrompt(seedBatch, int(executionContext.Job.TargetCount))
		response, invokeErr := a.client.InvokeModel(ctx, provider.Request{
			ProviderKey:         executionContext.ProviderAccount.ProviderKey,
			ProviderAccountID:   executionContext.ProviderAccount.ID.String(),
			CredentialReference: executionContext.ProviderAccount.CredentialReference,
			Model:               executionContext.ModelCatalog.ProviderModelID,
			TraceMode:           "required",
			StepTimeout:         120 * time.Second,
			Messages:            []provider.Message{{Role: "user", Content: prompt}},
			Metadata: mustMarshalJSON(map[string]any{
				"dataset_generation_job_id": executionContext.Job.ID,
				"dataset_id":                executionContext.Dataset.ID,
				"strategy":                  executionContext.Job.Strategy,
			}),
		})
		generatedCount++
		if invokeErr != nil {
			rejectedCount++
			if _, rejectErr := a.recordRejection(ctx, input.JobID, datasetgeneration.ReasonProviderError, invokeErr.Error(), nil, nil); rejectErr != nil {
				return wrapActivityError(rejectErr)
			}
			continue
		}
		totalInputTokens += response.Usage.InputTokens
		totalOutputTokens += response.Usage.OutputTokens
		totalCostUSD += datasetgeneration.ComputeCostUSD(response.Usage.InputTokens, response.Usage.OutputTokens, datasetgeneration.ModelPricing{
			InputCostPerMillionTokens:  executionContext.ModelCatalog.InputCostPerMillionTokens,
			OutputCostPerMillionTokens: executionContext.ModelCatalog.OutputCostPerMillionTokens,
		})

		candidate, parseErr := datasetgeneration.ParseSelfInstructResponse(response.OutputText)
		if parseErr != nil {
			rejectedCount++
			if _, rejectErr := a.recordRejection(ctx, input.JobID, datasetgeneration.ReasonParseError, parseErr.Error(), nil, nil); rejectErr != nil {
				return wrapActivityError(rejectErr)
			}
			continue
		}
		if schemaErr := datasetgeneration.ValidateCandidateInput(executionContext.Dataset.InputSchema, executionContext.Dataset.InputSchemaEnforced, candidate.Input); schemaErr != nil {
			rejectedCount++
			if _, rejectErr := a.recordRejection(ctx, input.JobID, datasetgeneration.ReasonSchemaViolation, schemaErr.Error(), candidate.Input, candidate.Expected); rejectErr != nil {
				return wrapActivityError(rejectErr)
			}
			continue
		}
		hash, hashErr := datasetgeneration.CanonicalInputHash(candidate.Input)
		if hashErr != nil {
			rejectedCount++
			if _, rejectErr := a.recordRejection(ctx, input.JobID, datasetgeneration.ReasonParseError, hashErr.Error(), candidate.Input, candidate.Expected); rejectErr != nil {
				return wrapActivityError(rejectErr)
			}
			continue
		}
		if _, exists := acceptedHashes[hash]; exists {
			rejectedCount++
			if _, rejectErr := a.recordRejection(ctx, input.JobID, datasetgeneration.ReasonDuplicateInput, "duplicate input", candidate.Input, candidate.Expected); rejectErr != nil {
				return wrapActivityError(rejectErr)
			}
			continue
		}

		externalID := fmt.Sprintf("gen:%s:%d", executionContext.Job.ID, acceptedCount+1)
		metadata := mustMarshalJSON(map[string]any{
			"generator":           executionContext.Job.Strategy,
			"generation_job_id":     executionContext.Job.ID,
			"model_alias_id":        executionContext.Config.ModelAliasID,
			"provider_account_id":   executionContext.Config.ProviderAccountID,
			"provider_model_id":     executionContext.ModelCatalog.ProviderModelID,
		})
		if _, upsertErr := a.repo.UpsertDatasetExample(ctx, repository.UpsertDatasetExampleParams{
			DatasetID:  executionContext.Dataset.ID,
			ExternalID: &externalID,
			Input:      candidate.Input,
			Expected:   candidate.Expected,
			Metadata:   metadata,
			Tags:       []string{"synthetic"},
			Status:     domain.DatasetExampleStatusActive,
			Source:     domain.DatasetExampleSourceSynthetic,
			Actor:      executionContext.Job.CreatedBy,
		}); upsertErr != nil {
			return wrapActivityError(upsertErr)
		}
		acceptedHashes[hash] = struct{}{}
		acceptedCount++

		if _, progressErr := a.repo.UpdateDatasetGenerationJobProgress(ctx, repository.UpdateDatasetGenerationJobProgressParams{
			ID:                input.JobID,
			GeneratedCount:    generatedCount,
			AcceptedCount:     acceptedCount,
			RejectedCount:     rejectedCount,
			TotalInputTokens:  totalInputTokens,
			TotalOutputTokens: totalOutputTokens,
			TotalCostUSD:      totalCostUSD,
		}); progressErr != nil {
			return wrapActivityError(progressErr)
		}
	}

	summary := map[string]any{
		"strategy": executionContext.Job.Strategy,
		"attempts": generatedCount,
	}
	if acceptedCount < executionContext.Job.TargetCount {
		summary["partial"] = true
	}

	var versionID *uuid.UUID
	if executionContext.Config.CreateVersion && acceptedCount > 0 {
		label := strings.TrimSpace(executionContext.Config.VersionLabel)
		if label == "" {
			label = fmt.Sprintf("generation-%s", executionContext.Job.ID.String()[:8])
		}
		labelCopy := label
		version, versionErr := a.repo.CreateDatasetVersion(ctx, repository.CreateDatasetVersionParams{
			DatasetID: executionContext.Dataset.ID,
			Label:     &labelCopy,
			Actor:     executionContext.Job.CreatedBy,
		})
		if versionErr != nil {
			return wrapActivityError(versionErr)
		}
		versionID = &version.ID
	}

	_, err = a.repo.UpdateDatasetGenerationJobProgress(ctx, repository.UpdateDatasetGenerationJobProgressParams{
		ID:                input.JobID,
		GeneratedCount:    generatedCount,
		AcceptedCount:     acceptedCount,
		RejectedCount:     rejectedCount,
		TotalInputTokens:  totalInputTokens,
		TotalOutputTokens: totalOutputTokens,
		TotalCostUSD:      totalCostUSD,
		Summary:           mustMarshalJSON(summary),
	})
	if err != nil {
		return wrapActivityError(err)
	}
	if versionID != nil {
		_, err = a.repo.UpdateDatasetGenerationJobStatus(ctx, repository.UpdateDatasetGenerationJobStatusParams{
			ID:        input.JobID,
			Status:    repository.DatasetGenerationStatusRunning,
			VersionID: versionID,
			Summary:   mustMarshalJSON(summary),
		})
	}
	return wrapActivityError(err)
}

func (a *DatasetGenerationActivities) recordRejection(ctx context.Context, jobID uuid.UUID, reasonCode, detail string, input, expected json.RawMessage) (repository.DatasetGenerationRejection, error) {
	detailCopy := detail
	return a.repo.CreateDatasetGenerationRejection(ctx, repository.CreateDatasetGenerationRejectionParams{
		JobID:             jobID,
		ReasonCode:        reasonCode,
		ReasonDetail:      &detailCopy,
		CandidateInput:    input,
		CandidateExpected: expected,
	})
}

func pickSeedBatch(seeds []datasetgeneration.SeedExample, rng *rand.Rand, size int) []datasetgeneration.SeedExample {
	if len(seeds) <= size {
		return append([]datasetgeneration.SeedExample(nil), seeds...)
	}
	picked := make([]datasetgeneration.SeedExample, 0, size)
	indices := rng.Perm(len(seeds))
	for i := 0; i < size; i++ {
		picked = append(picked, seeds[indices[i]])
	}
	return picked
}
