package workflow

import (
	"errors"
	"fmt"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

const (
	defaultActivityTimeout = 5 * time.Second
	scoreRunAgentTimeout   = 30 * time.Second
	// judgeRunAgentTimeout caps the LLM-as-judge fan-out per agent.
	// Multi-sample × multi-model judges can run several minutes when
	// providers are slow; 5 minutes is the safety ceiling. Phase 4 of
	// issue #148 introduces this activity; Phase 6 will revisit when
	// run-level n_wise needs an even longer ceiling.
	judgeRunAgentTimeout = 5 * time.Minute
	fakeStageDelay       = 1 * time.Second
)

var defaultActivityOptions = sdkworkflow.ActivityOptions{
	StartToCloseTimeout: defaultActivityTimeout,
	RetryPolicy: &temporal.RetryPolicy{
		MaximumAttempts: 1,
	},
}

func RunWorkflow(ctx sdkworkflow.Context, input RunWorkflowInput) error {
	ctx = sdkworkflow.WithActivityOptions(ctx, defaultActivityOptions)

	err := runWorkflow(ctx, input)
	if err == nil {
		return nil
	}

	if isWorkflowCanceled(err) {
		return markRunCancelled(ctx, input.RunID, err)
	}
	if shouldSkipRunFailureTransition(err) {
		return err
	}

	return markRunFailed(ctx, input.RunID, err)
}

func runWorkflow(ctx sdkworkflow.Context, input RunWorkflowInput) error {
	run, err := loadRun(ctx, input.RunID)
	if err != nil {
		return err
	}
	if err := validateRunQueued(run); err != nil {
		return err
	}

	runAgents, err := listRunAgents(ctx, input.RunID)
	if err != nil {
		return err
	}
	if len(runAgents) == 0 {
		return fmt.Errorf("%w: run %s", ErrRunHasNoAgents, input.RunID)
	}

	info := sdkworkflow.GetInfo(ctx)
	if err := attachRunTemporalIDs(ctx, input.RunID, info.WorkflowExecution.ID, info.WorkflowExecution.RunID); err != nil {
		return err
	}
	if err := transitionRunStatus(ctx, input.RunID, domain.RunStatusProvisioning, stringPtr("run workflow provisioning started")); err != nil {
		return err
	}
	if err := transitionRunStatus(ctx, input.RunID, domain.RunStatusRunning, stringPtr("run workflow launched run-agent children")); err != nil {
		return err
	}

	if err := executeRunAgents(ctx, runAgents); err != nil {
		return err
	}

	if err := transitionRunStatus(ctx, input.RunID, domain.RunStatusScoring, stringPtr("all run-agent workflows completed")); err != nil {
		return err
	}
	updatedRunAgents, err := listRunAgents(ctx, input.RunID)
	if err != nil {
		return err
	}
	scoreSummary, err := scoreEvaluatingRunAgents(ctx, updatedRunAgents)
	if err != nil {
		return err
	}
	if err := transitionRunStatus(ctx, input.RunID, domain.RunStatusCompleted, stringPtr(scoreSummary)); err != nil {
		return err
	}

	return nil
}

func executeRunAgents(ctx sdkworkflow.Context, runAgents []domain.RunAgent) error {
	selector := sdkworkflow.NewSelector(ctx)
	completedChildren := 0
	childErrors := make(map[uuid.UUID]error, len(runAgents))

	for _, runAgent := range runAgents {
		childCtx := sdkworkflow.WithChildOptions(ctx, sdkworkflow.ChildWorkflowOptions{
			WorkflowID:        fmt.Sprintf("%s/%s/%s", RunAgentWorkflowName, runAgent.RunID, runAgent.ID),
			ParentClosePolicy: enumspb.PARENT_CLOSE_POLICY_REQUEST_CANCEL,
		})

		runAgent := runAgent
		future := sdkworkflow.ExecuteChildWorkflow(childCtx, RunAgentWorkflowName, RunAgentWorkflowInput{
			RunID:      runAgent.RunID,
			RunAgentID: runAgent.ID,
		})
		selector.AddFuture(future, func(f sdkworkflow.Future) {
			completedChildren++
			if err := f.Get(ctx, nil); err != nil {
				childErrors[runAgent.ID] = err
			}
		})
	}

	for completedChildren < len(runAgents) {
		selector.Select(ctx)
	}

	if len(childErrors) == len(runAgents) {
		for _, err := range childErrors {
			return err
		}
	}

	return nil
}

func scoreEvaluatingRunAgents(ctx sdkworkflow.Context, runAgents []domain.RunAgent) (string, error) {
	outcomes := make(map[uuid.UUID]string, len(runAgents))
	completedRunAgents := make([]domain.RunAgent, 0, len(runAgents))
	for _, runAgent := range runAgents {
		switch runAgent.Status {
		case domain.RunAgentStatusEvaluating:
			completedRunAgents = append(completedRunAgents, runAgent)
		default:
			outcomes[runAgent.ID] = "skipped"
		}
	}

	if len(completedRunAgents) == 0 {
		return summarizeScoreOutcomes(outcomes), nil
	}

	scoreCtx := sdkworkflow.WithActivityOptions(ctx, sdkworkflow.ActivityOptions{
		StartToCloseTimeout: scoreRunAgentTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
			InitialInterval: 5 * time.Second,
		},
	})
	// First pass: ScoreRunAgent in parallel. We capture the
	// deterministic evaluations so the judge phase can finalize
	// them without re-running the deterministic pipeline.
	deterministicEvaluations := make(map[uuid.UUID]scoring.RunAgentEvaluation, len(completedRunAgents))
	scoreSelector := sdkworkflow.NewSelector(ctx)
	completedScoreActivities := 0

	for _, runAgent := range completedRunAgents {
		runAgent := runAgent
		future := sdkworkflow.ExecuteActivity(scoreCtx, scoreRunAgentActivityName, ScoreRunAgentInput{
			RunAgentID: runAgent.ID,
		})
		scoreSelector.AddFuture(future, func(f sdkworkflow.Future) {
			completedScoreActivities++

			evaluation, err := scoreRunAgentResult(ctx, f)
			switch {
			case err != nil:
				outcomes[runAgent.ID] = "errored"
			default:
				deterministicEvaluations[runAgent.ID] = evaluation
				if evaluation.Status == scoring.EvaluationStatusPartial {
					outcomes[runAgent.ID] = "partial"
				} else {
					outcomes[runAgent.ID] = "scored"
				}
			}
		})
	}

	for completedScoreActivities < len(completedRunAgents) {
		scoreSelector.Select(ctx)
	}

	// Second pass: JudgeRunAgent (#148 phase 4). Runs only for
	// agents that completed deterministic scoring successfully.
	// The activity is nil-safe at the worker layer — it is a
	// fast no-op when the spec has no LLM judges or when the
	// worker isn't wired with an evaluator (test fixtures, dev
	// loops without provider creds). The outcome is overwritten
	// based on the FINALIZED evaluation, which may upgrade a
	// partial result back to scored once judge dims become
	// available.
	judgeCtx := sdkworkflow.WithActivityOptions(ctx, sdkworkflow.ActivityOptions{
		StartToCloseTimeout: judgeRunAgentTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
		},
	})
	judgeSelector := sdkworkflow.NewSelector(ctx)
	judgeAgentCount := 0
	completedJudgeActivities := 0
	for _, runAgent := range completedRunAgents {
		detEval, ok := deterministicEvaluations[runAgent.ID]
		if !ok {
			// ScoreRunAgent errored — nothing for the judge to merge.
			continue
		}
		judgeAgentCount++
		runAgent := runAgent
		future := sdkworkflow.ExecuteActivity(judgeCtx, judgeRunAgentActivityName, JudgeRunAgentInput{
			RunAgentID:              runAgent.ID,
			DeterministicEvaluation: detEval,
		})
		judgeSelector.AddFuture(future, func(f sdkworkflow.Future) {
			completedJudgeActivities++
			finalized, err := scoreRunAgentResult(ctx, f)
			switch {
			case err != nil:
				// Judge activity itself failed — leave the outcome
				// from the deterministic pass in place, log for
				// observability. The scorecard is already persisted
				// from the deterministic write.
				sdkworkflow.GetLogger(ctx).Warn("judge run-agent activity failed",
					"run_agent_id", runAgent.ID.String(),
					"error", err,
				)
			case finalized.Status == scoring.EvaluationStatusPartial:
				outcomes[runAgent.ID] = "partial"
			default:
				outcomes[runAgent.ID] = "scored"
			}
		})
	}

	for completedJudgeActivities < judgeAgentCount {
		judgeSelector.Select(ctx)
	}

	for _, runAgent := range completedRunAgents {
		reason := scoreOutcomeReason(outcomes[runAgent.ID])
		if err := transitionRunAgentStatus(ctx, runAgent.ID, domain.RunAgentStatusCompleted, stringPtr(reason), nil); err != nil {
			sdkworkflow.GetLogger(ctx).Warn("failed to transition scored agent to completed",
				"run_agent_id", runAgent.ID.String(),
				"outcome", outcomes[runAgent.ID],
				"error", err,
			)
		}
	}

	if len(runAgents) > 0 {
		if err := buildRunScorecard(ctx, runAgents[0].RunID); err != nil {
			return "", err
		}
	}

	return summarizeScoreOutcomes(outcomes), nil
}

