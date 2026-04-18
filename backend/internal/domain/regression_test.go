package domain

import "testing"

func TestRegressionSuiteStatusTransitions(t *testing.T) {
	tests := []struct {
		name string
		from RegressionSuiteStatus
		to   RegressionSuiteStatus
		want bool
	}{
		{name: "active to archived", from: RegressionSuiteStatusActive, to: RegressionSuiteStatusArchived, want: true},
		{name: "archived to active", from: RegressionSuiteStatusArchived, to: RegressionSuiteStatusActive, want: true},
		{name: "active to active", from: RegressionSuiteStatusActive, to: RegressionSuiteStatusActive, want: false},
		{name: "archived to archived", from: RegressionSuiteStatusArchived, to: RegressionSuiteStatusArchived, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.from.CanTransitionTo(tc.to); got != tc.want {
				t.Fatalf("CanTransitionTo(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestRegressionCaseStatusTransitions(t *testing.T) {
	tests := []struct {
		name string
		from RegressionCaseStatus
		to   RegressionCaseStatus
		want bool
	}{
		{name: "active to muted", from: RegressionCaseStatusActive, to: RegressionCaseStatusMuted, want: true},
		{name: "active to archived", from: RegressionCaseStatusActive, to: RegressionCaseStatusArchived, want: true},
		{name: "muted to active", from: RegressionCaseStatusMuted, to: RegressionCaseStatusActive, want: true},
		{name: "muted to archived", from: RegressionCaseStatusMuted, to: RegressionCaseStatusArchived, want: true},
		{name: "archived to active denied", from: RegressionCaseStatusArchived, to: RegressionCaseStatusActive, want: false},
		{name: "archived to muted denied", from: RegressionCaseStatusArchived, to: RegressionCaseStatusMuted, want: false},
		{name: "active to active denied", from: RegressionCaseStatusActive, to: RegressionCaseStatusActive, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.from.CanTransitionTo(tc.to); got != tc.want {
				t.Fatalf("CanTransitionTo(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestRegressionParsersRejectInvalidValues(t *testing.T) {
	if _, err := ParseRegressionSuiteStatus("pending"); err == nil {
		t.Fatal("expected invalid regression suite status error")
	}
	if _, err := ParseRegressionSuiteStatus(""); err == nil {
		t.Fatal("expected empty regression suite status error")
	}
	if _, err := ParseRegressionCaseStatus("pending"); err == nil {
		t.Fatal("expected invalid regression case status error")
	}
	if _, err := ParseRegressionCaseStatus(""); err == nil {
		t.Fatal("expected empty regression case status error")
	}
	if _, err := ParseRegressionSeverity("critical"); err == nil {
		t.Fatal("expected invalid regression severity error")
	}
	if _, err := ParseRegressionSeverity(""); err == nil {
		t.Fatal("expected empty regression severity error")
	}
	if _, err := ParseRegressionPromotionMode("partial"); err == nil {
		t.Fatal("expected invalid promotion mode error")
	}
	if _, err := ParseRegressionPromotionMode(""); err == nil {
		t.Fatal("expected empty promotion mode error")
	}
}
