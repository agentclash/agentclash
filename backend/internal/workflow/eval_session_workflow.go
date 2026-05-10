package workflow

import (
	"errors"
	"fmt"
	"sort"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	enumspb "go.temporal.io/api/enums/v1"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

var ErrEvalSessionHasNoRuns = errors.New("eval session must have at least one run")
var errEvalSessionChildrenCancelled = errors.New("eval session child runs cancelled")

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
	runs, err = loadLatestEvalSessionRuns(ctx, runs)
	if err != nil {
		return err
	}
	if allRunsCancelled(runs) {
		return transitionEvalSessionStatus(ctx, input.EvalSessionID, domain.EvalSessionStatusCancelled)
	}

	if err := executeEvalSessionRuns(ctx, runs); err != nil {
		if errors.Is(err, errEvalSessionChildrenCancelled) {
			return transitionEvalSessionStatus(ctx, input.EvalSessionID, domain.EvalSessionStatusCancelled)
		}
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

func loadLatestEvalSessionRuns(ctx sdkworkflow.Context, runs []domain.Run) ([]domain.Run, error) {
	latestRuns := make([]domain.Run, 0, len(runs))
	for _, run := range runs {
		latest, err := loadRun(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		latestRuns = append(latestRuns, latest)
	}
	return latestRuns, nil
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
	startedChildren := 0
	childErrors := make(map[uuid.UUID]error, len(runs))

	for _, run := range runs {
		run := run
		if run.Status != domain.RunStatusQueued {
			continue
		}
		childCtx := sdkworkflow.WithChildOptions(ctx, sdkworkflow.ChildWorkflowOptions{
			WorkflowID:        fmt.Sprintf("%s/%s", RunWorkflowName, run.ID),
			ParentClosePolicy: enumspb.PARENT_CLOSE_POLICY_REQUEST_CANCEL,
		})
		future := sdkworkflow.ExecuteChildWorkflow(childCtx, RunWorkflowName, RunWorkflowInput{
			RunID: run.ID,
		})
		startedChildren++
		selector.AddFuture(future, func(f sdkworkflow.Future) {
			completedChildren++
			if err := f.Get(ctx, nil); err != nil {
				childErrors[run.ID] = err
			}
		})
	}

	for completedChildren < startedChildren {
		selector.Select(ctx)
	}

	actionableErrors, cancelledChildren, err := classifyEvalSessionChildErrors(ctx, childErrors)
	if err != nil {
		return err
	}
	if startedChildren > 0 && cancelledChildren == startedChildren {
		return errEvalSessionChildrenCancelled
	}
	successfulChildren := startedChildren - len(childErrors)
	if startedChildren > 0 && successfulChildren == 0 && len(actionableErrors) > 0 {
		return actionableErrors[0]
	}

	return nil
}

func classifyEvalSessionChildErrors(ctx sdkworkflow.Context, childErrors map[uuid.UUID]error) ([]error, int, error) {
	runIDs := sortedUUIDKeys(childErrors)
	actionableErrors := make([]error, 0, len(childErrors))
	cancelledChildren := 0
	for _, runID := range runIDs {
		childErr := childErrors[runID]
		if childRunMayAlreadyBeTerminal(childErr) {
			latest, loadErr := loadRun(ctx, runID)
			if loadErr != nil {
				return nil, 0, loadErr
			}
			if !latest.Status.CanTransitionTo(domain.RunStatusCancelled) {
				if latest.Status == domain.RunStatusCancelled {
					cancelledChildren++
				}
				continue
			}
		}
		actionableErrors = append(actionableErrors, childErr)
	}
	return actionableErrors, cancelledChildren, nil
}

func sortedUUIDKeys(values map[uuid.UUID]error) []uuid.UUID {
	keys := make([]uuid.UUID, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})
	return keys
}

func childRunMayAlreadyBeTerminal(err error) bool {
	return errors.Is(err, ErrRunMustBeQueued) ||
		hasApplicationErrorType(err, runMustBeQueuedErrorType) ||
		hasApplicationErrorType(err, repositoryInvalidTransitionType) ||
		hasApplicationErrorType(err, repositoryTransitionConflictType)
}

func allRunsCancelled(runs []domain.Run) bool {
	if len(runs) == 0 {
		return false
	}
	for _, run := range runs {
		if run.Status != domain.RunStatusCancelled {
			return false
		}
	}
	return true
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
