package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	billingpkg "github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BillingService interface {
	ListPlans(ctx context.Context, caller Caller) (ListBillingPlansResult, error)
	GetOrganizationBilling(ctx context.Context, caller Caller, orgID uuid.UUID) (repository.BillingOverview, error)
	StartTrial(ctx context.Context, caller Caller, orgID uuid.UUID, input StartBillingTrialInput) (repository.BillingOverview, error)
	CreateCheckout(ctx context.Context, caller Caller, orgID uuid.UUID, input CreateBillingCheckoutInput) (CreateBillingCheckoutResult, error)
	CreatePortal(ctx context.Context, caller Caller, orgID uuid.UUID) (CreateBillingPortalResult, error)
	GetWorkspaceEntitlements(ctx context.Context, caller Caller, workspaceID uuid.UUID) (WorkspaceEntitlementsResult, error)
	ProcessDodoWebhook(ctx context.Context, headers DodoWebhookHeaders, rawBody []byte) (ProcessDodoWebhookResult, error)
}

type EntitlementGateService interface {
	BuildRunGate(ctx context.Context, workspaceID uuid.UUID, participantCount int, raceCount int) (*repository.RunEntitlementGate, error)
	BuildWorkspaceCreationGate(ctx context.Context, orgID uuid.UUID) (*repository.OrganizationEntitlementGate, error)
	BuildSeatGate(ctx context.Context, orgID uuid.UUID, userAlreadyActive bool) (*repository.OrganizationEntitlementGate, error)
	CheckWorkspaceFeature(ctx context.Context, workspaceID uuid.UUID, feature string) error
}

type BillingRepository interface {
	ResolveWorkspaceEntitlements(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, billingpkg.EffectiveEntitlements, error)
	GetOrganizationEntitlements(ctx context.Context, orgID uuid.UUID) (billingpkg.EffectiveEntitlements, error)
	UpsertOrganizationEntitlements(ctx context.Context, orgID uuid.UUID, entitlements billingpkg.EffectiveEntitlements, sourceSubscriptionID *uuid.UUID, expiresAt *time.Time) error
	CountActiveOrgMembers(ctx context.Context, orgID uuid.UUID) (int, error)
	CountActiveWorkspaces(ctx context.Context, orgID uuid.UUID) (int, error)
	CountActiveWorkspaceRuns(ctx context.Context, workspaceID uuid.UUID) (int, error)
	GetWorkspaceUsageSnapshot(ctx context.Context, workspaceID uuid.UUID, windowStart, windowEnd time.Time) (repository.WorkspaceUsageSnapshot, error)
	CreateBillingCheckoutIntent(ctx context.Context, input repository.BillingCheckoutIntentInput) (repository.BillingCheckoutIntent, error)
	UpsertBillingAccount(ctx context.Context, orgID uuid.UUID, dodoCustomerID string, billingEmail string, status string) error
	GetBillingAccount(ctx context.Context, orgID uuid.UUID) (repository.BillingAccount, error)
	UpsertBillingSubscription(ctx context.Context, input repository.BillingSubscriptionInput) (repository.BillingSubscription, error)
	GetBillingOverview(ctx context.Context, orgID uuid.UUID) (repository.BillingOverview, error)
	FindOrganizationByDodoSubscriptionOrCustomer(ctx context.Context, subscriptionID string, customerID string) (uuid.UUID, error)
	ApplyBillingWebhookEvent(ctx context.Context, event repository.BillingWebhookEventInput, application repository.BillingWebhookApplication) (bool, error)
	CreateBillingTrialGrant(ctx context.Context, input repository.BillingTrialGrantInput) (repository.BillingTrialGrant, error)
}

type BillingManager struct {
	orgAuthz       OrganizationAuthorizer
	authorizer     WorkspaceAuthorizer
	repo           BillingRepository
	dodoAPIKey     string
	dodoAPIBaseURL string
	httpClient     *http.Client
	webhookSecret  string
	plans          []billingpkg.Plan
	now            func() time.Time
}

var errInvalidWebhookSignature = errors.New("invalid webhook signature")

const dodoWebhookTimestampTolerance = 5 * time.Minute

type BillingManagerConfig struct {
	DodoAPIKey      string
	DodoAPIBaseURL  string
	DodoEnvironment string
	DodoProductIDs  billingpkg.DodoProductIDs
	HTTPClient      *http.Client
	WebhookSecret   string
}

func NewBillingManager(orgAuthz OrganizationAuthorizer, authorizer WorkspaceAuthorizer, repo BillingRepository, cfg BillingManagerConfig) *BillingManager {
	dodoAPIBaseURL := strings.TrimRight(strings.TrimSpace(cfg.DodoAPIBaseURL), "/")
	if dodoAPIBaseURL == "" {
		dodoAPIBaseURL = dodoAPIBaseURLForEnvironment(cfg.DodoEnvironment)
	}
	return &BillingManager{
		orgAuthz:       orgAuthz,
		authorizer:     authorizer,
		repo:           repo,
		dodoAPIKey:     strings.TrimSpace(cfg.DodoAPIKey),
		dodoAPIBaseURL: dodoAPIBaseURL,
		httpClient:     defaultHTTPClient(cfg.HTTPClient),
		webhookSecret:  strings.TrimSpace(cfg.WebhookSecret),
		plans:          billingpkg.CatalogWithDodoProductIDs(cfg.DodoProductIDs),
		now:            time.Now,
	}
}

