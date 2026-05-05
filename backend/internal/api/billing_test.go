package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	billingpkg "github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestBillingManagerBuildRunGateRejectsFreeModelLimit(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret: "secret",
	})

	_, err := manager.BuildRunGate(context.Background(), workspaceID, 5, 1)
	if err == nil {
		t.Fatal("expected gate error")
	}
	var gateErr billingpkg.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("error = %v, want billing.GateError", err)
	}
	if gateErr.Decision.Code != billingpkg.GateCodePlanLimitExceeded {
		t.Fatalf("gate code = %q, want %q", gateErr.Decision.Code, billingpkg.GateCodePlanLimitExceeded)
	}
	if gateErr.Decision.PlanKey != billingpkg.PlanFree {
		t.Fatalf("plan key = %q, want free", gateErr.Decision.PlanKey)
	}
}

func TestBillingManagerBuildRunGateRejectsQuotaAndConcurrency(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	repo.usage.RaceCount = 25
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret: "secret",
	})

	_, err := manager.BuildRunGate(context.Background(), workspaceID, 1, 1)
	var gateErr billingpkg.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("quota error = %v, want billing.GateError", err)
	}
	if gateErr.Decision.Code != billingpkg.GateCodeQuotaExceeded {
		t.Fatalf("quota gate code = %q, want %q", gateErr.Decision.Code, billingpkg.GateCodeQuotaExceeded)
	}

	repo.usage.RaceCount = 0
	repo.usage.ActiveRuns = 1
	_, err = manager.BuildRunGate(context.Background(), workspaceID, 1, 1)
	if !errors.As(err, &gateErr) {
		t.Fatalf("concurrency error = %v, want billing.GateError", err)
	}
	if gateErr.Decision.Code != billingpkg.GateCodeConcurrencyLimitExceeded {
		t.Fatalf("concurrency gate code = %q, want %q", gateErr.Decision.Code, billingpkg.GateCodeConcurrencyLimitExceeded)
	}
}

func TestBillingManagerBuildRunGateRejectsExpiredTrial(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(-time.Second)
	repo.entitlements = billingpkg.MaterializeEntitlements(billingpkg.MustPlan(billingpkg.PlanPro), billingpkg.PeriodMonthly, 5, billingpkg.EntitlementStatusTrialing)
	repo.entitlements.ExpiresAt = &expiresAt
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret: "secret",
	})
	manager.now = func() time.Time { return now }

	_, err := manager.BuildRunGate(context.Background(), workspaceID, 1, 1)
	var gateErr billingpkg.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("error = %v, want billing.GateError", err)
	}
	if gateErr.Decision.Code != billingpkg.GateCodeEntitlementExpired {
		t.Fatalf("gate code = %q, want %q", gateErr.Decision.Code, billingpkg.GateCodeEntitlementExpired)
	}
	if gateErr.Decision.PlanKey != billingpkg.PlanPro || gateErr.Decision.UpgradeTarget != billingpkg.PlanPro {
		t.Fatalf("plan/upgrade = %q/%q, want pro/pro", gateErr.Decision.PlanKey, gateErr.Decision.UpgradeTarget)
	}
}

func TestBillingGateHTTPStatus(t *testing.T) {
	entitlements := billingpkg.DefaultEntitlements()
	featureDecision := billingpkg.CheckFeature(entitlements, billingpkg.FeaturePrivateChallengePacks)
	if got := billingGateHTTPStatus(featureDecision); got != http.StatusForbidden {
		t.Fatalf("feature gate status = %d, want %d", got, http.StatusForbidden)
	}

	quotaDecision := billingpkg.CheckRaceQuota(entitlements, 25, 1, time.Now())
	if got := billingGateHTTPStatus(quotaDecision); got != http.StatusPaymentRequired {
		t.Fatalf("quota gate status = %d, want %d", got, http.StatusPaymentRequired)
	}
}

