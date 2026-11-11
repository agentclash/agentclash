package vibe

import "testing"

// The billing table of a vibe operation. SETTLED and RELEASED are final, so a
// late cost conflict cannot reopen a settlement (#1382).
func TestStateMachineBillingKeepsSettlementFinal(t *testing.T) {
	states := []BillingState{Unreserved, BillingReserved, Reconciling, Settled, Released}
	allowed := map[[2]BillingState]bool{
		{Unreserved, BillingReserved}:  true, // Approve funds a quote.
		{Unreserved, Released}:         true, // A quote expires or stops.
		{BillingReserved, Reconciling}: true,
		{BillingReserved, Settled}:     true,
		{BillingReserved, Released}:    true,
		{Reconciling, Reconciling}:     true, // settle runs again for each late receipt.
		{Reconciling, Settled}:         true,
		{Reconciling, Released}:        true,
	}
	for _, from := range states {
		for _, to := range states {
			if got, want := CanTransitionBilling(from, to), allowed[[2]BillingState{from, to}]; got != want {
				t.Errorf("CanTransitionBilling(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
		if got, want := from.Final(), from == Settled || from == Released; got != want {
			t.Errorf("%s.Final() = %v, want %v", from, got, want)
		}
	}
}