type ListBillingPlansResult struct {
	Items []billingpkg.Plan `json:"items"`
}

type CreateBillingCheckoutInput struct {
	PlanKey       string `json:"plan_key"`
	BillingPeriod string `json:"billing_period"`
	SeatQuantity  int    `json:"seat_quantity"`
	ReturnURL     string `json:"return_url"`
}

type StartBillingTrialInput struct {
	PlanKey       string `json:"plan_key"`
	BillingPeriod string `json:"billing_period"`
}

type CreateBillingCheckoutResult struct {
	CheckoutIntentID uuid.UUID `json:"checkout_intent_id"`
	CheckoutURL      string    `json:"checkout_url"`
	PlanKey          string    `json:"plan_key"`
	BillingPeriod    string    `json:"billing_period"`
	SeatQuantity     int       `json:"seat_quantity"`
}

type CreateBillingPortalResult struct {
	PortalURL string `json:"portal_url"`
}

type WorkspaceEntitlementsResult struct {
	OrganizationID uuid.UUID                         `json:"organization_id"`
	WorkspaceID    uuid.UUID                         `json:"workspace_id"`
	Entitlements   billingpkg.EffectiveEntitlements  `json:"entitlements"`
	Usage          repository.WorkspaceUsageSnapshot `json:"usage"`
	Gates          WorkspaceGateSummary              `json:"gates"`
}

type WorkspaceGateSummary struct {
	Run billingpkg.GateDecision `json:"run"`
}

type DodoWebhookHeaders struct {
	WebhookID        string
	WebhookTimestamp string
	WebhookSignature string
}

type ProcessDodoWebhookResult struct {
	Duplicate bool   `json:"duplicate"`
	EventType string `json:"event_type"`
}

func (m *BillingManager) ListPlans(_ context.Context, _ Caller) (ListBillingPlansResult, error) {
	return ListBillingPlansResult{Items: clonePlans(m.plans)}, nil
}

func (m *BillingManager) GetOrganizationBilling(ctx context.Context, caller Caller, orgID uuid.UUID) (repository.BillingOverview, error) {
	if err := m.orgAuthz.AuthorizeOrganizationAdmin(ctx, caller, orgID); err != nil {
		return repository.BillingOverview{}, err
	}
	return m.repo.GetBillingOverview(ctx, orgID)
}

func (m *BillingManager) StartTrial(ctx context.Context, caller Caller, orgID uuid.UUID, input StartBillingTrialInput) (repository.BillingOverview, error) {
	if err := m.orgAuthz.AuthorizeOrganizationAdmin(ctx, caller, orgID); err != nil {
		return repository.BillingOverview{}, err
	}
	current, err := m.repo.GetOrganizationEntitlements(ctx, orgID)
	if err != nil {
		return repository.BillingOverview{}, err
	}
	if current.PlanKey != billingpkg.PlanFree {
		return repository.BillingOverview{}, validationError("trial_not_available", "trial is only available from the Free plan")
	}
	planKey := strings.TrimSpace(input.PlanKey)
	if planKey != billingpkg.PlanPro && planKey != billingpkg.PlanTeam {
		return repository.BillingOverview{}, validationError("invalid_plan_key", "plan_key must be pro or team")
	}
	plan, ok := m.planByKey(planKey)
	if !ok {
		return repository.BillingOverview{}, validationError("invalid_plan_key", "plan_key must be pro or team")
	}
	period := strings.TrimSpace(input.BillingPeriod)
	if period == "" {
		period = billingpkg.PeriodMonthly
	}
	if err := billingpkg.ValidateBillingPeriod(plan, period); err != nil {
		return repository.BillingOverview{}, validationError("invalid_billing_period", err.Error())
	}
	seatQuantity := plan.DefaultSeats
	if seatQuantity < plan.MinimumSeats {
		seatQuantity = plan.MinimumSeats
	}
	expiresAt := m.now().UTC().Add(45 * 24 * time.Hour)
	entitlements := billingpkg.MaterializeEntitlements(plan, period, seatQuantity, billingpkg.EntitlementStatusTrialing)
	entitlements.ExpiresAt = &expiresAt
	if _, err := m.repo.CreateBillingTrialGrant(ctx, repository.BillingTrialGrantInput{
		OrganizationID:  orgID,
		PlanKey:         plan.Key,
		BillingPeriod:   period,
		StartedByUserID: caller.UserID,
		StartedAt:       m.now().UTC(),
		ExpiresAt:       expiresAt,
	}); err != nil {
		if errors.Is(err, repository.ErrBillingTrialAlreadyUsed) {
			return repository.BillingOverview{}, validationError("trial_not_available", "this organization has already used its self-serve trial")
		}
		return repository.BillingOverview{}, err
	}
	if err := m.repo.UpsertOrganizationEntitlements(ctx, orgID, entitlements, nil, &expiresAt); err != nil {
		return repository.BillingOverview{}, err
	}
	return m.repo.GetBillingOverview(ctx, orgID)
}

