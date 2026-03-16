package workflow

import (
	"errors"
	"fmt"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/domain"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/hostedruns"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

const (
	nativeActivityBootBuffer    = 20 * time.Second
	nativeActivityCleanupBuffer = 15 * time.Second
)

func RunAgentWorkflow(ctx sdkworkflow.Context, input RunAgentWorkflowInput) error {
	ctx = sdkworkflow.WithActivityOptions(ctx, defaultActivityOptions)

	err := runAgentWorkflow(ctx, input)
	if err == nil {
		return nil
	}
	if shouldSkipRunAgentFailureTransition(err) {
		return err
	}

	return markRunAgentFailed(ctx, input.RunAgentID, err)
}

func runAgentWorkflow(ctx sdkworkflow.Context, input RunAgentWorkflowInput) error {
	runAgent, err := loadRunAgent(ctx, input.RunAgentID)
	if err != nil {
		return err
	}
	if err := validateRunAgentQueued(runAgent, input.RunID); err != nil {
		return err
	}

	executionContext, err := loadRunAgentExecutionContext(ctx, input.RunAgentID)
	if err != nil {
		return err
	}

	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusReady, stringPtr("execution lane prepared"), nil); err != nil {
		return err
	}
	if err := sdkworkflow.ExecuteActivity(ctx, prepareLaneActivityName, input).Get(ctx, nil); err != nil {
		return err
	}

	if executionContext.Deployment.DeploymentType == "hosted_external" {
		return runHostedRunAgent(ctx, input, executionContext)
	}

	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusExecuting, stringPtr("native execution started"), nil); err != nil {
		return err
	}
	if err := executeNativeModelStep(ctx, input, executionContext).Get(ctx, nil); err != nil {
		return err
	}
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusEvaluating, stringPtr("native execution completed; evaluation hook pending"), nil); err != nil {
		return err
	}
	if err := generateRunAgentEvaluation(ctx, input.RunAgentID); err != nil {
		return err
	}
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusCompleted, stringPtr("native run-agent execution completed"), nil); err != nil {
		return err
	}
	if err := buildRunAgentReplay(ctx, input.RunAgentID); err != nil {
		sdkworkflow.GetLogger(ctx).Warn("replay build failed after successful execution", "run_agent_id", input.RunAgentID.String(), "error", err)
	}

	return nil
}

func executeNativeModelStep(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext) sdkworkflow.Future {
	return sdkworkflow.ExecuteActivity(
		sdkworkflow.WithActivityOptions(ctx, nativeModelActivityOptions(executionContext)),
		executeNativeModelStepActivityName,
		input,
	)
}

func nativeModelActivityOptions(executionContext repository.RunAgentExecutionContext) sdkworkflow.ActivityOptions {
	options := defaultActivityOptions
	if executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds > 0 {
		options.StartToCloseTimeout = time.Duration(executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds)*time.Second + nativeActivityBootBuffer + nativeActivityCleanupBuffer
	}
	return options
}

func runHostedRunAgent(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext) error {
	var startResult StartHostedRunResult
	if err := sdkworkflow.ExecuteActivity(ctx, startHostedRunActivityName, StartHostedRunInput{
		RunAgentID: input.RunAgentID,
		TraceLevel: hostedruns.TraceLevelBlackBox,
	}).Get(ctx, &startResult); err != nil {
		return err
	}

	reason := fmt.Sprintf("hosted run accepted by external deployment as %s", startResult.ExternalRunID)
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusExecuting, &reason, nil); err != nil {
		return err
	}

	if err := waitForHostedRunTerminalEvent(ctx, input, executionContext, startResult.ExternalRunID); err != nil {
		return err
	}

	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusEvaluating, stringPtr("hosted black-box result received"), nil); err != nil {
		return err
	}
	if err := generateRunAgentEvaluation(ctx, input.RunAgentID); err != nil {
		return err
	}
	if err := transitionRunAgentStatus(ctx, input.RunAgentID, domain.RunAgentStatusCompleted, stringPtr("hosted black-box completion recorded"), nil); err != nil {
		return err
	}

	return nil
}

