package billing

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

const (
	PlanFree       = "free"
	PlanPro        = "pro"
	PlanTeam       = "team"
	PlanEnterprise = "enterprise"

	PeriodMonthly = "monthly"
	PeriodYearly  = "yearly"
	PeriodCustom  = "custom"

	EntitlementStatusActive   = "active"
	EntitlementStatusTrialing = "trialing"
	EntitlementStatusExpired  = "expired"
	EntitlementStatusInactive = "inactive"

	FeaturePrivateChallengePacks = "private_challenge_packs"
)

var (
	ErrUnknownPlan            = errors.New("unknown billing plan")
	ErrInvalidBillingPeriod   = errors.New("invalid billing period")
	ErrSeatQuantityBelowLimit = errors.New("seat quantity is below the plan minimum")
	ErrMissingDodoProductIDs  = errors.New("missing dodo product ids")
)

type Limit struct {
	Value     *int `json:"value,omitempty"`
	PerSeat   bool `json:"per_seat,omitempty"`
	Unlimited bool `json:"unlimited,omitempty"`
	Custom    bool `json:"custom,omitempty"`
}

type PlanLimits struct {
	Seats                  Limit `json:"seats"`
	Workspaces             Limit `json:"workspaces"`
	RacesPerWorkspaceMonth Limit `json:"races_per_workspace_month"`
	MaxModelsPerRace       Limit `json:"max_models_per_race"`
	ReplayRetentionDays    Limit `json:"replay_retention_days"`
	ConcurrentRaces        Limit `json:"concurrent_races"`
}

type Plan struct {
	Key            string            `json:"key"`
	DisplayName    string            `json:"display_name"`
	MinimumSeats   int               `json:"minimum_seats"`
	DefaultSeats   int               `json:"default_seats"`
	BillingPeriods []string          `json:"billing_periods"`
	Limits         PlanLimits        `json:"limits"`
	FeatureFlags   map[string]bool   `json:"feature_flags"`
	UpgradeTarget  string            `json:"upgrade_target,omitempty"`
	DodoProductIDs map[string]string `json:"dodo_product_ids,omitempty"`
}

type DodoProductIDs struct {
	ProMonthly  string
	ProYearly   string
	TeamMonthly string
	TeamYearly  string
}

func (ids DodoProductIDs) IsZero() bool {
	return ids.ProMonthly == "" && ids.ProYearly == "" && ids.TeamMonthly == "" && ids.TeamYearly == ""
}

func (ids DodoProductIDs) IsComplete() bool {
	return ids.ProMonthly != "" && ids.ProYearly != "" && ids.TeamMonthly != "" && ids.TeamYearly != ""
}

func (ids DodoProductIDs) Validate() error {
	if ids.IsZero() || ids.IsComplete() {
		return nil
	}
	return ErrMissingDodoProductIDs
}

type EffectiveEntitlements struct {
	PlanKey                string          `json:"plan_key"`
	BillingPeriod          string          `json:"billing_period"`
	Status                 string          `json:"status"`
	SeatQuantity           int             `json:"seat_quantity"`
	SeatsLimit             *int            `json:"seats_limit"`
	WorkspacesLimit        *int            `json:"workspaces_limit"`
	RacesPerWorkspaceMonth *int            `json:"races_per_workspace_month"`
	MaxModelsPerRace       *int            `json:"max_models_per_race"`
	ReplayRetentionDays    *int            `json:"replay_retention_days"`
	ConcurrentRaces        *int            `json:"concurrent_races"`
	FeatureFlags           map[string]bool `json:"feature_flags"`
	UpgradeTarget          string          `json:"upgrade_target,omitempty"`
	ExpiresAt              *time.Time      `json:"expires_at,omitempty"`
}

func Catalog() []Plan {
	return CatalogWithDodoProductIDs(DodoProductIDs{})
}

