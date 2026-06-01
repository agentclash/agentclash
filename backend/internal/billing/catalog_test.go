package billing

import (
	"errors"
	"testing"
	"time"
)

func TestCatalogContainsPublicPlansAndLimits(t *testing.T) {
	plans := Catalog()
	if len(plans) != 4 {
		t.Fatalf("Catalog length = %d, want 4", len(plans))
	}

	free := MustPlan(PlanFree)
	freeEntitlements := MaterializeEntitlements(free, PeriodMonthly, 1, "active")
	assertIntPtr(t, "free seats", freeEntitlements.SeatsLimit, 1)
	assertIntPtr(t, "free workspaces", freeEntitlements.WorkspacesLimit, 1)
	assertIntPtr(t, "free quota", freeEntitlements.RacesPerWorkspaceMonth, 25)
	assertIntPtr(t, "free models", freeEntitlements.MaxModelsPerRace, 4)
	assertIntPtr(t, "free retention", freeEntitlements.ReplayRetentionDays, 7)
	assertIntPtr(t, "free concurrency", freeEntitlements.ConcurrentRaces, 1)

	proEntitlements := MaterializeEntitlements(MustPlan(PlanPro), PeriodMonthly, 5, "active")
	assertIntPtr(t, "pro quota", proEntitlements.RacesPerWorkspaceMonth, 2500)
	assertIntPtr(t, "pro models", proEntitlements.MaxModelsPerRace, 8)
	assertIntPtr(t, "pro concurrency", proEntitlements.ConcurrentRaces, 3)

	teamEntitlements := MaterializeEntitlements(MustPlan(PlanTeam), PeriodYearly, 2, "active")
	assertIntPtr(t, "team quota", teamEntitlements.RacesPerWorkspaceMonth, 4000)
	assertIntPtr(t, "team models", teamEntitlements.MaxModelsPerRace, 12)
	assertIntPtr(t, "team retention", teamEntitlements.ReplayRetentionDays, 90)
	assertIntPtr(t, "team concurrency", teamEntitlements.ConcurrentRaces, 10)
	if teamEntitlements.WorkspacesLimit != nil {
		t.Fatalf("team workspaces limit = %d, want unlimited", *teamEntitlements.WorkspacesLimit)
	}
}

func TestDodoProductMappingAndSeatValidation(t *testing.T) {
	plans := CatalogWithDodoProductIDs(testDodoProductIDs())
	mapping, err := MapDodoProductFromCatalog("agentclash_pro_yearly", plans)
	if err != nil {
		t.Fatalf("MapDodoProduct returned error: %v", err)
	}
	if mapping.PlanKey != PlanPro || mapping.BillingPeriod != PeriodYearly {
		t.Fatalf("mapping = %+v, want pro yearly", mapping)
	}

	_, err = SubscriptionEntitlementsFromCatalog(DodoSubscriptionInput{
		ProductID: "agentclash_pro_monthly",
		Status:    "active",
		Quantity:  4,
	}, plans)
	if !errors.Is(err, ErrSeatQuantityBelowLimit) {
		t.Fatalf("SubscriptionEntitlements error = %v, want ErrSeatQuantityBelowLimit", err)
	}

	entitlements, err := SubscriptionEntitlementsFromCatalog(DodoSubscriptionInput{
		ProductID: "agentclash_team_monthly",
		Status:    "trialing",
		Quantity:  3,
	}, plans)
	if err != nil {
		t.Fatalf("SubscriptionEntitlements returned error: %v", err)
	}
	if entitlements.PlanKey != PlanTeam || entitlements.BillingPeriod != PeriodMonthly {
		t.Fatalf("entitlements = %+v, want team monthly", entitlements)
	}
	if entitlements.Status != EntitlementStatusTrialing {
		t.Fatalf("entitlement status = %q, want trialing", entitlements.Status)
	}
	assertIntPtr(t, "team per-seat quota", entitlements.RacesPerWorkspaceMonth, 6000)
}

