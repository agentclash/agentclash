package workflow

import (
	"errors"
	"fmt"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	enumspb "go.temporal.io/api/enums/v1"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

var ErrEvalSessionHasNoRuns = errors.New("eval session must have at least one run")

func EvalSessionWorkflow(ctx sdkworkflow.Context, input EvalSessionWorkflowInput) error {
	ctx = sdkworkflow.WithActivityOptions(ctx, defaultActivityOptions)

	err := runEvalSessionWorkflow(ctx, input)
	if err == nil {
		return nil
	}

	if isWorkflowCanceled(err) {
		return markEvalSessionCancelled(ctx, input.EvalSessionID, err)
	}
	if shouldSkipEvalSessionFailureTransition(err) {
		return err
	}

	return markEvalSessionFailed(ctx, input.EvalSessionID, err)
}

func runEvalSessionWorkflow(ctx sdkworkflow.Context, input EvalSessionWorkflowInput) error {
	session, err := loadEvalSession(ctx, input.EvalSessionID)
	if err != nil {
		return err
	}
	if err := validateEvalSessionQueued(session); err != nil {
		return err
	}

	if err := transitionEvalSessionStatus(ctx, input.EvalSessionID, domain.EvalSessionStatusRunning); err != nil {
		return err
	}

	runs, err := listEvalSessionRuns(ctx, input.EvalSessionID)
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		return fmt.Errorf("%w: eval session %s", ErrEvalSessionHasNoRuns, input.EvalSessionID)
	}

	if err := executeEvalSessionRuns(ctx, runs); err != nil {
		return err
	}

	if err := transitionEvalSessionStatus(ctx, input.EvalSessionID, domain.EvalSessionStatusAggregating); err != nil {
		return err
	}
	if err := aggregateEvalSession(ctx, input.EvalSessionID); err != nil {
		return err
	}

	return transitionEvalSessionStatus(ctx, input.EvalSessionID, domain.EvalSessionStatusCompleted)
}

func loadEvalSession(ctx sdkworkflow.Context, evalSessionID uuid.UUID) (domain.EvalSession, error) {
	var session domain.EvalSession
	err := sdkworkflow.ExecuteActivity(ctx, loadEvalSessionActivityName, LoadEvalSessionInput{
		EvalSessionID: evalSessionID,
	}).Get(ctx, &session)
	return session, err
}

func listEvalSessionRuns(ctx sdkworkflow.Context, evalSessionID uuid.UUID) ([]domain.Run, error) {
	var runs []domain.Run
	err := sdkworkflow.ExecuteActivity(ctx, listEvalSessionRunsActivityName, ListEvalSessionRunsInput{
		EvalSessionID: evalSessionID,
	}).Get(ctx, &runs)
	return runs, err
}

func transitionEvalSessionStatus(ctx sdkworkflow.Context, evalSessionID uuid.UUID, toStatus domain.EvalSessionStatus) error {
	var session domain.EvalSession
	return sdkworkflow.ExecuteActivity(ctx, transitionEvalSessionStatusActivityName, TransitionEvalSessionStatusInput{
		EvalSessionID: evalSessionID,
		ToStatus:      toStatus,
	}).Get(ctx, &session)
}

func aggregateEvalSession(ctx sdkworkflow.Context, evalSessionID uuid.UUID) error {
	var aggregateResult repository.EvalSessionAggregateRecord
	return sdkworkflow.ExecuteActivity(ctx, aggregateEvalSessionActivityName, AggregateEvalSessionInput{
		EvalSessionID: evalSessionID,
	}).Get(ctx, &aggregateResult)
}

func executeEvalSessionRuns(ctx sdkworkflow.Context, runs []domain.Run) error {
	selector := sdkworkflow.NewSelector(ctx)
	completedChildren := 0
	childErrors := make(map[uuid.UUID]error, len(runs))

	for _, run := range runs {
		run := run
		childCtx := sdkworkflow.WithChildOptions(ctx, sdkworkflow.ChildWorkflowOptions{
			WorkflowID:        fmt.Sprintf("%s/%s", RunWorkflowName, run.ID),
			ParentClosePolicy: enumspb.PARENT_CLOSE_POLICY_REQUEST_CANCEL,
		})
		future := sdkworkflow.ExecuteChildWorkflow(childCtx, RunWorkflowName, RunWorkflowInput{
			RunID: run.ID,
		})
		selector.AddFuture(future, func(f sdkworkflow.Future) {
			completedChildren++
			if err := f.Get(ctx, nil); err != nil {
				childErrors[run.ID] = err
			}
		})
	}

	for completedChildren < len(runs) {
		selector.Select(ctx)
	}

	if len(childErrors) == len(runs) {
		for _, err := range childErrors {
			return err
		}
	}

	return nil
}

func markEvalSessionFailed(ctx sdkworkflow.Context, evalSessionID uuid.UUID, workflowErr error) error {
	activityErr := transitionEvalSessionStatus(ctx, evalSessionID, domain.EvalSessionStatusFailed)
	if activityErr != nil {
		return fmt.Errorf("eval session workflow failed: %v; additionally failed to mark eval session failed: %w", workflowErr, activityErr)
	}

	return workflowErr
}

func markEvalSessionCancelled(ctx sdkworkflow.Context, evalSessionID uuid.UUID, workflowErr error) error {
	disconnectedCtx, _ := sdkworkflow.NewDisconnectedContext(ctx)
	disconnectedCtx = sdkworkflow.WithActivityOptions(disconnectedCtx, defaultActivityOptions)

	activityErr := transitionEvalSessionStatus(disconnectedCtx, evalSessionID, domain.EvalSessionStatusCancelled)
	if activityErr != nil {
		return fmt.Errorf("eval session workflow cancelled: %v; additionally failed to mark eval session cancelled: %w", workflowErr, activityErr)
	}

	return workflowErr
}

func shouldSkipEvalSessionFailureTransition(err error) bool {
	return errors.Is(err, ErrEvalSessionMustBeQueued) ||
		hasApplicationErrorType(err, repositoryEvalSessionNotFoundErrorType) ||
		hasApplicationErrorType(err, repositoryIllegalSessionTransitionType)
}