func TestBillingManagerProcessesSignedDodoWebhook(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	secret := dodoTestWebhookSecret()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret:  secret,
		DodoProductIDs: testDodoProductIDs(),
	})
	manager.now = func() time.Time { return time.Unix(1777420800, 0).UTC() }

	body := `{"business_id":"biz_test","type":"subscription.active","timestamp":"2026-04-29T00:00:00Z","data":{"payload_type":"Subscription","subscription_id":"sub_test","customer_id":"cus_test","product_id":"agentclash_pro_monthly","status":"active","quantity":5,"metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	headers := signedDodoHeaders(secret, "wh_test_432", "1777420800", body)

	result, err := manager.ProcessDodoWebhook(context.Background(), headers, []byte(body))
	if err != nil {
		t.Fatalf("ProcessDodoWebhook returned error: %v", err)
	}
	if result.Duplicate {
		t.Fatal("first webhook should not be duplicate")
	}
	if repo.subscription.PlanKey != billingpkg.PlanPro {
		t.Fatalf("subscription plan = %q, want pro", repo.subscription.PlanKey)
	}
	if repo.entitlements.PlanKey != billingpkg.PlanPro {
		t.Fatalf("materialized plan = %q, want pro", repo.entitlements.PlanKey)
	}
	if repo.entitlements.ExpiresAt != nil {
		t.Fatalf("active paid entitlement expires_at = %v, want nil", repo.entitlements.ExpiresAt)
	}
	assertIntPtr(t, "pro materialized quota", repo.entitlements.RacesPerWorkspaceMonth, 2500)

	result, err = manager.ProcessDodoWebhook(context.Background(), headers, []byte(body))
	if err != nil {
		t.Fatalf("duplicate ProcessDodoWebhook returned error: %v", err)
	}
	if !result.Duplicate {
		t.Fatal("second webhook should be duplicate")
	}
}

func TestBillingManagerProcessesTrialingDodoWebhookWithExpiry(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	secret := dodoTestWebhookSecret()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret:  secret,
		DodoProductIDs: testDodoProductIDs(),
	})
	manager.now = func() time.Time { return time.Unix(1777420800, 0).UTC() }

	body := `{"business_id":"biz_test","type":"subscription.updated","timestamp":"2026-04-29T00:00:00Z","data":{"payload_type":"Subscription","subscription_id":"sub_trial","customer_id":"cus_trial","product_id":"agentclash_team_monthly","status":"trialing","quantity":2,"expires_at":"2026-06-13T00:00:00Z","metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	headers := signedDodoHeaders(secret, "wh_test_trial_432", "1777420800", body)

	if _, err := manager.ProcessDodoWebhook(context.Background(), headers, []byte(body)); err != nil {
		t.Fatalf("ProcessDodoWebhook returned error: %v", err)
	}
	if repo.entitlements.PlanKey != billingpkg.PlanTeam {
		t.Fatalf("materialized plan = %q, want team", repo.entitlements.PlanKey)
	}
	if repo.entitlements.Status != billingpkg.EntitlementStatusTrialing {
		t.Fatalf("materialized status = %q, want trialing", repo.entitlements.Status)
	}
	if repo.entitlements.ExpiresAt == nil || repo.entitlements.ExpiresAt.Format(time.RFC3339) != "2026-06-13T00:00:00Z" {
		t.Fatalf("expires_at = %v, want 2026-06-13T00:00:00Z", repo.entitlements.ExpiresAt)
	}
	assertIntPtr(t, "team trial quota", repo.entitlements.RacesPerWorkspaceMonth, 4000)
	assertIntPtr(t, "team trial models", repo.entitlements.MaxModelsPerRace, 12)
}

func TestBillingManagerProcessesOnHoldWebhookAsInactivePaidPlan(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	secret := dodoTestWebhookSecret()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret:  secret,
		DodoProductIDs: testDodoProductIDs(),
	})
	manager.now = func() time.Time { return time.Unix(1777420800, 0).UTC() }

	body := `{"business_id":"biz_test","type":"subscription.on_hold","timestamp":"2026-04-29T00:00:00Z","data":{"payload_type":"Subscription","subscription_id":"sub_hold","customer_id":"cus_hold","product_id":"agentclash_pro_monthly","status":"on_hold","quantity":5,"metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	headers := signedDodoHeaders(secret, "wh_test_hold_432", "1777420800", body)

	if _, err := manager.ProcessDodoWebhook(context.Background(), headers, []byte(body)); err != nil {
		t.Fatalf("ProcessDodoWebhook returned error: %v", err)
	}
	if repo.entitlements.PlanKey != billingpkg.PlanPro {
		t.Fatalf("materialized plan = %q, want pro", repo.entitlements.PlanKey)
	}
	if repo.entitlements.Status != billingpkg.EntitlementStatusInactive {
		t.Fatalf("materialized status = %q, want inactive", repo.entitlements.Status)
	}
}

func TestBillingManagerProcessesPaymentSucceededWithoutSubscriptionID(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	secret := dodoTestWebhookSecret()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret:  secret,
		DodoProductIDs: testDodoProductIDs(),
	})
	manager.now = func() time.Time { return time.Unix(1777420800, 0).UTC() }

	body := `{"business_id":"biz_test","type":"payment.succeeded","timestamp":"2026-04-29T00:00:00Z","data":{"payload_type":"Payment","customer_id":"cus_pay","product_id":"agentclash_pro_monthly","quantity":5,"metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	headers := signedDodoHeaders(secret, "wh_payment_no_sub", "1777420800", body)

	if _, err := manager.ProcessDodoWebhook(context.Background(), headers, []byte(body)); err != nil {
		t.Fatalf("ProcessDodoWebhook returned error: %v", err)
	}
	if repo.subscription.DodoSubscriptionID != "" {
		t.Fatalf("subscription upserted with empty dodo_subscription_id = %q, want no upsert", repo.subscription.DodoSubscriptionID)
	}
	if repo.entitlements.PlanKey != billingpkg.PlanPro {
		t.Fatalf("materialized plan = %q, want pro", repo.entitlements.PlanKey)
	}
	if repo.entitlements.Status != billingpkg.EntitlementStatusActive {
		t.Fatalf("materialized status = %q, want active", repo.entitlements.Status)
	}
}

func TestBillingManagerProcessesOnHoldWebhookWithMissingQuantity(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	secret := dodoTestWebhookSecret()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret:  secret,
		DodoProductIDs: testDodoProductIDs(),
	})
	manager.now = func() time.Time { return time.Unix(1777420800, 0).UTC() }

	body := `{"business_id":"biz_test","type":"subscription.on_hold","timestamp":"2026-04-29T00:00:00Z","data":{"payload_type":"Subscription","subscription_id":"sub_hold","customer_id":"cus_hold","product_id":"agentclash_pro_monthly","status":"on_hold","metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	headers := signedDodoHeaders(secret, "wh_test_hold_missing_qty", "1777420800", body)

	if _, err := manager.ProcessDodoWebhook(context.Background(), headers, []byte(body)); err != nil {
		t.Fatalf("ProcessDodoWebhook returned error: %v", err)
	}
	if repo.entitlements.PlanKey != billingpkg.PlanPro {
		t.Fatalf("materialized plan = %q, want pro", repo.entitlements.PlanKey)
	}
	if repo.entitlements.Status != billingpkg.EntitlementStatusInactive {
		t.Fatalf("materialized status = %q, want inactive", repo.entitlements.Status)
	}
}

func TestBillingManagerRejectsInvalidDodoWebhookSignature(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret: dodoTestWebhookSecret(),
	})

	_, err := manager.ProcessDodoWebhook(context.Background(), DodoWebhookHeaders{
		WebhookID:        "wh_bad",
		WebhookTimestamp: "1777420800",
		WebhookSignature: "not-valid",
	}, []byte(`{"type":"subscription.active","data":{}}`))
	if !errors.Is(err, errInvalidWebhookSignature) {
		t.Fatalf("error = %v, want invalid webhook signature", err)
	}
	if len(repo.webhookIDs) != 0 {
		t.Fatalf("recorded webhook count = %d, want 0", len(repo.webhookIDs))
	}
}

func TestBillingManagerRejectsLegacyDodoWebhookSignatureShapes(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	secret := dodoTestWebhookSecret()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret: secret,
	})
	manager.now = func() time.Time { return time.Unix(1777420800, 0).UTC() }

	body := `{"type":"subscription.active","data":{"subscription_id":"sub_test","customer_id":"cus_test","product_id":"agentclash_pro_monthly","status":"active","quantity":5,"metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	decoded, err := decodeDodoWebhookSecret(secret)
	if err != nil {
		t.Fatalf("decode test secret: %v", err)
	}
	mac := hmac.New(sha256.New, decoded)
	_, _ = mac.Write([]byte("wh_legacy_shape.1777420800." + body))
	digest := mac.Sum(nil)

	legacyHeaders := []string{
		base64.StdEncoding.EncodeToString(digest),
		"v1=" + base64.StdEncoding.EncodeToString(digest),
		fmt.Sprintf("%x", digest),
		"v1," + base64.StdEncoding.EncodeToString(digest) + ",extra",
	}
	for _, signature := range legacyHeaders {
		_, err := manager.ProcessDodoWebhook(context.Background(), DodoWebhookHeaders{
			WebhookID:        "wh_legacy_shape",
			WebhookTimestamp: "1777420800",
			WebhookSignature: signature,
		}, []byte(body))
		if !errors.Is(err, errInvalidWebhookSignature) {
			t.Fatalf("signature %q error = %v, want invalid webhook signature", signature, err)
		}
	}
	if len(repo.webhookIDs) != 0 {
		t.Fatalf("recorded webhook count = %d, want 0", len(repo.webhookIDs))
	}
}

func TestBillingManagerRejectsStaleDodoWebhookTimestamp(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	secret := dodoTestWebhookSecret()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret: secret,
	})
	manager.now = func() time.Time { return time.Unix(1777420800, 0).Add(10 * time.Minute).UTC() }

	body := `{"type":"subscription.active","data":{"subscription_id":"sub_test","customer_id":"cus_test","product_id":"agentclash_pro_monthly","status":"active","quantity":5,"metadata":{"organization_id":"` + repo.orgID.String() + `"}}}`
	headers := signedDodoHeaders(secret, "wh_stale_432", "1777420800", body)
	_, err := manager.ProcessDodoWebhook(context.Background(), headers, []byte(body))
	if !errors.Is(err, errInvalidWebhookSignature) {
		t.Fatalf("error = %v, want invalid webhook signature", err)
	}
	if len(repo.webhookIDs) != 0 {
		t.Fatalf("recorded webhook count = %d, want 0", len(repo.webhookIDs))
	}
}

func TestListBillingPlansHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/plans", nil)
	req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, Caller{UserID: uuid.New()}))
	rr := httptest.NewRecorder()

	listBillingPlansHandler(slog.Default(), noopBillingService{}).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"key":"free"`) || !strings.Contains(rr.Body.String(), `"key":"enterprise"`) {
		t.Fatalf("response missing plan catalog: %s", rr.Body.String())
	}
}

func TestCreateBillingCheckoutHandlerDecodesSnakeCaseJSON(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	dodoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkouts" {
			t.Fatalf("path = %s, want /checkouts", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"session_id":   "cks_handler_test",
			"checkout_url": "https://test.checkout.dodopayments.com/session/cks_handler_test",
		})
	}))
	defer dodoServer.Close()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		DodoAPIKey:     "dodo_test_key",
		DodoAPIBaseURL: dodoServer.URL,
		DodoProductIDs: testDodoProductIDs(),
		WebhookSecret:  "secret",
	})
	caller := Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			repo.orgID: {OrganizationID: repo.orgID, Role: "org_admin"},
		},
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_admin"},
		},
	}

	router := chi.NewRouter()
	router.Post("/v1/organizations/{organizationID}/billing/checkout", createBillingCheckoutHandler(slog.Default(), manager))
	req := httptest.NewRequest(http.MethodPost, "/v1/organizations/"+repo.orgID.String()+"/billing/checkout", bytes.NewBufferString(`{
		"plan_key":"pro",
		"billing_period":"monthly",
		"seat_quantity":5,
		"return_url":"http://localhost:3000/billing/return"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, caller))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"plan_key":"pro"`) || !strings.Contains(rr.Body.String(), `"seat_quantity":5`) {
		t.Fatalf("checkout response did not preserve decoded fields: %s", rr.Body.String())
	}
}

func TestBillingManagerStartTrialMaterializesSelectedPlan(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret: "secret",
	})
	manager.now = func() time.Time { return now }
	caller := Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			repo.orgID: {OrganizationID: repo.orgID, Role: "org_admin"},
		},
	}

	overview, err := manager.StartTrial(context.Background(), caller, repo.orgID, StartBillingTrialInput{
		PlanKey:       billingpkg.PlanTeam,
		BillingPeriod: billingpkg.PeriodMonthly,
	})
	if err != nil {
		t.Fatalf("StartTrial returned error: %v", err)
	}
	if overview.Entitlements.PlanKey != billingpkg.PlanTeam {
		t.Fatalf("plan = %q, want team", overview.Entitlements.PlanKey)
	}
	if overview.Entitlements.Status != billingpkg.EntitlementStatusTrialing {
		t.Fatalf("status = %q, want trialing", overview.Entitlements.Status)
	}
	if overview.Entitlements.ExpiresAt == nil || !overview.Entitlements.ExpiresAt.Equal(now.Add(45*24*time.Hour)) {
		t.Fatalf("expires_at = %v, want %v", overview.Entitlements.ExpiresAt, now.Add(45*24*time.Hour))
	}
	assertIntPtr(t, "team trial model limit", overview.Entitlements.MaxModelsPerRace, 12)
}

func TestBillingManagerStartTrialRejectsExistingPaidPlan(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	repo.entitlements = billingpkg.MaterializeEntitlements(billingpkg.MustPlan(billingpkg.PlanPro), billingpkg.PeriodMonthly, 5, billingpkg.EntitlementStatusTrialing)
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret: "secret",
	})
	caller := Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			repo.orgID: {OrganizationID: repo.orgID, Role: "org_admin"},
		},
	}

	_, err := manager.StartTrial(context.Background(), caller, repo.orgID, StartBillingTrialInput{
		PlanKey:       billingpkg.PlanTeam,
		BillingPeriod: billingpkg.PeriodMonthly,
	})
	var validationErr validationErrorEnvelope
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "trial_not_available" {
		t.Fatalf("code = %q, want trial_not_available", validationErr.Code)
	}
}

func TestBillingManagerStartTrialRejectsRepeatAfterReturnToFree(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		WebhookSecret: "secret",
	})
	caller := Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			repo.orgID: {OrganizationID: repo.orgID, Role: "org_admin"},
		},
	}

	if _, err := manager.StartTrial(context.Background(), caller, repo.orgID, StartBillingTrialInput{PlanKey: billingpkg.PlanPro}); err != nil {
		t.Fatalf("first StartTrial returned error: %v", err)
	}
	repo.entitlements = billingpkg.DefaultEntitlements()
	_, err := manager.StartTrial(context.Background(), caller, repo.orgID, StartBillingTrialInput{PlanKey: billingpkg.PlanTeam})
	var validationErr validationErrorEnvelope
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Code != "trial_not_available" {
		t.Fatalf("code = %q, want trial_not_available", validationErr.Code)
	}
}

func TestBillingManagerCreateCheckoutUsesDodoAPIWhenConfigured(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	var capturedAuth string
	var capturedPayload map[string]any
	dodoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/checkouts" {
			t.Fatalf("path = %s, want /checkouts", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"session_id":   "cks_test_123",
			"checkout_url": "https://test.checkout.dodopayments.com/session/cks_test_123",
		})
	}))
	defer dodoServer.Close()

	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		DodoAPIKey:     "dodo_test_key",
		DodoAPIBaseURL: dodoServer.URL,
		DodoProductIDs: testDodoProductIDs(),
		WebhookSecret:  "secret",
	})
	caller := Caller{
		UserID:      uuid.New(),
		Email:       "owner@example.com",
		DisplayName: "Owner",
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			repo.orgID: {OrganizationID: repo.orgID, Role: "org_admin"},
		},
	}

	result, err := manager.CreateCheckout(context.Background(), caller, repo.orgID, CreateBillingCheckoutInput{
		PlanKey:       "pro",
		BillingPeriod: "monthly",
		SeatQuantity:  5,
		ReturnURL:     "http://localhost:3000/billing/return",
	})
	if err != nil {
		t.Fatalf("CreateCheckout returned error: %v", err)
	}
	if capturedAuth != "Bearer dodo_test_key" {
		t.Fatalf("authorization = %q, want bearer token", capturedAuth)
	}
	if result.CheckoutURL != "https://test.checkout.dodopayments.com/session/cks_test_123" {
		t.Fatalf("checkout URL = %q, want Dodo response URL", result.CheckoutURL)
	}
	if repo.checkoutInput.DodoCheckoutID != "cks_test_123" {
		t.Fatalf("stored Dodo checkout id = %q, want cks_test_123", repo.checkoutInput.DodoCheckoutID)
	}
	if repo.checkoutInput.ID != result.CheckoutIntentID {
		t.Fatalf("checkout intent id = %s, stored %s", result.CheckoutIntentID, repo.checkoutInput.ID)
	}
	productCart, ok := capturedPayload["product_cart"].([]any)
	if !ok || len(productCart) != 1 {
		t.Fatalf("product_cart = %#v, want one item", capturedPayload["product_cart"])
	}
	item, ok := productCart[0].(map[string]any)
	if !ok {
		t.Fatalf("product cart item = %#v, want object", productCart[0])
	}
	if item["product_id"] != "agentclash_pro_monthly" || item["quantity"] != float64(1) {
		t.Fatalf("product cart item = %#v, want pro monthly quantity 1", item)
	}
	metadata, ok := capturedPayload["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata = %#v, want object", capturedPayload["metadata"])
	}
	if metadata["organization_id"] != repo.orgID.String() || metadata["checkout_intent_id"] != result.CheckoutIntentID.String() || metadata["plan_key"] != "pro" {
		t.Fatalf("metadata = %#v, want org/intent/plan", metadata)
	}
	returnURL, ok := capturedPayload["return_url"].(string)
	if !ok || !strings.Contains(returnURL, "checkout=pending") || !strings.Contains(returnURL, "checkout_intent_id="+result.CheckoutIntentID.String()) {
		t.Fatalf("return_url = %#v, want checkout pending params", capturedPayload["return_url"])
	}
}

func TestBillingManagerCreateCheckoutIncludesDodoErrorBody(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	dodoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkouts" {
			t.Fatalf("path = %s, want /checkouts", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(" \n {\"code\":\"INVALID_REQUEST_BODY\",\"message\":\"product_cart.quantity is invalid\"}\n "))
	}))
	defer dodoServer.Close()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		DodoAPIKey:     "dodo_test_key",
		DodoAPIBaseURL: dodoServer.URL,
		DodoProductIDs: testDodoProductIDs(),
		WebhookSecret:  "secret",
	})
	caller := Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			repo.orgID: {OrganizationID: repo.orgID, Role: "org_admin"},
		},
	}

	_, err := manager.CreateCheckout(context.Background(), caller, repo.orgID, CreateBillingCheckoutInput{
		PlanKey:       "pro",
		BillingPeriod: "monthly",
		SeatQuantity:  5,
		ReturnURL:     "http://localhost:3000/billing/return",
	})
	if err == nil {
		t.Fatal("expected Dodo checkout error")
	}
	errText := err.Error()
	for _, want := range []string{"HTTP 422", "INVALID_REQUEST_BODY", "product_cart.quantity is invalid"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("error = %q, want substring %q", errText, want)
		}
	}
}

func TestBillingManagerCreateCheckoutOmitsEmptyDodoErrorBody(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	dodoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkouts" {
			t.Fatalf("path = %s, want /checkouts", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer dodoServer.Close()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		DodoAPIKey:     "dodo_test_key",
		DodoAPIBaseURL: dodoServer.URL,
		DodoProductIDs: testDodoProductIDs(),
		WebhookSecret:  "secret",
	})
	caller := Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			repo.orgID: {OrganizationID: repo.orgID, Role: "org_admin"},
		},
	}

	_, err := manager.CreateCheckout(context.Background(), caller, repo.orgID, CreateBillingCheckoutInput{
		PlanKey:       "pro",
		BillingPeriod: "monthly",
		SeatQuantity:  5,
		ReturnURL:     "http://localhost:3000/billing/return",
	})
	if err == nil {
		t.Fatal("expected Dodo checkout error")
	}
	if err.Error() != "create dodo checkout session: dodo returned HTTP 422" {
		t.Fatalf("error = %q, want status-only Dodo error", err.Error())
	}
}

func TestCreateBillingCheckoutHandlerKeepsDodoErrorGeneric(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	dodoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"INVALID_REQUEST_BODY","message":"product_cart.quantity is invalid"}`))
	}))
	defer dodoServer.Close()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		DodoAPIKey:     "dodo_test_key",
		DodoAPIBaseURL: dodoServer.URL,
		DodoProductIDs: testDodoProductIDs(),
		WebhookSecret:  "secret",
	})
	caller := Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			repo.orgID: {OrganizationID: repo.orgID, Role: "org_admin"},
		},
	}

	var logs bytes.Buffer
	router := chi.NewRouter()
	router.Post("/v1/organizations/{organizationID}/billing/checkout", createBillingCheckoutHandler(slog.New(slog.NewTextHandler(&logs, nil)), manager))
	req := httptest.NewRequest(http.MethodPost, "/v1/organizations/"+repo.orgID.String()+"/billing/checkout", bytes.NewBufferString(`{
		"plan_key":"pro",
		"billing_period":"monthly",
		"seat_quantity":5,
		"return_url":"http://localhost:3000/billing/return"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), callerContextKey{}, caller))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "INVALID_REQUEST_BODY") || strings.Contains(rr.Body.String(), "product_cart.quantity") {
		t.Fatalf("response leaked Dodo body: %s", rr.Body.String())
	}
	if !strings.Contains(logs.String(), "INVALID_REQUEST_BODY") || !strings.Contains(logs.String(), "product_cart.quantity is invalid") {
		t.Fatalf("logs = %q, want Dodo error details", logs.String())
	}
}

func TestBillingManagerCreatePortalUsesDodoCustomerPortalSession(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	if err := repo.UpsertBillingAccount(context.Background(), repo.orgID, "cus_portal_test", "owner@example.com", "active"); err != nil {
		t.Fatalf("seed billing account: %v", err)
	}
	var capturedAuth string
	dodoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/customers/cus_portal_test/customer-portal/session" {
			t.Fatalf("path = %s, want customer portal session path", r.URL.Path)
		}
		if r.URL.Query().Get("send_email") != "false" {
			t.Fatalf("send_email = %q, want false", r.URL.Query().Get("send_email"))
		}
		writeJSON(w, http.StatusOK, map[string]string{"link": "https://customer.dodopayments.com/session/cps_test"})
	}))
	defer dodoServer.Close()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		DodoAPIKey:     "dodo_test_key",
		DodoAPIBaseURL: dodoServer.URL,
		DodoProductIDs: testDodoProductIDs(),
		WebhookSecret:  "secret",
	})
	caller := Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			repo.orgID: {OrganizationID: repo.orgID, Role: "org_admin"},
		},
	}

	result, err := manager.CreatePortal(context.Background(), caller, repo.orgID)
	if err != nil {
		t.Fatalf("CreatePortal returned error: %v", err)
	}
	if capturedAuth != "Bearer dodo_test_key" {
		t.Fatalf("authorization = %q, want bearer token", capturedAuth)
	}
	if result.PortalURL != "https://customer.dodopayments.com/session/cps_test" {
		t.Fatalf("portal URL = %q, want Dodo link", result.PortalURL)
	}
}

func TestBillingManagerCreatePortalIncludesDodoErrorBody(t *testing.T) {
	workspaceID := uuid.New()
	repo := newFakeBillingRepository(workspaceID)
	if err := repo.UpsertBillingAccount(context.Background(), repo.orgID, "cus_portal_test", "owner@example.com", "active"); err != nil {
		t.Fatalf("seed billing account: %v", err)
	}
	dodoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/customers/cus_portal_test/customer-portal/session" {
			t.Fatalf("path = %s, want customer portal session path", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"INVALID_REQUEST_BODY","message":"customer portal unavailable"}`))
	}))
	defer dodoServer.Close()
	manager := NewBillingManager(NewCallerOrganizationAuthorizer(), NewCallerWorkspaceAuthorizer(), repo, BillingManagerConfig{
		DodoAPIKey:     "dodo_test_key",
		DodoAPIBaseURL: dodoServer.URL,
		DodoProductIDs: testDodoProductIDs(),
		WebhookSecret:  "secret",
	})
	caller := Caller{
		UserID: uuid.New(),
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{
			repo.orgID: {OrganizationID: repo.orgID, Role: "org_admin"},
		},
	}

	_, err := manager.CreatePortal(context.Background(), caller, repo.orgID)
	if err == nil {
		t.Fatal("expected Dodo portal error")
	}
	errText := err.Error()
	for _, want := range []string{"HTTP 422", "INVALID_REQUEST_BODY", "customer portal unavailable"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("error = %q, want substring %q", errText, want)
		}
	}
}

