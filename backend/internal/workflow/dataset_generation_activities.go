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
	"go.temporal.io/sdk/activity"
)

const (
	loadDatasetGenerationExecutionContextActivityName = "workflow.load_dataset_generation_execution_context"
	setDatasetGenerationJobTemporalIDsActivityName    = "workflow.set_dataset_generation_job_temporal_ids"
	updateDatasetGenerationJobStatusActivityName      = "workflow.update_dataset_generation_job_status"
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

type agenticJudgeUsage struct {
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}

type agenticSolverRolloutResult struct {
	WeakAttempts   []datasetgeneration.AgenticSolverAttempt
	StrongAttempts []datasetgeneration.AgenticSolverAttempt
	Usage          agenticJudgeUsage
}

type agenticJudgeParseError struct {
	err error
}

func (e agenticJudgeParseError) Error() string {
	return e.err.Error()
}

func (e agenticJudgeParseError) Unwrap() error {
	return e.err
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

	rng := rand.New(rand.NewSource(datasetGenerationRNGSeed(executionContext.Job.ID)))
	acceptedHashes := make(map[string]struct{}, len(executionContext.ExistingInputs)+int(executionContext.Job.TargetCount))
	for hash := range executionContext.ExistingInputs {
		acceptedHashes[hash] = struct{}{}
	}

	var generatedCount = executionContext.Job.GeneratedCount
	var acceptedCount = executionContext.Job.AcceptedCount
	var rejectedCount = executionContext.Job.RejectedCount
	var totalInputTokens = executionContext.Job.TotalInputTokens
	var totalOutputTokens = executionContext.Job.TotalOutputTokens
	var totalCostUSD = executionContext.Job.TotalCostUSD
	rejectionReasonCounts := make(map[string]int)
	var weakScoreTotal float64
	var strongScoreTotal float64
	var gapTotal float64
	var scoreObservations int
	maxAttempts := int(executionContext.Job.TargetCount) * 5
	if executionContext.Job.Strategy == datasetgeneration.StrategyAgenticSelfInstruct && executionContext.Config.MaxRoundsPerExample > 0 {
		maxAttempts = int(executionContext.Job.TargetCount) * executionContext.Config.MaxRoundsPerExample
	}
	if maxAttempts < 10 {
		maxAttempts = 10
	}

	for attempt := 0; attempt < maxAttempts && acceptedCount < executionContext.Job.TargetCount; attempt++ {
		recordDatasetGenerationHeartbeat(ctx, map[string]any{
			"attempt":  attempt + 1,
			"accepted": acceptedCount,
			"target":   executionContext.Job.TargetCount,
		})
		seedBatch := pickSeedBatch(executionContext.Seeds, rng, 3)
		prompt := datasetgeneration.BuildSelfInstructPrompt(seedBatch, int(executionContext.Job.TargetCount))
		response, invokeErr := a.client.InvokeModel(ctx, provider.Request{
			ProviderKey:         executionContext.ProviderAccount.ProviderKey,
			ProviderAccountID:   executionContext.ProviderAccount.ID.String(),
			CredentialReference: executionContext.ProviderAccount.CredentialReference,
			Model:               executionContext.Model,
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
			rejectionReasonCounts[datasetgeneration.ReasonProviderError]++
			if _, rejectErr := a.recordRejection(ctx, input.JobID, datasetgeneration.ReasonProviderError, invokeErr.Error(), nil, nil); rejectErr != nil {
				return wrapActivityError(rejectErr)
			}
			continue
		}
		totalInputTokens += response.Usage.InputTokens
		totalOutputTokens += response.Usage.OutputTokens
		totalCostUSD += datasetgeneration.ComputeCostUSD(response.Usage.InputTokens, response.Usage.OutputTokens, datasetgeneration.ModelPricing{
			InputCostPerMillionTokens:  executionContext.InputCostPerMillionTokens,
			OutputCostPerMillionTokens: executionContext.OutputCostPerMillionTokens,
		})

		candidate, parseErr := datasetgeneration.ParseSelfInstructResponse(response.OutputText)
		if parseErr != nil {
			rejectedCount++
			rejectionReasonCounts[datasetgeneration.ReasonParseError]++
			if _, rejectErr := a.recordRejection(ctx, input.JobID, datasetgeneration.ReasonParseError, parseErr.Error(), nil, nil); rejectErr != nil {
				return wrapActivityError(rejectErr)
			}
			continue
		}
		if schemaErr := datasetgeneration.ValidateCandidateInput(executionContext.Dataset.InputSchema, executionContext.Dataset.InputSchemaEnforced, candidate.Input); schemaErr != nil {
			rejectedCount++
			rejectionReasonCounts[datasetgeneration.ReasonSchemaViolation]++
			if _, rejectErr := a.recordRejection(ctx, input.JobID, datasetgeneration.ReasonSchemaViolation, schemaErr.Error(), candidate.Input, candidate.Expected); rejectErr != nil {
				return wrapActivityError(rejectErr)
			}
			continue
		}
		hash, hashErr := datasetgeneration.CanonicalInputHash(candidate.Input)
		if hashErr != nil {
			rejectedCount++
			rejectionReasonCounts[datasetgeneration.ReasonParseError]++
			if _, rejectErr := a.recordRejection(ctx, input.JobID, datasetgeneration.ReasonParseError, hashErr.Error(), candidate.Input, candidate.Expected); rejectErr != nil {
				return wrapActivityError(rejectErr)
			}
			continue
		}
		if _, exists := acceptedHashes[hash]; exists {
			rejectedCount++
			rejectionReasonCounts[datasetgeneration.ReasonDuplicateInput]++
			if _, rejectErr := a.recordRejection(ctx, input.JobID, datasetgeneration.ReasonDuplicateInput, "duplicate input", candidate.Input, candidate.Expected); rejectErr != nil {
				return wrapActivityError(rejectErr)
			}
			continue
		}

		var judgeVerdict *datasetgeneration.AgenticJudgeVerdict
		var weakAttempts []datasetgeneration.AgenticSolverAttempt
		var strongAttempts []datasetgeneration.AgenticSolverAttempt
		if executionContext.Job.Strategy == datasetgeneration.StrategyAgenticSelfInstruct {
			if datasetgeneration.NormalizeAgenticSolverMode(executionContext.Config.SolverMode) == datasetgeneration.SolverModeDirectProvider {
				solverResult, solverErr := a.runAgenticSolverRollouts(ctx, executionContext, candidate)
				if solverResult.Usage.InputTokens > 0 || solverResult.Usage.OutputTokens > 0 {
					totalInputTokens += solverResult.Usage.InputTokens
					totalOutputTokens += solverResult.Usage.OutputTokens
					totalCostUSD += solverResult.Usage.CostUSD
				}
				weakAttempts = solverResult.WeakAttempts
				strongAttempts = solverResult.StrongAttempts
				if solverErr != nil {
					rejectedCount++
					rejectionReasonCounts[datasetgeneration.ReasonSolverError]++
					if _, rejectErr := a.recordRejectionWithMetadata(ctx, input.JobID, datasetgeneration.ReasonSolverError, solverErr.Error(), candidate.Input, candidate.Expected, mustMarshalJSON(map[string]any{
						"solver_mode":     executionContext.Config.SolverMode,
						"weak_attempts":   weakAttempts,
						"strong_attempts": strongAttempts,
					})); rejectErr != nil {
						return wrapActivityError(rejectErr)
					}
					continue
				}
			}
			verdict, usage, judgeErr := a.judgeAgenticCandidate(ctx, executionContext, seedBatch, candidate, weakAttempts, strongAttempts)
			if usage.InputTokens > 0 || usage.OutputTokens > 0 {
				totalInputTokens += usage.InputTokens
				totalOutputTokens += usage.OutputTokens
				totalCostUSD += usage.CostUSD
			}
			if judgeErr != nil {
				rejectedCount++
				reasonCode := datasetgeneration.ReasonProviderError
				if _, ok := judgeErr.(agenticJudgeParseError); ok {
					reasonCode = datasetgeneration.ReasonJudgeParseError
				}
				rejectionReasonCounts[reasonCode]++
				if _, rejectErr := a.recordRejectionWithMetadata(ctx, input.JobID, reasonCode, judgeErr.Error(), candidate.Input, candidate.Expected, mustMarshalJSON(map[string]any{
					"role":            "judge",
					"weak_attempts":   weakAttempts,
					"strong_attempts": strongAttempts,
				})); rejectErr != nil {
					return wrapActivityError(rejectErr)
				}
				continue
			}
			if verdict == nil {
				rejectedCount++
				rejectionReasonCounts[datasetgeneration.ReasonJudgeParseError]++
				if _, rejectErr := a.recordRejection(ctx, input.JobID, datasetgeneration.ReasonJudgeParseError, "missing judge verdict", candidate.Input, candidate.Expected); rejectErr != nil {
					return wrapActivityError(rejectErr)
				}
				continue
			}
			if verdict.WeakScore != nil && verdict.StrongScore != nil && verdict.Gap != nil {
				weakScoreTotal += *verdict.WeakScore
				strongScoreTotal += *verdict.StrongScore
				gapTotal += *verdict.Gap
				scoreObservations++
			}
			if !datasetgeneration.ShouldAcceptJudgeVerdict(*verdict, datasetgeneration.AgenticAcceptanceConfig{
				Mode:           agenticAcceptanceMode(executionContext.Config.AcceptanceMode),
				MinGap:         executionContext.Config.MinGap,
				MaxWeakScore:   executionContext.Config.MaxWeakScore,
				MinStrongScore: executionContext.Config.MinStrongScore,
			}) {
				rejectedCount++
				rejectionReasonCounts[datasetgeneration.ReasonQualityRejected]++
				if _, rejectErr := a.recordRejectionWithMetadata(ctx, input.JobID, datasetgeneration.ReasonQualityRejected, datasetgeneration.AgenticJudgeRejectionDetail(*verdict), candidate.Input, candidate.Expected, datasetgeneration.AgenticJudgeMetadataWithSolvers(*verdict, weakAttempts, strongAttempts)); rejectErr != nil {
					return wrapActivityError(rejectErr)
				}
				continue
			}
			judgeVerdict = verdict
		}

		externalID := fmt.Sprintf("gen:%s:%s", executionContext.Job.ID, hash)
		exampleMetadata := map[string]any{
			"generator":           executionContext.Job.Strategy,
			"generation_job_id":   executionContext.Job.ID,
			"provider_account_id": executionContext.Config.ProviderAccountID,
			"provider_model_id":   executionContext.Model,
		}
		tags := []string{"synthetic"}
		if judgeVerdict != nil {
			exampleMetadata["judge_provider_account_id"] = executionContext.Config.JudgeProviderAccountID
			exampleMetadata["judge_model_id"] = executionContext.Config.JudgeModel
			exampleMetadata["judge_verdict"] = judgeVerdict.Verdict
			exampleMetadata["judge_quality_verdict"] = judgeVerdict.QualityVerdict
			exampleMetadata["weak_score"] = judgeVerdict.WeakScore
			exampleMetadata["strong_score"] = judgeVerdict.StrongScore
			exampleMetadata["gap"] = judgeVerdict.Gap
			exampleMetadata["capability_tags"] = judgeVerdict.CapabilityTags
			exampleMetadata["judge_summary"] = judgeVerdict.GapInterpretation
			exampleMetadata["solver_mode"] = datasetgeneration.NormalizeAgenticSolverMode(executionContext.Config.SolverMode)
			if len(weakAttempts) > 0 {
				exampleMetadata["weak_provider_account_id"] = executionContext.Config.WeakProviderAccountID
				exampleMetadata["weak_model_id"] = executionContext.Config.WeakModel
				exampleMetadata["weak_rollouts"] = executionContext.Config.WeakRollouts
				exampleMetadata["weak_solver_attempts"] = weakAttempts
			}
			if len(strongAttempts) > 0 {
				exampleMetadata["strong_provider_account_id"] = executionContext.Config.StrongProviderAccountID
				exampleMetadata["strong_model_id"] = executionContext.Config.StrongModel
				exampleMetadata["strong_rollouts"] = executionContext.Config.StrongRollouts
				exampleMetadata["strong_solver_attempts"] = strongAttempts
			}
			if datasetgeneration.HasAgenticDeploymentContext(executionContext.Config) {
				exampleMetadata["weak_deployment_id"] = executionContext.Config.WeakDeploymentID
				exampleMetadata["strong_deployment_id"] = executionContext.Config.StrongDeploymentID
				exampleMetadata["challenge_pack_version_id"] = executionContext.Config.ChallengePackVersionID
				exampleMetadata["challenge_key"] = executionContext.Config.ChallengeKey
				if len(executionContext.Config.FieldMapping) > 0 {
					exampleMetadata["field_mapping"] = json.RawMessage(executionContext.Config.FieldMapping)
				}
			}
			tags = append(tags, "agentic")
		}
		metadata := mustMarshalJSON(exampleMetadata)
		if _, upsertErr := a.repo.UpsertDatasetExample(ctx, repository.UpsertDatasetExampleParams{
			DatasetID:  executionContext.Dataset.ID,
			ExternalID: &externalID,
			Input:      candidate.Input,
			Expected:   candidate.Expected,
			Metadata:   metadata,
			Tags:       tags,
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
		"strategy":       executionContext.Job.Strategy,
		"solver_mode":    datasetgeneration.NormalizeAgenticSolverMode(executionContext.Config.SolverMode),
		"attempts":       generatedCount,
		"accepted_count": acceptedCount,
		"rejected_count": rejectedCount,
	}
	if len(rejectionReasonCounts) > 0 {
		summary["rejection_reasons"] = rejectionReasonCounts
	}
	if scoreObservations > 0 {
		summary["score_observations"] = scoreObservations
		summary["avg_weak_score"] = weakScoreTotal / float64(scoreObservations)
		summary["avg_strong_score"] = strongScoreTotal / float64(scoreObservations)
		summary["avg_gap"] = gapTotal / float64(scoreObservations)
	}
	if datasetgeneration.HasAgenticDeploymentContext(executionContext.Config) {
		summary["deployment_context"] = map[string]any{
			"weak_deployment_id":        executionContext.Config.WeakDeploymentID,
			"strong_deployment_id":      executionContext.Config.StrongDeploymentID,
			"challenge_pack_version_id": executionContext.Config.ChallengePackVersionID,
			"challenge_key":             executionContext.Config.ChallengeKey,
			"field_mapping":             json.RawMessage(executionContext.Config.FieldMapping),
		}
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
		VersionID:         versionID,
	})
	return wrapActivityError(err)
}

func (a *DatasetGenerationActivities) recordRejection(ctx context.Context, jobID uuid.UUID, reasonCode, detail string, input, expected json.RawMessage) (repository.DatasetGenerationRejection, error) {
	return a.recordRejectionWithMetadata(ctx, jobID, reasonCode, detail, input, expected, nil)
}

func (a *DatasetGenerationActivities) recordRejectionWithMetadata(ctx context.Context, jobID uuid.UUID, reasonCode, detail string, input, expected, metadata json.RawMessage) (repository.DatasetGenerationRejection, error) {
	detailCopy := detail
	return a.repo.CreateDatasetGenerationRejection(ctx, repository.CreateDatasetGenerationRejectionParams{
		JobID:             jobID,
		ReasonCode:        reasonCode,
		ReasonDetail:      &detailCopy,
		CandidateInput:    input,
		CandidateExpected: expected,
		Metadata:          metadata,
	})
}

func (a *DatasetGenerationActivities) runAgenticSolverRollouts(ctx context.Context, executionContext repository.DatasetGenerationExecutionContext, candidate datasetgeneration.Candidate) (agenticSolverRolloutResult, error) {
	weakAttempts, weakUsage, weakErr := a.invokeAgenticSolverRollouts(ctx, executionContext, "weak_solver", executionContext.WeakProviderAccount, executionContext.Config.WeakModel, executionContext.Config.WeakRollouts, candidate)
	result := agenticSolverRolloutResult{
		WeakAttempts: weakAttempts,
		Usage:        weakUsage,
	}
	if weakErr != nil {
		return result, weakErr
	}

	strongAttempts, strongUsage, strongErr := a.invokeAgenticSolverRollouts(ctx, executionContext, "strong_solver", executionContext.StrongProviderAccount, executionContext.Config.StrongModel, executionContext.Config.StrongRollouts, candidate)
	result.StrongAttempts = strongAttempts
	result.Usage.InputTokens += strongUsage.InputTokens
	result.Usage.OutputTokens += strongUsage.OutputTokens
	result.Usage.CostUSD += strongUsage.CostUSD
	if strongErr != nil {
		return result, strongErr
	}
	return result, nil
}

func (a *DatasetGenerationActivities) invokeAgenticSolverRollouts(ctx context.Context, executionContext repository.DatasetGenerationExecutionContext, role string, account *repository.ProviderAccountRow, model string, rolloutCount int, candidate datasetgeneration.Candidate) ([]datasetgeneration.AgenticSolverAttempt, agenticJudgeUsage, error) {
	if account == nil {
		return nil, agenticJudgeUsage{}, fmt.Errorf("%s provider account is required", role)
	}
	if rolloutCount <= 0 {
		rolloutCount = 1
	}
	attempts := make([]datasetgeneration.AgenticSolverAttempt, 0, rolloutCount)
	var usage agenticJudgeUsage
	for i := 0; i < rolloutCount; i++ {
		prompt := datasetgeneration.BuildAgenticSolverPrompt(role, candidate)
		response, invokeErr := a.client.InvokeModel(ctx, provider.Request{
			ProviderKey:         account.ProviderKey,
			ProviderAccountID:   account.ID.String(),
			CredentialReference: account.CredentialReference,
			Model:               model,
			TraceMode:           "required",
			StepTimeout:         120 * time.Second,
			Messages:            []provider.Message{{Role: "user", Content: prompt}},
			Metadata: mustMarshalJSON(map[string]any{
				"dataset_generation_job_id": executionContext.Job.ID,
				"dataset_id":                executionContext.Dataset.ID,
				"strategy":                  executionContext.Job.Strategy,
				"role":                      role,
				"attempt":                   i + 1,
			}),
		})
		if response.Usage.InputTokens > 0 || response.Usage.OutputTokens > 0 {
			inputCost, outputCost, _ := provider.StaticModelPrice(account.ProviderKey, model)
			usage.InputTokens += response.Usage.InputTokens
			usage.OutputTokens += response.Usage.OutputTokens
			usage.CostUSD += datasetgeneration.ComputeCostUSD(response.Usage.InputTokens, response.Usage.OutputTokens, datasetgeneration.ModelPricing{
				InputCostPerMillionTokens:  inputCost,
				OutputCostPerMillionTokens: outputCost,
			})
		}
		if invokeErr != nil {
			return attempts, usage, fmt.Errorf("%s attempt %d: %w", role, i+1, invokeErr)
		}
		attempts = append(attempts, datasetgeneration.AgenticSolverAttempt{
			Role:              role,
			Attempt:           i + 1,
			ProviderAccountID: account.ID.String(),
			Model:             model,
			Output:            strings.TrimSpace(response.OutputText),
		})
	}
	return attempts, usage, nil
}

func (a *DatasetGenerationActivities) judgeAgenticCandidate(ctx context.Context, executionContext repository.DatasetGenerationExecutionContext, seedBatch []datasetgeneration.SeedExample, candidate datasetgeneration.Candidate, weakAttempts, strongAttempts []datasetgeneration.AgenticSolverAttempt) (*datasetgeneration.AgenticJudgeVerdict, agenticJudgeUsage, error) {
	if executionContext.JudgeProviderAccount == nil {
		return nil, agenticJudgeUsage{}, fmt.Errorf("judge provider account is required")
	}
	judgeAccount := *executionContext.JudgeProviderAccount
	prompt := datasetgeneration.BuildAgenticJudgePrompt(datasetgeneration.AgenticJudgeInput{
		Seeds:          seedBatch,
		Candidate:      candidate,
		WeakAttempts:   weakAttempts,
		StrongAttempts: strongAttempts,
	})
	response, invokeErr := a.client.InvokeModel(ctx, provider.Request{
		ProviderKey:         judgeAccount.ProviderKey,
		ProviderAccountID:   judgeAccount.ID.String(),
		CredentialReference: judgeAccount.CredentialReference,
		Model:               executionContext.Config.JudgeModel,
		TraceMode:           "required",
		StepTimeout:         120 * time.Second,
		Messages:            []provider.Message{{Role: "user", Content: prompt}},
		Metadata: mustMarshalJSON(map[string]any{
			"dataset_generation_job_id": executionContext.Job.ID,
			"dataset_id":                executionContext.Dataset.ID,
			"strategy":                  executionContext.Job.Strategy,
			"role":                      "judge",
		}),
	})
	if invokeErr != nil {
		return nil, agenticJudgeUsage{}, invokeErr
	}
	inputCost, outputCost, _ := provider.StaticModelPrice(judgeAccount.ProviderKey, executionContext.Config.JudgeModel)
	usage := agenticJudgeUsage{
		InputTokens:  response.Usage.InputTokens,
		OutputTokens: response.Usage.OutputTokens,
		CostUSD: datasetgeneration.ComputeCostUSD(response.Usage.InputTokens, response.Usage.OutputTokens, datasetgeneration.ModelPricing{
			InputCostPerMillionTokens:  inputCost,
			OutputCostPerMillionTokens: outputCost,
		}),
	}
	verdict, parseErr := datasetgeneration.ParseAgenticJudgeResponse(response.OutputText)
	if parseErr != nil {
		return nil, usage, agenticJudgeParseError{err: parseErr}
	}
	return &verdict, usage, nil
}

func agenticAcceptanceMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return datasetgeneration.AcceptanceModeJudge
	}
	return strings.TrimSpace(mode)
}

func recordDatasetGenerationHeartbeat(ctx context.Context, details any) {
	defer func() {
		_ = recover()
	}()
	activity.RecordHeartbeat(ctx, details)
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

func datasetGenerationRNGSeed(jobID uuid.UUID) int64 {
	idBytes := jobID
	return int64(idBytes[0])<<56 | int64(idBytes[1])<<48 | int64(idBytes[2])<<40 | int64(idBytes[3])<<32 |
		int64(idBytes[4])<<24 | int64(idBytes[5])<<16 | int64(idBytes[6])<<8 | int64(idBytes[7])
}
