package vibe

import (
	"context"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

// allAttemptStates matches the CHECK constraint of vibe_attempts.state in
// migration 00070.
var allAttemptStates = []AttemptState{AttemptDispatching, AttemptSucceeded, AttemptUncertain, AttemptReconciled}

func TestVibeAttemptTransitionsNameEveryLegalMove(t *testing.T) {
	type move struct{ from, to AttemptState }
	legal := map[move]string{
		{AttemptDispatching, AttemptSucceeded}:  "EndAttempt",
		{AttemptDispatching, AttemptUncertain}:  "EndAttempt, sweepAttempts after AttemptAbandonAfter",
		{AttemptDispatching, AttemptReconciled}: "EndAttempt, sweepAttempts when a receipt came first",
		{AttemptUncertain, AttemptReconciled}:   "ReconcileCost, a late EndAttempt, sweepAttempts after AttemptChargeAfter",
	}
	for _, from := range allAttemptStates {
		if !from.Valid() {
			t.Fatalf("state %q from the CHECK constraint is not in the transition table", from)
		}
		for _, to := range allAttemptStates {
			_, want := legal[move{from, to}]
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("CanTransitionTo(%s -> %s) = %v, want %v", from, to, got, want)
			}
		}
	}
	if len(attemptTransitions) != len(allAttemptStates) {
		t.Fatalf("transition table has %d states, the test lists %d", len(attemptTransitions), len(allAttemptStates))
	}
	terminal := map[AttemptState]bool{AttemptSucceeded: true, AttemptReconciled: true}
	for _, state := range allAttemptStates {
		if got := state.Terminal(); got != terminal[state] {
			t.Errorf("%s.Terminal() = %v, want %v", state, got, terminal[state])
		}
	}
	if AttemptState("ABANDONED").Valid() {
		t.Fatal("a state outside the CHECK constraint is valid")
	}

	migration, err := os.ReadFile("../../db/migrations/00070_vibe_sessions.sql")
	if err != nil {
		t.Fatal(err)
	}
	table := string(migration)
	start := strings.Index(table, "CREATE TABLE vibe_attempts")
	if start < 0 {
		t.Fatal("migration 00070 has no vibe_attempts table")
	}
	table = table[start:]
	table = table[:strings.Index(table, ");")]
	check := regexp.MustCompile(`state text NOT NULL CHECK\(state IN \(([^)]*)\)\)`).FindStringSubmatch(table)
	if check == nil {
		t.Fatal("vibe_attempts has no state CHECK constraint")
	}
	inCheck := []string{}
	for _, quoted := range strings.Split(check[1], ",") {
		inCheck = append(inCheck, strings.Trim(strings.TrimSpace(quoted), "'"))
	}
	inTable := []string{}
	for state := range attemptTransitions {
		inTable = append(inTable, string(state))
	}
	sort.Strings(inCheck)
	sort.Strings(inTable)
	if strings.Join(inCheck, ",") != strings.Join(inTable, ",") {
		t.Fatalf("CHECK names %v, the transition table names %v", inCheck, inTable)
	}
}

// The sweep must not end a call that can still finish. Each bound on a live
// call must be shorter than AttemptAbandonAfter.
func TestVibeAttemptLimitsCoverLiveCalls(t *testing.T) {
	bounds := map[string]time.Duration{
		"paid activity and the last journal write": paidActivityTimeout + attemptJournalTimeout,
		"default provider HTTP timeout":            provider.DefaultHTTPTimeout,
		"grounded review provider timeout":         90 * time.Second,
		"largest admitted operation timeout":       16 * time.Minute,
	}
	for _, anonymous := range []bool{true, false} {
		for _, local := range []bool{true, false} {
			l := executionLimits(anonymous, local)
			bounds["provider timeout of the limits"] = max(bounds["provider timeout of the limits"], time.Duration(l.ProviderSeconds)*time.Second)
			bounds["operation timeout of the limits"] = max(bounds["operation timeout of the limits"], l.OperationTimeout())
		}
	}
	for name, bound := range bounds {
		if bound >= AttemptAbandonAfter {
			t.Errorf("%s (%v) is not shorter than AttemptAbandonAfter (%v)", name, bound, AttemptAbandonAfter)
		}
	}
	if AttemptChargeAfter <= receiptWindow {
		t.Fatalf("AttemptChargeAfter (%v) must end after the receipt window (%v)", AttemptChargeAfter, receiptWindow)
	}
	if AttemptAbandonAfter != 20*time.Minute || AttemptChargeAfter != 25*time.Hour {
		t.Fatalf("the owner decision is 20 minutes and 25 hours, got %v and %v", AttemptAbandonAfter, AttemptChargeAfter)
	}

	// The workflow must give the paid activity the same timeout.
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var got time.Duration
	env.RegisterActivityWithOptions(func(ctx context.Context, _ string) error {
		got = activity.GetInfo(ctx).StartToCloseTimeout
		return nil
	}, activity.RegisterOptions{Name: "vibe.execute"})
	env.RegisterActivityWithOptions(func(context.Context, string, *Fault) error { return nil }, activity.RegisterOptions{Name: "vibe.finalize-with-fault"})
	env.ExecuteWorkflow(OperationWorkflow, "limits")
	if env.GetWorkflowError() != nil || got != paidActivityTimeout {
		t.Fatalf("paid activity timeout = %v, want %v (error %v)", got, paidActivityTimeout, env.GetWorkflowError())
	}
}
