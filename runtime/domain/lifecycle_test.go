package domain

import (
	"slices"
	"testing"
)

// checkLifecycleOrder makes sure that an order list names each status of its
// transition table exactly once, so a new status cannot miss the published
// lifecycle.
func checkLifecycleOrder[S ~string](t *testing.T, name string, order []S, transitions map[S]map[S]struct{}) {
	t.Helper()
	seen := map[S]bool{}
	for _, state := range order {
		if seen[state] {
			t.Errorf("%s: status %q appears twice in the order list", name, state)
		}
		seen[state] = true
	}
	for from, next := range transitions {
		if !seen[from] {
			t.Errorf("%s: status %q is in the transition table but not in the order list", name, from)
		}
		for to := range next {
			if !seen[to] {
				t.Errorf("%s: status %q is a transition target but not in the order list", name, to)
			}
		}
	}
}

func TestLifecycleOrderListsCoverTheTransitionTables(t *testing.T) {
	checkLifecycleOrder(t, "run", runStatusOrder, runTransitions)
	checkLifecycleOrder(t, "run_agent", runAgentStatusOrder, runAgentTransitions)
	checkLifecycleOrder(t, "eval_session", evalSessionStatusOrder, evalSessionTransitions)
	checkLifecycleOrder(t, "eval_set", evalSetStatusOrder, evalSetTransitions)
	checkLifecycleOrder(t, "regression_suite", regressionSuiteStatusOrder, regressionSuiteTransitions)
	checkLifecycleOrder(t, "regression_case", regressionCaseStatusOrder, regressionCaseTransitions)

	// The eval-set table has no rows for its terminal statuses, so the check
	// above cannot see a terminal status that the order list leaves out.
	// ValidateEvalSetStatus is the complete list.
	for _, state := range evalSetStatusOrder {
		if err := ValidateEvalSetStatus(state); err != nil {
			t.Errorf("eval_set order list has an unknown status: %v", err)
		}
	}
	if got, want := len(evalSetStatusOrder), 8; got != want {
		t.Errorf("eval_set order list has %d statuses, want %d; update it with the new status", got, want)
	}
}

func TestLifecyclesAgreeWithTheTransitionTables(t *testing.T) {
	for _, lifecycle := range Lifecycles() {
		for _, from := range lifecycle.States {
			for _, to := range lifecycle.States {
				published := slices.Contains(lifecycle.Transitions[from], to)
				if enforced := canTransition(lifecycle.Resource, from, to); published != enforced {
					t.Errorf("%s: %s -> %s is published as %v but enforced as %v", lifecycle.Resource, from, to, published, enforced)
				}
			}
		}
	}
}

func canTransition(resource, from, to string) bool {
	switch resource {
	case "run":
		return RunStatus(from).CanTransitionTo(RunStatus(to))
	case "run_agent":
		return RunAgentStatus(from).CanTransitionTo(RunAgentStatus(to))
	case "eval_session":
		return EvalSessionStatus(from).CanTransitionTo(EvalSessionStatus(to))
	case "eval_set":
		return EvalSetStatus(from).CanTransitionTo(EvalSetStatus(to))
	case "regression_suite":
		return RegressionSuiteStatus(from).CanTransitionTo(RegressionSuiteStatus(to))
	case "regression_case":
		return RegressionCaseStatus(from).CanTransitionTo(RegressionCaseStatus(to))
	}
	panic("unknown lifecycle " + resource)
}

func TestLifecycleTerminalStatusesMatchTheDomainPredicates(t *testing.T) {
	byResource := map[string]Lifecycle{}
	for _, lifecycle := range Lifecycles() {
		byResource[lifecycle.Resource] = lifecycle
	}

	want := map[string][]string{
		"run":              {"completed", "failed", "cancelled"},
		"run_agent":        {"completed", "failed"},
		"eval_session":     {"completed", "failed", "cancelled"},
		"eval_set":         {"completed", "failed", "cancelled", "budget_exceeded"},
		"regression_suite": {},
		"regression_case":  {"archived", "rejected"},
	}
	if len(byResource) != len(want) {
		t.Fatalf("Lifecycles() returned %d resources, want %d", len(byResource), len(want))
	}
	for resource, terminal := range want {
		if got := byResource[resource].Terminal; !slices.Equal(got, terminal) {
			t.Errorf("%s terminal = %v, want %v", resource, got, terminal)
		}
	}

	for _, state := range byResource["eval_session"].States {
		inList := slices.Contains(byResource["eval_session"].Terminal, state)
		if inList != EvalSessionStatus(state).Terminal() {
			t.Errorf("eval_session %q: terminal list says %v, Terminal() says %v", state, inList, !inList)
		}
	}
	for _, state := range byResource["eval_set"].States {
		inList := slices.Contains(byResource["eval_set"].Terminal, state)
		if inList != IsEvalSetTerminal(EvalSetStatus(state)) {
			t.Errorf("eval_set %q: terminal list says %v, IsEvalSetTerminal says %v", state, inList, !inList)
		}
	}
}

func TestLifecyclesReturnsIndependentCopies(t *testing.T) {
	first := Lifecycles()
	first[0].States[0] = "changed"
	first[0].Transitions["queued"] = nil
	second := Lifecycles()
	if second[0].States[0] == "changed" || len(second[0].Transitions["queued"]) == 0 {
		t.Fatal("Lifecycles() shares state between calls")
	}
}