func (m *BillingManager) CreateCheckout(ctx context.Context, caller Caller, orgID uuid.UUID, input CreateBillingCheckoutInput) (CreateBillingCheckoutResult, error) {
	if err := m.orgAuthz.AuthorizeOrganizationAdmin(ctx, caller, orgID); err != nil {
		return CreateBillingCheckoutResult{}, err
	}
	plan, ok := m.planByKey(strings.TrimSpace(input.PlanKey))
	if !ok || (plan.Key != billingpkg.PlanPro && plan.Key != billingpkg.PlanTeam) {
		return CreateBillingCheckoutResult{}, validationError("invalid_plan_key", "plan_key must be pro or team")
	}
	if err := billingpkg.ValidateBillingPeriod(plan, input.BillingPeriod); err != nil {
		return CreateBillingCheckoutResult{}, validationError("invalid_billing_period", err.Error())
	}
	if err := billingpkg.ValidateSeatQuantity(plan, input.SeatQuantity); err != nil {
		return CreateBillingCheckoutResult{}, validationError("invalid_seat_quantity", err.Error())
	}
	if _, err := url.ParseRequestURI(input.ReturnURL); err != nil {
		return CreateBillingCheckoutResult{}, validationError("invalid_return_url", "return_url must be an absolute URL")
	}
	if m.dodoAPIKey == "" {
		return CreateBillingCheckoutResult{}, validationError("billing_not_configured", "Dodo Payments checkout is not configured")
	}
	intentID := uuid.New()
	returnURL, err := appendCheckoutReturnParams(input.ReturnURL, intentID)
	if err != nil {
		return CreateBillingCheckoutResult{}, validationError("invalid_return_url", "return_url must be an absolute URL")
	}
	metadata, err := json.Marshal(map[string]any{
		"organization_id":    orgID.String(),
		"checkout_intent_id": intentID.String(),
		"plan_key":           plan.Key,
		"billing_period":     input.BillingPeriod,
		"seat_quantity":      input.SeatQuantity,
	})
	if err != nil {
		return CreateBillingCheckoutResult{}, err
	}
	input.ReturnURL = returnURL
	checkoutURL, dodoCheckoutID, err := m.createDodoCheckoutSession(ctx, caller, orgID, intentID, plan, input, metadata)
	if err != nil {
		return CreateBillingCheckoutResult{}, err
	}
	intent, err := m.repo.CreateBillingCheckoutIntent(ctx, repository.BillingCheckoutIntentInput{
		ID:               intentID,
		OrganizationID:   orgID,
		CreatedByUserID:  caller.UserID,
		RequestedPlanKey: plan.Key,
		BillingPeriod:    input.BillingPeriod,
		SeatQuantity:     input.SeatQuantity,
		ReturnURL:        input.ReturnURL,
		CheckoutURL:      checkoutURL,
		DodoCheckoutID:   dodoCheckoutID,
		Metadata:         metadata,
	})
	if err != nil {
		return CreateBillingCheckoutResult{}, err
	}
	return CreateBillingCheckoutResult{
		CheckoutIntentID: intent.ID,
		CheckoutURL:      intent.CheckoutURL,
		PlanKey:          intent.RequestedPlanKey,
		BillingPeriod:    intent.BillingPeriod,
		SeatQuantity:     intent.SeatQuantity,
	}, nil
}

func (m *BillingManager) CreatePortal(ctx context.Context, caller Caller, orgID uuid.UUID) (CreateBillingPortalResult, error) {
	if err := m.orgAuthz.AuthorizeOrganizationAdmin(ctx, caller, orgID); err != nil {
		return CreateBillingPortalResult{}, err
	}
	if m.dodoAPIKey == "" {
		return CreateBillingPortalResult{}, validationError("billing_not_configured", "Dodo Payments customer portal is not configured")
	}
	account, err := m.repo.GetBillingAccount(ctx, orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CreateBillingPortalResult{}, validationError("billing_customer_not_found", "Dodo customer does not exist yet for this organization")
		}
		return CreateBillingPortalResult{}, err
	}
	if account.DodoCustomerID == nil || strings.TrimSpace(*account.DodoCustomerID) == "" {
		return CreateBillingPortalResult{}, validationError("billing_customer_not_found", "Dodo customer does not exist yet for this organization")
	}
	portalURL, err := m.createDodoCustomerPortalSession(ctx, *account.DodoCustomerID)
	if err != nil {
		return CreateBillingPortalResult{}, err
	}
	return CreateBillingPortalResult{PortalURL: portalURL}, nil
}

func (m *BillingManager) GetWorkspaceEntitlements(ctx context.Context, caller Caller, workspaceID uuid.UUID) (WorkspaceEntitlementsResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionReadWorkspace); err != nil {
		return WorkspaceEntitlementsResult{}, err
	}
	orgID, entitlements, err := m.repo.ResolveWorkspaceEntitlements(ctx, workspaceID)
	if err != nil {
		return WorkspaceEntitlementsResult{}, err
	}
	now := m.now().UTC()
	windowStart, windowEnd := usageWindow(now)
	usage, err := m.repo.GetWorkspaceUsageSnapshot(ctx, workspaceID, windowStart, windowEnd)
	if err != nil {
		return WorkspaceEntitlementsResult{}, err
	}
	runDecision := billingpkg.CheckEntitlementActive(entitlements, now)
	if runDecision.Allowed {
		runDecision = billingpkg.CheckRaceQuota(entitlements, usage.RaceCount, 1, windowEnd)
		if runDecision.Allowed {
			runDecision = billingpkg.CheckConcurrency(entitlements, usage.ActiveRuns, 1)
		}
	}
	return WorkspaceEntitlementsResult{
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		Entitlements:   entitlements,
		Usage:          usage,
		Gates: WorkspaceGateSummary{
			Run: runDecision,
		},
	}, nil
}