func CatalogWithDodoProductIDs(productIDs DodoProductIDs) []Plan {
	plans := []Plan{
		{
			Key:            PlanFree,
			DisplayName:    "Free",
			MinimumSeats:   1,
			DefaultSeats:   1,
			BillingPeriods: []string{PeriodMonthly},
			Limits: PlanLimits{
				Seats:                  unlimitedLimit(),
				Workspaces:             valueLimit(1),
				RacesPerWorkspaceMonth: valueLimit(25),
				MaxModelsPerRace:       valueLimit(4),
				ReplayRetentionDays:    valueLimit(7),
				ConcurrentRaces:        valueLimit(1),
			},
			FeatureFlags: map[string]bool{
				"byok_llm":          true,
				"byok_e2b":          true,
				"community_support": true,
			},
			UpgradeTarget: PlanPro,
		},
		{
			Key:            PlanPro,
			DisplayName:    "Pro",
			MinimumSeats:   1,
			DefaultSeats:   1,
			BillingPeriods: []string{PeriodMonthly, PeriodYearly},
			Limits: PlanLimits{
				Seats:                  unlimitedLimit(),
				Workspaces:             valueLimit(1),
				RacesPerWorkspaceMonth: valueLimit(500),
				MaxModelsPerRace:       valueLimit(8),
				ReplayRetentionDays:    valueLimit(30),
				ConcurrentRaces:        valueLimit(3),
			},
			FeatureFlags: map[string]bool{
				"byok_llm":                   true,
				"byok_e2b":                   true,
				"hosted_sandbox_credit":      true,
				FeaturePrivateChallengePacks: true,
				"ci_integration":             true,
				"email_support":              true,
			},
			UpgradeTarget: PlanTeam,
		},
		{
			Key:            PlanTeam,
			DisplayName:    "Team",
			MinimumSeats:   1,
			DefaultSeats:   1,
			BillingPeriods: []string{PeriodMonthly, PeriodYearly},
			Limits: PlanLimits{
				Seats:                  unlimitedLimit(),
				Workspaces:             unlimitedLimit(),
				RacesPerWorkspaceMonth: valueLimit(2000),
				MaxModelsPerRace:       valueLimit(12),
				ReplayRetentionDays:    valueLimit(90),
				ConcurrentRaces:        valueLimit(10),
			},
			FeatureFlags: map[string]bool{
				"byok_llm":                   true,
				"byok_e2b":                   true,
				"hosted_sandbox_credit":      true,
				FeaturePrivateChallengePacks: true,
				"ci_integration":             true,
				"audit_log":                  true,
				"slack_notifications":        true,
				"priority_support":           true,
			},
			UpgradeTarget: PlanEnterprise,
		},
		{
			Key:            PlanEnterprise,
			DisplayName:    "Enterprise",
			MinimumSeats:   1,
			DefaultSeats:   1,
			BillingPeriods: []string{PeriodCustom},
			Limits: PlanLimits{
				Seats:                  unlimitedLimit(),
				Workspaces:             customLimit(),
				RacesPerWorkspaceMonth: customLimit(),
				MaxModelsPerRace:       customLimit(),
				ReplayRetentionDays:    unlimitedLimit(),
				ConcurrentRaces:        customLimit(),
			},
			FeatureFlags: map[string]bool{
				"byok_llm":                   true,
				"byok_e2b":                   true,
				"hosted_sandbox_credit":      true,
				FeaturePrivateChallengePacks: true,
				"ci_integration":             true,
				"audit_log":                  true,
				"slack_notifications":        true,
				"priority_support":           true,
				"sso_saml":                   true,
				"org_wide_audit_logs":        true,
				"sla":                        true,
				"dedicated_support":          true,
				"custom_billing":             true,
			},
		},
	}
	attachDodoProductIDs(plans, productIDs)

	for i := range plans {
		plans[i].FeatureFlags = cloneFlags(plans[i].FeatureFlags)
		plans[i].BillingPeriods = append([]string(nil), plans[i].BillingPeriods...)
		plans[i].DodoProductIDs = cloneStringMap(plans[i].DodoProductIDs)
	}
	return plans
}

func attachDodoProductIDs(plans []Plan, productIDs DodoProductIDs) {
	if productIDs.IsZero() {
		return
	}
	for i := range plans {
		switch plans[i].Key {
		case PlanPro:
			plans[i].DodoProductIDs = map[string]string{
				PeriodMonthly: productIDs.ProMonthly,
				PeriodYearly:  productIDs.ProYearly,
			}
		case PlanTeam:
			plans[i].DodoProductIDs = map[string]string{
				PeriodMonthly: productIDs.TeamMonthly,
				PeriodYearly:  productIDs.TeamYearly,
			}
		}
	}
}

