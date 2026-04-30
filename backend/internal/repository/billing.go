package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrBillingWebhookAlreadyProcessed = errors.New("billing webhook already processed")

type repositoryExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type BillingCheckoutIntentInput struct {
	OrganizationID   uuid.UUID
	CreatedByUserID  uuid.UUID
	RequestedPlanKey string
	BillingPeriod    string
	SeatQuantity     int
	ReturnURL        string
	CheckoutURL      string
	DodoCheckoutID   string
	Metadata         json.RawMessage
}

type BillingCheckoutIntent struct {
	ID               uuid.UUID       `json:"id"`
	OrganizationID   uuid.UUID       `json:"organization_id"`
	RequestedPlanKey string          `json:"requested_plan_key"`
	BillingPeriod    string          `json:"billing_period"`
	SeatQuantity     int             `json:"seat_quantity"`
	ReturnURL        string          `json:"return_url"`
	CheckoutURL      string          `json:"checkout_url"`
	DodoCheckoutID   *string         `json:"dodo_checkout_session_id,omitempty"`
	Status           string          `json:"status"`
	Metadata         json.RawMessage `json:"metadata"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type BillingSubscriptionInput struct {
	OrganizationID          uuid.UUID
	DodoSubscriptionID      string
	DodoCustomerID          string
	DodoProductID           string
	PlanKey                 string
	BillingPeriod           string
	Status                  string
	NextBillingDate         *time.Time
	CancelAtNextBillingDate bool
	CancelledAt             *time.Time
	ExpiresAt               *time.Time
	TrialPeriodDays         *int
	SeatQuantity            int
	AddonQuantities         json.RawMessage
	LatestDodoEventAt       *time.Time
}

type BillingSubscription struct {
	ID                      uuid.UUID       `json:"id"`
	OrganizationID          uuid.UUID       `json:"organization_id"`
	DodoSubscriptionID      string          `json:"dodo_subscription_id"`
	DodoCustomerID          *string         `json:"dodo_customer_id,omitempty"`
	DodoProductID           string          `json:"dodo_product_id"`
	PlanKey                 string          `json:"plan_key"`
	BillingPeriod           string          `json:"billing_period"`
	Status                  string          `json:"status"`
	NextBillingDate         *time.Time      `json:"next_billing_date,omitempty"`
	CancelAtNextBillingDate bool            `json:"cancel_at_next_billing_date"`
	CancelledAt             *time.Time      `json:"cancelled_at,omitempty"`
	ExpiresAt               *time.Time      `json:"expires_at,omitempty"`
	TrialPeriodDays         *int            `json:"trial_period_days,omitempty"`
	SeatQuantity            int             `json:"seat_quantity"`
	AddonQuantities         json.RawMessage `json:"addon_quantities"`
	LatestDodoEventAt       *time.Time      `json:"latest_dodo_event_at,omitempty"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
}

type BillingWebhookEventInput struct {
	WebhookID      string
	EventType      string
	DodoBusinessID *string
	PayloadType    *string
	EventTimestamp *time.Time
	Payload        json.RawMessage
	Status         string
	Error          *string
}

type BillingAccountInput struct {
	OrganizationID uuid.UUID
	DodoCustomerID string
	BillingEmail   string
	Status         string
}

type BillingWebhookEntitlementsInput struct {
	OrganizationID          uuid.UUID
	Entitlements            billing.EffectiveEntitlements
	UseSubscriptionAsSource bool
	ExpiresAt               *time.Time
}

type BillingWebhookApplication struct {
	Account      *BillingAccountInput
	Subscription *BillingSubscriptionInput
	Entitlements *BillingWebhookEntitlementsInput
}

type BillingOverview struct {
	Entitlements billing.EffectiveEntitlements `json:"entitlements"`
	Subscription *BillingSubscription          `json:"subscription,omitempty"`
}

