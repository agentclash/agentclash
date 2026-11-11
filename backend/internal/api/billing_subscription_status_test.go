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

	// A payment event with no time and no subscription ID has no row to order
	// against. It must not change the plan of the organization.
	untimedFailure := `{"business_id":"biz_test","type":"payment.failed","data":{"payload_type":"Payment","payment_id":"pay_untimed","customer_id":"cus_status","status":"failed","metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	deliverSubscriptionStatusWebhook(t, manager, secret, "wh_untimed_payment_failed", untimedFailure)

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
