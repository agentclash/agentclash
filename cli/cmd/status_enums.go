package cmd

// StatusEnum documents the lifecycle statuses of a polled resource and which
// of them are terminal — the set every --follow loop stops on. The values
// reference the canonical constants in run_status.go (the same source
// isTerminalRunStatus uses), and a lockstep test keeps the two aligned, so
// the published contract cannot drift from the code. Published via
// `agentclash schema`.
type StatusEnum struct {
	Resource string   `json:"resource" yaml:"resource"`
	Values   []string `json:"values" yaml:"values"`
	Terminal []string `json:"terminal" yaml:"terminal"`
}

var documentedStatusEnums = []StatusEnum{
	{
		// Runs, run-agent executions, agent-harness executions, eval
		// sessions, and dataset generation jobs all share this lifecycle.
		Resource: "run",
		Values:   []string{runStatusPending, runStatusRunning, runStatusCompleted, runStatusFailed, runStatusCancelled},
		Terminal: []string{runStatusCompleted, runStatusFailed, runStatusCancelled},
	},
}
