package workflow

import (
	sdkactivity "go.temporal.io/sdk/activity"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

type Registrar interface {
	RegisterWorkflowWithOptions(workflowFunc interface{}, options sdkworkflow.RegisterOptions)
	RegisterActivityWithOptions(activityFunc interface{}, options sdkactivity.RegisterOptions)
}

func Register(registrar Registrar, activities *Activities) {
	registrar.RegisterWorkflowWithOptions(RunWorkflow, sdkworkflow.RegisterOptions{Name: RunWorkflowName})
	registrar.RegisterWorkflowWithOptions(RunAgentWorkflow, sdkworkflow.RegisterOptions{Name: RunAgentWorkflowName})
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
	registrar.RegisterActivityWithOptions(activities.BuildRunAgentReplay, sdkactivity.RegisterOptions{Name: buildRunAgentReplayActivityName})
	registrar.RegisterActivityWithOptions(activities.SimulateExecution, sdkactivity.RegisterOptions{Name: simulateExecutionActivityName})
	registrar.RegisterActivityWithOptions(activities.SimulateEvaluation, sdkactivity.RegisterOptions{Name: simulateEvaluationActivityName})
}
