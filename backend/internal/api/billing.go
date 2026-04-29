package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
)

type BillingService interface {
	ListPlans(ctx context.Context, caller Caller) (ListBillingPlansResult, error)
	GetOrganizationBilling(ctx context.Context, caller Caller, orgID uuid.UUID) (repository.BillingOverview, error)
	CreateCheckout(ctx context.Context, caller Caller, orgID uuid.UUID, input CreateBillingCheckoutInput) (CreateBillingCheckoutResult, error)
	CreatePortal(ctx context.Context, caller Caller, orgID uuid.UUID) (CreateBillingPortalResult, error)
	GetWorkspaceEntitlements(ctx context.Context, caller Caller, workspaceID uuid.UUID) (WorkspaceEntitlementsResult, error)
	ProcessDodoWebhook(ctx context.Context, headers DodoWebhookHeaders, rawBody []byte) (ProcessDodoWebhookResult, error)
}

type EntitlementGateService interface {
	BuildRunGate(ctx context.Context, workspaceID uuid.UUID, participantCount int, raceCount int) (*repository.RunEntitlementGate, error)
	CheckWorkspaceCreation(ctx context.Context, orgID uuid.UUID) error
	CheckSeatAvailability(ctx context.Context, orgID uuid.UUID, userAlreadyActive bool) error
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
	UpsertBillingSubscription(ctx context.Context, input repository.BillingSubscriptionInput) (repository.BillingSubscription, error)
	GetBillingOverview(ctx context.Context, orgID uuid.UUID) (repository.BillingOverview, error)
	FindOrganizationByDodoSubscriptionOrCustomer(ctx context.Context, subscriptionID string, customerID string) (uuid.UUID, error)
	RecordBillingWebhookEvent(ctx context.Context, input repository.BillingWebhookEventInput) error
}

type BillingManager struct {
	orgAuthz        OrganizationAuthorizer
	authorizer      WorkspaceAuthorizer
	repo            BillingRepository
	dodoAPIKey      string
	dodoAPIBaseURL  string
	httpClient      *http.Client
	webhookSecret   string
	checkoutBaseURL string
	portalBaseURL   string
	now             func() time.Time
}

var errInvalidWebhookSignature = errors.New("invalid webhook signature")

type BillingManagerConfig struct {
	DodoAPIKey      string
	DodoAPIBaseURL  string
	HTTPClient      *http.Client
	WebhookSecret   string
	CheckoutBaseURL string
	PortalBaseURL   string
}

