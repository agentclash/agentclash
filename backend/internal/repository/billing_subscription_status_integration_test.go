package repository_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func billingStatusTestSubscription(orgID uuid.UUID, subscriptionID string, status string, eventAt *time.Time) *repository.BillingSubscriptionInput {
	return &repository.BillingSubscriptionInput{
		OrganizationID:     orgID,
		DodoSubscriptionID: subscriptionID,
		DodoCustomerID:     "cus_" + subscriptionID,
		DodoProductID:      "agentclash_pro_monthly",
		PlanKey:            billing.PlanPro,
		BillingPeriod:      billing.PeriodMonthly,
		Status:             status,
		SeatQuantity:       5,
		AddonQuantities:    []byte(`{}`),
		LatestDodoEventAt:  eventAt,
	}
}

func TestRepositoryBillingWebhookWithoutEventTimeKeepsNewerStatus(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	subscriptionID := "sub_untimed_" + uuid.NewString()
	proEntitlements := billing.MaterializeEntitlements(billing.MustPlan(billing.PlanPro), billing.PeriodMonthly, 5, billing.EntitlementStatusActive)

	// An event with no time can make a subscription row that does not exist.
	if _, err := repo.ApplyBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
		WebhookID: "wh_untimed_insert_" + uuid.NewString(),
		EventType: "subscription.active",
		Payload:   []byte(`{"type":"subscription.active"}`),
	}, repository.BillingWebhookApplication{
		Subscription: billingStatusTestSubscription(fixture.organizationID, subscriptionID, "active", nil),
		Entitlements: &repository.BillingWebhookEntitlementsInput{
			OrganizationID:          fixture.organizationID,
			Entitlements:            proEntitlements,
			UseSubscriptionAsSource: true,
		},
	}); err != nil {
		t.Fatalf("untimed insert webhook returned error: %v", err)
	}
	overview, err := repo.GetBillingOverview(ctx, fixture.organizationID)
	if err != nil {
		t.Fatalf("GetBillingOverview returned error: %v", err)
	}
	if overview.Subscription == nil {
		t.Fatal("the untimed event did not insert the missing subscription")
	}
	if overview.Subscription.Status != "active" || overview.Subscription.LatestDodoEventAt != nil {
		t.Fatalf("subscription after the untimed insert = %q at %v, want active with no event time", overview.Subscription.Status, overview.Subscription.LatestDodoEventAt)
	}
	if overview.Entitlements.PlanKey != billing.PlanPro {
		t.Fatalf("plan after the untimed insert = %q, want pro", overview.Entitlements.PlanKey)
	}

	cancelledAt := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)
	if _, err := repo.ApplyBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
		WebhookID:      "wh_cancelled_" + uuid.NewString(),
		EventType:      "subscription.cancelled",
		EventTimestamp: &cancelledAt,
		Payload:        []byte(`{"type":"subscription.cancelled"}`),
	}, repository.BillingWebhookApplication{
		Subscription: billingStatusTestSubscription(fixture.organizationID, subscriptionID, "cancelled", &cancelledAt),
		Entitlements: &repository.BillingWebhookEntitlementsInput{
			OrganizationID: fixture.organizationID,
			Entitlements:   billing.DefaultEntitlements(),
		},
	}); err != nil {
		t.Fatalf("cancelled webhook returned error: %v", err)
	}

	// An old event with no time arrives after the cancellation.
	untimedWebhookID := "wh_untimed_active_" + uuid.NewString()
	duplicate, err := repo.ApplyBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
		WebhookID: untimedWebhookID,
		EventType: "subscription.active",
		Payload:   []byte(`{"type":"subscription.active"}`),
	}, repository.BillingWebhookApplication{
		Subscription: billingStatusTestSubscription(fixture.organizationID, subscriptionID, "active", nil),
		Entitlements: &repository.BillingWebhookEntitlementsInput{
			OrganizationID:          fixture.organizationID,
			Entitlements:            proEntitlements,
			UseSubscriptionAsSource: true,
		},
	})
	if err != nil {
		t.Fatalf("untimed active webhook returned error: %v", err)
	}
	if duplicate {
		t.Fatal("untimed active webhook is a new event, not a duplicate")
	}

	var eventStatus string
	if err := db.QueryRow(ctx, `SELECT status FROM billing_webhook_events WHERE webhook_id = $1`, untimedWebhookID).Scan(&eventStatus); err != nil {
		t.Fatalf("select untimed webhook event returned error: %v", err)
	}
	if eventStatus != "processed" {
		t.Fatalf("untimed webhook event status = %q, want processed", eventStatus)
	}
	overview, err = repo.GetBillingOverview(ctx, fixture.organizationID)
	if err != nil {
		t.Fatalf("GetBillingOverview returned error: %v", err)
	}
	if overview.Subscription == nil {
		t.Fatal("subscription after the untimed event is missing")
	}
	if overview.Subscription.Status != "cancelled" {
		t.Fatalf("status after the untimed event = %q, want cancelled", overview.Subscription.Status)
	}
	if overview.Subscription.LatestDodoEventAt == nil || !overview.Subscription.LatestDodoEventAt.Equal(cancelledAt) {
		t.Fatalf("latest event time = %v, want %v", overview.Subscription.LatestDodoEventAt, cancelledAt)
	}
	if overview.Entitlements.PlanKey != billing.PlanFree {
		t.Fatalf("plan after the untimed event = %q, want free", overview.Entitlements.PlanKey)
	}
}