func signedDodoHeaders(secret string, id string, timestamp string, body string) DodoWebhookHeaders {
	decoded, err := decodeDodoWebhookSecret(secret)
	if err != nil {
		panic(err)
	}
	mac := hmac.New(sha256.New, decoded)
	_, _ = mac.Write([]byte(id + "." + timestamp + "." + body))
	return DodoWebhookHeaders{
		WebhookID:        id,
		WebhookTimestamp: timestamp,
		WebhookSignature: "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	}
}

func dodoTestWebhookSecret() string {
	return "whsec_" + base64.StdEncoding.EncodeToString([]byte("agentclash-test-webhook-secret"))
}

func testDodoProductIDs() billingpkg.DodoProductIDs {
	return billingpkg.DodoProductIDs{
		ProMonthly:  "agentclash_pro_monthly",
		ProYearly:   "agentclash_pro_yearly",
		TeamMonthly: "agentclash_team_monthly",
		TeamYearly:  "agentclash_team_yearly",
	}
}

type fakeBillingRepository struct {
	orgID         uuid.UUID
	workspaceID   uuid.UUID
	entitlements  billingpkg.EffectiveEntitlements
	usage         repository.WorkspaceUsageSnapshot
	webhookIDs    map[string]bool
	subscription  repository.BillingSubscription
	account       repository.BillingAccount
	checkoutInput repository.BillingCheckoutIntentInput
	trialUsed     bool
}

func newFakeBillingRepository(workspaceID uuid.UUID) *fakeBillingRepository {
	windowStart, windowEnd := usageWindow(time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC))
	return &fakeBillingRepository{
		orgID:        uuid.New(),
		workspaceID:  workspaceID,
		entitlements: billingpkg.DefaultEntitlements(),
		usage: repository.WorkspaceUsageSnapshot{
			WorkspaceID: workspaceID,
			WindowStart: windowStart,
			WindowEnd:   windowEnd,
		},
		webhookIDs: map[string]bool{},
	}
}

