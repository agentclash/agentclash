package billing

import (
	"testing"
	"time"
)

// TestGuideAgentAllowanceMaterialization pins the §C limits: Free flat 50, Pro 1000·seats, Team
// 4000·seats, Enterprise unlimited (nil).
func TestGuideAgentAllowanceMaterialization(t *testing.T) {
	free := MaterializeEntitlements(MustPlan(PlanFree), PeriodMonthly, 1, "active")
	assertIntPtr(t, "free guide turns", free.GuideAgentTurnsPerWorkspaceMonth, 50)

	pro := MaterializeEntitlements(MustPlan(PlanPro), PeriodMonthly, 5, "active")
	assertIntPtr(t, "pro guide turns (1000*5)", pro.GuideAgentTurnsPerWorkspaceMonth, 5000)

	team := MaterializeEntitlements(MustPlan(PlanTeam), PeriodYearly, 2, "active")
	assertIntPtr(t, "team guide turns (4000*2)", team.GuideAgentTurnsPerWorkspaceMonth, 8000)

	enterprise := MaterializeEntitlements(MustPlan(PlanEnterprise), PeriodCustom, 3, "active")
	if enterprise.GuideAgentTurnsPerWorkspaceMonth != nil {
		t.Fatalf("enterprise guide turns = %v, want nil (custom/unlimited)", *enterprise.GuideAgentTurnsPerWorkspaceMonth)
	}
}

func TestCheckGuideAgentAllowance(t *testing.T) {
	reset := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	limited := MaterializeEntitlements(MustPlan(PlanFree), PeriodMonthly, 1, "active") // 50

	// Under the limit → allowed.
	if d := CheckGuideAgentAllowance(limited, 49, 1, reset); !d.Allowed {
		t.Fatalf("used 49 + 1 should be allowed, got %+v", d)
	}
	// At the limit → blocked, structured quota decision.
	d := CheckGuideAgentAllowance(limited, 50, 1, reset)
	if d.Allowed || d.Code != GateCodeQuotaExceeded {
		t.Fatalf("used 50 + 1 should be quota-exceeded, got %+v", d)
	}
	if d.Limit == nil || *d.Limit != 50 || d.Remaining == nil || *d.Remaining != 0 {
		t.Fatalf("decision limits = %+v, want limit 50 / remaining 0", d)
	}

	// Unlimited (nil) → always allowed, even at a high used count.
	unlimited := MaterializeEntitlements(MustPlan(PlanEnterprise), PeriodCustom, 1, "active")
	if d := CheckGuideAgentAllowance(unlimited, 1_000_000, 1, reset); !d.Allowed {
		t.Fatalf("nil limit must be unlimited, got %+v", d)
	}
}