func (m *BillingManager) BuildRunGate(ctx context.Context, workspaceID uuid.UUID, participantCount int, raceCount int) (*repository.RunEntitlementGate, error) {
	_, entitlements, err := m.repo.ResolveWorkspaceEntitlements(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	now := m.now().UTC()
	if decision := billingpkg.CheckEntitlementActive(entitlements, now); !decision.Allowed {
		return nil, billingpkg.GateError{Decision: decision}
	}
	if decision := billingpkg.CheckMaxModels(entitlements, participantCount); !decision.Allowed {
		return nil, billingpkg.GateError{Decision: decision}
	}
	if raceCount <= 0 {
		raceCount = 1
	}
	windowStart, windowEnd := usageWindow(now)
	usage, err := m.repo.GetWorkspaceUsageSnapshot(ctx, workspaceID, windowStart, windowEnd)
	if err != nil {
		return nil, err
	}
	if decision := billingpkg.CheckRaceQuota(entitlements, usage.RaceCount, raceCount, windowEnd); !decision.Allowed {
		return nil, billingpkg.GateError{Decision: decision}
	}
	if decision := billingpkg.CheckConcurrency(entitlements, usage.ActiveRuns, raceCount); !decision.Allowed {
		return nil, billingpkg.GateError{Decision: decision}
	}
	return &repository.RunEntitlementGate{
		Entitlements:    entitlements,
		RaceCost:        raceCount,
		ConcurrencyCost: raceCount,
		WindowStart:     windowStart,
		WindowEnd:       windowEnd,
	}, nil
}

func (m *BillingManager) BuildWorkspaceCreationGate(ctx context.Context, orgID uuid.UUID) (*repository.OrganizationEntitlementGate, error) {
	entitlements, err := m.repo.GetOrganizationEntitlements(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if decision := billingpkg.CheckEntitlementActive(entitlements, m.now().UTC()); !decision.Allowed {
		return nil, billingpkg.GateError{Decision: decision}
	}
	return &repository.OrganizationEntitlementGate{OrganizationID: orgID, Entitlements: entitlements}, nil
}

func (m *BillingManager) BuildSeatGate(ctx context.Context, orgID uuid.UUID, userAlreadyActive bool) (*repository.OrganizationEntitlementGate, error) {
	if userAlreadyActive {
		return nil, nil
	}
	entitlements, err := m.repo.GetOrganizationEntitlements(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if decision := billingpkg.CheckEntitlementActive(entitlements, m.now().UTC()); !decision.Allowed {
		return nil, billingpkg.GateError{Decision: decision}
	}
	return &repository.OrganizationEntitlementGate{OrganizationID: orgID, Entitlements: entitlements}, nil
}

func (m *BillingManager) CheckWorkspaceFeature(ctx context.Context, workspaceID uuid.UUID, feature string) error {
	_, entitlements, err := m.repo.ResolveWorkspaceEntitlements(ctx, workspaceID)
	if err != nil {
		return err
	}
	if decision := billingpkg.CheckFeature(entitlements, feature); !decision.Allowed {
		return billingpkg.GateError{Decision: decision}
	}
	return nil
}

func (m *BillingManager) ProcessDodoWebhook(ctx context.Context, headers DodoWebhookHeaders, rawBody []byte) (ProcessDodoWebhookResult, error) {
	if err := m.verifyWebhook(headers, rawBody); err != nil {
		return ProcessDodoWebhookResult{}, err
	}

	envelope, err := parseDodoWebhookEnvelope(rawBody)
	if err != nil {
		return ProcessDodoWebhookResult{}, err
	}
	eventTime := envelope.EventTime()
	payloadType := envelope.PayloadType()
	eventInput := repository.BillingWebhookEventInput{
		WebhookID:      headers.WebhookID,
		EventType:      envelope.Type,
		DodoBusinessID: optionalString(envelope.BusinessID),
		PayloadType:    optionalString(payloadType),
		EventTimestamp: eventTime,
		Payload:        rawBody,
	}

	var duplicate bool
	if strings.HasPrefix(envelope.Type, "subscription.") || envelope.Type == "payment.succeeded" || envelope.Type == "payment.failed" || strings.HasPrefix(envelope.Type, "dunning.") {
		duplicate, err = m.applySubscriptionWebhook(ctx, eventInput, envelope)
	} else {
		duplicate, err = m.repo.ApplyBillingWebhookEvent(ctx, eventInput, repository.BillingWebhookApplication{})
	}
	if errors.Is(err, repository.ErrBillingWebhookAlreadyProcessed) {
		return ProcessDodoWebhookResult{Duplicate: true, EventType: envelope.Type}, nil
	}
	if err != nil {
		return ProcessDodoWebhookResult{}, err
	}
	if duplicate {
		return ProcessDodoWebhookResult{Duplicate: true, EventType: envelope.Type}, nil
	}
	return ProcessDodoWebhookResult{EventType: envelope.Type}, nil
}

func (m *BillingManager) applySubscriptionWebhook(ctx context.Context, eventInput repository.BillingWebhookEventInput, envelope dodoWebhookEnvelope) (bool, error) {
	subscriptionID := envelope.DataString("subscription_id", "id", "dodo_subscription_id")
	customerID := envelope.DataString("customer_id", "dodo_customer_id")
	productID := envelope.DataString("product_id", "dodo_product_id")
	status := billingpkg.DodoStatusFromEvent(envelope.Type, envelope.DataString("status", "subscription_status"))
	seatQuantity := envelope.DataInt("quantity", "seat_quantity", "seats")

	orgID, err := envelope.OrganizationID()
	if err != nil {
		orgID, err = m.repo.FindOrganizationByDodoSubscriptionOrCustomer(ctx, subscriptionID, customerID)
	}
	if err != nil {
		return false, fmt.Errorf("resolve webhook organization: %w", err)
	}
	if productID == "" || seatQuantity == 0 {
		overview, overviewErr := m.repo.GetBillingOverview(ctx, orgID)
		if overviewErr == nil && overview.Subscription != nil && (subscriptionID == "" || overview.Subscription.DodoSubscriptionID == subscriptionID) {
			if productID == "" {
				productID = overview.Subscription.DodoProductID
			}
			if seatQuantity == 0 {
				seatQuantity = overview.Subscription.SeatQuantity
			}
		}
	}
	if seatQuantity == 0 {
		seatQuantity = 1
	}

	mapping, err := billingpkg.MapDodoProductFromCatalog(productID, m.plans)
	if err != nil {
		return false, err
	}
	plan, ok := m.planByKey(mapping.PlanKey)
	if !ok {
		return false, fmt.Errorf("%w: %s", billingpkg.ErrUnknownPlan, mapping.PlanKey)
	}
	addons, _ := json.Marshal(envelope.DataObject("addon_quantities"))
	if len(addons) == 0 || string(addons) == "null" {
		addons = []byte(`{}`)
	}
	eventTime := envelope.EventTimeValue(m.now().UTC())
	subscriptionInput := repository.BillingSubscriptionInput{
		OrganizationID:          orgID,
		DodoSubscriptionID:      subscriptionID,
		DodoCustomerID:          customerID,
		DodoProductID:           productID,
		PlanKey:                 mapping.PlanKey,
		BillingPeriod:           mapping.BillingPeriod,
		Status:                  status,
		NextBillingDate:         envelope.DataTime("next_billing_date", "current_period_end"),
		CancelAtNextBillingDate: envelope.DataBool("cancel_at_next_billing_date", "cancel_at_period_end"),
		CancelledAt:             envelope.DataTime("cancelled_at", "canceled_at"),
		ExpiresAt:               envelope.DataTime("expires_at", "current_period_end"),
		TrialPeriodDays:         envelope.DataIntPtr("trial_period_days"),
		SeatQuantity:            seatQuantity,
		AddonQuantities:         addons,
		LatestDodoEventAt:       &eventTime,
	}

	entitlements := billingpkg.DefaultEntitlements()
	useSubscriptionAsSource := false
	entitlementExpiresAt := (*time.Time)(nil)
	if shouldRetainPaidPlanInactive(status) {
		// Inactive entitlements do not grant access, so the seat-minimum check
		// is not load-bearing here. Skipping it prevents on_hold/failed
		// webhooks with malformed quantities from looping in Dodo retries.
		entitlements = billingpkg.MaterializeEntitlements(plan, mapping.BillingPeriod, seatQuantity, billingpkg.EntitlementStatusInactive)
		useSubscriptionAsSource = true
		entitlementExpiresAt = subscriptionInput.ExpiresAt
		entitlements.ExpiresAt = subscriptionInput.ExpiresAt
	} else if billingpkg.DodoStatusIsEntitled(status) {
		if err := billingpkg.ValidateSeatQuantity(plan, seatQuantity); err != nil {
			return false, err
		}
		entitlements = billingpkg.MaterializeEntitlements(plan, mapping.BillingPeriod, seatQuantity, entitlementStatusForDodoStatus(status))
		useSubscriptionAsSource = true
		if shouldExpireMaterializedEntitlement(status, subscriptionInput.CancelAtNextBillingDate, subscriptionInput.ExpiresAt) {
			entitlementExpiresAt = subscriptionInput.ExpiresAt
			entitlements.ExpiresAt = subscriptionInput.ExpiresAt
		}
	}

	application := repository.BillingWebhookApplication{
		Entitlements: &repository.BillingWebhookEntitlementsInput{
			OrganizationID:          orgID,
			Entitlements:            entitlements,
			UseSubscriptionAsSource: useSubscriptionAsSource,
			ExpiresAt:               entitlementExpiresAt,
		},
		CheckoutIntentID: envelope.CheckoutIntentID(),
	}
	// Skip the subscription upsert when there's no Dodo subscription ID
	// (e.g. one-off payment.succeeded events): otherwise every such event
	// collides on UNIQUE(dodo_subscription_id) into a single ghost row keyed
	// by '' that can shadow the real subscription.
	if subscriptionID != "" {
		application.Subscription = &subscriptionInput
	}
	if customerID != "" {
		application.Account = &repository.BillingAccountInput{
			OrganizationID: orgID,
			DodoCustomerID: customerID,
			BillingEmail:   envelope.DataString("customer_email", "email"),
			Status:         "active",
		}
	}
	return m.repo.ApplyBillingWebhookEvent(ctx, eventInput, application)
}

func shouldExpireMaterializedEntitlement(status string, cancelAtNextBillingDate bool, expiresAt *time.Time) bool {
	if expiresAt == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case billingpkg.EntitlementStatusTrialing, billingpkg.EntitlementStatusExpired, billingpkg.EntitlementStatusInactive, "cancelled", "canceled", "failed", "on_hold":
		return true
	default:
		return cancelAtNextBillingDate
	}
}

func shouldRetainPaidPlanInactive(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "on_hold", "failed", billingpkg.EntitlementStatusInactive:
		return true
	default:
		return false
	}
}

func entitlementStatusForDodoStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case billingpkg.EntitlementStatusTrialing:
		return billingpkg.EntitlementStatusTrialing
	default:
		return billingpkg.EntitlementStatusActive
	}
}

