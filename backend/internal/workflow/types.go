package workflow

import "github.com/google/uuid"

const (
	EvalSessionWorkflowName           = "EvalSessionWorkflow"
	RunWorkflowName                   = "RunWorkflow"
	RunAgentWorkflowName              = "RunAgentWorkflow"
	AgentHarnessExecutionWorkflowName = "AgentHarnessExecutionWorkflow"
	PlaygroundExperimentWorkflowName  = "PlaygroundExperimentWorkflow"
	HostedRunEventSignal              = "hosted_run_event"
	WorkflowTaskQueue                 = RunWorkflowName
)

type EvalSessionWorkflowInput struct {
	EvalSessionID uuid.UUID `json:"eval_session_id"`
}

type RunWorkflowInput struct {
	RunID uuid.UUID `json:"run_id"`
}

type RunAgentWorkflowInput struct {
	RunID      uuid.UUID `json:"run_id"`
	RunAgentID uuid.UUID `json:"run_agent_id"`
}

type AgentHarnessExecutionWorkflowInput struct {
	ExecutionID uuid.UUID `json:"execution_id"`
}

type PlaygroundExperimentWorkflowInput struct {
	ExperimentID uuid.UUID `json:"experiment_id"`
}