func TestInactiveDodoStatusDoesNotGrantPaidEntitlements(t *testing.T) {
	_, err := SubscriptionEntitlementsFromCatalog(DodoSubscriptionInput{
		ProductID: "agentclash_pro_monthly",
		Status:    "cancelled",
		Quantity:  5,
	}, CatalogWithDodoProductIDs(testDodoProductIDs()))
	if !errors.Is(err, ErrInactiveDodoStatus) {
		t.Fatalf("SubscriptionEntitlements error = %v, want ErrInactiveDodoStatus", err)
	}
}

func testDodoProductIDs() DodoProductIDs {
	return DodoProductIDs{
		ProMonthly:  "agentclash_pro_monthly",
		ProYearly:   "agentclash_pro_yearly",
		TeamMonthly: "agentclash_team_monthly",
		TeamYearly:  "agentclash_team_yearly",
	}
}

func TestGateDecisionsCarryStructuredLimits(t *testing.T) {
	entitlements := MaterializeEntitlements(MustPlan(PlanFree), PeriodMonthly, 1, "active")

	decision := CheckMaxModels(entitlements, 5)
	if decision.Allowed {
		t.Fatal("CheckMaxModels allowed 5 models on Free")
	}
	if decision.Code != GateCodePlanLimitExceeded {
		t.Fatalf("code = %q, want %q", decision.Code, GateCodePlanLimitExceeded)
	}
	assertIntPtr(t, "max models limit", decision.Limit, 4)
	if decision.Used != 5 {
		t.Fatalf("used = %d, want 5", decision.Used)
	}

	resetAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	decision = CheckRaceQuota(entitlements, 25, 1, resetAt)
	if decision.Code != GateCodeQuotaExceeded {
		t.Fatalf("quota code = %q, want %q", decision.Code, GateCodeQuotaExceeded)
	}
	if decision.ResetAt == nil || !decision.ResetAt.Equal(resetAt) {
		t.Fatalf("reset_at = %v, want %v", decision.ResetAt, resetAt)
	}
}

func TestExpiredTrialEntitlementBlocksGates(t *testing.T) {
	expiresAt := time.Now().UTC().Add(-time.Minute)
	entitlements := MaterializeEntitlements(MustPlan(PlanPro), PeriodMonthly, 5, EntitlementStatusTrialing)
	entitlements.ExpiresAt = &expiresAt

	decision := CheckMaxModels(entitlements, 1)
	if decision.Allowed {
		t.Fatal("CheckMaxModels allowed expired trial")
	}
	if decision.Code != GateCodeEntitlementExpired {
		t.Fatalf("code = %q, want %q", decision.Code, GateCodeEntitlementExpired)
	}
	if decision.PlanKey != PlanPro || decision.UpgradeTarget != PlanPro {
		t.Fatalf("plan/upgrade = %q/%q, want pro/pro", decision.PlanKey, decision.UpgradeTarget)
	}
	if decision.ExpiresAt == nil || !decision.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expires_at = %v, want %v", decision.ExpiresAt, expiresAt)
	}
}

func TestEmptyEntitlementStatusFailsClosed(t *testing.T) {
	entitlements := MaterializeEntitlements(MustPlan(PlanPro), PeriodMonthly, 5, EntitlementStatusActive)
	entitlements.Status = ""

	decision := CheckMaxModels(entitlements, 1)
	if decision.Allowed {
		t.Fatal("CheckMaxModels allowed an entitlement with empty status")
	}
	if decision.Code != GateCodeEntitlementInactive {
		t.Fatalf("code = %q, want %q", decision.Code, GateCodeEntitlementInactive)
	}
}

func assertIntPtr(t *testing.T, label string, got *int, want int) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %d", label, want)
	}
	if *got != want {
		t.Fatalf("%s = %d, want %d", label, *got, want)
	}
}