func PlanByKey(key string) (Plan, bool) {
	for _, plan := range Catalog() {
		if plan.Key == key {
			return plan, true
		}
	}
	return Plan{}, false
}

func MustPlan(key string) Plan {
	plan, ok := PlanByKey(key)
	if !ok {
		panic(fmt.Sprintf("billing plan %q is not registered", key))
	}
	return plan
}

func ValidateBillingPeriod(plan Plan, period string) error {
	if period == "" {
		return nil
	}
	for _, allowed := range plan.BillingPeriods {
		if period == allowed {
			return nil
		}
	}
	return fmt.Errorf("%w: %s does not support %s", ErrInvalidBillingPeriod, plan.Key, period)
}

func ValidateSeatQuantity(plan Plan, seatQuantity int) error {
	if seatQuantity <= 0 {
		return fmt.Errorf("%w: seat_quantity must be positive", ErrSeatQuantityBelowLimit)
	}
	if seatQuantity < plan.MinimumSeats {
		return fmt.Errorf("%w: %s requires subscription quantity of at least %d", ErrSeatQuantityBelowLimit, plan.Key, plan.MinimumSeats)
	}
	return nil
}

func DefaultEntitlements() EffectiveEntitlements {
	return MaterializeEntitlements(MustPlan(PlanFree), PeriodMonthly, 1, EntitlementStatusActive)
}

func MaterializeEntitlements(plan Plan, billingPeriod string, seatQuantity int, status string) EffectiveEntitlements {
	if billingPeriod == "" {
		billingPeriod = PeriodMonthly
	}
	if seatQuantity <= 0 {
		seatQuantity = plan.DefaultSeats
	}
	if status == "" {
		status = EntitlementStatusActive
	}
	return EffectiveEntitlements{
		PlanKey:                plan.Key,
		BillingPeriod:          billingPeriod,
		Status:                 status,
		SeatQuantity:           seatQuantity,
		SeatsLimit:             materializeLimit(plan.Limits.Seats, seatQuantity),
		WorkspacesLimit:        materializeLimit(plan.Limits.Workspaces, seatQuantity),
		RacesPerWorkspaceMonth: materializeLimit(plan.Limits.RacesPerWorkspaceMonth, seatQuantity),
		MaxModelsPerRace:       materializeLimit(plan.Limits.MaxModelsPerRace, seatQuantity),
		ReplayRetentionDays:    materializeLimit(plan.Limits.ReplayRetentionDays, seatQuantity),
		ConcurrentRaces:        materializeLimit(plan.Limits.ConcurrentRaces, seatQuantity),
		FeatureFlags:           cloneFlags(plan.FeatureFlags),
		UpgradeTarget:          plan.UpgradeTarget,
	}
}

func (e EffectiveEntitlements) IsExpired(now time.Time) bool {
	if e.ExpiresAt == nil {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return !now.UTC().Before(e.ExpiresAt.UTC())
}

func (e EffectiveEntitlements) WithComputedStatus(now time.Time) EffectiveEntitlements {
	if e.IsExpired(now) {
		e.Status = EntitlementStatusExpired
		e.UpgradeTarget = expiredUpgradeTarget(e.PlanKey, e.UpgradeTarget)
	}
	return e
}

func expiredUpgradeTarget(planKey string, fallback string) string {
	if planKey != "" && planKey != PlanFree {
		return planKey
	}
	return fallback
}

func SortedPlanKeys() []string {
	keys := make([]string, 0, len(Catalog()))
	for _, plan := range Catalog() {
		keys = append(keys, plan.Key)
	}
	sort.Strings(keys)
	return keys
}

func materializeLimit(limit Limit, seats int) *int {
	if limit.Value == nil || limit.Unlimited || limit.Custom {
		return nil
	}
	value := *limit.Value
	if limit.PerSeat {
		value *= seats
	}
	return &value
}

func valueLimit(value int) Limit {
	return Limit{Value: &value}
}

func perSeatLimit(value int) Limit {
	return Limit{Value: &value, PerSeat: true}
}

func unlimitedLimit() Limit {
	return Limit{Unlimited: true}
}

func customLimit() Limit {
	return Limit{Custom: true}
}

func cloneFlags(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