func (m *BillingManager) createDodoCheckoutSession(ctx context.Context, caller Caller, _ uuid.UUID, intentID uuid.UUID, plan billingpkg.Plan, input CreateBillingCheckoutInput, metadata json.RawMessage) (string, string, error) {
	productID := plan.DodoProductIDs[input.BillingPeriod]
	if productID == "" {
		return "", "", validationError("invalid_billing_period", "selected plan does not have a Dodo product for this billing period")
	}
	// Dodo subscription line items must have quantity=1; per-seat billing is
	// expressed via add-ons, not by multiplying the subscription quantity.
	// See https://docs.dodopayments.com/developer-resources/seat-based-pricing.
	requestPayload := map[string]any{
		"product_cart": []map[string]any{
			{
				"product_id": productID,
				"quantity":   1,
			},
		},
		"return_url": input.ReturnURL,
		"metadata":   json.RawMessage(metadata),
	}
	if caller.Email != "" || caller.DisplayName != "" {
		requestPayload["customer"] = map[string]string{
			"email": caller.Email,
			"name":  caller.DisplayName,
		}
	}
	payload, err := json.Marshal(requestPayload)
	if err != nil {
		return "", "", fmt.Errorf("marshal dodo checkout request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.dodoAPIBaseURL+"/checkouts", strings.NewReader(string(payload)))
	if err != nil {
		return "", "", fmt.Errorf("build dodo checkout request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.dodoAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("create dodo checkout session: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", fmt.Errorf("read dodo checkout response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("create dodo checkout session: dodo returned HTTP %d", resp.StatusCode)
	}
	var decoded struct {
		SessionID   string `json:"session_id"`
		CheckoutURL string `json:"checkout_url"`
	}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", "", fmt.Errorf("decode dodo checkout response: %w", err)
	}
	if strings.TrimSpace(decoded.CheckoutURL) == "" {
		return "", "", errors.New("dodo checkout response did not include checkout_url")
	}
	parsed, err := url.Parse(decoded.CheckoutURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", "", errors.New("dodo checkout response url is not a valid https URL")
	}
	return parsed.String(), decoded.SessionID, nil
}

func (m *BillingManager) createDodoCustomerPortalSession(ctx context.Context, customerID string) (string, error) {
	endpoint := fmt.Sprintf("%s/customers/%s/customer-portal/session?send_email=false", m.dodoAPIBaseURL, url.PathEscape(customerID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(`{}`))
	if err != nil {
		return "", fmt.Errorf("build dodo customer portal request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.dodoAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create dodo customer portal session: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read dodo customer portal response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("create dodo customer portal session: dodo returned HTTP %d", resp.StatusCode)
	}
	var decoded struct {
		Link string `json:"link"`
	}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", fmt.Errorf("decode dodo customer portal response: %w", err)
	}
	if strings.TrimSpace(decoded.Link) == "" {
		return "", errors.New("dodo customer portal response did not include link")
	}
	parsed, err := url.Parse(decoded.Link)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", errors.New("dodo customer portal response url is not a valid https URL")
	}
	return parsed.String(), nil
}

func (m *BillingManager) verifyWebhook(headers DodoWebhookHeaders, rawBody []byte) error {
	if m.webhookSecret == "" {
		return errors.New("dodo webhook secret is not configured")
	}
	if strings.TrimSpace(headers.WebhookID) == "" || strings.TrimSpace(headers.WebhookTimestamp) == "" || strings.TrimSpace(headers.WebhookSignature) == "" {
		return errInvalidWebhookSignature
	}
	timestamp, err := parseDodoWebhookTimestamp(headers.WebhookTimestamp)
	if err != nil {
		return errInvalidWebhookSignature
	}
	now := m.now().UTC()
	if timestamp.Before(now.Add(-dodoWebhookTimestampTolerance)) || timestamp.After(now.Add(dodoWebhookTimestampTolerance)) {
		return errInvalidWebhookSignature
	}
	secret, err := decodeDodoWebhookSecret(m.webhookSecret)
	if err != nil {
		return errInvalidWebhookSignature
	}
	signedPayload := headers.WebhookID + "." + headers.WebhookTimestamp + "." + string(rawBody)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signedPayload))
	expected := mac.Sum(nil)
	if !dodoSignatureMatches(headers.WebhookSignature, expected) {
		return errInvalidWebhookSignature
	}
	return nil
}