func (f *fakeBillingRepository) ResolveWorkspaceEntitlements(_ context.Context, workspaceID uuid.UUID) (uuid.UUID, billingpkg.EffectiveEntitlements, error) {
	if workspaceID != f.workspaceID {
		return uuid.Nil, billingpkg.EffectiveEntitlements{}, repository.ErrWorkspaceNotFound
	}
	return f.orgID, f.entitlements, nil
}

func (f *fakeBillingRepository) GetOrganizationEntitlements(_ context.Context, orgID uuid.UUID) (billingpkg.EffectiveEntitlements, error) {
	if orgID != f.orgID {
		return billingpkg.EffectiveEntitlements{}, pgx.ErrNoRows
	}
	return f.entitlements, nil
}

func (f *fakeBillingRepository) UpsertOrganizationEntitlements(_ context.Context, orgID uuid.UUID, entitlements billingpkg.EffectiveEntitlements, _ *uuid.UUID, expiresAt *time.Time) error {
	if orgID != f.orgID {
		return pgx.ErrNoRows
	}
	if expiresAt != nil {
		entitlements.ExpiresAt = expiresAt
	}
	f.entitlements = entitlements
	return nil
}

func (f *fakeBillingRepository) CountActiveOrgMembers(context.Context, uuid.UUID) (int, error) {
	return 1, nil
}