func scoreRunAgentResult(ctx sdkworkflow.Context, future sdkworkflow.Future) (scoring.RunAgentEvaluation, error) {
	var evaluation scoring.RunAgentEvaluation
	err := future.Get(ctx, &evaluation)
	return evaluation, err
}

func scoreOutcomeReason(outcome string) string {
	switch outcome {
	case "partial":
		return "run-agent scoring completed with partial evidence"
	case "errored":
		return "run-agent scoring errored; see scoring events for details"
	default:
		return "run-agent scoring completed"
	}
}

func summarizeScoreOutcomes(outcomes map[uuid.UUID]string) string {
	counts := map[string]int{
		"scored":  0,
		"partial": 0,
		"errored": 0,
		"skipped": 0,
	}
	for _, outcome := range outcomes {
		counts[outcome]++
	}
	return fmt.Sprintf("%d scored, %d partial, %d errored, %d skipped", counts["scored"], counts["partial"], counts["errored"], counts["skipped"])
}

func loadRun(ctx sdkworkflow.Context, runID uuid.UUID) (domain.Run, error) {
	var run domain.Run
	err := sdkworkflow.ExecuteActivity(ctx, loadRunActivityName, LoadRunInput{RunID: runID}).Get(ctx, &run)
	return run, err
}

func listRunAgents(ctx sdkworkflow.Context, runID uuid.UUID) ([]domain.RunAgent, error) {
	var runAgents []domain.RunAgent
	err := sdkworkflow.ExecuteActivity(ctx, listRunAgentsActivityName, ListRunAgentsInput{RunID: runID}).Get(ctx, &runAgents)
	return runAgents, err
}