func decodeDodoWebhookSecret(secret string) ([]byte, error) {
	secret = strings.TrimSpace(secret)
	if !strings.HasPrefix(secret, "whsec_") {
		return nil, errInvalidWebhookSignature
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		return nil, errInvalidWebhookSignature
	}
	if len(decoded) == 0 {
		return nil, errInvalidWebhookSignature
	}
	return decoded, nil
}

func parseDodoWebhookTimestamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		return time.Unix(seconds, 0).UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func dodoSignatureMatches(header string, expected []byte) bool {
	base64Expected := base64.StdEncoding.EncodeToString(expected)
	for _, part := range strings.Fields(strings.TrimSpace(header)) {
		version, signature, ok := strings.Cut(part, ",")
		if !ok || version != "v1" || signature == "" || strings.Contains(signature, ",") {
			continue
		}
		if hmac.Equal([]byte(signature), []byte(base64Expected)) {
			return true
		}
	}
	return false
}

type dodoWebhookEnvelope struct {
	BusinessID string         `json:"business_id"`
	Type       string         `json:"type"`
	Timestamp  string         `json:"timestamp"`
	CreatedAt  string         `json:"created_at"`
	Data       map[string]any `json:"data"`
	Metadata   map[string]any `json:"metadata"`
}

func parseDodoWebhookEnvelope(rawBody []byte) (dodoWebhookEnvelope, error) {
	var envelope dodoWebhookEnvelope
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		return dodoWebhookEnvelope{}, validationError("invalid_webhook_payload", "webhook body must be valid JSON")
	}
	envelope.Type = strings.TrimSpace(envelope.Type)
	if envelope.Type == "" {
		return dodoWebhookEnvelope{}, validationError("invalid_webhook_payload", "webhook type is required")
	}
	if envelope.Data == nil {
		envelope.Data = map[string]any{}
	}
	return envelope, nil
}

