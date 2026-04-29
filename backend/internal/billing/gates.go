package billing

import (
	"fmt"
	"time"
)

const (
	GateCodePlanLimitExceeded        = "plan_limit_exceeded"
	GateCodeQuotaExceeded            = "quota_exceeded"
	GateCodeConcurrencyLimitExceeded = "concurrency_limit_exceeded"
	GateCodeSeatLimitExceeded        = "seat_limit_exceeded"
	GateCodeFeatureNotEntitled       = "feature_not_entitled"
)

type GateDecision struct {
	Allowed       bool       `json:"allowed"`
	Code          string     `json:"code,omitempty"`
	Message       string     `json:"message,omitempty"`
	PlanKey       string     `json:"plan_key"`
	UpgradeTarget string     `json:"upgrade_target,omitempty"`
	Limit         *int       `json:"limit,omitempty"`
	Used          int        `json:"used,omitempty"`
	Remaining     *int       `json:"remaining,omitempty"`
	ResetAt       *time.Time `json:"reset_at,omitempty"`
}

type GateError struct {
	Decision GateDecision
}

func (e GateError) Error() string {
	if e.Decision.Message != "" {
		return e.Decision.Message
	}
	if e.Decision.Code != "" {
		return e.Decision.Code
	}
	return "billing gate denied"
}

func Allow(entitlements EffectiveEntitlements) GateDecision {
	return GateDecision{
		Allowed:       true,
		PlanKey:       entitlements.PlanKey,
		UpgradeTarget: entitlements.UpgradeTarget,
	}
}

func CheckMaxModels(entitlements EffectiveEntitlements, participantCount int) GateDecision {
	if entitlements.MaxModelsPerRace == nil || participantCount <= *entitlements.MaxModelsPerRace {
		return Allow(entitlements)
	}
	remaining := *entitlements.MaxModelsPerRace
	return GateDecision{
		Allowed:       false,
		Code:          GateCodePlanLimitExceeded,
		Message:       fmt.Sprintf("%s allows up to %d models per race", entitlements.PlanKey, *entitlements.MaxModelsPerRace),
		PlanKey:       entitlements.PlanKey,
		UpgradeTarget: entitlements.UpgradeTarget,
		Limit:         cloneInt(entitlements.MaxModelsPerRace),
		Used:          participantCount,
		Remaining:     &remaining,
	}
}

func CheckRaceQuota(entitlements EffectiveEntitlements, used int, requested int, resetAt time.Time) GateDecision {
	if entitlements.RacesPerWorkspaceMonth == nil || used+requested <= *entitlements.RacesPerWorkspaceMonth {
		return Allow(entitlements)
	}
	remaining := *entitlements.RacesPerWorkspaceMonth - used
	if remaining < 0 {
		remaining = 0
	}
	return GateDecision{
		Allowed:       false,
		Code:          GateCodeQuotaExceeded,
		Message:       fmt.Sprintf("%s workspace race quota is exhausted", entitlements.PlanKey),
		PlanKey:       entitlements.PlanKey,
		UpgradeTarget: entitlements.UpgradeTarget,
		Limit:         cloneInt(entitlements.RacesPerWorkspaceMonth),
		Used:          used,
		Remaining:     &remaining,
		ResetAt:       &resetAt,
	}
}

func CheckConcurrency(entitlements EffectiveEntitlements, active int, requested int) GateDecision {
	if entitlements.ConcurrentRaces == nil || active+requested <= *entitlements.ConcurrentRaces {
		return Allow(entitlements)
	}
	remaining := *entitlements.ConcurrentRaces - active
	if remaining < 0 {
		remaining = 0
	}
	return GateDecision{
		Allowed:       false,
		Code:          GateCodeConcurrencyLimitExceeded,
		Message:       fmt.Sprintf("%s allows up to %d concurrent races", entitlements.PlanKey, *entitlements.ConcurrentRaces),
		PlanKey:       entitlements.PlanKey,
		UpgradeTarget: entitlements.UpgradeTarget,
		Limit:         cloneInt(entitlements.ConcurrentRaces),
		Used:          active,
		Remaining:     &remaining,
	}
}

func CheckSeatLimit(entitlements EffectiveEntitlements, activeSeats int, requestedSeats int) GateDecision {
	if entitlements.SeatsLimit == nil || activeSeats+requestedSeats <= *entitlements.SeatsLimit {
		return Allow(entitlements)
	}
	remaining := *entitlements.SeatsLimit - activeSeats
	if remaining < 0 {
		remaining = 0
	}
	return GateDecision{
		Allowed:       false,
		Code:          GateCodeSeatLimitExceeded,
		Message:       fmt.Sprintf("%s seat limit is exhausted", entitlements.PlanKey),
		PlanKey:       entitlements.PlanKey,
		UpgradeTarget: entitlements.UpgradeTarget,
		Limit:         cloneInt(entitlements.SeatsLimit),
		Used:          activeSeats,
		Remaining:     &remaining,
	}
}

func CheckWorkspaceLimit(entitlements EffectiveEntitlements, activeWorkspaces int, requestedWorkspaces int) GateDecision {
	if entitlements.WorkspacesLimit == nil || activeWorkspaces+requestedWorkspaces <= *entitlements.WorkspacesLimit {
		return Allow(entitlements)
	}
	remaining := *entitlements.WorkspacesLimit - activeWorkspaces
	if remaining < 0 {
		remaining = 0
	}
	return GateDecision{
		Allowed:       false,
		Code:          GateCodePlanLimitExceeded,
		Message:       fmt.Sprintf("%s workspace limit is exhausted", entitlements.PlanKey),
		PlanKey:       entitlements.PlanKey,
		UpgradeTarget: entitlements.UpgradeTarget,
		Limit:         cloneInt(entitlements.WorkspacesLimit),
		Used:          activeWorkspaces,
		Remaining:     &remaining,
	}
}

func CheckFeature(entitlements EffectiveEntitlements, feature string) GateDecision {
	if entitlements.FeatureFlags[feature] {
		return Allow(entitlements)
	}
	return GateDecision{
		Allowed:       false,
		Code:          GateCodeFeatureNotEntitled,
		Message:       fmt.Sprintf("%s is not enabled on %s", feature, entitlements.PlanKey),
		PlanKey:       entitlements.PlanKey,
		UpgradeTarget: entitlements.UpgradeTarget,
	}
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
