package workflow

import (
	sdkactivity "go.temporal.io/sdk/activity"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

type Registrar interface {
	RegisterWorkflowWithOptions(workflowFunc interface{}, options sdkworkflow.RegisterOptions)
	RegisterActivityWithOptions(activityFunc interface{}, options sdkactivity.RegisterOptions)
}

// Register registers all workflows/activities on a single worker. Prefer
// RegisterForTaskQueue when serving partitioned Fleet queues.
func Register(registrar Registrar, activities *Activities) {
	RegisterForTaskQueue(registrar, activities, LegacyTaskQueue)
}

// RegisterForTaskQueue registers the workflow/activity subset for a Fleet queue
// class. LegacyTaskQueue registers the full set (pre-partition behavior).
func RegisterForTaskQueue(registrar Registrar, activities *Activities, taskQueue string) {
	switch taskQueue {
	case TaskQueueExecution:
		registerExecutionWorkflows(registrar)
		registerExecutionActivities(registrar, activities)
	case TaskQueueScoring:
		registerScoringActivities(registrar, activities)
	case TaskQueueBackground:
		registerBackgroundWorkflows(registrar)
		registerBackgroundActivities(registrar, activities)
	case LegacyTaskQueue:
		registerExecutionWorkflows(registrar)
		registerBackgroundWorkflows(registrar)
		registerExecutionActivities(registrar, activities)
		registerScoringActivities(registrar, activities)
		registerBackgroundActivities(registrar, activities)
	default:
		// Unknown queue names get the full set so misconfiguration fails open
		// rather than silently dropping work.
		registerExecutionWorkflows(registrar)
		registerBackgroundWorkflows(registrar)
		registerExecutionActivities(registrar, activities)
		registerScoringActivities(registrar, activities)
		registerBackgroundActivities(registrar, activities)
	}
}

func RegisterDatasetGeneration(registrar Registrar, activities *DatasetGenerationActivities) {
	RegisterDatasetGenerationForTaskQueue(registrar, activities, LegacyTaskQueue)
}

func RegisterDatasetGenerationForTaskQueue(registrar Registrar, activities *DatasetGenerationActivities, taskQueue string) {
	switch taskQueue {
	case TaskQueueBackground, LegacyTaskQueue:
		registrar.RegisterWorkflowWithOptions(SyntheticDatasetGenerationWorkflow, sdkworkflow.RegisterOptions{Name: SyntheticDatasetGenerationWorkflowName})
		registrar.RegisterActivityWithOptions(activities.LoadDatasetGenerationExecutionContext, sdkactivity.RegisterOptions{Name: loadDatasetGenerationExecutionContextActivityName})
		registrar.RegisterActivityWithOptions(activities.SetDatasetGenerationJobTemporalIDs, sdkactivity.RegisterOptions{Name: setDatasetGenerationJobTemporalIDsActivityName})
		registrar.RegisterActivityWithOptions(activities.UpdateDatasetGenerationJobStatus, sdkactivity.RegisterOptions{Name: updateDatasetGenerationJobStatusActivityName})
		registrar.RegisterActivityWithOptions(activities.ExecuteSyntheticDatasetGeneration, sdkactivity.RegisterOptions{Name: executeSyntheticDatasetGenerationActivityName})
	case TaskQueueExecution, TaskQueueScoring:
		// Dataset generation is background-only.
	default:
		RegisterDatasetGenerationForTaskQueue(registrar, activities, LegacyTaskQueue)
	}
}

func registerExecutionWorkflows(registrar Registrar) {
	registrar.RegisterWorkflowWithOptions(EvalSessionWorkflow, sdkworkflow.RegisterOptions{Name: EvalSessionWorkflowName})
	registrar.RegisterWorkflowWithOptions(EvalSetWorkflow, sdkworkflow.RegisterOptions{Name: EvalSetWorkflowName})
	registrar.RegisterWorkflowWithOptions(RunWorkflow, sdkworkflow.RegisterOptions{Name: RunWorkflowName})
	registrar.RegisterWorkflowWithOptions(RunAgentWorkflow, sdkworkflow.RegisterOptions{Name: RunAgentWorkflowName})
	registrar.RegisterWorkflowWithOptions(AgentHarnessExecutionWorkflow, sdkworkflow.RegisterOptions{Name: AgentHarnessExecutionWorkflowName})
}

func registerBackgroundWorkflows(registrar Registrar) {
	registrar.RegisterWorkflowWithOptions(PublicAgentTryoutExecutionWorkflow, sdkworkflow.RegisterOptions{Name: PublicAgentTryoutExecutionWorkflowName})
	registrar.RegisterWorkflowWithOptions(ScanEvalSetWorkflow, sdkworkflow.RegisterOptions{Name: ScanEvalSetWorkflowName})
}

func registerExecutionActivities(registrar Registrar, activities *Activities) {
	registrar.RegisterActivityWithOptions(activities.LoadEvalSession, sdkactivity.RegisterOptions{Name: loadEvalSessionActivityName})
	registrar.RegisterActivityWithOptions(activities.ListEvalSessionRuns, sdkactivity.RegisterOptions{Name: listEvalSessionRunsActivityName})
	registrar.RegisterActivityWithOptions(activities.TransitionEvalSessionStatus, sdkactivity.RegisterOptions{Name: transitionEvalSessionStatusActivityName})
	registrar.RegisterActivityWithOptions(activities.AggregateEvalSession, sdkactivity.RegisterOptions{Name: aggregateEvalSessionActivityName})
	registrar.RegisterActivityWithOptions(activities.TransitionEvalSetStatus, sdkactivity.RegisterOptions{Name: transitionEvalSetStatusActivityName})
	registrar.RegisterActivityWithOptions(activities.LoadEvalSet, sdkactivity.RegisterOptions{Name: loadEvalSetActivityName})
	registrar.RegisterActivityWithOptions(activities.ListEvalSetSessionIDs, sdkactivity.RegisterOptions{Name: listEvalSetSessionIDsActivityName})
	registrar.RegisterActivityWithOptions(activities.ListEvalSetManifestScanners, sdkactivity.RegisterOptions{Name: listEvalSetManifestScannersActivityName})
	registrar.RegisterActivityWithOptions(activities.AggregateEvalSet, sdkactivity.RegisterOptions{Name: aggregateEvalSetActivityName})
	registrar.RegisterActivityWithOptions(activities.CheckEvalSetBudget, sdkactivity.RegisterOptions{Name: checkEvalSetBudgetActivityName})
	registrar.RegisterActivityWithOptions(activities.RefreshEvalSetSpend, sdkactivity.RegisterOptions{Name: refreshEvalSetSpendActivityName})
	registrar.RegisterActivityWithOptions(activities.WaitWorkspaceRunCapacity, sdkactivity.RegisterOptions{Name: waitWorkspaceRunCapacityActivityName})
	registrar.RegisterActivityWithOptions(activities.RecordEvalSetSpendEvent, sdkactivity.RegisterOptions{Name: recordEvalSetSpendEventActivityName})
	registrar.RegisterActivityWithOptions(activities.LoadRun, sdkactivity.RegisterOptions{Name: loadRunActivityName})
	registrar.RegisterActivityWithOptions(activities.ListRunAgents, sdkactivity.RegisterOptions{Name: listRunAgentsActivityName})
	registrar.RegisterActivityWithOptions(activities.LoadRunAgent, sdkactivity.RegisterOptions{Name: loadRunAgentActivityName})
	registrar.RegisterActivityWithOptions(activities.LoadRunAgentExecutionContext, sdkactivity.RegisterOptions{Name: loadRunAgentExecutionContextActivityName})
	registrar.RegisterActivityWithOptions(activities.AttachRunTemporalIDs, sdkactivity.RegisterOptions{Name: attachTemporalIDsActivityName})
	registrar.RegisterActivityWithOptions(activities.TransitionRunStatus, sdkactivity.RegisterOptions{Name: transitionRunStatusActivityName})
	registrar.RegisterActivityWithOptions(activities.TransitionRunAgentStatus, sdkactivity.RegisterOptions{Name: transitionRunAgentStatusActivityName})
	registrar.RegisterActivityWithOptions(activities.PrepareExecutionLane, sdkactivity.RegisterOptions{Name: prepareLaneActivityName})
	registrar.RegisterActivityWithOptions(activities.StartHostedRun, sdkactivity.RegisterOptions{Name: startHostedRunActivityName})
	registrar.RegisterActivityWithOptions(activities.MarkHostedRunTimedOut, sdkactivity.RegisterOptions{Name: markHostedRunTimedOutActivityName})
	registrar.RegisterActivityWithOptions(activities.ExecuteNativeModelStep, sdkactivity.RegisterOptions{Name: executeNativeModelStepActivityName})
	registrar.RegisterActivityWithOptions(activities.ExecuteRunAgentCase, sdkactivity.RegisterOptions{Name: executeRunAgentCaseActivityName})
	registrar.RegisterActivityWithOptions(activities.ExecutePromptEvalStep, sdkactivity.RegisterOptions{Name: executePromptEvalStepActivityName})
	registrar.RegisterActivityWithOptions(activities.ExecuteResponsesStep, sdkactivity.RegisterOptions{Name: executeResponsesStepActivityName})
	registrar.RegisterActivityWithOptions(activities.ExecuteMultiTurnStep, sdkactivity.RegisterOptions{Name: executeMultiTurnStepActivityName})
	registrar.RegisterActivityWithOptions(activities.FinalizeMultiTurnPostRun, sdkactivity.RegisterOptions{Name: finalizeMultiTurnPostRunActivityName})
	registrar.RegisterActivityWithOptions(activities.SimulateExecution, sdkactivity.RegisterOptions{Name: simulateExecutionActivityName})
	registrar.RegisterActivityWithOptions(activities.SimulateEvaluation, sdkactivity.RegisterOptions{Name: simulateEvaluationActivityName})
	registrar.RegisterActivityWithOptions(activities.TransitionAgentHarnessExecutionStatus, sdkactivity.RegisterOptions{Name: transitionAgentHarnessExecutionStatusActivityName})
	registrar.RegisterActivityWithOptions(activities.ExecuteAgentHarnessExecution, sdkactivity.RegisterOptions{Name: executeAgentHarnessExecutionActivityName})
}

func registerScoringActivities(registrar Registrar, activities *Activities) {
	registrar.RegisterActivityWithOptions(activities.ScoreRunAgent, sdkactivity.RegisterOptions{Name: scoreRunAgentActivityName})
	registrar.RegisterActivityWithOptions(activities.BuildRunScorecard, sdkactivity.RegisterOptions{Name: buildRunScorecardActivityName})
	registrar.RegisterActivityWithOptions(activities.BuildRunAgentReplay, sdkactivity.RegisterOptions{Name: buildRunAgentReplayActivityName})
}

func registerBackgroundActivities(registrar Registrar, activities *Activities) {
	registrar.RegisterActivityWithOptions(activities.ExecutePublicAgentTryout, sdkactivity.RegisterOptions{Name: executePublicAgentTryoutActivityName})
	registrar.RegisterActivityWithOptions(activities.FailPublicAgentTryout, sdkactivity.RegisterOptions{Name: failPublicAgentTryoutActivityName})
	registrar.RegisterActivityWithOptions(activities.ListScanTargets, sdkactivity.RegisterOptions{Name: listScanTargetsActivityName})
	registrar.RegisterActivityWithOptions(activities.ScanOneTarget, sdkactivity.RegisterOptions{Name: scanOneTargetActivityName})
	registrar.RegisterActivityWithOptions(activities.ScanEvalSet, sdkactivity.RegisterOptions{Name: scanEvalSetActivityName})
}
