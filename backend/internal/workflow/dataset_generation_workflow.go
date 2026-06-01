package workflow

import (
	"errors"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

var errNoDatasetGenerationSeeds = errors.New("dataset must have at least one seed example for generation")

const syntheticDatasetGenerationTimeout = 30 * time.Minute

func SyntheticDatasetGenerationWorkflow(ctx sdkworkflow.Context, input SyntheticDatasetGenerationWorkflowInput) error {
	ctx = sdkworkflow.WithActivityOptions(ctx, defaultActivityOptions)

	executionContext, err := loadDatasetGenerationExecutionContext(ctx, input.JobID)
	if err != nil {
		return markDatasetGenerationJobFailed(ctx, input.JobID, err)
	}
	if len(executionContext.Seeds) == 0 {
		return markDatasetGenerationJobFailed(ctx, input.JobID, errNoDatasetGenerationSeeds)
	}

	info := sdkworkflow.GetInfo(ctx)
	if err := setDatasetGenerationJobTemporalIDs(ctx, input.JobID, info.WorkflowExecution.ID, info.WorkflowExecution.RunID); err != nil {
		return markDatasetGenerationJobFailed(ctx, input.JobID, err)
	}

	now := sdkworkflow.Now(ctx).UTC()
	if err := transitionDatasetGenerationJobStatus(ctx, UpdateDatasetGenerationJobStatusInput{
		JobID:     input.JobID,
		Status:    repository.DatasetGenerationStatusRunning,
		StartedAt: &now,
	}); err != nil {
		return markDatasetGenerationJobFailed(ctx, input.JobID, err)
	}

	executeCtx := sdkworkflow.WithActivityOptions(ctx, sdkworkflow.ActivityOptions{
		StartToCloseTimeout: syntheticDatasetGenerationTimeout,
	})
	if err := sdkworkflow.ExecuteActivity(executeCtx, executeSyntheticDatasetGenerationActivityName, ExecuteSyntheticDatasetGenerationInput{
		JobID: input.JobID,
	}).Get(ctx, nil); err != nil {
		return markDatasetGenerationJobFailed(ctx, input.JobID, err)
	}

	finishedAt := sdkworkflow.Now(ctx).UTC()
	return transitionDatasetGenerationJobStatus(ctx, UpdateDatasetGenerationJobStatusInput{
		JobID:      input.JobID,
		Status:     repository.DatasetGenerationStatusCompleted,
		FinishedAt: &finishedAt,
	})
}

func loadDatasetGenerationExecutionContext(ctx sdkworkflow.Context, jobID uuid.UUID) (repository.DatasetGenerationExecutionContext, error) {
	var executionContext repository.DatasetGenerationExecutionContext
	err := sdkworkflow.ExecuteActivity(ctx, loadDatasetGenerationExecutionContextActivityName, LoadDatasetGenerationExecutionContextInput{
		JobID: jobID,
	}).Get(ctx, &executionContext)
	return executionContext, err
}

func setDatasetGenerationJobTemporalIDs(ctx sdkworkflow.Context, jobID uuid.UUID, workflowID, runID string) error {
	var job repository.DatasetGenerationJob
	return sdkworkflow.ExecuteActivity(ctx, setDatasetGenerationJobTemporalIDsActivityName, SetDatasetGenerationJobTemporalIDsInput{
		JobID:              jobID,
		TemporalWorkflowID: workflowID,
		TemporalRunID:      runID,
	}).Get(ctx, &job)
}

func transitionDatasetGenerationJobStatus(ctx sdkworkflow.Context, input UpdateDatasetGenerationJobStatusInput) error {
	var job repository.DatasetGenerationJob
	return sdkworkflow.ExecuteActivity(ctx, updateDatasetGenerationJobStatusActivityName, input).Get(ctx, &job)
}

func markDatasetGenerationJobFailed(ctx sdkworkflow.Context, jobID uuid.UUID, cause error) error {
	failedAt := sdkworkflow.Now(ctx).UTC()
	message := cause.Error()
	_ = transitionDatasetGenerationJobStatus(ctx, UpdateDatasetGenerationJobStatusInput{
		JobID:        jobID,
		Status:       repository.DatasetGenerationStatusFailed,
		ErrorMessage: &message,
		FailedAt:     &failedAt,
	})
	return cause
}