func TestRepositoryBillingSubscriptionStatusCheckMatchesGo(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)
	const insert = `
		INSERT INTO billing_subscriptions (organization_id, dodo_subscription_id, dodo_product_id, plan_key, billing_period, status, seat_quantity)
		VALUES ($1, $2, 'agentclash_pro_monthly', 'pro', 'monthly', $3, 1)`

	for _, status := range billing.SubscriptionStatuses() {
		if _, err := db.Exec(ctx, insert, fixture.organizationID, "sub_check_"+uuid.NewString(), string(status)); err != nil {
			t.Errorf("the CHECK constraint refused the Go status %q: %v", status, err)
		}
	}

	_, err := db.Exec(ctx, insert, fixture.organizationID, "sub_check_"+uuid.NewString(), "grace_period")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "billing_subscriptions_status_check" {
		t.Fatalf("insert of status grace_period returned %v, want SQLSTATE 23514 from billing_subscriptions_status_check", err)
	}

	eventAt := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)
	_, err = repo.UpsertBillingSubscription(ctx, *billingStatusTestSubscription(fixture.organizationID, "sub_check_"+uuid.NewString(), "succeeded", &eventAt))
	if !errors.Is(err, billing.ErrInvalidSubscriptionStatus) {
		t.Fatalf("UpsertBillingSubscription with status succeeded returned %v, want ErrInvalidSubscriptionStatus", err)
	}
}

func TestRepositoryBillingSubscriptionStatusMigrationMapsLegacyValues(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	fixture := seedFixture(t, ctx, db)

	body, err := os.ReadFile("../../db/migrations/00099_billing_subscriptions_status_check.sql")
	if err != nil {
		t.Fatalf("read the status migration: %v", err)
	}
	_, afterUp, found := strings.Cut(string(body), "-- +goose Up")
	if !found {
		t.Fatal("the status migration has no Up section")
	}
	up, _, found := strings.Cut(afterUp, "-- +goose Down")
	if !found {
		t.Fatal("the status migration has no Down section")
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Put the table back in its state before the migration.
	if _, err := tx.Exec(ctx, `ALTER TABLE billing_subscriptions DROP CONSTRAINT IF EXISTS billing_subscriptions_status_check`); err != nil {
		t.Fatalf("drop the status constraint: %v", err)
	}
	want := map[string]string{
		"active":       "active",
		" Active ":     "active",
		"renewed":      "active",
		"succeeded":    "active",
		"canceled":     "cancelled",
		"ON_HOLD":      "on_hold",
		"trialing":     "trialing",
		"past_due":     "past_due",
		"grace_period": "unknown",
		"":             "unknown",
	}
	ids := make(map[string]string, len(want))
	for legacy := range want {
		id := "sub_legacy_" + uuid.NewString()
		ids[legacy] = id
		if _, err := tx.Exec(ctx, `
			INSERT INTO billing_subscriptions (organization_id, dodo_subscription_id, dodo_product_id, plan_key, billing_period, status, seat_quantity)
			VALUES ($1, $2, 'agentclash_pro_monthly', 'pro', 'monthly', $3, 1)
		`, fixture.organizationID, id, legacy); err != nil {
			t.Fatalf("insert legacy status %q: %v", legacy, err)
		}
	}

	if _, err := tx.Exec(ctx, up); err != nil {
		t.Fatalf("run the Up section of the status migration: %v", err)
	}

	for legacy, wantStatus := range want {
		var got string
		if err := tx.QueryRow(ctx, `SELECT status FROM billing_subscriptions WHERE dodo_subscription_id = $1`, ids[legacy]).Scan(&got); err != nil {
			t.Fatalf("select migrated status for %q: %v", legacy, err)
		}
		if got != wantStatus {
			t.Errorf("legacy status %q became %q, want %q", legacy, got, wantStatus)
		}
	}
	var validated bool
	if err := tx.QueryRow(ctx, `
		SELECT convalidated FROM pg_constraint
		WHERE conrelid = 'billing_subscriptions'::regclass AND conname = 'billing_subscriptions_status_check'
	`).Scan(&validated); err != nil {
		t.Fatalf("select the status constraint: %v", err)
	}
	if !validated {
		t.Fatal("billing_subscriptions_status_check is not validated after the migration")
	}
}
