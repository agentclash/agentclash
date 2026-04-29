package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
)

func TestRepositoryBillingEntitlementDefaultsAndWebhookIdempotency(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	orgID, entitlements, err := repo.ResolveWorkspaceEntitlements(ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("ResolveWorkspaceEntitlements returned error: %v", err)
	}
	if orgID != fixture.organizationID {
		t.Fatalf("org id = %s, want %s", orgID, fixture.organizationID)
	}
	if entitlements.PlanKey != billing.PlanFree {
		t.Fatalf("default plan = %q, want free", entitlements.PlanKey)
	}
	assertIntPtr(t, "free race quota", entitlements.RacesPerWorkspaceMonth, 25)

	eventTime := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)
	if err := repo.RecordBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
		WebhookID:      "wh_repo_test",
		EventType:      "subscription.active",
		EventTimestamp: &eventTime,
		Payload:        []byte(`{"type":"subscription.active"}`),
	}); err != nil {
		t.Fatalf("RecordBillingWebhookEvent first returned error: %v", err)
	}
	err = repo.RecordBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
		WebhookID:      "wh_repo_test",
		EventType:      "subscription.active",
		EventTimestamp: &eventTime,
		Payload:        []byte(`{"type":"subscription.active"}`),
	})
	if !errors.Is(err, repository.ErrBillingWebhookAlreadyProcessed) {
		t.Fatalf("RecordBillingWebhookEvent duplicate error = %v, want ErrBillingWebhookAlreadyProcessed", err)
	}

	proEntitlements := billing.MaterializeEntitlements(billing.MustPlan(billing.PlanPro), billing.PeriodMonthly, 5, "active")
	subscription, err := repo.UpsertBillingSubscription(ctx, repository.BillingSubscriptionInput{
		OrganizationID:     fixture.organizationID,
		DodoSubscriptionID: "sub_repo_test",
		DodoCustomerID:     "cus_repo_test",
		DodoProductID:      "agentclash_pro_monthly",
		PlanKey:            billing.PlanPro,
		BillingPeriod:      billing.PeriodMonthly,
		Status:             "active",
		SeatQuantity:       5,
		AddonQuantities:    []byte(`{}`),
		LatestDodoEventAt:  &eventTime,
	})
	if err != nil {
		t.Fatalf("UpsertBillingSubscription returned error: %v", err)
	}
	if err := repo.UpsertOrganizationEntitlements(ctx, fixture.organizationID, proEntitlements, &subscription.ID, nil); err != nil {
		t.Fatalf("UpsertOrganizationEntitlements returned error: %v", err)
	}
	overview, err := repo.GetBillingOverview(ctx, fixture.organizationID)
	if err != nil {
		t.Fatalf("GetBillingOverview returned error: %v", err)
	}
	if overview.Entitlements.PlanKey != billing.PlanPro {
		t.Fatalf("overview plan = %q, want pro", overview.Entitlements.PlanKey)
	}
	if overview.Subscription == nil || overview.Subscription.DodoSubscriptionID != "sub_repo_test" {
		t.Fatalf("overview subscription = %+v, want sub_repo_test", overview.Subscription)
	}
}

func TestRepositoryRunEntitlementGateConsumesQuotaAndBlocksOverage(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	windowStart, windowEnd := billingTestWindow()

	entitlements := billing.DefaultEntitlements()
	entitlements.ConcurrentRaces = nil
	entitlementGate := &repository.RunEntitlementGate{
		Entitlements:    entitlements,
		RaceCost:        25,
		ConcurrencyCost: 0,
		WindowStart:     windowStart,
		WindowEnd:       windowEnd,
	}
	if _, err := repo.CreateQueuedRun(ctx, billingTestRunParams(fixture, "quota fill", entitlementGate)); err != nil {
		t.Fatalf("CreateQueuedRun fill returned error: %v", err)
	}
	usage, err := repo.GetWorkspaceUsageSnapshot(ctx, fixture.workspaceID, windowStart, windowEnd)
	if err != nil {
		t.Fatalf("GetWorkspaceUsageSnapshot returned error: %v", err)
	}
	if usage.RaceCount != 25 {
		t.Fatalf("race count = %d, want 25", usage.RaceCount)
	}

	entitlementGate.RaceCost = 1
	_, err = repo.CreateQueuedRun(ctx, billingTestRunParams(fixture, "quota overage", entitlementGate))
	var gateErr billing.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("CreateQueuedRun overage error = %v, want billing.GateError", err)
	}
	if gateErr.Decision.Code != billing.GateCodeQuotaExceeded {
		t.Fatalf("gate code = %q, want %q", gateErr.Decision.Code, billing.GateCodeQuotaExceeded)
	}
}

func TestRepositoryRunEntitlementGateBlocksConcurrentOverage(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	windowStart, windowEnd := billingTestWindow()

	entitlements := billing.DefaultEntitlements()
	entitlements.RacesPerWorkspaceMonth = nil
	gate := &repository.RunEntitlementGate{
		Entitlements:    entitlements,
		RaceCost:        1,
		ConcurrencyCost: 1,
		WindowStart:     windowStart,
		WindowEnd:       windowEnd,
	}
	if _, err := repo.CreateQueuedRun(ctx, billingTestRunParams(fixture, "concurrency fill", gate)); err != nil {
		t.Fatalf("CreateQueuedRun fill returned error: %v", err)
	}
	_, err := repo.CreateQueuedRun(ctx, billingTestRunParams(fixture, "concurrency overage", gate))
	var gateErr billing.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("CreateQueuedRun overage error = %v, want billing.GateError", err)
	}
	if gateErr.Decision.Code != billing.GateCodeConcurrencyLimitExceeded {
		t.Fatalf("gate code = %q, want %q", gateErr.Decision.Code, billing.GateCodeConcurrencyLimitExceeded)
	}
}

func billingTestRunParams(fixture testFixture, name string, gate *repository.RunEntitlementGate) repository.CreateQueuedRunParams {
	return repository.CreateQueuedRunParams{
		OrganizationID:         fixture.organizationID,
		WorkspaceID:            fixture.workspaceID,
		ChallengePackVersionID: fixture.challengePackVersionID,
		ChallengeInputSetID:    &fixture.challengeInputSetID,
		OfficialPackMode:       domain.OfficialPackModeFull,
		CreatedByUserID:        &fixture.userID,
		Name:                   name,
		ExecutionMode:          "single_agent",
		ExecutionPlan:          []byte(`{}`),
		RunAgents: []repository.CreateQueuedRunAgentParams{
			{
				AgentDeploymentID:         fixture.agentDeploymentID,
				AgentDeploymentSnapshotID: fixture.agentDeploymentSnapshotID,
				LaneIndex:                 0,
				Label:                     "Agent",
			},
		},
		EntitlementGate: gate,
	}
}

func billingTestWindow() (time.Time, time.Time) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
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
