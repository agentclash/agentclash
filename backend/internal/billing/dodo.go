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

func MapDodoProduct(productID string) (DodoProductMapping, error) {
	productID = strings.TrimSpace(productID)
	for _, plan := range Catalog() {
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

func SubscriptionEntitlements(input DodoSubscriptionInput) (EffectiveEntitlements, error) {
	mapping, err := MapDodoProduct(input.ProductID)
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
	return MaterializeEntitlements(plan, mapping.BillingPeriod, input.Quantity, "active"), nil
}

func DodoStatusIsEntitled(status string) bool {
	switch normalizeStatus(status) {
	case "active", "trialing", "renewed":
		return true
	default:
		return false
	}
}

func DodoStatusFromEvent(eventType string, payloadStatus string) string {
	payloadStatus = normalizeStatus(payloadStatus)
	if payloadStatus != "" {
		return payloadStatus
	}
	switch normalizeStatus(eventType) {
	case "subscription.active", "subscription.renewed", "subscription.updated", "subscription.plan_changed":
		return "active"
	case "subscription.on_hold":
		return "on_hold"
	case "subscription.failed", "payment.failed", "dunning.failed":
		return "failed"
	case "subscription.cancelled":
		return "cancelled"
	case "subscription.expired":
		return "expired"
	default:
		return "unknown"
	}
}

func normalizeStatus(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
