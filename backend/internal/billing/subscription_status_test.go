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
		want := status == SubscriptionStatusActive || status == SubscriptionStatusTrialing
		if got := status.Entitled(); got != want {
			t.Errorf("%q.Entitled() = %v, want %v", status, got, want)
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
		{"payment.failed", "Payment", "failed", SubscriptionStatusFailed},
		{"payment.failed", "Payment", "requires_payment_method", SubscriptionStatusFailed},
		{"dunning.succeeded", "", "succeeded", SubscriptionStatusActive},
		{"dunning.recovered", "", "", SubscriptionStatusActive},
		{"dunning.failed", "", "", SubscriptionStatusFailed},
		{"dunning.exhausted", "", "cancelled", SubscriptionStatusFailed},
		{"dunning.started", "", "active", SubscriptionStatusUnknown},
		{"refund.succeeded", "Refund", "succeeded", SubscriptionStatusUnknown},
	}
	for _, tc := range cases {
		got := DodoSubscriptionStatusFromEvent(tc.eventType, tc.payloadType, tc.payloadStatus)
		if got != tc.want {
			t.Errorf("DodoSubscriptionStatusFromEvent(%q, %q, %q) = %q, want %q", tc.eventType, tc.payloadType, tc.payloadStatus, got, tc.want)
		}
	}
}

func TestDodoStatusIsEntitledUsesTheStatusSet(t *testing.T) {
	for status, want := range map[string]bool{
		"active":    true,
		"trialing":  true,
		"renewed":   true,
		"succeeded": false,
		"on_hold":   false,
		"unknown":   false,
	} {
		if got := DodoStatusIsEntitled(status); got != want {
			t.Errorf("DodoStatusIsEntitled(%q) = %v, want %v", status, got, want)
		}
	}
}
