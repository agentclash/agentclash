package api

import (
	"context"
	"testing"
	"time"

	billingpkg "github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/google/uuid"
)

// The manager clock is 2026-04-29T00:00:00Z. Dodo stamps each delivery with
// this time in the webhook-timestamp header. The event times in the bodies are
// earlier, as they are for a real delivery.
const subscriptionStatusTestNow = 1777420800

func newSubscriptionStatusTestManager() (*BillingManager, *fakeBillingRepository, string) {
	repo := newFakeBillingRepository(uuid.New())
	secret := dodoTestWebhookSecret()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret:  secret,
		DodoProductIDs: testDodoProductIDs(),
	})
	manager.now = func() time.Time { return time.Unix(subscriptionStatusTestNow, 0).UTC() }
	return manager, repo, secret
}

func deliverSubscriptionStatusWebhook(t *testing.T, manager *BillingManager, secret string, webhookID string, body string) {
	t.Helper()
	headers := signedDodoHeaders(secret, webhookID, "1777420800", body)
	result, err := manager.ProcessDodoWebhook(context.Background(), headers, []byte(body))
	if err != nil {
		t.Fatalf("ProcessDodoWebhook(%s) returned error: %v", webhookID, err)
	}
	if result.Duplicate {
		t.Fatalf("ProcessDodoWebhook(%s) returned duplicate, want a new event", webhookID)
	}
}

