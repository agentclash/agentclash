package domain

// Lifecycle describes one status machine in a form that clients can read.
//
// The transition tables in this package own each lifecycle. The server
// enforces them, and the CLI publishes them through `agentclash schema`, so
// scripts and agents learn the legal states from the same source. Clients
// must read a Lifecycle and must not keep their own copy of a status list.
type Lifecycle struct {
	// Resource is the stable name that clients use, for example "run".
	Resource string
	// States lists every status in lifecycle order.
	States []string
	// Terminal lists the statuses that have no next status. A follow loop
	// stops on these.
	Terminal []string
	// Transitions maps each status to its legal next statuses, in lifecycle
	// order. A terminal status maps to an empty list.
	Transitions map[string][]string
}

// The order lists give each lifecycle a stable, readable order. The tests
// make sure that each list names every status of its transition table.
var (
	runStatusOrder = []RunStatus{
		RunStatusDraft, RunStatusQueued, RunStatusProvisioning, RunStatusRunning,
		RunStatusScoring, RunStatusCompleted, RunStatusFailed, RunStatusCancelled,
	}
	runAgentStatusOrder = []RunAgentStatus{
		RunAgentStatusQueued, RunAgentStatusReady, RunAgentStatusExecuting,
		RunAgentStatusEvaluating, RunAgentStatusCompleted, RunAgentStatusFailed,
	}
	evalSessionStatusOrder = []EvalSessionStatus{
		EvalSessionStatusQueued, EvalSessionStatusRunning, EvalSessionStatusAggregating,
		EvalSessionStatusCompleted, EvalSessionStatusFailed, EvalSessionStatusCancelled,
	}
	evalSetStatusOrder = []EvalSetStatus{
		EvalSetStatusQueued, EvalSetStatusExpanding, EvalSetStatusRunning,
		EvalSetStatusAggregating, EvalSetStatusCompleted, EvalSetStatusFailed,
		EvalSetStatusCancelled, EvalSetStatusBudgetExceeded,
	}
	regressionSuiteStatusOrder = []RegressionSuiteStatus{
		RegressionSuiteStatusActive, RegressionSuiteStatusArchived,
	}
	regressionCaseStatusOrder = []RegressionCaseStatus{
		RegressionCaseStatusProposed, RegressionCaseStatusActive, RegressionCaseStatusMuted,
		RegressionCaseStatusArchived, RegressionCaseStatusRejected,
	}
)

// Lifecycles returns the published status machines in a stable order. Each
// call returns new values, so a caller can change the result safely.
func Lifecycles() []Lifecycle {
	return []Lifecycle{
		describeLifecycle("run", runStatusOrder, runTransitions),
		describeLifecycle("run_agent", runAgentStatusOrder, runAgentTransitions),
		describeLifecycle("eval_session", evalSessionStatusOrder, evalSessionTransitions),
		describeLifecycle("eval_set", evalSetStatusOrder, evalSetTransitions),
		describeLifecycle("regression_suite", regressionSuiteStatusOrder, regressionSuiteTransitions),
		describeLifecycle("regression_case", regressionCaseStatusOrder, regressionCaseTransitions),
	}
}

func describeLifecycle[S ~string](resource string, order []S, transitions map[S]map[S]struct{}) Lifecycle {
	lifecycle := Lifecycle{
		Resource:    resource,
		States:      make([]string, 0, len(order)),
		Terminal:    []string{},
		Transitions: make(map[string][]string, len(order)),
	}
	for _, state := range order {
		next := []string{}
		for _, candidate := range order {
			if _, ok := transitions[state][candidate]; ok {
				next = append(next, string(candidate))
			}
		}
		lifecycle.States = append(lifecycle.States, string(state))
		lifecycle.Transitions[string(state)] = next
		if len(next) == 0 {
			lifecycle.Terminal = append(lifecycle.Terminal, string(state))
		}
	}
	return lifecycle
}