type WorkspaceUsageSnapshot struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	RaceCount   int       `json:"race_count"`
	ActiveRuns  int       `json:"active_runs"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
}

type RunEntitlementGate struct {
	Entitlements    billing.EffectiveEntitlements
	RaceCost        int
	ConcurrencyCost int
	WindowStart     time.Time
	WindowEnd       time.Time
}

type OrganizationEntitlementGate struct {
	OrganizationID uuid.UUID
	Entitlements   billing.EffectiveEntitlements
}

func (r *Repository) ResolveWorkspaceEntitlements(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, billing.EffectiveEntitlements, error) {
	var orgID uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT organization_id FROM workspaces WHERE id = $1 AND status = 'active'`, workspaceID).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, billing.EffectiveEntitlements{}, ErrWorkspaceNotFound
		}
		return uuid.Nil, billing.EffectiveEntitlements{}, fmt.Errorf("resolve workspace organization: %w", err)
	}

	entitlements, err := r.GetOrganizationEntitlements(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := r.ensureDefaultOrganizationEntitlements(ctx, orgID); err != nil {
			return uuid.Nil, billing.EffectiveEntitlements{}, err
		}
		entitlements, err = r.GetOrganizationEntitlements(ctx, orgID)
	}
	if err != nil {
		return uuid.Nil, billing.EffectiveEntitlements{}, err
	}
	return orgID, entitlements, nil
}

func (r *Repository) GetOrganizationEntitlements(ctx context.Context, orgID uuid.UUID) (billing.EffectiveEntitlements, error) {
	var entitlements billing.EffectiveEntitlements
	var featureFlags []byte
	err := r.db.QueryRow(ctx, `
		SELECT
			plan_key,
			billing_period,
			status,
			seat_quantity,
			seats_limit,
			workspaces_limit,
			races_per_workspace_month,
			max_models_per_race,
			replay_retention_days,
			concurrency_limit,
			feature_flags,
			expires_at
		FROM organization_entitlements
		WHERE organization_id = $1
	`, orgID).Scan(
		&entitlements.PlanKey,
		&entitlements.BillingPeriod,
		&entitlements.Status,
		&entitlements.SeatQuantity,
		&entitlements.SeatsLimit,
		&entitlements.WorkspacesLimit,
		&entitlements.RacesPerWorkspaceMonth,
		&entitlements.MaxModelsPerRace,
		&entitlements.ReplayRetentionDays,
		&entitlements.ConcurrentRaces,
		&featureFlags,
		&entitlements.ExpiresAt,
	)
	if err != nil {
		return billing.EffectiveEntitlements{}, err
	}
	if len(featureFlags) > 0 {
		if err := json.Unmarshal(featureFlags, &entitlements.FeatureFlags); err != nil {
			return billing.EffectiveEntitlements{}, fmt.Errorf("decode feature flags: %w", err)
		}
	}
	if entitlements.FeatureFlags == nil {
		entitlements.FeatureFlags = map[string]bool{}
	}
	if plan, ok := billing.PlanByKey(entitlements.PlanKey); ok {
		entitlements.UpgradeTarget = plan.UpgradeTarget
	}
	return entitlements.WithComputedStatus(time.Now().UTC()), nil
}

func (r *Repository) UpsertOrganizationEntitlements(ctx context.Context, orgID uuid.UUID, entitlements billing.EffectiveEntitlements, sourceSubscriptionID *uuid.UUID, expiresAt *time.Time) error {
	return upsertOrganizationEntitlements(ctx, r.db, orgID, entitlements, sourceSubscriptionID, expiresAt)
}