func (f *fakeBillingRepository) CountActiveWorkspaces(context.Context, uuid.UUID) (int, error) {
	return 1, nil
}

func (f *fakeBillingRepository) CountActiveWorkspaceRuns(context.Context, uuid.UUID) (int, error) {
	return f.usage.ActiveRuns, nil
}

func (f *fakeBillingRepository) GetWorkspaceUsageSnapshot(_ context.Context, _ uuid.UUID, windowStart, windowEnd time.Time) (repository.WorkspaceUsageSnapshot, error) {
	usage := f.usage
	usage.WindowStart = windowStart
	usage.WindowEnd = windowEnd
	return usage, nil
}

func (f *fakeBillingRepository) CreateBillingCheckoutIntent(_ context.Context, input repository.BillingCheckoutIntentInput) (repository.BillingCheckoutIntent, error) {
	f.checkoutInput = input
	intentID := input.ID
	if intentID == uuid.Nil {
		intentID = uuid.New()
	}
	return repository.BillingCheckoutIntent{
		ID:               intentID,
		OrganizationID:   input.OrganizationID,
		RequestedPlanKey: input.RequestedPlanKey,
		BillingPeriod:    input.BillingPeriod,
		SeatQuantity:     input.SeatQuantity,
		ReturnURL:        input.ReturnURL,
		CheckoutURL:      input.CheckoutURL,
		Metadata:         input.Metadata,
		Status:           "created",
	}, nil
}