func (e dodoWebhookEnvelope) EventTime() *time.Time {
	value := e.EventTimeValue(time.Time{})
	if value.IsZero() {
		return nil
	}
	return &value
}

func (e dodoWebhookEnvelope) EventTimeValue(fallback time.Time) time.Time {
	for _, raw := range []string{e.Timestamp, e.CreatedAt, e.DataString("timestamp", "created_at")} {
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, raw)
		if err == nil {
			return parsed.UTC()
		}
	}
	return fallback
}

func (e dodoWebhookEnvelope) PayloadType() string {
	return e.DataString("payload_type")
}

func (e dodoWebhookEnvelope) OrganizationID() (uuid.UUID, error) {
	for _, raw := range []string{
		e.DataString("organization_id"),
		stringFromMap(e.Metadata, "organization_id"),
		stringFromMap(e.DataObject("metadata"), "organization_id"),
	} {
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err == nil {
			return id, nil
		}
	}
	return uuid.Nil, errors.New("organization_id metadata is required")
}

func (e dodoWebhookEnvelope) CheckoutIntentID() *uuid.UUID {
	for _, raw := range []string{
		e.DataString("checkout_intent_id"),
		stringFromMap(e.Metadata, "checkout_intent_id"),
		stringFromMap(e.DataObject("metadata"), "checkout_intent_id"),
	} {
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err == nil {
			return &id
		}
	}
	return nil
}

func (e dodoWebhookEnvelope) DataString(keys ...string) string {
	for _, key := range keys {
		if value := stringFromMap(e.Data, key); value != "" {
			return value
		}
	}
	return ""
}

func (e dodoWebhookEnvelope) DataBool(keys ...string) bool {
	for _, key := range keys {
		value, ok := e.Data[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			parsed, _ := strconv.ParseBool(typed)
			return parsed
		}
	}
	return false
}

func (e dodoWebhookEnvelope) DataInt(keys ...string) int {
	for _, key := range keys {
		value, ok := e.Data[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int(typed)
		case int:
			return typed
		case string:
			parsed, _ := strconv.Atoi(typed)
			return parsed
		}
	}
	return 0
}

func (e dodoWebhookEnvelope) DataIntPtr(keys ...string) *int {
	value := e.DataInt(keys...)
	if value == 0 {
		return nil
	}
	return &value
}

