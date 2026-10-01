package api

import (
	"context"
	"encoding/json"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
	"testing"
	"time"
)

// Called from the real-PostgreSQL journey fixture. Only the billing transport
// boundary is synthetic; signature verification and ledger writes are real.
func verifyVibeCreditWebhook(t *testing.T, store *vibe.Store, user, org, workspace uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	payment := "payment-" + id.String()
	p := vibe.CreditProduct{ID: "vibe-test-product", Credits: 5 * vibe.NanoUSD, PriceMinor: 500, Currency: "USD"}
	if _, _, err := store.BeginCheckout(ctx, user, org, id, p); err != nil {
		t.Fatal(err)
	}
	secret := dodoTestWebhookSecret()
	m := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), newFakeBillingRepository(workspace), BillingManagerConfig{WebhookSecret: secret})
	m.vibeCredits = &VibeCredits{Store: store}
	m.now = func() time.Time { return time.Unix(1777420800, 0).UTC() }
	before, held, err := store.Balance(ctx, "org:"+org.String())
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"business_id": "biz_test", "type": "payment.succeeded", "timestamp": "2026-04-29T00:00:00Z", "data": map[string]any{"payload_type": "Payment", "payment_id": payment, "checkout_session_id": "checkout-test", "status": "succeeded", "currency": "USD", "total_amount": 500, "tax": 0, "product_cart": []map[string]any{{"product_id": p.ID, "quantity": 1}}, "metadata": map[string]string{"purpose": "vibe_credit_topup", "organization_id": org.String(), "checkout_intent_id": id.String()}}})
	if _, err = m.ProcessDodoWebhook(ctx, DodoWebhookHeaders{WebhookID: "bad", WebhookTimestamp: "1777420800", WebhookSignature: "forged"}, body); err == nil {
		t.Fatal("forged top-up credited")
	}
	for _, delivery := range []string{"delivery-1", "delivery-2"} {
		if _, err = m.ProcessDodoWebhook(ctx, signedDodoHeaders(secret, delivery, "1777420800", string(body)), body); err != nil {
			t.Fatal(err)
		}
	}
	after, afterHeld, err := store.Balance(ctx, "org:"+org.String())
	if err != nil || after != before+p.Credits || afterHeld != held {
		t.Fatalf("duplicate signed payment changed ledger twice: %d %d %v", after, afterHeld, err)
	}
	refund, _ := json.Marshal(map[string]any{"business_id": "biz_test", "type": "refund.succeeded", "timestamp": "2026-04-29T00:00:00Z", "data": map[string]any{"payload_type": "Refund", "payment_id": payment, "refund_id": "refund-" + id.String(), "status": "succeeded", "amount": 500, "currency": "USD"}})
	if _, err = m.ProcessDodoWebhook(ctx, signedDodoHeaders(secret, "refund-delivery", "1777420800", string(refund)), refund); err != nil {
		t.Fatal(err)
	}
	var disabled bool
	if err = store.DB.QueryRow(ctx, "SELECT disabled FROM vibe_accounts WHERE id=$1", "org:"+org.String()).Scan(&disabled); err != nil || !disabled {
		t.Fatal("refund did not freeze wallet", err)
	}
}