func (f *fakeBillingRepository) UpsertBillingAccount(_ context.Context, orgID uuid.UUID, dodoCustomerID string, billingEmail string, status string) error {
	if orgID != f.orgID {
		return pgx.ErrNoRows
	}
	if status == "" {
		status = "active"
	}
	f.account = repository.BillingAccount{
		ID:             uuid.New(),
		OrganizationID: orgID,
		DodoCustomerID: optionalString(dodoCustomerID),
		BillingEmail:   optionalString(billingEmail),
		Status:         status,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	return nil
}

func (f *fakeBillingRepository) GetBillingAccount(_ context.Context, orgID uuid.UUID) (repository.BillingAccount, error) {
	if orgID != f.orgID || f.account.ID == uuid.Nil {
		return repository.BillingAccount{}, pgx.ErrNoRows
	}
	return f.account, nil
}

func (f *fakeBillingRepository) UpsertBillingSubscription(_ context.Context, input repository.BillingSubscriptionInput) (repository.BillingSubscription, error) {
	addons := input.AddonQuantities
	if len(addons) == 0 {
		addons = json.RawMessage(`{}`)
	}
	f.subscription = repository.BillingSubscription{
		ID:                      uuid.New(),
		OrganizationID:          input.OrganizationID,
		DodoSubscriptionID:      input.DodoSubscriptionID,
		DodoProductID:           input.DodoProductID,
		PlanKey:                 input.PlanKey,
		BillingPeriod:           input.BillingPeriod,
		Status:                  input.Status,
		SeatQuantity:            input.SeatQuantity,
		AddonQuantities:         addons,
		CancelAtNextBillingDate: input.CancelAtNextBillingDate,
		ExpiresAt:               input.ExpiresAt,
		LatestDodoEventAt:       input.LatestDodoEventAt,
	}
	return f.subscription, nil
}

func (f *fakeBillingRepository) GetBillingOverview(_ context.Context, orgID uuid.UUID) (repository.BillingOverview, error) {
	if orgID != f.orgID {
		return repository.BillingOverview{}, pgx.ErrNoRows
	}
	overview := repository.BillingOverview{Entitlements: f.entitlements}
	if f.account.ID != uuid.Nil {
		overview.Account = &f.account
	}
	if f.subscription.ID != uuid.Nil {
		overview.Subscription = &f.subscription
	}
	return overview, nil
}

func (f *fakeBillingRepository) FindOrganizationByDodoSubscriptionOrCustomer(context.Context, string, string) (uuid.UUID, error) {
	return f.orgID, nil
}

func (f *fakeBillingRepository) ApplyBillingWebhookEvent(ctx context.Context, event repository.BillingWebhookEventInput, application repository.BillingWebhookApplication) (bool, error) {
	if f.webhookIDs[event.WebhookID] {
		return true, nil
	}
	f.webhookIDs[event.WebhookID] = true
	if application.Account != nil {
		if err := f.UpsertBillingAccount(ctx, application.Account.OrganizationID, application.Account.DodoCustomerID, application.Account.BillingEmail, application.Account.Status); err != nil {
			return false, err
		}
	}
	var sourceSubscriptionID *uuid.UUID
	if application.Subscription != nil {
		subscription, err := f.UpsertBillingSubscription(ctx, *application.Subscription)
		if err != nil {
			return false, err
		}
		sourceSubscriptionID = &subscription.ID
	}
	if application.Entitlements != nil {
		if !application.Entitlements.UseSubscriptionAsSource {
			sourceSubscriptionID = nil
		}
		if err := f.UpsertOrganizationEntitlements(ctx, application.Entitlements.OrganizationID, application.Entitlements.Entitlements, sourceSubscriptionID, application.Entitlements.ExpiresAt); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (f *fakeBillingRepository) CreateBillingTrialGrant(_ context.Context, input repository.BillingTrialGrantInput) (repository.BillingTrialGrant, error) {
	if input.OrganizationID != f.orgID {
		return repository.BillingTrialGrant{}, pgx.ErrNoRows
	}
	if f.trialUsed {
		return repository.BillingTrialGrant{}, repository.ErrBillingTrialAlreadyUsed
	}
	f.trialUsed = true
	return repository.BillingTrialGrant{
		ID:              uuid.New(),
		OrganizationID:  input.OrganizationID,
		PlanKey:         input.PlanKey,
		BillingPeriod:   input.BillingPeriod,
		StartedByUserID: &input.StartedByUserID,
		StartedAt:       input.StartedAt,
		ExpiresAt:       input.ExpiresAt,
		CreatedAt:       input.StartedAt,
		UpdatedAt:       input.StartedAt,
	}, nil
}

func assertIntPtr(t *testing.T, label string, got *int, want int) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %d", label, want)
	}
	if *got != want {
		t.Fatalf("%s = %d, want %d", label, *got, want)
	}
}
