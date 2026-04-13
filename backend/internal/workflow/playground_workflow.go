package workflow

import (
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

const playgroundExecutionConcurrency = 5
const playgroundFinalizeTimeout = 10 * time.Second
const playgroundCaseTimeout = 120 * time.Second

func PlaygroundExperimentWorkflow(ctx sdkworkflow.Context, input PlaygroundExperimentWorkflowInput) error {
	ctx = sdkworkflow.WithActivityOptions(ctx, defaultActivityOptions)

	executionContext, err := loadPlaygroundExperimentExecutionContext(ctx, input.ExperimentID)
	if err != nil {
		return markPlaygroundExperimentFailed(ctx, input.ExperimentID, err)
	}

	info := sdkworkflow.GetInfo(ctx)
	if err := setPlaygroundExperimentTemporalIDs(ctx, input.ExperimentID, info.WorkflowExecution.ID, info.WorkflowExecution.RunID); err != nil {
		return markPlaygroundExperimentFailed(ctx, input.ExperimentID, err)
	}

	now := sdkworkflow.Now(ctx).UTC()
	if err := transitionPlaygroundExperimentStatus(ctx, UpdatePlaygroundExperimentStatusInput{
		ExperimentID: input.ExperimentID,
		Status:       repository.PlaygroundExperimentStatusRunning,
		Summary:      emptyJSONObject(),
		StartedAt:    &now,
	}); err != nil {
		return markPlaygroundExperimentFailed(ctx, input.ExperimentID, err)
	}

	if err := executePlaygroundTestCases(ctx, input.ExperimentID, executionContext.TestCases); err != nil {
		return markPlaygroundExperimentFailed(ctx, input.ExperimentID, err)
	}

	if err := finalizePlaygroundExperiment(ctx, input.ExperimentID); err != nil {
		return markPlaygroundExperimentFailed(ctx, input.ExperimentID, err)
	}

	return nil
}

func loadPlaygroundExperimentExecutionContext(ctx sdkworkflow.Context, experimentID uuid.UUID) (repository.PlaygroundExperimentExecutionContext, error) {
	var executionContext repository.PlaygroundExperimentExecutionContext
	err := sdkworkflow.ExecuteActivity(ctx, loadPlaygroundExperimentExecutionContextActivityName, LoadPlaygroundExperimentExecutionContextInput{
		ExperimentID: experimentID,
	}).Get(ctx, &executionContext)
	return executionContext, err
}

func setPlaygroundExperimentTemporalIDs(ctx sdkworkflow.Context, experimentID uuid.UUID, workflowID string, runID string) error {
	var experiment repository.PlaygroundExperiment
	return sdkworkflow.ExecuteActivity(ctx, setPlaygroundExperimentTemporalIDsActivityName, SetPlaygroundExperimentTemporalIDsInput{
		ExperimentID:       experimentID,
		TemporalWorkflowID: workflowID,
		TemporalRunID:      runID,
	}).Get(ctx, &experiment)
}

func transitionPlaygroundExperimentStatus(ctx sdkworkflow.Context, input UpdatePlaygroundExperimentStatusInput) error {
	var experiment repository.PlaygroundExperiment
	return sdkworkflow.ExecuteActivity(ctx, updatePlaygroundExperimentStatusActivityName, input).Get(ctx, &experiment)
}

func executePlaygroundTestCases(ctx sdkworkflow.Context, experimentID uuid.UUID, testCases []repository.PlaygroundTestCase) error {
	if len(testCases) == 0 {
		return nil
	}

	executeCtx := sdkworkflow.WithActivityOptions(ctx, sdkworkflow.ActivityOptions{
		StartToCloseTimeout: playgroundCaseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	})

	selector := sdkworkflow.NewSelector(ctx)
	inFlight := 0
	nextIndex := 0
	var firstErr error

	launch := func(testCase repository.PlaygroundTestCase) {
		inFlight++
		future := sdkworkflow.ExecuteActivity(executeCtx, executePlaygroundTestCaseActivityName, ExecutePlaygroundTestCaseInput{
			ExperimentID:         experimentID,
			PlaygroundTestCaseID: testCase.ID,
		})
		selector.AddFuture(future, func(f sdkworkflow.Future) {
			inFlight--
			var ignored struct{}
			if err := f.Get(ctx, &ignored); err != nil && firstErr == nil {
				firstErr = err
			}
		})
	}

	for nextIndex < len(testCases) && inFlight < playgroundExecutionConcurrency {
		launch(testCases[nextIndex])
		nextIndex++
	}

	for inFlight > 0 {
		selector.Select(ctx)
		if firstErr != nil {
			return firstErr
		}
		for nextIndex < len(testCases) && inFlight < playgroundExecutionConcurrency {
			launch(testCases[nextIndex])
			nextIndex++
		}
	}

	return firstErr
}

func finalizePlaygroundExperiment(ctx sdkworkflow.Context, experimentID uuid.UUID) error {
	finalizeCtx := sdkworkflow.WithActivityOptions(ctx, sdkworkflow.ActivityOptions{
		StartToCloseTimeout: playgroundFinalizeTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	})
	var experiment repository.PlaygroundExperiment
	return sdkworkflow.ExecuteActivity(finalizeCtx, finalizePlaygroundExperimentActivityName, FinalizePlaygroundExperimentInput{
		ExperimentID: experimentID,
	}).Get(ctx, &experiment)
}

func markPlaygroundExperimentFailed(ctx sdkworkflow.Context, experimentID uuid.UUID, cause error) error {
	now := sdkworkflow.Now(ctx).UTC()
	updateErr := transitionPlaygroundExperimentStatus(ctx, UpdatePlaygroundExperimentStatusInput{
		ExperimentID: experimentID,
		Status:       repository.PlaygroundExperimentStatusFailed,
		Summary:      mustMarshalJSON(map[string]any{"error": cause.Error()}),
		FailedAt:     &now,
	})
	if updateErr != nil {
		return updateErr
	}
	return cause
}

func emptyJSONObject() []byte {
	return []byte(`{}`)
}
