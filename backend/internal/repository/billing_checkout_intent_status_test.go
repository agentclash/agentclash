package repository

import (
	"testing"
	"time"
)

func TestBillingCheckoutIntentStatusCanTransitionTo(t *testing.T) {
	all := []BillingCheckoutIntentStatus{
		BillingCheckoutIntentStatusCreated,
		BillingCheckoutIntentStatusExpired,
		BillingCheckoutIntentStatusCompleted,
	}
	allowed := map[BillingCheckoutIntentStatus][]BillingCheckoutIntentStatus{
		BillingCheckoutIntentStatusCreated: {BillingCheckoutIntentStatusExpired, BillingCheckoutIntentStatusCompleted},
		BillingCheckoutIntentStatusExpired: {BillingCheckoutIntentStatusCompleted},
	}
	if len(all) != len(billingCheckoutIntentTransitions) {
		t.Fatalf("test covers %d statuses, table has %d", len(all), len(billingCheckoutIntentTransitions))
	}
	for _, from := range all {
		for _, to := range all {
			want := false
			for _, next := range allowed[from] {
				if next == to {
					want = true
				}
			}
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%s -> %s = %v, want %v", from, to, got, want)
			}
		}
	}
	// Migration 00098 removed these two statuses from the CHECK constraint.
	for _, removed := range []BillingCheckoutIntentStatus{"pending", "cancelled"} {
		if removed.Valid() {
			t.Errorf("%s must not be valid", removed)
		}
	}
}

func TestBillingCheckoutIntentTerminalStatuses(t *testing.T) {
	terminal := map[BillingCheckoutIntentStatus]bool{
		BillingCheckoutIntentStatusCreated:   false,
		BillingCheckoutIntentStatusExpired:   false,
		BillingCheckoutIntentStatusCompleted: true,
	}
	for status, want := range terminal {
		if got := status.Terminal(); got != want {
			t.Errorf("%s terminal = %v, want %v", status, got, want)
		}
	}
	// A late payment must still complete an expired intent.
	if !BillingCheckoutIntentStatusExpired.CanTransitionTo(BillingCheckoutIntentStatusCompleted) {
		t.Error("expired -> completed must be allowed")
	}
	// The Dodo checkout session lives for 24 hours.
	if BillingCheckoutIntentLifetime != 24*time.Hour {
		t.Errorf("lifetime = %s, want 24h", BillingCheckoutIntentLifetime)
	}
}
