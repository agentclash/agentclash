package domain

import (
	"encoding/json"
	"testing"
)

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
		{name: "proposed to active", from: RegressionCaseStatusProposed, to: RegressionCaseStatusActive, want: true},
		{name: "proposed to archived", from: RegressionCaseStatusProposed, to: RegressionCaseStatusArchived, want: true},
		{name: "proposed to rejected", from: RegressionCaseStatusProposed, to: RegressionCaseStatusRejected, want: true},
		{name: "active to muted", from: RegressionCaseStatusActive, to: RegressionCaseStatusMuted, want: true},
		{name: "active to archived", from: RegressionCaseStatusActive, to: RegressionCaseStatusArchived, want: true},
		{name: "muted to active", from: RegressionCaseStatusMuted, to: RegressionCaseStatusActive, want: true},
		{name: "muted to archived", from: RegressionCaseStatusMuted, to: RegressionCaseStatusArchived, want: true},
		{name: "archived to active denied", from: RegressionCaseStatusArchived, to: RegressionCaseStatusActive, want: false},
		{name: "archived to muted denied", from: RegressionCaseStatusArchived, to: RegressionCaseStatusMuted, want: false},
		{name: "rejected to active denied", from: RegressionCaseStatusRejected, to: RegressionCaseStatusActive, want: false},
		{name: "rejected to proposed denied", from: RegressionCaseStatusRejected, to: RegressionCaseStatusProposed, want: false},
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

func TestDefaultPromotionSeverityForFailureClass(t *testing.T) {
	tests := []struct {
		failureClass string
		want         RegressionSeverity
	}{
		{failureClass: "policy_violation", want: RegressionSeverityBlocking},
		{failureClass: "sandbox_failure", want: RegressionSeverityBlocking},
		{failureClass: "incorrect_final_output", want: RegressionSeverityWarning},
	}

	for _, tc := range tests {
		if got := DefaultPromotionSeverityForFailureClass(tc.failureClass); got != tc.want {
			t.Fatalf("DefaultPromotionSeverityForFailureClass(%q) = %q, want %q", tc.failureClass, got, tc.want)
		}
	}
}

func TestValidatePromotionOverrides(t *testing.T) {
	valid, err := ValidatePromotionOverrides(json.RawMessage(`{
		"judge_threshold_overrides":{"policy.filesystem":0.9},
		"assertion_toggles":{"capture.files":true}
	}`))
	if err != nil {
		t.Fatalf("ValidatePromotionOverrides returned error: %v", err)
	}
	if len(valid) == 0 {
		t.Fatal("expected normalized overrides JSON")
	}

	if _, err := ValidatePromotionOverrides(json.RawMessage(`{"unsupported":true}`)); err == nil {
		t.Fatal("expected unsupported-key validation error")
	}
	if _, err := ValidatePromotionOverrides(json.RawMessage(`{"judge_threshold_overrides":["bad"]}`)); err == nil {
		t.Fatal("expected invalid threshold map validation error")
	}
	if value, err := ValidatePromotionOverrides(nil); err != nil || value != nil {
		t.Fatalf("ValidatePromotionOverrides(nil) = (%s, %v), want (nil, nil)", value, err)
	}
}
