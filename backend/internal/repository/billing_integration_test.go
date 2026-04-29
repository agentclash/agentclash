package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
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
	webhookID := "wh_repo_test_" + uuid.NewString()
	if err := repo.RecordBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
		WebhookID:      webhookID,
		EventType:      "subscription.active",
		EventTimestamp: &eventTime,
		Payload:        []byte(`{"type":"subscription.active"}`),
	}); err != nil {
		t.Fatalf("RecordBillingWebhookEvent first returned error: %v", err)
	}
	err = repo.RecordBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
		WebhookID:      webhookID,
		EventType:      "subscription.active",
		EventTimestamp: &eventTime,
		Payload:        []byte(`{"type":"subscription.active"}`),
	})
	if !errors.Is(err, repository.ErrBillingWebhookAlreadyProcessed) {
		t.Fatalf("RecordBillingWebhookEvent duplicate error = %v, want ErrBillingWebhookAlreadyProcessed", err)
	}

	proEntitlements := billing.MaterializeEntitlements(billing.MustPlan(billing.PlanPro), billing.PeriodMonthly, 5, "active")
	subscriptionID := "sub_repo_test_" + uuid.NewString()
	subscription, err := repo.UpsertBillingSubscription(ctx, repository.BillingSubscriptionInput{
		OrganizationID:     fixture.organizationID,
		DodoSubscriptionID: subscriptionID,
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
	if overview.Subscription == nil || overview.Subscription.DodoSubscriptionID != subscriptionID {
		t.Fatalf("overview subscription = %+v, want %s", overview.Subscription, subscriptionID)
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

func TestRepositoryExpiredTrialEntitlementsBlockRunCreation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	windowStart, windowEnd := billingTestWindow()
	expiresAt := time.Now().UTC().Add(-time.Minute)

	entitlements := billing.MaterializeEntitlements(billing.MustPlan(billing.PlanPro), billing.PeriodMonthly, 5, billing.EntitlementStatusTrialing)
	entitlements.ExpiresAt = &expiresAt
	if err := repo.UpsertOrganizationEntitlements(ctx, fixture.organizationID, entitlements, nil, nil); err != nil {
		t.Fatalf("UpsertOrganizationEntitlements returned error: %v", err)
	}

	_, resolved, err := repo.ResolveWorkspaceEntitlements(ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("ResolveWorkspaceEntitlements returned error: %v", err)
	}
	if resolved.PlanKey != billing.PlanPro || resolved.Status != billing.EntitlementStatusExpired {
		t.Fatalf("resolved entitlements = %+v, want expired pro trial", resolved)
	}

	_, err = repo.CreateQueuedRun(ctx, billingTestRunParams(fixture, "expired trial blocked", &repository.RunEntitlementGate{
		Entitlements:    resolved,
		RaceCost:        1,
		ConcurrencyCost: 1,
		WindowStart:     windowStart,
		WindowEnd:       windowEnd,
	}))
	var gateErr billing.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("CreateQueuedRun expired trial error = %v, want billing.GateError", err)
	}
	if gateErr.Decision.Code != billing.GateCodeEntitlementExpired {
		t.Fatalf("gate code = %q, want %q", gateErr.Decision.Code, billing.GateCodeEntitlementExpired)
	}
}

func TestRepositoryBillingWebhookFailureIsRetryable(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	webhookID := "wh_retry_test_" + uuid.NewString()
	event := repository.BillingWebhookEventInput{
		WebhookID: webhookID,
		EventType: "subscription.active",
		Payload:   []byte(`{"type":"subscription.active"}`),
	}

	badEntitlements := billing.DefaultEntitlements()
	badEntitlements.PlanKey = "bad-plan"
	duplicate, err := repo.ApplyBillingWebhookEvent(ctx, event, repository.BillingWebhookApplication{
		Entitlements: &repository.BillingWebhookEntitlementsInput{
			OrganizationID: fixture.organizationID,
			Entitlements:   badEntitlements,
		},
	})
	if err == nil {
		t.Fatal("ApplyBillingWebhookEvent with invalid entitlements returned nil error")
	}
	if duplicate {
		t.Fatal("failed first delivery should not be duplicate")
	}

	var eventCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM billing_webhook_events WHERE webhook_id = $1`, webhookID).Scan(&eventCount); err != nil {
		t.Fatalf("count failed webhook rows returned error: %v", err)
	}
	if eventCount != 0 {
		t.Fatalf("failed webhook rows = %d, want rollback to leave 0", eventCount)
	}

	proEntitlements := billing.MaterializeEntitlements(billing.MustPlan(billing.PlanPro), billing.PeriodMonthly, 5, billing.EntitlementStatusActive)
	duplicate, err = repo.ApplyBillingWebhookEvent(ctx, event, repository.BillingWebhookApplication{
		Entitlements: &repository.BillingWebhookEntitlementsInput{
			OrganizationID: fixture.organizationID,
			Entitlements:   proEntitlements,
		},
	})
	if err != nil {
		t.Fatalf("retry ApplyBillingWebhookEvent returned error: %v", err)
	}
	if duplicate {
		t.Fatal("retry of failed webhook should not be duplicate")
	}
	var status string
	if err := db.QueryRow(ctx, `SELECT status FROM billing_webhook_events WHERE webhook_id = $1`, webhookID).Scan(&status); err != nil {
		t.Fatalf("select processed webhook status returned error: %v", err)
	}
	if status != "processed" {
		t.Fatalf("processed webhook status = %q, want processed", status)
	}

	duplicate, err = repo.ApplyBillingWebhookEvent(ctx, event, repository.BillingWebhookApplication{})
	if err != nil {
		t.Fatalf("duplicate processed ApplyBillingWebhookEvent returned error: %v", err)
	}
	if !duplicate {
		t.Fatal("processed webhook retry should be duplicate")
	}
}

func TestRepositoryBillingWebhookIgnoresStaleSubscriptionState(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	subscriptionID := "sub_stale_test_" + uuid.NewString()
	customerID := "cus_stale_test_" + uuid.NewString()
	olderEventAt := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)
	newerEventAt := olderEventAt.Add(time.Hour)

	freeEntitlements := billing.DefaultEntitlements()
	duplicate, err := repo.ApplyBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
		WebhookID:      "wh_cancelled_" + uuid.NewString(),
		EventType:      "subscription.cancelled",
		EventTimestamp: &newerEventAt,
		Payload:        []byte(`{"type":"subscription.cancelled"}`),
	}, repository.BillingWebhookApplication{
		Subscription: &repository.BillingSubscriptionInput{
			OrganizationID:     fixture.organizationID,
			DodoSubscriptionID: subscriptionID,
			DodoCustomerID:     customerID,
			DodoProductID:      "agentclash_pro_monthly",
			PlanKey:            billing.PlanPro,
			BillingPeriod:      billing.PeriodMonthly,
			Status:             "cancelled",
			SeatQuantity:       5,
			AddonQuantities:    []byte(`{}`),
			LatestDodoEventAt:  &newerEventAt,
		},
		Entitlements: &repository.BillingWebhookEntitlementsInput{
			OrganizationID: fixture.organizationID,
			Entitlements:   freeEntitlements,
		},
	})
	if err != nil {
		t.Fatalf("newer cancelled webhook returned error: %v", err)
	}
	if duplicate {
		t.Fatal("newer cancelled webhook should not be duplicate")
	}

	proEntitlements := billing.MaterializeEntitlements(billing.MustPlan(billing.PlanPro), billing.PeriodMonthly, 5, billing.EntitlementStatusActive)
	duplicate, err = repo.ApplyBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
		WebhookID:      "wh_older_active_" + uuid.NewString(),
		EventType:      "subscription.active",
		EventTimestamp: &olderEventAt,
		Payload:        []byte(`{"type":"subscription.active"}`),
	}, repository.BillingWebhookApplication{
		Subscription: &repository.BillingSubscriptionInput{
			OrganizationID:     fixture.organizationID,
			DodoSubscriptionID: subscriptionID,
			DodoCustomerID:     customerID,
			DodoProductID:      "agentclash_pro_monthly",
			PlanKey:            billing.PlanPro,
			BillingPeriod:      billing.PeriodMonthly,
			Status:             billing.EntitlementStatusActive,
			SeatQuantity:       5,
			AddonQuantities:    []byte(`{}`),
			LatestDodoEventAt:  &olderEventAt,
		},
		Entitlements: &repository.BillingWebhookEntitlementsInput{
			OrganizationID:          fixture.organizationID,
			Entitlements:            proEntitlements,
			UseSubscriptionAsSource: true,
		},
	})
	if err != nil {
		t.Fatalf("older active webhook returned error: %v", err)
	}
	if duplicate {
		t.Fatal("older active webhook should be recorded, not treated as duplicate")
	}

	overview, err := repo.GetBillingOverview(ctx, fixture.organizationID)
	if err != nil {
		t.Fatalf("GetBillingOverview returned error: %v", err)
	}
	if overview.Subscription == nil {
		t.Fatal("overview subscription is nil")
	}
	if overview.Subscription.Status != "cancelled" {
		t.Fatalf("subscription status = %q, want cancelled", overview.Subscription.Status)
	}
	if overview.Subscription.LatestDodoEventAt == nil || !overview.Subscription.LatestDodoEventAt.Equal(newerEventAt) {
		t.Fatalf("latest event = %v, want %v", overview.Subscription.LatestDodoEventAt, newerEventAt)
	}
	if overview.Entitlements.PlanKey != billing.PlanFree {
		t.Fatalf("entitlement plan = %q, want free after stale active webhook", overview.Entitlements.PlanKey)
	}
}

func TestRepositoryOrganizationEntitlementGatesBlockWorkspaceAndSeatWrites(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	entitlements := billing.DefaultEntitlements()
	gate := &repository.OrganizationEntitlementGate{
		OrganizationID: fixture.organizationID,
		Entitlements:   entitlements,
	}

	_, err := repo.CreateWorkspaceWithAdmin(ctx, repository.CreateWorkspaceWithAdminInput{
		OrganizationID:  fixture.organizationID,
		Name:            "Blocked Workspace",
		Slug:            "blocked-workspace",
		UserID:          fixture.userID,
		EntitlementGate: gate,
	})
	var gateErr billing.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("CreateWorkspaceWithAdmin error = %v, want billing.GateError", err)
	}
	if gateErr.Decision.Code != billing.GateCodePlanLimitExceeded {
		t.Fatalf("workspace gate code = %q, want %q", gateErr.Decision.Code, billing.GateCodePlanLimitExceeded)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO organization_memberships (id, organization_id, user_id, role, membership_status)
		VALUES (gen_random_uuid(), $1, $2, 'org_admin', 'active')
	`, fixture.organizationID, fixture.userID); err != nil {
		t.Fatalf("insert active org membership returned error: %v", err)
	}
	inviteeID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO users (id, workos_user_id, email, display_name)
		VALUES ($1, $2, $3, $4)
	`, inviteeID, "workos-invitee", "invitee@example.com", "Invitee"); err != nil {
		t.Fatalf("insert invitee returned error: %v", err)
	}

	_, err = repo.CreateOrgMembership(ctx, repository.CreateOrgMembershipInput{
		OrganizationID:  fixture.organizationID,
		UserID:          inviteeID,
		Role:            "org_member",
		EntitlementGate: gate,
	})
	if !errors.As(err, &gateErr) {
		t.Fatalf("CreateOrgMembership error = %v, want billing.GateError", err)
	}
	if gateErr.Decision.Code != billing.GateCodeSeatLimitExceeded {
		t.Fatalf("seat gate code = %q, want %q", gateErr.Decision.Code, billing.GateCodeSeatLimitExceeded)
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