func buildRunScorecard(ctx sdkworkflow.Context, runID uuid.UUID) error {
	var scorecard struct{}
	return sdkworkflow.ExecuteActivity(ctx, buildRunScorecardActivityName, BuildRunScorecardInput{
		RunID: runID,
	}).Get(ctx, &scorecard)
}

func attachRunTemporalIDs(ctx sdkworkflow.Context, runID uuid.UUID, workflowID string, temporalRunID string) error {
	var run domain.Run
	return sdkworkflow.ExecuteActivity(ctx, attachTemporalIDsActivityName, AttachRunTemporalIDsInput{
		RunID:              runID,
		TemporalWorkflowID: workflowID,
		TemporalRunID:      temporalRunID,
	}).Get(ctx, &run)
}

func transitionRunStatus(ctx sdkworkflow.Context, runID uuid.UUID, toStatus domain.RunStatus, reason *string) error {
	var run domain.Run
	return sdkworkflow.ExecuteActivity(ctx, transitionRunStatusActivityName, TransitionRunStatusInput{
		RunID:    runID,
		ToStatus: toStatus,
		Reason:   reason,
	}).Get(ctx, &run)
}

func markRunFailed(ctx sdkworkflow.Context, runID uuid.UUID, workflowErr error) error {
	reason := workflowErr.Error()
	var run domain.Run
	activityErr := sdkworkflow.ExecuteActivity(ctx, transitionRunStatusActivityName, TransitionRunStatusInput{
		RunID:    runID,
		ToStatus: domain.RunStatusFailed,
		Reason:   &reason,
	}).Get(ctx, &run)
	if activityErr != nil {
		return fmt.Errorf("run workflow failed: %v; additionally failed to mark run failed: %w", workflowErr, activityErr)
	}

	return workflowErr
}

func markRunCancelled(ctx sdkworkflow.Context, runID uuid.UUID, workflowErr error) error {
	disconnectedCtx, _ := sdkworkflow.NewDisconnectedContext(ctx)
	disconnectedCtx = sdkworkflow.WithActivityOptions(disconnectedCtx, defaultActivityOptions)

	reason := "run workflow cancelled"
	var run domain.Run
	activityErr := sdkworkflow.ExecuteActivity(disconnectedCtx, transitionRunStatusActivityName, TransitionRunStatusInput{
		RunID:    runID,
		ToStatus: domain.RunStatusCancelled,
		Reason:   &reason,
	}).Get(disconnectedCtx, &run)
	if activityErr != nil {
		return fmt.Errorf("run workflow cancelled: %v; additionally failed to mark run cancelled: %w", workflowErr, activityErr)
	}

	return workflowErr
}

func shouldSkipRunFailureTransition(err error) bool {
	return errors.Is(err, ErrRunMustBeQueued) ||
		hasApplicationErrorType(err, repositoryRunNotFoundErrorType) ||
		hasApplicationErrorType(err, repositoryTemporalIDConflictType) ||
		hasApplicationErrorType(err, repositoryInvalidTransitionType) ||
		hasApplicationErrorType(err, repositoryTransitionConflictType)
}

func isWorkflowCanceled(err error) bool {
	var canceledErr *temporal.CanceledError
	return errors.As(err, &canceledErr)
}

func hasApplicationErrorType(err error, wantType string) bool {
	var applicationErr *temporal.ApplicationError
	if !errors.As(err, &applicationErr) {
		return false
	}

	return applicationErr.Type() == wantType
}

func stringPtr(value string) *string {
	return &value
}