func activateProSubscription(t *testing.T, manager *BillingManager, repo *fakeBillingRepository, secret string, subscriptionID string) {
	t.Helper()
	body := `{"business_id":"biz_test","type":"subscription.active","timestamp":"2026-04-28T22:00:00Z","data":{"payload_type":"Subscription","subscription_id":"` + subscriptionID + `","customer_id":"cus_status","product_id":"agentclash_pro_monthly","status":"active","quantity":5,"metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_activate_"+subscriptionID, body)
	if repo.entitlements.PlanKey != billingpkg.PlanPro || repo.entitlements.Status != billingpkg.EntitlementStatusActive {
		t.Fatalf("setup entitlements = %s/%s, want pro/active", repo.entitlements.PlanKey, repo.entitlements.Status)
	}
}

func TestBillingManagerWebhookWithoutEventTimeKeepsNewerStatus(t *testing.T) {
	manager, repo, secret := newSubscriptionStatusTestManager()
	orgID := repo.orgID.String()

	cancelled := `{"business_id":"biz_test","type":"subscription.cancelled","timestamp":"2026-04-28T23:00:00Z","data":{"payload_type":"Subscription","subscription_id":"sub_order","customer_id":"cus_order","product_id":"agentclash_pro_monthly","status":"cancelled","quantity":5,"metadata":{"organization_id":"` + orgID + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_order_cancelled", cancelled)
	if repo.subscription.Status != "cancelled" {
		t.Fatalf("status after the cancelled event = %q, want cancelled", repo.subscription.Status)
	}

	// An old event with no time arrives after the cancellation.
	untimedActive := `{"business_id":"biz_test","type":"subscription.active","data":{"payload_type":"Subscription","subscription_id":"sub_order","customer_id":"cus_order","product_id":"agentclash_pro_monthly","status":"active","quantity":5,"metadata":{"organization_id":"` + orgID + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_order_untimed_active", untimedActive)

	if !repo.webhookIDs["wh_order_untimed_active"] {
		t.Fatal("the event with no time was not stored")
	}
	if repo.subscription.Status != "cancelled" {
		t.Fatalf("status after an event with no time = %q, want cancelled", repo.subscription.Status)
	}
	wantEventAt := time.Date(2026, 4, 28, 23, 0, 0, 0, time.UTC)
	if repo.subscription.LatestDodoEventAt == nil || !repo.subscription.LatestDodoEventAt.Equal(wantEventAt) {
		t.Fatalf("latest event time = %v, want %v", repo.subscription.LatestDodoEventAt, wantEventAt)
	}
	if repo.entitlements.PlanKey != billingpkg.PlanFree {
		t.Fatalf("plan after an event with no time = %q, want free", repo.entitlements.PlanKey)
	}
}

func TestBillingManagerWebhookWithoutEventTimeCreatesMissingSubscription(t *testing.T) {
	manager, repo, secret := newSubscriptionStatusTestManager()

	untimedActive := `{"business_id":"biz_test","type":"subscription.active","data":{"payload_type":"Subscription","subscription_id":"sub_new","customer_id":"cus_new","product_id":"agentclash_pro_monthly","status":"active","quantity":5,"metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_new_untimed_active", untimedActive)

	if repo.subscription.DodoSubscriptionID != "sub_new" || repo.subscription.Status != "active" {
		t.Fatalf("subscription = %q/%q, want sub_new/active", repo.subscription.DodoSubscriptionID, repo.subscription.Status)
	}
	if repo.entitlements.PlanKey != billingpkg.PlanPro {
		t.Fatalf("plan = %q, want pro", repo.entitlements.PlanKey)
	}
}

func TestBillingManagerWebhookWithoutEventTimeOrSubscriptionIDKeepsEntitlements(t *testing.T) {
	manager, repo, secret := newSubscriptionStatusTestManager()
	activateProSubscription(t, manager, repo, secret, "sub_no_id")

	// An event with no time and no subscription ID has no row to order
	// against. It must not change the plan of the organization.
	untimedHold := `{"business_id":"biz_test","type":"subscription.on_hold","data":{"payload_type":"Subscription","customer_id":"cus_status","product_id":"agentclash_pro_monthly","status":"on_hold","quantity":5,"metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_untimed_payment_failed", untimedHold)

	if !repo.webhookIDs["wh_untimed_payment_failed"] {
		t.Fatal("the event with no time was not stored")
	}
	if repo.entitlements.PlanKey != billingpkg.PlanPro || repo.entitlements.Status != billingpkg.EntitlementStatusActive {
		t.Fatalf("entitlements = %s/%s, want pro/active", repo.entitlements.PlanKey, repo.entitlements.Status)
	}
	if repo.subscription.Status != "active" {
		t.Fatalf("subscription status = %q, want active", repo.subscription.Status)
	}
}

func TestBillingManagerWebhookUnknownDodoStatusKeepsEntitlements(t *testing.T) {
	manager, repo, secret := newSubscriptionStatusTestManager()
	activateProSubscription(t, manager, repo, secret, "sub_unknown")

	updated := `{"business_id":"biz_test","type":"subscription.updated","timestamp":"2026-04-28T23:00:00Z","data":{"payload_type":"Subscription","subscription_id":"sub_unknown","customer_id":"cus_status","product_id":"agentclash_pro_monthly","status":"grace_period","quantity":5,"metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_unknown_status", updated)

	if repo.entitlements.PlanKey != billingpkg.PlanPro || repo.entitlements.Status != billingpkg.EntitlementStatusActive {
		t.Fatalf("entitlements = %s/%s, want pro/active", repo.entitlements.PlanKey, repo.entitlements.Status)
	}
	if repo.subscription.Status != "unknown" {
		t.Fatalf("subscription status = %q, want unknown", repo.subscription.Status)
	}
}

func TestBillingManagerWebhookPaymentSucceededKeepsPaidPlan(t *testing.T) {
	manager, repo, secret := newSubscriptionStatusTestManager()
	activateProSubscription(t, manager, repo, secret, "sub_payment")

	// A Dodo Payment body has the payment status ("succeeded"), not the
	// subscription status.
	payment := `{"business_id":"biz_test","type":"payment.succeeded","timestamp":"2026-04-28T23:00:00Z","data":{"payload_type":"Payment","payment_id":"pay_renewal","subscription_id":"sub_payment","customer_id":"cus_status","status":"succeeded","total_amount":2900,"currency":"USD","metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_payment_succeeded", payment)

	if repo.entitlements.PlanKey != billingpkg.PlanPro || repo.entitlements.Status != billingpkg.EntitlementStatusActive {
		t.Fatalf("entitlements = %s/%s, want pro/active", repo.entitlements.PlanKey, repo.entitlements.Status)
	}
	if repo.subscription.Status != "active" {
		t.Fatalf("subscription status = %q, want active", repo.subscription.Status)
	}
}

func TestBillingManagerWebhookPastDueKeepsPaidPlanUntilGraceEnds(t *testing.T) {
	manager, repo, secret := newSubscriptionStatusTestManager()
	activateProSubscription(t, manager, repo, secret, "sub_grace")
	orgID := repo.orgID.String()

	// A renewal failed. Dodo starts the grace period: the customer keeps
	// access until past_due_ends_at.
	pastDue := `{"business_id":"biz_test","type":"subscription.past_due","timestamp":"2026-04-28T23:00:00Z","data":{"payload_type":"Subscription","subscription_id":"sub_grace","customer_id":"cus_status","product_id":"agentclash_pro_monthly","status":"past_due","quantity":5,"next_billing_date":"2026-04-28T00:00:00Z","cancel_at_next_billing_date":true,"past_due_ends_at":"2026-05-05T00:00:00Z","metadata":{"organization_id":"` + orgID + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_grace_past_due", pastDue)

	if repo.subscription.Status != "past_due" {
		t.Fatalf("subscription status = %q, want past_due", repo.subscription.Status)
	}
	if repo.entitlements.PlanKey != billingpkg.PlanPro || repo.entitlements.Status != billingpkg.EntitlementStatusActive {
		t.Fatalf("entitlements in the grace period = %s/%s, want pro/active", repo.entitlements.PlanKey, repo.entitlements.Status)
	}
	graceEnd := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	if repo.entitlements.ExpiresAt == nil || !repo.entitlements.ExpiresAt.Equal(graceEnd) {
		t.Fatalf("entitlements expire at %v, want the end of the grace period %v", repo.entitlements.ExpiresAt, graceEnd)
	}

	// The grace period ends with no payment. Dodo moves the subscription to
	// on_hold, and the paid plan stays, but inactive.
	onHold := `{"business_id":"biz_test","type":"subscription.on_hold","timestamp":"2026-04-28T23:30:00Z","data":{"payload_type":"Subscription","subscription_id":"sub_grace","customer_id":"cus_status","product_id":"agentclash_pro_monthly","status":"on_hold","quantity":5,"metadata":{"organization_id":"` + orgID + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_grace_on_hold", onHold)

	if repo.entitlements.PlanKey != billingpkg.PlanPro || repo.entitlements.Status != billingpkg.EntitlementStatusInactive {
		t.Fatalf("entitlements after the grace period = %s/%s, want pro/inactive", repo.entitlements.PlanKey, repo.entitlements.Status)
	}
}

func TestBillingManagerWebhookFailedPaymentKeepsSubscriptionState(t *testing.T) {
	manager, repo, secret := newSubscriptionStatusTestManager()
	activateProSubscription(t, manager, repo, secret, "sub_card")

	// A failed charge does not end the subscription. Dodo sends a
	// subscription event (past_due, on_hold) when the state changes.
	failed := `{"business_id":"biz_test","type":"payment.failed","timestamp":"2026-04-28T23:00:00Z","data":{"payload_type":"Payment","payment_id":"pay_declined","subscription_id":"sub_card","customer_id":"cus_status","status":"failed","total_amount":2900,"currency":"USD","metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_card_failed", failed)

	if !repo.webhookIDs["wh_card_failed"] {
		t.Fatal("the payment.failed event was not stored")
	}
	if repo.subscription.Status != "active" {
		t.Fatalf("subscription status = %q, want active", repo.subscription.Status)
	}
	if repo.entitlements.PlanKey != billingpkg.PlanPro || repo.entitlements.Status != billingpkg.EntitlementStatusActive {
		t.Fatalf("entitlements = %s/%s, want pro/active", repo.entitlements.PlanKey, repo.entitlements.Status)
	}
}

func TestBillingManagerWebhookDunningStartedKeepsSubscriptionState(t *testing.T) {
	manager, repo, secret := newSubscriptionStatusTestManager()
	activateProSubscription(t, manager, repo, secret, "sub_dunning")

	// The Dodo dunning payload has the status of the dunning run
	// ("recovering"), not a subscription status.
	started := `{"business_id":"biz_test","type":"dunning.started","timestamp":"2026-04-28T23:00:00Z","data":{"brand_id":"brand_test","created_at":"2026-04-28T23:00:00Z","customer_id":"cus_status","status":"recovering","subscription_id":"sub_dunning","trigger_state":"past_due","metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_dunning_started", started)

	if !repo.webhookIDs["wh_dunning_started"] {
		t.Fatal("the dunning.started event was not stored")
	}
	if repo.subscription.Status != "active" {
		t.Fatalf("subscription status = %q, want active", repo.subscription.Status)
	}
	if repo.entitlements.PlanKey != billingpkg.PlanPro || repo.entitlements.Status != billingpkg.EntitlementStatusActive {
		t.Fatalf("entitlements = %s/%s, want pro/active", repo.entitlements.PlanKey, repo.entitlements.Status)
	}
}
