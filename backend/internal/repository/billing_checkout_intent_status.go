package repository

import "time"

// BillingCheckoutIntentStatus is billing_checkout_intents.status.
type BillingCheckoutIntentStatus string

const (
	// BillingCheckoutIntentStatusCreated is the first status. The user has a
	// Dodo checkout session and has not paid yet.
	BillingCheckoutIntentStatusCreated BillingCheckoutIntentStatus = "created"
	// BillingCheckoutIntentStatusExpired means that the checkout session
	// ended without a payment. The worker sweep writes it
	// BillingCheckoutIntentLifetime after created_at.
	BillingCheckoutIntentStatusExpired BillingCheckoutIntentStatus = "expired"
	// BillingCheckoutIntentStatusCompleted means that a verified Dodo webhook
	// named this checkout intent. It does not prove a successful payment.
	BillingCheckoutIntentStatusCompleted BillingCheckoutIntentStatus = "completed"
)

// BillingCheckoutIntentLifetime is the time after created_at when the sweep
// expires a created intent. createDodoCheckoutSession (api/billing.go) sets
// no expiry and no "confirm" field. Thus Dodo uses its default session
// lifetime, which is 24 hours.
const BillingCheckoutIntentLifetime = 24 * time.Hour

// billingCheckoutIntentTransitions is the checkout intent lifecycle.
// Completed is terminal.
//
// Writers:
//   - ExpireStaleBillingCheckoutIntents: created -> expired.
//   - MarkBillingCheckoutIntentCompleted: created, expired -> completed. A
//     payment can arrive after the sweep, so an expired intent can still
//     complete.
//
// Each UPDATE guards the old status in SQL. A repeat complete matches no row
// and is a no-op, not an error. A billing webhook must never get HTTP 409 for
// it, because the rollback also drops the subscription and entitlement writes
// of the same webhook.
var billingCheckoutIntentTransitions = map[BillingCheckoutIntentStatus]map[BillingCheckoutIntentStatus]struct{}{
	BillingCheckoutIntentStatusCreated: {
		BillingCheckoutIntentStatusExpired:   {},
		BillingCheckoutIntentStatusCompleted: {},
	},
	BillingCheckoutIntentStatusExpired: {
		BillingCheckoutIntentStatusCompleted: {},
	},
	BillingCheckoutIntentStatusCompleted: {},
}

func (s BillingCheckoutIntentStatus) Valid() bool {
	_, ok := billingCheckoutIntentTransitions[s]
	return ok
}

func (s BillingCheckoutIntentStatus) CanTransitionTo(next BillingCheckoutIntentStatus) bool {
	nextStatuses, ok := billingCheckoutIntentTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

// Terminal reports whether the status is final.
func (s BillingCheckoutIntentStatus) Terminal() bool {
	nextStatuses, ok := billingCheckoutIntentTransitions[s]
	return ok && len(nextStatuses) == 0
}
