package cmd

import "github.com/agentclash/agentclash/runtime/domain"

// StatusEnum documents the lifecycle of a polled resource: its statuses, the
// terminal statuses that every --follow loop stops on, and the legal next
// statuses. Published via `agentclash schema`.
//
// The registry is not a copy. It comes from domain.Lifecycles(), which reads
// the same transition tables that the server enforces, so the published
// contract cannot drift from the server. A lockstep test also keeps the
// follow-loop predicates (isTerminalRunStatus, evalsetStatusTerminal) equal
// to the terminal sets that the registry publishes.
type StatusEnum struct {
	Resource    string              `json:"resource" yaml:"resource"`
	Values      []string            `json:"values" yaml:"values"`
	Terminal    []string            `json:"terminal" yaml:"terminal"`
	Transitions map[string][]string `json:"transitions" yaml:"transitions"`
	Note        string              `json:"note,omitempty" yaml:"note,omitempty"`
}

// statusEnumNotes holds the facts about a lifecycle that its transition table
// cannot show.
var statusEnumNotes = map[string]string{
	"run": "The CLI also stops follow loops for agent-harness executions and dataset generation jobs on these terminal statuses.",
	"run_agent": "A cancelled run leaves its agents in their last status, because run agents have no cancelled status. " +
		"Follow the run status, not the agent statuses, to know when a run ends.",
	"eval_set": "budget_exceeded is terminal: the set stopped before it finished, and the CLI exits with code 1.",
}

var documentedStatusEnums = statusEnumsFromLifecycles(domain.Lifecycles())

func statusEnumsFromLifecycles(lifecycles []domain.Lifecycle) []StatusEnum {
	enums := make([]StatusEnum, 0, len(lifecycles))
	for _, lifecycle := range lifecycles {
		enums = append(enums, StatusEnum{
			Resource:    lifecycle.Resource,
			Values:      lifecycle.States,
			Terminal:    lifecycle.Terminal,
			Transitions: lifecycle.Transitions,
			Note:        statusEnumNotes[lifecycle.Resource],
		})
	}
	return enums
}