func upsertOrganizationEntitlements(ctx context.Context, exec repositoryExecutor, orgID uuid.UUID, entitlements billing.EffectiveEntitlements, sourceSubscriptionID *uuid.UUID, expiresAt *time.Time) error {
	if expiresAt == nil && entitlements.ExpiresAt != nil {
		expiresAt = entitlements.ExpiresAt
	}
	featureFlags, err := json.Marshal(entitlements.FeatureFlags)
	if err != nil {
		return fmt.Errorf("marshal feature flags: %w", err)
	}
	_, err = exec.Exec(ctx, `
		INSERT INTO organization_entitlements (
			organization_id,
			plan_key,
			billing_period,
			status,
			seat_quantity,
			seats_limit,
			workspaces_limit,
			races_per_workspace_month,
			max_models_per_race,
			replay_retention_days,
			concurrency_limit,
			feature_flags,
			source_subscription_id,
			effective_at,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13, now(), $14)
		ON CONFLICT (organization_id) DO UPDATE SET
			plan_key = EXCLUDED.plan_key,
			billing_period = EXCLUDED.billing_period,
			status = EXCLUDED.status,
			seat_quantity = EXCLUDED.seat_quantity,
			seats_limit = EXCLUDED.seats_limit,
			workspaces_limit = EXCLUDED.workspaces_limit,
			races_per_workspace_month = EXCLUDED.races_per_workspace_month,
			max_models_per_race = EXCLUDED.max_models_per_race,
			replay_retention_days = EXCLUDED.replay_retention_days,
			concurrency_limit = EXCLUDED.concurrency_limit,
			feature_flags = EXCLUDED.feature_flags,
			source_subscription_id = EXCLUDED.source_subscription_id,
			effective_at = EXCLUDED.effective_at,
			expires_at = EXCLUDED.expires_at
	`, orgID, entitlements.PlanKey, entitlements.BillingPeriod, entitlements.Status, entitlements.SeatQuantity, entitlements.SeatsLimit, entitlements.WorkspacesLimit, entitlements.RacesPerWorkspaceMonth, entitlements.MaxModelsPerRace, entitlements.ReplayRetentionDays, entitlements.ConcurrentRaces, featureFlags, sourceSubscriptionID, expiresAt)
	if err != nil {
		return fmt.Errorf("upsert organization entitlements: %w", err)
	}
	return nil
}

func (r *Repository) ensureDefaultOrganizationEntitlements(ctx context.Context, orgID uuid.UUID) error {
	defaultEntitlements := billing.DefaultEntitlements()
	return r.UpsertOrganizationEntitlements(ctx, orgID, defaultEntitlements, nil, nil)
}

func (r *Repository) CountActiveOrgMembers(ctx context.Context, orgID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM organization_memberships
		WHERE organization_id = $1
		  AND membership_status = 'active'
	`, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active org members: %w", err)
	}
	return count, nil
}

func (r *Repository) CountActiveWorkspaces(ctx context.Context, orgID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM workspaces
		WHERE organization_id = $1
		  AND status = 'active'
	`, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active workspaces: %w", err)
	}
	return count, nil
}

func (r *Repository) CountActiveWorkspaceRuns(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, activeWorkspaceRunsSQL, workspaceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active workspace runs: %w", err)
	}
	return count, nil
}