func (e dodoWebhookEnvelope) DataTime(keys ...string) *time.Time {
	for _, key := range keys {
		raw := e.DataString(key)
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, raw)
		if err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func (e dodoWebhookEnvelope) DataObject(key string) map[string]any {
	value, ok := e.Data[key]
	if !ok {
		return nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return object
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

type validationErrorEnvelope struct {
	Code    string
	Message string
}

func (e validationErrorEnvelope) Error() string {
	return e.Message
}

func validationError(code, message string) validationErrorEnvelope {
	return validationErrorEnvelope{Code: code, Message: message}
}

func usageWindow(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}

func (m *BillingManager) planByKey(key string) (billingpkg.Plan, bool) {
	key = strings.TrimSpace(key)
	for _, plan := range m.plans {
		if plan.Key == key {
			return plan, true
		}
	}
	return billingpkg.Plan{}, false
}

func clonePlans(in []billingpkg.Plan) []billingpkg.Plan {
	out := make([]billingpkg.Plan, len(in))
	for i, plan := range in {
		out[i] = plan
		out[i].BillingPeriods = append([]string(nil), plan.BillingPeriods...)
		out[i].DodoProductIDs = cloneStringMap(plan.DodoProductIDs)
		out[i].FeatureFlags = cloneBoolMap(plan.FeatureFlags)
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

func cloneBoolMap(in map[string]bool) map[string]bool {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]bool, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func dodoAPIBaseURLForEnvironment(environment string) string {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "live":
		return "https://live.dodopayments.com"
	default:
		return "https://test.dodopayments.com"
	}
}

func appendCheckoutReturnParams(rawReturnURL string, intentID uuid.UUID) (string, error) {
	parsed, err := url.Parse(rawReturnURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("return_url must be an absolute URL")
	}
	values := parsed.Query()
	values.Set("checkout", "pending")
	values.Set("checkout_intent_id", intentID.String())
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func defaultHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func registerDodoWebhookRoute(router chi.Router, logger *slog.Logger, service BillingService) {
	if service == nil {
		service = noopBillingService{}
	}
	router.Post("/v1/dodo/webhooks", func(w http.ResponseWriter, r *http.Request) {
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		rawBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body must be 1 MiB or smaller")
			return
		}
		result, err := service.ProcessDodoWebhook(r.Context(), DodoWebhookHeaders{
			WebhookID:        r.Header.Get("webhook-id"),
			WebhookTimestamp: r.Header.Get("webhook-timestamp"),
			WebhookSignature: r.Header.Get("webhook-signature"),
		}, rawBody)
		if err != nil {
			switch {
			case errors.Is(err, errInvalidWebhookSignature):
				writeError(w, http.StatusUnauthorized, "invalid_webhook_signature", "webhook signature is invalid")
			default:
				var validationErr validationErrorEnvelope
				if errors.As(err, &validationErr) {
					writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
				} else {
					logger.Error("failed to process dodo webhook", "error", err)
					writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				}
			}
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	})
}

func registerBillingRoutes(router chi.Router, logger *slog.Logger, service BillingService) {
	if service == nil {
		service = noopBillingService{}
	}
	router.Get("/billing/plans", listBillingPlansHandler(logger, service))
	router.Get("/organizations/{organizationID}/billing", getOrganizationBillingHandler(logger, service))
	router.Post("/organizations/{organizationID}/billing/trial", startBillingTrialHandler(logger, service))
	router.Post("/organizations/{organizationID}/billing/checkout", createBillingCheckoutHandler(logger, service))
	router.Post("/organizations/{organizationID}/billing/portal", createBillingPortalHandler(logger, service))
	router.Get("/workspaces/{workspaceID}/entitlements", getWorkspaceEntitlementsHandler(logger, service))
}

func listBillingPlansHandler(logger *slog.Logger, service BillingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		result, err := service.ListPlans(r.Context(), caller)
		if err != nil {
			logger.Error("failed to list billing plans", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func getOrganizationBillingHandler(logger *slog.Logger, service BillingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		orgID, err := uuid.Parse(chi.URLParam(r, "organizationID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_organization_id", "organization ID is malformed")
			return
		}
		result, err := service.GetOrganizationBilling(r.Context(), caller, orgID)
		if err != nil {
			writeBillingServiceError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func startBillingTrialHandler(logger *slog.Logger, service BillingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		orgID, err := uuid.Parse(chi.URLParam(r, "organizationID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_organization_id", "organization ID is malformed")
			return
		}
		var input StartBillingTrialInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_body", "request body must be valid JSON")
			return
		}
		result, err := service.StartTrial(r.Context(), caller, orgID, input)
		if err != nil {
			writeBillingServiceError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	}
}

func createBillingCheckoutHandler(logger *slog.Logger, service BillingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		orgID, err := uuid.Parse(chi.URLParam(r, "organizationID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_organization_id", "organization ID is malformed")
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		var input CreateBillingCheckoutInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		result, err := service.CreateCheckout(r.Context(), caller, orgID, input)
		if err != nil {
			writeBillingServiceError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	}
}

func createBillingPortalHandler(logger *slog.Logger, service BillingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		orgID, err := uuid.Parse(chi.URLParam(r, "organizationID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_organization_id", "organization ID is malformed")
			return
		}
		result, err := service.CreatePortal(r.Context(), caller, orgID)
		if err != nil {
			writeBillingServiceError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func getWorkspaceEntitlementsHandler(logger *slog.Logger, service BillingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		workspaceID, err := uuid.Parse(chi.URLParam(r, "workspaceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is malformed")
			return
		}
		result, err := service.GetWorkspaceEntitlements(r.Context(), caller, workspaceID)
		if err != nil {
			writeBillingServiceError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func writeBillingServiceError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var validationErr validationErrorEnvelope
	var gateErr billingpkg.GateError
	switch {
	case errors.Is(err, ErrForbidden):
		writeAuthzError(w, err)
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
	case errors.As(err, &gateErr):
		writeBillingGateError(w, gateErr.Decision)
	default:
		logger.Error("billing service error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

type noopBillingService struct{}

func (noopBillingService) ListPlans(context.Context, Caller) (ListBillingPlansResult, error) {
	return ListBillingPlansResult{Items: billingpkg.Catalog()}, nil
}
func (noopBillingService) GetOrganizationBilling(context.Context, Caller, uuid.UUID) (repository.BillingOverview, error) {
	return repository.BillingOverview{}, errors.New("billing service is not configured")
}
func (noopBillingService) StartTrial(context.Context, Caller, uuid.UUID, StartBillingTrialInput) (repository.BillingOverview, error) {
	return repository.BillingOverview{}, errors.New("billing service is not configured")
}
func (noopBillingService) CreateCheckout(context.Context, Caller, uuid.UUID, CreateBillingCheckoutInput) (CreateBillingCheckoutResult, error) {
	return CreateBillingCheckoutResult{}, errors.New("billing service is not configured")
}
func (noopBillingService) CreatePortal(context.Context, Caller, uuid.UUID) (CreateBillingPortalResult, error) {
	return CreateBillingPortalResult{}, errors.New("billing service is not configured")
}
func (noopBillingService) GetWorkspaceEntitlements(context.Context, Caller, uuid.UUID) (WorkspaceEntitlementsResult, error) {
	return WorkspaceEntitlementsResult{}, errors.New("billing service is not configured")
}
func (noopBillingService) ProcessDodoWebhook(context.Context, DodoWebhookHeaders, []byte) (ProcessDodoWebhookResult, error) {
	return ProcessDodoWebhookResult{}, errors.New("billing service is not configured")
}