func NewBillingManager(orgAuthz OrganizationAuthorizer, authorizer WorkspaceAuthorizer, repo BillingRepository, cfg BillingManagerConfig) *BillingManager {
	return &BillingManager{
		orgAuthz:        orgAuthz,
		authorizer:      authorizer,
		repo:            repo,
		dodoAPIKey:      strings.TrimSpace(cfg.DodoAPIKey),
		dodoAPIBaseURL:  strings.TrimRight(defaultBillingString(cfg.DodoAPIBaseURL, "https://api.dodopayments.com"), "/"),
		httpClient:      defaultHTTPClient(cfg.HTTPClient),
		webhookSecret:   strings.TrimSpace(cfg.WebhookSecret),
		checkoutBaseURL: strings.TrimRight(defaultBillingString(cfg.CheckoutBaseURL, "https://checkout.dodopayments.com/checkout"), "/"),
		portalBaseURL:   strings.TrimRight(defaultBillingString(cfg.PortalBaseURL, "https://app.dodopayments.com/customer-portal"), "/"),
		now:             time.Now,
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
	return ListBillingPlansResult{Items: billingpkg.Catalog()}, nil
}

func (m *BillingManager) GetOrganizationBilling(ctx context.Context, caller Caller, orgID uuid.UUID) (repository.BillingOverview, error) {
	if err := m.orgAuthz.AuthorizeOrganizationAdmin(ctx, caller, orgID); err != nil {
		return repository.BillingOverview{}, err
	}
	return m.repo.GetBillingOverview(ctx, orgID)
}

func (m *BillingManager) CreateCheckout(ctx context.Context, caller Caller, orgID uuid.UUID, input CreateBillingCheckoutInput) (CreateBillingCheckoutResult, error) {
	if err := m.orgAuthz.AuthorizeOrganizationAdmin(ctx, caller, orgID); err != nil {
		return CreateBillingCheckoutResult{}, err
	}
	plan, ok := billingpkg.PlanByKey(strings.TrimSpace(input.PlanKey))
	if !ok || plan.Key == billingpkg.PlanFree {
		return CreateBillingCheckoutResult{}, validationError("invalid_plan_key", "plan_key must be pro, team, or enterprise")
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
	metadata, err := json.Marshal(map[string]any{
		"organization_id":    orgID.String(),
		"requested_plan_key": plan.Key,
		"billing_period":     input.BillingPeriod,
		"seat_quantity":      input.SeatQuantity,
	})
	if err != nil {
		return CreateBillingCheckoutResult{}, err
	}
	intentID := uuid.New()
	checkoutURL, dodoCheckoutID, err := m.createDodoCheckoutSession(ctx, caller, orgID, intentID, plan, input, metadata)
	if err != nil {
		return CreateBillingCheckoutResult{}, err
	}
	intent, err := m.repo.CreateBillingCheckoutIntent(ctx, repository.BillingCheckoutIntentInput{
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
	portalURL := fmt.Sprintf("%s?organization_id=%s", m.portalBaseURL, url.QueryEscape(orgID.String()))
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

func (m *BillingManager) CheckWorkspaceCreation(ctx context.Context, orgID uuid.UUID) error {
	entitlements, err := m.repo.GetOrganizationEntitlements(ctx, orgID)
	if err != nil {
		return err
	}
	if decision := billingpkg.CheckEntitlementActive(entitlements, m.now().UTC()); !decision.Allowed {
		return billingpkg.GateError{Decision: decision}
	}
	count, err := m.repo.CountActiveWorkspaces(ctx, orgID)
	if err != nil {
		return err
	}
	if decision := billingpkg.CheckWorkspaceLimit(entitlements, count, 1); !decision.Allowed {
		return billingpkg.GateError{Decision: decision}
	}
	return nil
}

func (m *BillingManager) CheckSeatAvailability(ctx context.Context, orgID uuid.UUID, userAlreadyActive bool) error {
	if userAlreadyActive {
		return nil
	}
	entitlements, err := m.repo.GetOrganizationEntitlements(ctx, orgID)
	if err != nil {
		return err
	}
	if decision := billingpkg.CheckEntitlementActive(entitlements, m.now().UTC()); !decision.Allowed {
		return billingpkg.GateError{Decision: decision}
	}
	activeSeats, err := m.repo.CountActiveOrgMembers(ctx, orgID)
	if err != nil {
		return err
	}
	if decision := billingpkg.CheckSeatLimit(entitlements, activeSeats, 1); !decision.Allowed {
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
	recordErr := m.repo.RecordBillingWebhookEvent(ctx, repository.BillingWebhookEventInput{
		WebhookID:      headers.WebhookID,
		EventType:      envelope.Type,
		DodoBusinessID: optionalString(envelope.BusinessID),
		PayloadType:    optionalString(payloadType),
		EventTimestamp: eventTime,
		Payload:        rawBody,
	})
	if errors.Is(recordErr, repository.ErrBillingWebhookAlreadyProcessed) {
		return ProcessDodoWebhookResult{Duplicate: true, EventType: envelope.Type}, nil
	}
	if recordErr != nil {
		return ProcessDodoWebhookResult{}, recordErr
	}

	if strings.HasPrefix(envelope.Type, "subscription.") || envelope.Type == "payment.failed" || strings.HasPrefix(envelope.Type, "dunning.") {
		if err := m.applySubscriptionWebhook(ctx, envelope); err != nil {
			return ProcessDodoWebhookResult{}, err
		}
	}
	return ProcessDodoWebhookResult{EventType: envelope.Type}, nil
}

func (m *BillingManager) applySubscriptionWebhook(ctx context.Context, envelope dodoWebhookEnvelope) error {
	subscriptionID := envelope.DataString("subscription_id", "id", "dodo_subscription_id")
	customerID := envelope.DataString("customer_id", "dodo_customer_id")
	productID := envelope.DataString("product_id", "dodo_product_id")
	status := billingpkg.DodoStatusFromEvent(envelope.Type, envelope.DataString("status", "subscription_status"))
	seatQuantity := envelope.DataInt("quantity", "seat_quantity", "seats")
	if seatQuantity == 0 {
		seatQuantity = 1
	}

	orgID, err := envelope.OrganizationID()
	if err != nil {
		orgID, err = m.repo.FindOrganizationByDodoSubscriptionOrCustomer(ctx, subscriptionID, customerID)
	}
	if err != nil {
		return fmt.Errorf("resolve webhook organization: %w", err)
	}

	mapping, err := billingpkg.MapDodoProduct(productID)
	if err != nil {
		return err
	}
	plan, ok := billingpkg.PlanByKey(mapping.PlanKey)
	if !ok {
		return fmt.Errorf("%w: %s", billingpkg.ErrUnknownPlan, mapping.PlanKey)
	}
	addons, _ := json.Marshal(envelope.DataObject("addon_quantities"))
	if len(addons) == 0 || string(addons) == "null" {
		addons = []byte(`{}`)
	}
	eventTime := envelope.EventTimeValue(m.now().UTC())
	subscription, err := m.repo.UpsertBillingSubscription(ctx, repository.BillingSubscriptionInput{
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
	})
	if err != nil {
		return err
	}
	if customerID != "" {
		if err := m.repo.UpsertBillingAccount(ctx, orgID, customerID, envelope.DataString("customer_email", "email"), "active"); err != nil {
			return err
		}
	}

	entitlements := billingpkg.DefaultEntitlements()
	sourceSubscriptionID := (*uuid.UUID)(nil)
	entitlementExpiresAt := (*time.Time)(nil)
	if billingpkg.DodoStatusIsEntitled(status) {
		if err := billingpkg.ValidateSeatQuantity(plan, seatQuantity); err != nil {
			return err
		}
		entitlements = billingpkg.MaterializeEntitlements(plan, mapping.BillingPeriod, seatQuantity, status)
		sourceSubscriptionID = &subscription.ID
		if shouldExpireMaterializedEntitlement(status, subscription.CancelAtNextBillingDate, subscription.ExpiresAt) {
			entitlementExpiresAt = subscription.ExpiresAt
			entitlements.ExpiresAt = subscription.ExpiresAt
		}
	}
	return m.repo.UpsertOrganizationEntitlements(ctx, orgID, entitlements, sourceSubscriptionID, entitlementExpiresAt)
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

func (m *BillingManager) createDodoCheckoutSession(ctx context.Context, caller Caller, _ uuid.UUID, intentID uuid.UUID, plan billingpkg.Plan, input CreateBillingCheckoutInput, metadata json.RawMessage) (string, string, error) {
	if m.dodoAPIKey == "" {
		return m.checkoutURL(intentID, plan.Key, input.BillingPeriod, input.SeatQuantity, input.ReturnURL), "", nil
	}
	productID := plan.DodoProductIDs[input.BillingPeriod]
	if productID == "" {
		return "", "", validationError("invalid_billing_period", "selected plan does not have a Dodo product for this billing period")
	}
	requestPayload := map[string]any{
		"product_cart": []map[string]any{
			{
				"product_id": productID,
				"quantity":   input.SeatQuantity,
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
	return decoded.CheckoutURL, decoded.SessionID, nil
}

func (m *BillingManager) checkoutURL(intentID uuid.UUID, planKey string, period string, seats int, returnURL string) string {
	values := url.Values{}
	values.Set("client_reference_id", intentID.String())
	values.Set("plan_key", planKey)
	values.Set("billing_period", period)
	values.Set("seat_quantity", strconv.Itoa(seats))
	values.Set("return_url", returnURL)
	return m.checkoutBaseURL + "?" + values.Encode()
}

func (m *BillingManager) verifyWebhook(headers DodoWebhookHeaders, rawBody []byte) error {
	if m.webhookSecret == "" {
		return errors.New("dodo webhook secret is not configured")
	}
	if strings.TrimSpace(headers.WebhookID) == "" || strings.TrimSpace(headers.WebhookTimestamp) == "" || strings.TrimSpace(headers.WebhookSignature) == "" {
		return errInvalidWebhookSignature
	}
	signedPayload := headers.WebhookID + "." + headers.WebhookTimestamp + "." + string(rawBody)
	mac := hmac.New(sha256.New, []byte(m.webhookSecret))
	_, _ = mac.Write([]byte(signedPayload))
	expected := mac.Sum(nil)
	if !signatureMatches(headers.WebhookSignature, expected) {
		return errInvalidWebhookSignature
	}
	return nil
}

func signatureMatches(header string, expected []byte) bool {
	hexExpected := hex.EncodeToString(expected)
	base64Expected := base64.StdEncoding.EncodeToString(expected)
	for _, candidate := range strings.Fields(strings.TrimSpace(header)) {
		for _, part := range strings.Split(candidate, ",") {
			part = strings.TrimSpace(part)
			if part == "" || part == "v1" {
				continue
			}
			if strings.HasPrefix(part, "v1=") {
				part = strings.TrimPrefix(part, "v1=")
			}
			if hmac.Equal([]byte(part), []byte(hexExpected)) || hmac.Equal([]byte(part), []byte(base64Expected)) {
				return true
			}
			if decoded, err := hex.DecodeString(part); err == nil && hmac.Equal(decoded, expected) {
				return true
			}
			if decoded, err := base64.StdEncoding.DecodeString(part); err == nil && hmac.Equal(decoded, expected) {
				return true
			}
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

func defaultBillingString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
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
		writeBillingGateError(w, http.StatusConflict, gateErr.Decision)
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
