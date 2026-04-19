package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrInvalidRegressionSuiteStatus = errors.New("invalid regression suite status")
	ErrInvalidRegressionCaseStatus  = errors.New("invalid regression case status")
	ErrInvalidRegressionSeverity    = errors.New("invalid regression severity")
	ErrInvalidPromotionMode         = errors.New("invalid regression promotion mode")
	ErrInvalidPromotionOverrides    = errors.New("invalid regression promotion overrides")
)

type RegressionSuiteStatus string

const (
	RegressionSuiteStatusActive   RegressionSuiteStatus = "active"
	RegressionSuiteStatusArchived RegressionSuiteStatus = "archived"
)

var regressionSuiteTransitions = map[RegressionSuiteStatus]map[RegressionSuiteStatus]struct{}{
	RegressionSuiteStatusActive: {
		RegressionSuiteStatusArchived: {},
	},
	// Reactivation is allowed; repository uniqueness checks still guard
	// workspace/name collisions if another active suite has claimed the name.
	RegressionSuiteStatusArchived: {
		RegressionSuiteStatusActive: {},
	},
}

func ParseRegressionSuiteStatus(raw string) (RegressionSuiteStatus, error) {
	status := RegressionSuiteStatus(raw)
	if !status.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidRegressionSuiteStatus, raw)
	}
	return status, nil
}

func (s RegressionSuiteStatus) Valid() bool {
	_, ok := regressionSuiteTransitions[s]
	return ok
}

func (s RegressionSuiteStatus) CanTransitionTo(next RegressionSuiteStatus) bool {
	nextStatuses, ok := regressionSuiteTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

type RegressionCaseStatus string

const (
	RegressionCaseStatusActive   RegressionCaseStatus = "active"
	RegressionCaseStatusMuted    RegressionCaseStatus = "muted"
	RegressionCaseStatusArchived RegressionCaseStatus = "archived"
)

var regressionCaseTransitions = map[RegressionCaseStatus]map[RegressionCaseStatus]struct{}{
	RegressionCaseStatusActive: {
		RegressionCaseStatusMuted:    {},
		RegressionCaseStatusArchived: {},
	},
	RegressionCaseStatusMuted: {
		RegressionCaseStatusActive:   {},
		RegressionCaseStatusArchived: {},
	},
	RegressionCaseStatusArchived: {},
}

func ParseRegressionCaseStatus(raw string) (RegressionCaseStatus, error) {
	status := RegressionCaseStatus(raw)
	if !status.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidRegressionCaseStatus, raw)
	}
	return status, nil
}

func (s RegressionCaseStatus) Valid() bool {
	_, ok := regressionCaseTransitions[s]
	return ok
}

func (s RegressionCaseStatus) CanTransitionTo(next RegressionCaseStatus) bool {
	nextStatuses, ok := regressionCaseTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

type RegressionSeverity string

const (
	RegressionSeverityInfo     RegressionSeverity = "info"
	RegressionSeverityWarning  RegressionSeverity = "warning"
	RegressionSeverityBlocking RegressionSeverity = "blocking"
)

var regressionSeverities = map[RegressionSeverity]struct{}{
	RegressionSeverityInfo:     {},
	RegressionSeverityWarning:  {},
	RegressionSeverityBlocking: {},
}

func ParseRegressionSeverity(raw string) (RegressionSeverity, error) {
	severity := RegressionSeverity(raw)
	if !severity.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidRegressionSeverity, raw)
	}
	return severity, nil
}

func (s RegressionSeverity) Valid() bool {
	_, ok := regressionSeverities[s]
	return ok
}

type RegressionPromotionMode string

const (
	RegressionPromotionModeFullExecutable RegressionPromotionMode = "full_executable"
	RegressionPromotionModeOutputOnly     RegressionPromotionMode = "output_only"
	RegressionPromotionModeManual         RegressionPromotionMode = "manual"
)

var regressionPromotionModes = map[RegressionPromotionMode]struct{}{
	RegressionPromotionModeFullExecutable: {},
	RegressionPromotionModeOutputOnly:     {},
	RegressionPromotionModeManual:         {},
}

func ParseRegressionPromotionMode(raw string) (RegressionPromotionMode, error) {
	mode := RegressionPromotionMode(raw)
	if !mode.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidPromotionMode, raw)
	}
	return mode, nil
}

func (m RegressionPromotionMode) Valid() bool {
	_, ok := regressionPromotionModes[m]
	return ok
}

type PromotionRequest struct {
	SuiteID             uuid.UUID
	PromotionMode       RegressionPromotionMode
	Title               string
	FailureSummary      string
	Severity            *RegressionSeverity
	ValidatorOverrides  json.RawMessage
	Metadata            json.RawMessage
}

type PromotionOverrides struct {
	JudgeThresholdOverrides map[string]float64 `json:"judge_threshold_overrides,omitempty"`
	AssertionToggles        map[string]bool    `json:"assertion_toggles,omitempty"`
}

func DefaultPromotionSeverityForFailureClass(failureClass string) RegressionSeverity {
	switch strings.TrimSpace(failureClass) {
	case "policy_violation", "sandbox_failure":
		return RegressionSeverityBlocking
	default:
		return RegressionSeverityWarning
	}
}

func ValidatePromotionOverrides(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
		return nil, fmt.Errorf("%w: must be a JSON object", ErrInvalidPromotionOverrides)
	}

	for key := range envelope {
		switch key {
		case "judge_threshold_overrides", "assertion_toggles":
		default:
			return nil, fmt.Errorf("%w: unsupported key %q", ErrInvalidPromotionOverrides, key)
		}
	}

	var overrides PromotionOverrides
	if payload, ok := envelope["judge_threshold_overrides"]; ok {
		if err := json.Unmarshal(payload, &overrides.JudgeThresholdOverrides); err != nil {
			return nil, fmt.Errorf("%w: judge_threshold_overrides must be a string:number map", ErrInvalidPromotionOverrides)
		}
	}
	if payload, ok := envelope["assertion_toggles"]; ok {
		if err := json.Unmarshal(payload, &overrides.AssertionToggles); err != nil {
			return nil, fmt.Errorf("%w: assertion_toggles must be a string:boolean map", ErrInvalidPromotionOverrides)
		}
	}

	normalized, err := json.Marshal(overrides)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPromotionOverrides, err)
	}
	return normalized, nil
}
