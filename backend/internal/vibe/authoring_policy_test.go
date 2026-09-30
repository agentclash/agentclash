package vibe

import (
	"testing"
)

func TestAuthoringPoliciesRetainContractsAndBounds(t *testing.T) {
	for _, version := range []int{15, 18, 20} {
		policy, ok := authoringPolicyFor(version)
		if !ok {
			t.Fatal(version)
		}
		want := 8
		if version != 15 {
			want = 10
		}
		calls, cost := policy.allowance(7, nil)
		if calls != want || cost != int64(want*7) {
			t.Fatal(policy, calls, cost)
		}
		calls, cost = policy.allowance(7, &AssistantRecovery{MaxCost: 3})
		if calls != want+1 || cost != int64(want*7+3) {
			t.Fatal(calls, cost)
		}
		for _, stage := range []string{"route", "handler", "review"} {
			for _, suffix := range []string{"", ":repair", ":fallback"} {
				if !policy.stageAllowed(stage+suffix, false) {
					t.Fatal(stage + suffix)
				}
			}
		}
		if policy.stageAllowed("prototype", false) != (version != 15) || policy.stageAllowed("route:repair:fallback", false) || policy.stageAllowed("repair", false) || policy.stageAllowed("handler", true) || !policy.stageAllowed("review", true) {
			t.Fatal("invalid graph", version)
		}

	}
	for _, version := range []int{0, 4, 9, 10, 11, 12, 13, 14, 16, 17, 19, 21} {
		if retainedAuthoring(version) {
			t.Fatal("retired contract executable", version)
		}
	}
}