func (r *Repository) GetWorkspaceUsageSnapshot(ctx context.Context, workspaceID uuid.UUID, windowStart, windowEnd time.Time) (WorkspaceUsageSnapshot, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT race_count
		FROM workspace_usage_windows
		WHERE workspace_id = $1
		  AND window_start = $2
	`, workspaceID, windowStart).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		count = 0
	} else if err != nil {
		return WorkspaceUsageSnapshot{}, fmt.Errorf("get workspace usage window: %w", err)
	}
	activeRuns, err := r.CountActiveWorkspaceRuns(ctx, workspaceID)
	if err != nil {
		return WorkspaceUsageSnapshot{}, err
	}
	return WorkspaceUsageSnapshot{
		WorkspaceID: workspaceID,
		RaceCount:   count,
		ActiveRuns:  activeRuns,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	}, nil
}

func (r *Repository) CreateBillingCheckoutIntent(ctx context.Context, input BillingCheckoutIntentInput) (BillingCheckoutIntent, error) {
	metadata := normalizeJSON(input.Metadata)
	var intent BillingCheckoutIntent
	err := r.db.QueryRow(ctx, `
		INSERT INTO billing_checkout_intents (
			organization_id,
			created_by_user_id,
			requested_plan_key,
			billing_period,
			seat_quantity,
			return_url,
			dodo_checkout_session_id,
			checkout_url,
			status,
			metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, 'created', $9::jsonb)
		RETURNING id, organization_id, requested_plan_key, billing_period, seat_quantity, return_url, checkout_url, dodo_checkout_session_id, status, metadata, created_at, updated_at
	`, input.OrganizationID, input.CreatedByUserID, input.RequestedPlanKey, input.BillingPeriod, input.SeatQuantity, input.ReturnURL, input.DodoCheckoutID, input.CheckoutURL, metadata).Scan(
		&intent.ID,
		&intent.OrganizationID,
		&intent.RequestedPlanKey,
		&intent.BillingPeriod,
		&intent.SeatQuantity,
		&intent.ReturnURL,
		&intent.CheckoutURL,
		&intent.DodoCheckoutID,
		&intent.Status,
		&intent.Metadata,
		&intent.CreatedAt,
		&intent.UpdatedAt,
	)
	if err != nil {
		return BillingCheckoutIntent{}, fmt.Errorf("create billing checkout intent: %w", err)
	}
	return intent, nil
}

func (r *Repository) UpsertBillingAccount(ctx context.Context, orgID uuid.UUID, dodoCustomerID string, billingEmail string, status string) error {
	return upsertBillingAccount(ctx, r.db, BillingAccountInput{
		OrganizationID: orgID,
		DodoCustomerID: dodoCustomerID,
		BillingEmail:   billingEmail,
		Status:         status,
	})
}

func upsertBillingAccount(ctx context.Context, exec repositoryExecutor, input BillingAccountInput) error {
	status := input.Status
	if status == "" {
		status = "active"
	}
	_, err := exec.Exec(ctx, `
		INSERT INTO billing_accounts (organization_id, dodo_customer_id, billing_email, status)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4)
		ON CONFLICT (organization_id) DO UPDATE SET
			dodo_customer_id = COALESCE(EXCLUDED.dodo_customer_id, billing_accounts.dodo_customer_id),
			billing_email = COALESCE(EXCLUDED.billing_email, billing_accounts.billing_email),
			status = EXCLUDED.status
	`, input.OrganizationID, input.DodoCustomerID, input.BillingEmail, status)
	if err != nil {
		return fmt.Errorf("upsert billing account: %w", err)
	}
	return nil
}

func (r *Repository) UpsertBillingSubscription(ctx context.Context, input BillingSubscriptionInput) (BillingSubscription, error) {
	subscription, _, err := upsertBillingSubscription(ctx, r.db, input)
	return subscription, err
}

func upsertBillingSubscription(ctx context.Context, exec repositoryExecutor, input BillingSubscriptionInput) (BillingSubscription, bool, error) {
	addons := normalizeJSON(input.AddonQuantities)
	subscription, err := scanBillingSubscription(exec.QueryRow(ctx, `
		INSERT INTO billing_subscriptions (
			organization_id,
			dodo_subscription_id,
			dodo_customer_id,
			dodo_product_id,
			plan_key,
			billing_period,
			status,
			next_billing_date,
			cancel_at_next_billing_date,
			cancelled_at,
			expires_at,
			trial_period_days,
			seat_quantity,
			addon_quantities,
			latest_dodo_event_at
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15)
		ON CONFLICT (dodo_subscription_id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			dodo_customer_id = COALESCE(EXCLUDED.dodo_customer_id, billing_subscriptions.dodo_customer_id),
			dodo_product_id = EXCLUDED.dodo_product_id,
			plan_key = EXCLUDED.plan_key,
			billing_period = EXCLUDED.billing_period,
			status = EXCLUDED.status,
			next_billing_date = EXCLUDED.next_billing_date,
			cancel_at_next_billing_date = EXCLUDED.cancel_at_next_billing_date,
			cancelled_at = EXCLUDED.cancelled_at,
			expires_at = EXCLUDED.expires_at,
			trial_period_days = EXCLUDED.trial_period_days,
			seat_quantity = EXCLUDED.seat_quantity,
			addon_quantities = EXCLUDED.addon_quantities,
			latest_dodo_event_at = COALESCE(
				GREATEST(billing_subscriptions.latest_dodo_event_at, EXCLUDED.latest_dodo_event_at),
				billing_subscriptions.latest_dodo_event_at,
				EXCLUDED.latest_dodo_event_at
			)
		WHERE billing_subscriptions.latest_dodo_event_at IS NULL
		   OR EXCLUDED.latest_dodo_event_at IS NULL
		   OR EXCLUDED.latest_dodo_event_at >= billing_subscriptions.latest_dodo_event_at
		RETURNING id, organization_id, dodo_subscription_id, dodo_customer_id, dodo_product_id, plan_key, billing_period, status, next_billing_date, cancel_at_next_billing_date, cancelled_at, expires_at, trial_period_days, seat_quantity, addon_quantities, latest_dodo_event_at, created_at, updated_at
	`, input.OrganizationID, input.DodoSubscriptionID, input.DodoCustomerID, input.DodoProductID, input.PlanKey, input.BillingPeriod, input.Status, input.NextBillingDate, input.CancelAtNextBillingDate, input.CancelledAt, input.ExpiresAt, input.TrialPeriodDays, input.SeatQuantity, addons, input.LatestDodoEventAt))
	if errors.Is(err, pgx.ErrNoRows) {
		subscription, selectErr := getBillingSubscriptionByDodoID(ctx, exec, input.DodoSubscriptionID)
		if selectErr != nil {
			return BillingSubscription{}, false, selectErr
		}
		return subscription, false, nil
	}
	if err != nil {
		return BillingSubscription{}, false, fmt.Errorf("upsert billing subscription: %w", err)
	}
	return subscription, true, nil
}

func (r *Repository) GetBillingOverview(ctx context.Context, orgID uuid.UUID) (BillingOverview, error) {
	entitlements, err := r.GetOrganizationEntitlements(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := r.ensureDefaultOrganizationEntitlements(ctx, orgID); err != nil {
			return BillingOverview{}, err
		}
		entitlements, err = r.GetOrganizationEntitlements(ctx, orgID)
	}
	if err != nil {
		return BillingOverview{}, err
	}

	subscription, err := r.getLatestBillingSubscription(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return BillingOverview{Entitlements: entitlements}, nil
	}
	if err != nil {
		return BillingOverview{}, err
	}
	return BillingOverview{Entitlements: entitlements, Subscription: &subscription}, nil
}

func (r *Repository) FindOrganizationByDodoSubscriptionOrCustomer(ctx context.Context, subscriptionID string, customerID string) (uuid.UUID, error) {
	var orgID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT organization_id FROM (
			SELECT organization_id, 1 AS priority, latest_dodo_event_at AS event_at
			FROM billing_subscriptions
			WHERE dodo_subscription_id = $1
			UNION ALL
			SELECT organization_id, 2 AS priority, updated_at AS event_at
			FROM billing_accounts
			WHERE dodo_customer_id = NULLIF($2, '')
		) candidates
		ORDER BY priority, event_at DESC NULLS LAST
		LIMIT 1
	`, subscriptionID, customerID).Scan(&orgID)
	if err != nil {
		return uuid.Nil, err
	}
	return orgID, nil
}