func waitForHostedRunTerminalEvent(ctx sdkworkflow.Context, input RunAgentWorkflowInput, executionContext repository.RunAgentExecutionContext, externalRunID string) error {
	timeout := time.Duration(executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds) * time.Second
	signalCh := sdkworkflow.GetSignalChannel(ctx, HostedRunEventSignal)
	timer := sdkworkflow.NewTimer(ctx, timeout)

	for {
		var (
			event    hostedruns.Event
			timedOut bool
			gotEvent bool
		)

		selector := sdkworkflow.NewSelector(ctx)
		selector.AddReceive(signalCh, func(c sdkworkflow.ReceiveChannel, more bool) {
			gotEvent = true
			c.Receive(ctx, &event)
		})
		selector.AddFuture(timer, func(sdkworkflow.Future) {
			timedOut = true
		})
		selector.Select(ctx)

		if timedOut {
			reason := fmt.Sprintf("hosted run timed out waiting for callback after %s", timeout)
			if err := sdkworkflow.ExecuteActivity(ctx, markHostedRunTimedOutActivityName, MarkHostedRunTimedOutInput{
				RunAgentID:   input.RunAgentID,
				ErrorMessage: reason,
			}).Get(ctx, nil); err != nil {
				return err
			}
			return errors.New(reason)
		}
		if !gotEvent {
			continue
		}
		if err := event.Validate(); err != nil {
			return err
		}
		if event.RunAgentID != input.RunAgentID {
			return fmt.Errorf("hosted callback run_agent_id %s does not match workflow run_agent_id %s", event.RunAgentID, input.RunAgentID)
		}
		if event.ExternalRunID != externalRunID {
			return fmt.Errorf("hosted callback external_run_id %q does not match accepted external_run_id %q", event.ExternalRunID, externalRunID)
		}

		switch event.EventType {
		case hostedruns.EventTypeError:
			return errors.New(*event.ErrorMessage)
		case hostedruns.EventTypeRunFinished:
			if event.FinalStatus != nil && *event.FinalStatus == hostedruns.FinalStatusCompleted {
				return nil
			}
			if event.ErrorMessage != nil {
				return errors.New(*event.ErrorMessage)
			}
			return errors.New("hosted run finished with failed status")
		default:
			return fmt.Errorf("unsupported hosted terminal event type %q", event.EventType)
		}
	}
}

func loadRunAgent(ctx sdkworkflow.Context, runAgentID uuid.UUID) (domain.RunAgent, error) {
	var runAgent domain.RunAgent
	err := sdkworkflow.ExecuteActivity(ctx, loadRunAgentActivityName, LoadRunAgentInput{
		RunAgentID: runAgentID,
	}).Get(ctx, &runAgent)
	return runAgent, err
}

func loadRunAgentExecutionContext(ctx sdkworkflow.Context, runAgentID uuid.UUID) (repository.RunAgentExecutionContext, error) {
	var executionContext repository.RunAgentExecutionContext
	err := sdkworkflow.ExecuteActivity(ctx, loadRunAgentExecutionContextActivityName, LoadRunAgentExecutionContextInput{
		RunAgentID: runAgentID,
	}).Get(ctx, &executionContext)
	return executionContext, err
}

func transitionRunAgentStatus(ctx sdkworkflow.Context, runAgentID uuid.UUID, toStatus domain.RunAgentStatus, reason *string, failureReason *string) error {
	var runAgent domain.RunAgent
	return sdkworkflow.ExecuteActivity(ctx, transitionRunAgentStatusActivityName, TransitionRunAgentStatusInput{
		RunAgentID:    runAgentID,
		ToStatus:      toStatus,
		Reason:        reason,
		FailureReason: failureReason,
	}).Get(ctx, &runAgent)
}

func markRunAgentFailed(ctx sdkworkflow.Context, runAgentID uuid.UUID, workflowErr error) error {
	reason := workflowErr.Error()
	var runAgent domain.RunAgent
	activityErr := sdkworkflow.ExecuteActivity(ctx, transitionRunAgentStatusActivityName, TransitionRunAgentStatusInput{
		RunAgentID:    runAgentID,
		ToStatus:      domain.RunAgentStatusFailed,
		Reason:        &reason,
		FailureReason: &reason,
	}).Get(ctx, &runAgent)
	if activityErr != nil {
		return fmt.Errorf("run-agent workflow failed: %v; additionally failed to mark run agent failed: %w", workflowErr, activityErr)
	}
	if replayErr := buildRunAgentReplay(ctx, runAgentID); replayErr != nil {
		sdkworkflow.GetLogger(ctx).Warn("replay build failed after execution failure", "run_agent_id", runAgentID.String(), "error", replayErr)
	}

	return workflowErr
}

func buildRunAgentReplay(ctx sdkworkflow.Context, runAgentID uuid.UUID) error {
	var replay repository.RunAgentReplay
	return sdkworkflow.ExecuteActivity(ctx, buildRunAgentReplayActivityName, BuildRunAgentReplayInput{
		RunAgentID: runAgentID,
	}).Get(ctx, &replay)
}

func generateRunAgentEvaluation(ctx sdkworkflow.Context, runAgentID uuid.UUID) error {
	var evaluation scoring.RunAgentEvaluation
	return sdkworkflow.ExecuteActivity(ctx, generateRunAgentEvaluationActivityName, GenerateRunAgentEvaluationInput{
		RunAgentID: runAgentID,
	}).Get(ctx, &evaluation)
}

func shouldSkipRunAgentFailureTransition(err error) bool {
	var canceledErr *temporal.CanceledError
	return errors.As(err, &canceledErr) ||
		errors.Is(err, ErrRunAgentMustBeQueued) ||
		errors.Is(err, ErrRunAgentRunMismatch) ||
		hasApplicationErrorType(err, repositoryRunAgentNotFoundErrorType) ||
		hasApplicationErrorType(err, repositoryFrozenExecutionContextType) ||
		hasApplicationErrorType(err, repositoryInvalidTransitionType) ||
		hasApplicationErrorType(err, repositoryTransitionConflictType)
}
