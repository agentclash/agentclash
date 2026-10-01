package vibe

import "testing"

func TestTrialProtectedAllocationFundsCheckAndRetest(t *testing.T) {
	// Worst admitted protected flow: two authoring calls, three target + judge
	// cases, one identical retest. Exploratory messages cannot consume this pool.
	high := ModelProfile{InputNanoPerToken: 2000, OutputNanoPerToken: 8000}
	judge := ModelProfile{InputNanoPerToken: 400, OutputNanoPerToken: 1600}
	h, _ := high.BoundCost(16384, 2048)
	j, _ := judge.BoundCost(16384, 2048)
	if 8*h+6*j > TrialBudget-TrialExploreBudget {
		t.Fatal("protected trial allocation cannot fund the intended scorecard journey")
	}
}