func (r *Repository) ApplyBillingWebhookEvent(ctx context.Context, event BillingWebhookEventInput, application BillingWebhookApplication) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin billing webhook transaction: %w", err)
	}
	defer rollback(ctx, tx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 435))`, event.WebhookID); err != nil {
		return false, fmt.Errorf("lock billing webhook event: %w", err)
	}

	duplicate, err := beginBillingWebhookEvent(ctx, tx, event)
	if err != nil || duplicate {
		return duplicate, err
	}

	if err := applyBillingWebhookApplication(ctx, tx, application); err != nil {
		return false, err
	}

	if err := finishBillingWebhookEvent(ctx, tx, event.WebhookID, "processed", ""); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit billing webhook transaction: %w", err)
	}
	return false, nil
}

func beginBillingWebhookEvent(ctx context.Context, exec repositoryExecutor, input BillingWebhookEventInput) (bool, error) {
	payload := normalizeJSON(input.Payload)
	hash := sha256.Sum256(payload)
	var status string
	err := exec.QueryRow(ctx, `
		INSERT INTO billing_webhook_events (
			webhook_id,
			event_type,
			dodo_business_id,
			payload_type,
			event_timestamp,
			processed_at,
			payload_hash,
			status,
			error,
			payload
		)
		VALUES ($1, $2, $3, $4, $5, NULL, $6, 'failed', NULL, $7::jsonb)
		ON CONFLICT (webhook_id) DO UPDATE SET
			event_type = EXCLUDED.event_type,
			dodo_business_id = EXCLUDED.dodo_business_id,
			payload_type = EXCLUDED.payload_type,
			event_timestamp = EXCLUDED.event_timestamp,
			processed_at = NULL,
			payload_hash = EXCLUDED.payload_hash,
			status = 'failed',
			error = NULL,
			payload = EXCLUDED.payload
		WHERE billing_webhook_events.status = 'failed'
		RETURNING status
	`, input.WebhookID, input.EventType, input.DodoBusinessID, input.PayloadType, input.EventTimestamp, hex.EncodeToString(hash[:]), payload).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("begin billing webhook event: %w", err)
	}
	return false, nil
}

func finishBillingWebhookEvent(ctx context.Context, exec repositoryExecutor, webhookID string, status string, message string) error {
	var errorMessage *string
	if message != "" {
		errorMessage = &message
	}
	tag, err := exec.Exec(ctx, `
		UPDATE billing_webhook_events
		SET status = $2,
		    error = $3,
		    processed_at = now()
		WHERE webhook_id = $1
	`, webhookID, status, errorMessage)
	if err != nil {
		return fmt.Errorf("finish billing webhook event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBillingWebhookAlreadyProcessed
	}
	return nil
}

func applyBillingWebhookApplication(ctx context.Context, exec repositoryExecutor, application BillingWebhookApplication) error {
	var subscriptionID *uuid.UUID
	if application.Subscription != nil {
		subscription, applied, err := upsertBillingSubscription(ctx, exec, *application.Subscription)
		if err != nil {
			return err
		}
		if !applied {
			return nil
		}
		subscriptionID = &subscription.ID
	}
	if application.Account != nil {
		if err := upsertBillingAccount(ctx, exec, *application.Account); err != nil {
			return err
		}
	}
	if application.Entitlements != nil {
		sourceSubscriptionID := (*uuid.UUID)(nil)
		if application.Entitlements.UseSubscriptionAsSource {
			sourceSubscriptionID = subscriptionID
		}
		if err := upsertOrganizationEntitlements(ctx, exec, application.Entitlements.OrganizationID, application.Entitlements.Entitlements, sourceSubscriptionID, application.Entitlements.ExpiresAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) getLatestBillingSubscription(ctx context.Context, orgID uuid.UUID) (BillingSubscription, error) {
	return scanBillingSubscription(r.db.QueryRow(ctx, `
		SELECT id, organization_id, dodo_subscription_id, dodo_customer_id, dodo_product_id, plan_key, billing_period, status, next_billing_date, cancel_at_next_billing_date, cancelled_at, expires_at, trial_period_days, seat_quantity, addon_quantities, latest_dodo_event_at, created_at, updated_at
		FROM billing_subscriptions
		WHERE organization_id = $1
		ORDER BY latest_dodo_event_at DESC NULLS LAST, updated_at DESC
		LIMIT 1
	`, orgID))
}

func getBillingSubscriptionByDodoID(ctx context.Context, exec repositoryExecutor, subscriptionID string) (BillingSubscription, error) {
	return scanBillingSubscription(exec.QueryRow(ctx, `
		SELECT id, organization_id, dodo_subscription_id, dodo_customer_id, dodo_product_id, plan_key, billing_period, status, next_billing_date, cancel_at_next_billing_date, cancelled_at, expires_at, trial_period_days, seat_quantity, addon_quantities, latest_dodo_event_at, created_at, updated_at
		FROM billing_subscriptions
		WHERE dodo_subscription_id = $1
	`, subscriptionID))
}

func scanBillingSubscription(row pgx.Row) (BillingSubscription, error) {
	var subscription BillingSubscription
	err := row.Scan(
		&subscription.ID,
		&subscription.OrganizationID,
		&subscription.DodoSubscriptionID,
		&subscription.DodoCustomerID,
		&subscription.DodoProductID,
		&subscription.PlanKey,
		&subscription.BillingPeriod,
		&subscription.Status,
		&subscription.NextBillingDate,
		&subscription.CancelAtNextBillingDate,
		&subscription.CancelledAt,
		&subscription.ExpiresAt,
		&subscription.TrialPeriodDays,
		&subscription.SeatQuantity,
		&subscription.AddonQuantities,
		&subscription.LatestDodoEventAt,
		&subscription.CreatedAt,
		&subscription.UpdatedAt,
	)
	if err != nil {
		return BillingSubscription{}, err
	}
	return subscription, nil
}

const activeWorkspaceRunsSQL = `
	SELECT count(*)
	FROM runs
	WHERE workspace_id = $1
	  AND status IN ('queued', 'provisioning', 'running', 'scoring')
`
