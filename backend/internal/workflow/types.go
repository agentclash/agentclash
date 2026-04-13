package workflow

import "github.com/google/uuid"

const (
	RunWorkflowName                  = "RunWorkflow"
	RunAgentWorkflowName             = "RunAgentWorkflow"
	PlaygroundExperimentWorkflowName = "PlaygroundExperimentWorkflow"
	HostedRunEventSignal             = "hosted_run_event"
)

type RunWorkflowInput struct {
	RunID uuid.UUID `json:"run_id"`
}

type RunAgentWorkflowInput struct {
	RunID      uuid.UUID `json:"run_id"`
	RunAgentID uuid.UUID `json:"run_agent_id"`
}

type PlaygroundExperimentWorkflowInput struct {
	ExperimentID uuid.UUID `json:"experiment_id"`
}
