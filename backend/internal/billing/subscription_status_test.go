package billing

import (
	"slices"
	"testing"
)

func TestSubscriptionStatusesAreTheOwnerSet(t *testing.T) {
	want := []SubscriptionStatus{"pending", "active", "trialing", "on_hold", "paused", "past_due", "failed", "cancelled", "expired", "unknown"}
	got := SubscriptionStatuses()
	if !slices.Equal(got, want) {
		t.Fatalf("SubscriptionStatuses() = %v, want %v", got, want)
	}
	for _, status := range got {
		if !status.Valid() {
			t.Errorf("%q is in the set but Valid() is false", status)
		}
	}
	got[0] = "changed"
	if SubscriptionStatuses()[0] != SubscriptionStatusPending {
		t.Fatal("SubscriptionStatuses() returned its internal slice")
	}
	for _, status := range []SubscriptionStatus{"", "renewed", "canceled", "succeeded", "Active"} {
		if status.Valid() {
			t.Errorf("%q is not in the set but Valid() is true", status)
		}
	}
}

func TestSubscriptionStatusEntitled(t *testing.T) {
	for _, status := range SubscriptionStatuses() {
		// past_due is the Dodo grace period: the customer keeps access.
		want := status == SubscriptionStatusActive || status == SubscriptionStatusTrialing || status == SubscriptionStatusPastDue
		if got := status.Entitled(); got != want {
			t.Errorf("%q.Entitled() = %v, want %v", status, got, want)
		}
	}
}

func TestSubscriptionStatusEntitlementStatus(t *testing.T) {
	for status, want := range map[SubscriptionStatus]string{
		SubscriptionStatusActive:   EntitlementStatusActive,
		SubscriptionStatusPastDue:  EntitlementStatusActive,
		SubscriptionStatusTrialing: EntitlementStatusTrialing,
	} {
		if got := status.EntitlementStatus(); got != want {
			t.Errorf("%q.EntitlementStatus() = %q, want %q", status, got, want)
		}
	}
}

func TestParseDodoSubscriptionStatus(t *testing.T) {
	cases := map[string]SubscriptionStatus{
		"renewed":      SubscriptionStatusActive,
		"canceled":     SubscriptionStatusCancelled,
		"Canceled":     SubscriptionStatusCancelled,
		" ACTIVE ":     SubscriptionStatusActive,
		"On_Hold":      SubscriptionStatusOnHold,
		"succeeded":    SubscriptionStatusUnknown,
		"grace_period": SubscriptionStatusUnknown,
		"":             SubscriptionStatusUnknown,
	}
	for _, status := range SubscriptionStatuses() {
		cases[string(status)] = status
	}
	for input, want := range cases {
		got := ParseDodoSubscriptionStatus(input)
		if got != want {
			t.Errorf("ParseDodoSubscriptionStatus(%q) = %q, want %q", input, got, want)
		}
		if !got.Valid() {
			t.Errorf("ParseDodoSubscriptionStatus(%q) = %q, which is not in the set", input, got)
		}
	}
}

func TestDodoSubscriptionStatusFromEvent(t *testing.T) {
	cases := []struct {
		eventType, payloadType, payloadStatus string
		want                                  SubscriptionStatus
	}{
		// A subscription payload: the payload status is the subscription status.
		{"subscription.active", "Subscription", "active", SubscriptionStatusActive},
		{"subscription.updated", "Subscription", "on_hold", SubscriptionStatusOnHold},
		{"subscription.renewed", "", "renewed", SubscriptionStatusActive},
		{"subscription.cancelled", "Subscription", "canceled", SubscriptionStatusCancelled},
		{"subscription.updated", "Subscription", "grace_period", SubscriptionStatusUnknown},
		{"dunning.started", "Subscription", "past_due", SubscriptionStatusPastDue},
		// A subscription payload with no status: map by the event type.
		{"subscription.cancelled", "Subscription", "", SubscriptionStatusCancelled},
		{"subscription.expired", "", "", SubscriptionStatusExpired},
		{"subscription.on_hold", "", "", SubscriptionStatusOnHold},
		{"subscription.failed", "", "", SubscriptionStatusFailed},
		{"subscription.plan_changed", "", "", SubscriptionStatusActive},
		{"subscription.paused", "", "", SubscriptionStatusUnknown},
		// Other payloads: the payload status is not a subscription status.
		{"payment.succeeded", "Payment", "succeeded", SubscriptionStatusActive},
		{"payment.succeeded", "Payment", "", SubscriptionStatusActive},
		{"dunning.recovered", "", "recovered", SubscriptionStatusActive},
		{"dunning.recovered", "", "", SubscriptionStatusActive},
		// These events do not change the subscription (see
		// DodoEventChangesSubscription), so no status maps from them.
		{"payment.failed", "Payment", "failed", SubscriptionStatusUnknown},
		{"dunning.started", "", "recovering", SubscriptionStatusUnknown},
		{"refund.succeeded", "Refund", "succeeded", SubscriptionStatusUnknown},
	}
	for _, tc := range cases {
		got := DodoSubscriptionStatusFromEvent(tc.eventType, tc.payloadType, tc.payloadStatus)
		if got != tc.want {
			t.Errorf("DodoSubscriptionStatusFromEvent(%q, %q, %q) = %q, want %q", tc.eventType, tc.payloadType, tc.payloadStatus, got, tc.want)
		}
	}
}

func TestDodoEventChangesSubscription(t *testing.T) {
	for eventType, want := range map[string]bool{
		"subscription.active":    true,
		"subscription.past_due":  true,
		"subscription.on_hold":   true,
		"subscription.paused":    true,
		"subscription.unpaused":  true,
		"payment.succeeded":      true,
		"dunning.recovered":      true,
		"payment.failed":         false,
		"payment.processing":     false,
		"dunning.started":        false,
		"refund.succeeded":       false,
		"invoice.payment_failed": false,
	} {
		if got := DodoEventChangesSubscription(eventType); got != want {
			t.Errorf("DodoEventChangesSubscription(%q) = %v, want %v", eventType, got, want)
		}
	}
}

func TestDodoStatusIsEntitledUsesTheStatusSet(t *testing.T) {
	for status, want := range map[string]bool{
		"active":    true,
		"trialing":  true,
		"renewed":   true,
		"past_due":  true,
		"succeeded": false,
		"on_hold":   false,
		"paused":    false,
		"failed":    false,
		"cancelled": false,
		"expired":   false,
		"pending":   false,
		"unknown":   false,
	} {
		if got := DodoStatusIsEntitled(status); got != want {
			t.Errorf("DodoStatusIsEntitled(%q) = %v, want %v", status, got, want)
		}
	}
}
