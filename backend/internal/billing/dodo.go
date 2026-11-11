package billing

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrUnknownDodoProduct = errors.New("unknown dodo product")
	ErrInactiveDodoStatus = errors.New("inactive dodo subscription status")
)

type DodoSubscriptionInput struct {
	SubscriptionID    string
	CustomerID        string
	ProductID         string
	Status            string
	Quantity          int
	AddonQuantities   map[string]int
	NextBillingDate   *time.Time
	CancelAtNextBill  bool
	CancelledAt       *time.Time
	ExpiresAt         *time.Time
	TrialPeriodDays   *int
	LatestDodoEventAt time.Time
}

type DodoProductMapping struct {
	ProductID     string
	PlanKey       string
	BillingPeriod string
}

func MapDodoProductFromCatalog(productID string, plans []Plan) (DodoProductMapping, error) {
	productID = strings.TrimSpace(productID)
	for _, plan := range plans {
		for period, id := range plan.DodoProductIDs {
			if id == productID {
				return DodoProductMapping{
					ProductID:     productID,
					PlanKey:       plan.Key,
					BillingPeriod: period,
				}, nil
			}
		}
	}
	return DodoProductMapping{}, fmt.Errorf("%w: %s", ErrUnknownDodoProduct, productID)
}

func SubscriptionEntitlementsFromCatalog(input DodoSubscriptionInput, plans []Plan) (EffectiveEntitlements, error) {
	mapping, err := MapDodoProductFromCatalog(input.ProductID, plans)
	if err != nil {
		return EffectiveEntitlements{}, err
	}
	if !DodoStatusIsEntitled(input.Status) {
		return EffectiveEntitlements{}, fmt.Errorf("%w: %s", ErrInactiveDodoStatus, input.Status)
	}
	plan, ok := PlanByKey(mapping.PlanKey)
	if !ok {
		return EffectiveEntitlements{}, fmt.Errorf("%w: %s", ErrUnknownPlan, mapping.PlanKey)
	}
	if err := ValidateSeatQuantity(plan, input.Quantity); err != nil {
		return EffectiveEntitlements{}, err
	}
	entitlements := MaterializeEntitlements(plan, mapping.BillingPeriod, input.Quantity, input.Status)
	entitlements.ExpiresAt = input.ExpiresAt
	return entitlements, nil
}

// DodoStatusIsEntitled reports whether a Dodo subscription status grants the
// paid plan. It maps the value into the status set first.
func DodoStatusIsEntitled(status string) bool {
	return ParseDodoSubscriptionStatus(status).Entitled()
}

func normalizeStatus(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
