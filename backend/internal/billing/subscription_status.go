package billing

import (
	"errors"
	"slices"
	"strings"
)

// SubscriptionStatus is the value of billing_subscriptions.status.
//
// Dodo owns the transitions of a subscription, so this lifecycle has no Go
// transition table. The webhook stores the status that Dodo sends. The guard
// on the old state is the event time: the UpsertBillingSubscription query
// changes a row only for an event that is not older than the row. An event
// with no time can insert a missing row, but it does not change a row.
//
// The CHECK constraint billing_subscriptions_status_check (migration 00099)
// names the same set.
type SubscriptionStatus string

const (
	SubscriptionStatusPending   SubscriptionStatus = "pending"
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusTrialing  SubscriptionStatus = "trialing"
	SubscriptionStatusOnHold    SubscriptionStatus = "on_hold"
	SubscriptionStatusPaused    SubscriptionStatus = "paused"
	SubscriptionStatusPastDue   SubscriptionStatus = "past_due"
	SubscriptionStatusFailed    SubscriptionStatus = "failed"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
	SubscriptionStatusExpired   SubscriptionStatus = "expired"
	// SubscriptionStatusUnknown is a Dodo value that is not in the set. The
	// webhook stores it, logs a warning, and keeps the current entitlements.
	SubscriptionStatusUnknown SubscriptionStatus = "unknown"
)

var ErrInvalidSubscriptionStatus = errors.New("invalid billing subscription status")

var subscriptionStatuses = []SubscriptionStatus{
	SubscriptionStatusPending,
	SubscriptionStatusActive,
	SubscriptionStatusTrialing,
	SubscriptionStatusOnHold,
	SubscriptionStatusPaused,
	SubscriptionStatusPastDue,
	SubscriptionStatusFailed,
	SubscriptionStatusCancelled,
	SubscriptionStatusExpired,
	SubscriptionStatusUnknown,
}

// dodoSubscriptionStatusAliases maps other Dodo spellings into the set.
var dodoSubscriptionStatusAliases = map[string]SubscriptionStatus{
	"renewed":  SubscriptionStatusActive,
	"canceled": SubscriptionStatusCancelled,
}

// SubscriptionStatuses returns the permitted statuses in a fixed order.
func SubscriptionStatuses() []SubscriptionStatus {
	return slices.Clone(subscriptionStatuses)
}

// Valid reports whether the status is in the permitted set.
func (s SubscriptionStatus) Valid() bool {
	return slices.Contains(subscriptionStatuses, s)
}

// Entitled reports whether the status grants the paid plan.
//
// past_due is the grace period of Dodo: a renewal failed, and the customer
// keeps access until past_due_ends_at. After that time, Dodo moves the
// subscription to on_hold or cancelled.
func (s SubscriptionStatus) Entitled() bool {
	return s == SubscriptionStatusActive || s == SubscriptionStatusTrialing || s == SubscriptionStatusPastDue
}

// EntitlementStatus returns the entitlement status for a status that grants
// the paid plan.
func (s SubscriptionStatus) EntitlementStatus() string {
	if s == SubscriptionStatusTrialing {
		return EntitlementStatusTrialing
	}
	return EntitlementStatusActive
}

// ParseDodoSubscriptionStatus maps a Dodo subscription status into the set.
// It maps each value that it does not know to SubscriptionStatusUnknown.
func ParseDodoSubscriptionStatus(value string) SubscriptionStatus {
	normalized := normalizeStatus(value)
	if alias, ok := dodoSubscriptionStatusAliases[normalized]; ok {
		return alias
	}
	status := SubscriptionStatus(normalized)
	if status.Valid() {
		return status
	}
	return SubscriptionStatusUnknown
}

// DodoEventChangesSubscription reports whether a Dodo webhook event changes
// the subscription and the entitlements.
//
// Dodo sends a subscription event for each change of the subscription state.
// A payment.failed event does not change the state: after a failed renewal,
// Dodo sends subscription.past_due or subscription.on_hold. A dunning.started
// event also does not change the state. Its data.status is the status of the
// dunning run ("recovering"), and Dodo sent the subscription event before it.
func DodoEventChangesSubscription(eventType string) bool {
	eventType = normalizeStatus(eventType)
	return strings.HasPrefix(eventType, "subscription.") || eventType == "payment.succeeded" || eventType == "dunning.recovered"
}

// DodoSubscriptionStatusFromEvent returns the subscription status that a Dodo
// webhook event gives.
//
// Only a subscription payload has a subscription status. That is an event type
// that starts with "subscription.", or a payload_type of "Subscription". For a
// payment or dunning payload, data.status is the status of that object (for
// example "succeeded"), so the event type gives the status.
func DodoSubscriptionStatusFromEvent(eventType string, payloadType string, payloadStatus string) SubscriptionStatus {
	eventType = normalizeStatus(eventType)
	isSubscriptionPayload := strings.HasPrefix(eventType, "subscription.") || normalizeStatus(payloadType) == "subscription"
	if isSubscriptionPayload && normalizeStatus(payloadStatus) != "" {
		return ParseDodoSubscriptionStatus(payloadStatus)
	}
	switch eventType {
	case "subscription.active", "subscription.renewed", "subscription.updated", "subscription.plan_changed":
		return SubscriptionStatusActive
	case "subscription.on_hold":
		return SubscriptionStatusOnHold
	case "payment.succeeded", "dunning.recovered":
		return SubscriptionStatusActive
	case "subscription.failed":
		return SubscriptionStatusFailed
	case "subscription.cancelled":
		return SubscriptionStatusCancelled
	case "subscription.expired":
		return SubscriptionStatusExpired
	default:
		return SubscriptionStatusUnknown
	}
}
