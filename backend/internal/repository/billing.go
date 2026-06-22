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
	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrBillingWebhookAlreadyProcessed = errors.New("billing webhook already processed")
	ErrBillingTrialAlreadyUsed        = errors.New("billing trial already used")
)

type BillingCheckoutIntentInput struct {
	ID               uuid.UUID
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

type BillingAccount struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	DodoCustomerID *string   `json:"dodo_customer_id,omitempty"`
	BillingEmail   *string   `json:"billing_email,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BillingTrialGrantInput struct {
	OrganizationID  uuid.UUID
	PlanKey         string
	BillingPeriod   string
	StartedByUserID uuid.UUID
	StartedAt       time.Time
	ExpiresAt       time.Time
}

type BillingTrialGrant struct {
	ID              uuid.UUID  `json:"id"`
	OrganizationID  uuid.UUID  `json:"organization_id"`
	PlanKey         string     `json:"plan_key"`
	BillingPeriod   string     `json:"billing_period"`
	StartedByUserID *uuid.UUID `json:"started_by_user_id,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	ExpiresAt       time.Time  `json:"expires_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
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
	Account          *BillingAccountInput
	Subscription     *BillingSubscriptionInput
	Entitlements     *BillingWebhookEntitlementsInput
	CheckoutIntentID *uuid.UUID
}

type BillingOverview struct {
	Entitlements         billing.EffectiveEntitlements `json:"entitlements"`
	Account              *BillingAccount               `json:"account,omitempty"`
	Subscription         *BillingSubscription          `json:"subscription,omitempty"`
	LatestCheckoutIntent *BillingCheckoutIntent        `json:"latest_checkout_intent,omitempty"`
}

type WorkspaceUsageSnapshot struct {
	WorkspaceID         uuid.UUID `json:"workspace_id"`
	RaceCount           int       `json:"race_count"`
	GuideAgentTurnCount int       `json:"guide_agent_turn_count"`
	ActiveRuns          int       `json:"active_runs"`
	WindowStart         time.Time `json:"window_start"`
	WindowEnd           time.Time `json:"window_end"`
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
	orgID, err := r.queries.ResolveWorkspaceOrganization(ctx, repositorysqlc.ResolveWorkspaceOrganizationParams{WorkspaceID: workspaceID})
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
	row, err := r.queries.GetOrganizationEntitlements(ctx, repositorysqlc.GetOrganizationEntitlementsParams{OrganizationID: orgID})
	if err != nil {
		return billing.EffectiveEntitlements{}, err
	}
	entitlements, err := mapEffectiveEntitlements(row)
	if err != nil {
		return billing.EffectiveEntitlements{}, err
	}
	return entitlements.WithComputedStatus(time.Now().UTC()), nil
}

func (r *Repository) UpsertOrganizationEntitlements(ctx context.Context, orgID uuid.UUID, entitlements billing.EffectiveEntitlements, sourceSubscriptionID *uuid.UUID, expiresAt *time.Time) error {
	return upsertOrganizationEntitlements(ctx, r.queries, orgID, entitlements, sourceSubscriptionID, expiresAt)
}

func upsertOrganizationEntitlements(ctx context.Context, queries *repositorysqlc.Queries, orgID uuid.UUID, entitlements billing.EffectiveEntitlements, sourceSubscriptionID *uuid.UUID, expiresAt *time.Time) error {
	if expiresAt == nil && entitlements.ExpiresAt != nil {
		expiresAt = entitlements.ExpiresAt
	}
	featureFlags, err := json.Marshal(entitlements.FeatureFlags)
	if err != nil {
		return fmt.Errorf("marshal feature flags: %w", err)
	}
	if err := queries.UpsertOrganizationEntitlements(ctx, repositorysqlc.UpsertOrganizationEntitlementsParams{
		OrganizationID:                   orgID,
		PlanKey:                          entitlements.PlanKey,
		BillingPeriod:                    entitlements.BillingPeriod,
		Status:                           entitlements.Status,
		SeatQuantity:                     int32(entitlements.SeatQuantity),
		SeatsLimit:                       int32Ptr(entitlements.SeatsLimit),
		WorkspacesLimit:                  int32Ptr(entitlements.WorkspacesLimit),
		RacesPerWorkspaceMonth:           int32Ptr(entitlements.RacesPerWorkspaceMonth),
		MaxModelsPerRace:                 int32Ptr(entitlements.MaxModelsPerRace),
		ReplayRetentionDays:              int32Ptr(entitlements.ReplayRetentionDays),
		ConcurrencyLimit:                 int32Ptr(entitlements.ConcurrentRaces),
		GuideAgentTurnsPerWorkspaceMonth: int32Ptr(entitlements.GuideAgentTurnsPerWorkspaceMonth),
		FeatureFlags:                     featureFlags,
		SourceSubscriptionID:             sourceSubscriptionID,
		ExpiresAt:                        toPGTimestamp(expiresAt),
	}); err != nil {
		return fmt.Errorf("upsert organization entitlements: %w", err)
	}
	return nil
}

func (r *Repository) ensureDefaultOrganizationEntitlements(ctx context.Context, orgID uuid.UUID) error {
	defaultEntitlements := billing.DefaultEntitlements()
	return r.UpsertOrganizationEntitlements(ctx, orgID, defaultEntitlements, nil, nil)
}

func (r *Repository) CountActiveOrgMembers(ctx context.Context, orgID uuid.UUID) (int, error) {
	count, err := r.queries.CountActiveOrgMembers(ctx, repositorysqlc.CountActiveOrgMembersParams{OrganizationID: orgID})
	if err != nil {
		return 0, fmt.Errorf("count active org members: %w", err)
	}
	return int(count), nil
}

func (r *Repository) CountActiveWorkspaces(ctx context.Context, orgID uuid.UUID) (int, error) {
	count, err := r.queries.CountActiveWorkspaces(ctx, repositorysqlc.CountActiveWorkspacesParams{OrganizationID: orgID})
	if err != nil {
		return 0, fmt.Errorf("count active workspaces: %w", err)
	}
	return int(count), nil
}

func (r *Repository) CountActiveWorkspaceRuns(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	count, err := r.queries.CountActiveWorkspaceRuns(ctx, repositorysqlc.CountActiveWorkspaceRunsParams{WorkspaceID: workspaceID})
	if err != nil {
		return 0, fmt.Errorf("count active workspace runs: %w", err)
	}
	return int(count), nil
}

func (r *Repository) GetWorkspaceUsageSnapshot(ctx context.Context, workspaceID uuid.UUID, windowStart, windowEnd time.Time) (WorkspaceUsageSnapshot, error) {
	counts, err := r.queries.GetWorkspaceUsageWindowCounts(ctx, repositorysqlc.GetWorkspaceUsageWindowCountsParams{
		WorkspaceID: workspaceID,
		WindowStart: pgtype.Timestamptz{Time: windowStart.UTC(), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		counts = repositorysqlc.GetWorkspaceUsageWindowCountsRow{}
	} else if err != nil {
		return WorkspaceUsageSnapshot{}, fmt.Errorf("get workspace usage window: %w", err)
	}
	activeRuns, err := r.CountActiveWorkspaceRuns(ctx, workspaceID)
	if err != nil {
		return WorkspaceUsageSnapshot{}, err
	}
	return WorkspaceUsageSnapshot{
		WorkspaceID:         workspaceID,
		RaceCount:           int(counts.RaceCount),
		GuideAgentTurnCount: int(counts.GuideAgentTurnCount),
		ActiveRuns:          activeRuns,
		WindowStart:         windowStart,
		WindowEnd:           windowEnd,
	}, nil
}

func (r *Repository) CreateBillingCheckoutIntent(ctx context.Context, input BillingCheckoutIntentInput) (BillingCheckoutIntent, error) {
	metadata := normalizeJSON(input.Metadata)
	intentID := input.ID
	if intentID == uuid.Nil {
		intentID = uuid.New()
	}
	createdBy := &input.CreatedByUserID
	if input.CreatedByUserID == uuid.Nil {
		createdBy = nil
	}
	row, err := r.queries.CreateBillingCheckoutIntent(ctx, repositorysqlc.CreateBillingCheckoutIntentParams{
		ID:                    intentID,
		OrganizationID:        input.OrganizationID,
		CreatedByUserID:       createdBy,
		RequestedPlanKey:      input.RequestedPlanKey,
		BillingPeriod:         input.BillingPeriod,
		SeatQuantity:          int32(input.SeatQuantity),
		ReturnUrl:             input.ReturnURL,
		DodoCheckoutSessionID: input.DodoCheckoutID,
		CheckoutUrl:           input.CheckoutURL,
		Metadata:              metadata,
	})
	if err != nil {
		return BillingCheckoutIntent{}, fmt.Errorf("create billing checkout intent: %w", err)
	}
	return mapBillingCheckoutIntent(row)
}

func (r *Repository) UpsertBillingAccount(ctx context.Context, orgID uuid.UUID, dodoCustomerID string, billingEmail string, status string) error {
	return upsertBillingAccount(ctx, r.queries, BillingAccountInput{
		OrganizationID: orgID,
		DodoCustomerID: dodoCustomerID,
		BillingEmail:   billingEmail,
		Status:         status,
	})
}

func upsertBillingAccount(ctx context.Context, queries *repositorysqlc.Queries, input BillingAccountInput) error {
	status := input.Status
	if status == "" {
		status = "active"
	}
	if err := queries.UpsertBillingAccount(ctx, repositorysqlc.UpsertBillingAccountParams{
		OrganizationID: input.OrganizationID,
		DodoCustomerID: input.DodoCustomerID,
		BillingEmail:   input.BillingEmail,
		Status:         status,
	}); err != nil {
		return fmt.Errorf("upsert billing account: %w", err)
	}
	return nil
}

func (r *Repository) GetBillingAccount(ctx context.Context, orgID uuid.UUID) (BillingAccount, error) {
	row, err := r.queries.GetBillingAccountByOrganizationID(ctx, repositorysqlc.GetBillingAccountByOrganizationIDParams{OrganizationID: orgID})
	if err != nil {
		return BillingAccount{}, err
	}
	return mapBillingAccount(row)
}

func (r *Repository) UpsertBillingSubscription(ctx context.Context, input BillingSubscriptionInput) (BillingSubscription, error) {
	subscription, _, err := upsertBillingSubscription(ctx, r.queries, input)
	return subscription, err
}

func upsertBillingSubscription(ctx context.Context, queries *repositorysqlc.Queries, input BillingSubscriptionInput) (BillingSubscription, bool, error) {
	addons := normalizeJSON(input.AddonQuantities)
	subscriptionRow, err := queries.UpsertBillingSubscription(ctx, repositorysqlc.UpsertBillingSubscriptionParams{
		OrganizationID:          input.OrganizationID,
		DodoSubscriptionID:      input.DodoSubscriptionID,
		DodoCustomerID:          input.DodoCustomerID,
		DodoProductID:           input.DodoProductID,
		PlanKey:                 input.PlanKey,
		BillingPeriod:           input.BillingPeriod,
		Status:                  input.Status,
		NextBillingDate:         toPGTimestamp(input.NextBillingDate),
		CancelAtNextBillingDate: input.CancelAtNextBillingDate,
		CancelledAt:             toPGTimestamp(input.CancelledAt),
		ExpiresAt:               toPGTimestamp(input.ExpiresAt),
		TrialPeriodDays:         int32Ptr(input.TrialPeriodDays),
		SeatQuantity:            int32(input.SeatQuantity),
		AddonQuantities:         addons,
		LatestDodoEventAt:       toPGTimestamp(input.LatestDodoEventAt),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		subscription, selectErr := getBillingSubscriptionByDodoID(ctx, queries, input.DodoSubscriptionID)
		if selectErr != nil {
			return BillingSubscription{}, false, selectErr
		}
		return subscription, false, nil
	}
	if err != nil {
		return BillingSubscription{}, false, fmt.Errorf("upsert billing subscription: %w", err)
	}
	subscription, err := mapBillingSubscription(subscriptionRow)
	if err != nil {
		return BillingSubscription{}, false, err
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

	overview := BillingOverview{Entitlements: entitlements}
	account, err := r.GetBillingAccount(ctx, orgID)
	if err == nil {
		overview.Account = &account
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return BillingOverview{}, err
	}

	subscription, err := r.getLatestBillingSubscription(ctx, orgID)
	if err == nil {
		overview.Subscription = &subscription
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return BillingOverview{}, err
	}

	intent, err := r.getLatestBillingCheckoutIntent(ctx, orgID)
	if err == nil {
		overview.LatestCheckoutIntent = &intent
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return BillingOverview{}, err
	}
	return overview, nil
}

func (r *Repository) FindOrganizationByDodoSubscriptionOrCustomer(ctx context.Context, subscriptionID string, customerID string) (uuid.UUID, error) {
	orgID, err := r.queries.FindOrganizationByDodoSubscriptionOrCustomer(ctx, repositorysqlc.FindOrganizationByDodoSubscriptionOrCustomerParams{
		DodoSubscriptionID: subscriptionID,
		DodoCustomerID:     customerID,
	})
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
	queries := r.queries.WithTx(tx)

	duplicate, err := beginBillingWebhookEvent(ctx, queries, event)
	if err != nil || duplicate {
		return duplicate, err
	}

	if err := applyBillingWebhookApplication(ctx, queries, application); err != nil {
		return false, err
	}

	if err := finishBillingWebhookEvent(ctx, queries, event.WebhookID, "processed", ""); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit billing webhook transaction: %w", err)
	}
	return false, nil
}

func beginBillingWebhookEvent(ctx context.Context, queries *repositorysqlc.Queries, input BillingWebhookEventInput) (bool, error) {
	payload := normalizeJSON(input.Payload)
	hash := sha256.Sum256(payload)
	_, err := queries.BeginBillingWebhookEvent(ctx, repositorysqlc.BeginBillingWebhookEventParams{
		WebhookID:      input.WebhookID,
		EventType:      input.EventType,
		DodoBusinessID: input.DodoBusinessID,
		PayloadType:    input.PayloadType,
		EventTimestamp: toPGTimestamp(input.EventTimestamp),
		PayloadHash:    hex.EncodeToString(hash[:]),
		Payload:        payload,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("begin billing webhook event: %w", err)
	}
	return false, nil
}

func finishBillingWebhookEvent(ctx context.Context, queries *repositorysqlc.Queries, webhookID string, status string, message string) error {
	var errorMessage *string
	if message != "" {
		errorMessage = &message
	}
	rows, err := queries.FinishBillingWebhookEvent(ctx, repositorysqlc.FinishBillingWebhookEventParams{
		WebhookID: webhookID,
		Status:    status,
		Error:     errorMessage,
	})
	if err != nil {
		return fmt.Errorf("finish billing webhook event: %w", err)
	}
	if rows == 0 {
		return ErrBillingWebhookAlreadyProcessed
	}
	return nil
}

func applyBillingWebhookApplication(ctx context.Context, queries *repositorysqlc.Queries, application BillingWebhookApplication) error {
	var subscriptionID *uuid.UUID
	if application.Subscription != nil {
		subscription, applied, err := upsertBillingSubscription(ctx, queries, *application.Subscription)
		if err != nil {
			return err
		}
		if !applied {
			return nil
		}
		subscriptionID = &subscription.ID
	}
	if application.Account != nil {
		if err := upsertBillingAccount(ctx, queries, *application.Account); err != nil {
			return err
		}
	}
	if application.CheckoutIntentID != nil {
		if _, err := queries.MarkBillingCheckoutIntentCompleted(ctx, repositorysqlc.MarkBillingCheckoutIntentCompletedParams{ID: *application.CheckoutIntentID}); err != nil {
			return fmt.Errorf("mark billing checkout intent completed: %w", err)
		}
	}
	if application.Entitlements != nil {
		sourceSubscriptionID := (*uuid.UUID)(nil)
		if application.Entitlements.UseSubscriptionAsSource {
			sourceSubscriptionID = subscriptionID
		}
		if err := upsertOrganizationEntitlements(ctx, queries, application.Entitlements.OrganizationID, application.Entitlements.Entitlements, sourceSubscriptionID, application.Entitlements.ExpiresAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) getLatestBillingSubscription(ctx context.Context, orgID uuid.UUID) (BillingSubscription, error) {
	row, err := r.queries.GetLatestBillingSubscriptionByOrganizationID(ctx, repositorysqlc.GetLatestBillingSubscriptionByOrganizationIDParams{OrganizationID: orgID})
	if err != nil {
		return BillingSubscription{}, err
	}
	return mapBillingSubscription(row)
}

func (r *Repository) getLatestBillingCheckoutIntent(ctx context.Context, orgID uuid.UUID) (BillingCheckoutIntent, error) {
	row, err := r.queries.GetLatestBillingCheckoutIntentByOrganizationID(ctx, repositorysqlc.GetLatestBillingCheckoutIntentByOrganizationIDParams{OrganizationID: orgID})
	if err != nil {
		return BillingCheckoutIntent{}, err
	}
	return mapBillingCheckoutIntent(row)
}

func getBillingSubscriptionByDodoID(ctx context.Context, queries *repositorysqlc.Queries, subscriptionID string) (BillingSubscription, error) {
	row, err := queries.GetBillingSubscriptionByDodoID(ctx, repositorysqlc.GetBillingSubscriptionByDodoIDParams{DodoSubscriptionID: subscriptionID})
	if err != nil {
		return BillingSubscription{}, err
	}
	return mapBillingSubscription(row)
}

func (r *Repository) CreateBillingTrialGrant(ctx context.Context, input BillingTrialGrantInput) (BillingTrialGrant, error) {
	startedBy := &input.StartedByUserID
	if input.StartedByUserID == uuid.Nil {
		startedBy = nil
	}
	row, err := r.queries.CreateBillingTrialGrant(ctx, repositorysqlc.CreateBillingTrialGrantParams{
		OrganizationID:  input.OrganizationID,
		PlanKey:         input.PlanKey,
		BillingPeriod:   input.BillingPeriod,
		StartedByUserID: startedBy,
		StartedAt:       pgtype.Timestamptz{Time: input.StartedAt.UTC(), Valid: true},
		ExpiresAt:       pgtype.Timestamptz{Time: input.ExpiresAt.UTC(), Valid: true},
	})
	if err != nil {
		if isUniqueViolation(err) {
			return BillingTrialGrant{}, ErrBillingTrialAlreadyUsed
		}
		return BillingTrialGrant{}, fmt.Errorf("create billing trial grant: %w", err)
	}
	return mapBillingTrialGrant(row)
}

func mapEffectiveEntitlements(row repositorysqlc.GetOrganizationEntitlementsRow) (billing.EffectiveEntitlements, error) {
	var featureFlags map[string]bool
	if len(row.FeatureFlags) > 0 {
		if err := json.Unmarshal(row.FeatureFlags, &featureFlags); err != nil {
			return billing.EffectiveEntitlements{}, fmt.Errorf("decode feature flags: %w", err)
		}
	}
	if featureFlags == nil {
		featureFlags = map[string]bool{}
	}
	entitlements := billing.EffectiveEntitlements{
		PlanKey:                          row.PlanKey,
		BillingPeriod:                    row.BillingPeriod,
		Status:                           row.Status,
		SeatQuantity:                     int(row.SeatQuantity),
		SeatsLimit:                       intPtrFromInt32(row.SeatsLimit),
		WorkspacesLimit:                  intPtrFromInt32(row.WorkspacesLimit),
		RacesPerWorkspaceMonth:           intPtrFromInt32(row.RacesPerWorkspaceMonth),
		MaxModelsPerRace:                 intPtrFromInt32(row.MaxModelsPerRace),
		ReplayRetentionDays:              intPtrFromInt32(row.ReplayRetentionDays),
		ConcurrentRaces:                  intPtrFromInt32(row.ConcurrencyLimit),
		GuideAgentTurnsPerWorkspaceMonth: intPtrFromInt32(row.GuideAgentTurnsPerWorkspaceMonth),
		FeatureFlags:                     featureFlags,
		ExpiresAt:                        optionalTime(row.ExpiresAt),
	}
	if plan, ok := billing.PlanByKey(entitlements.PlanKey); ok {
		entitlements.UpgradeTarget = plan.UpgradeTarget
	}
	return entitlements, nil
}

func mapBillingCheckoutIntent(row any) (BillingCheckoutIntent, error) {
	switch typed := row.(type) {
	case repositorysqlc.CreateBillingCheckoutIntentRow:
		createdAt, err := requiredTime("billing_checkout_intents.created_at", typed.CreatedAt)
		if err != nil {
			return BillingCheckoutIntent{}, err
		}
		updatedAt, err := requiredTime("billing_checkout_intents.updated_at", typed.UpdatedAt)
		if err != nil {
			return BillingCheckoutIntent{}, err
		}
		return BillingCheckoutIntent{
			ID:               typed.ID,
			OrganizationID:   typed.OrganizationID,
			RequestedPlanKey: typed.RequestedPlanKey,
			BillingPeriod:    typed.BillingPeriod,
			SeatQuantity:     int(typed.SeatQuantity),
			ReturnURL:        typed.ReturnUrl,
			CheckoutURL:      typed.CheckoutUrl,
			DodoCheckoutID:   typed.DodoCheckoutSessionID,
			Status:           typed.Status,
			Metadata:         cloneJSON(typed.Metadata),
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
		}, nil
	case repositorysqlc.GetLatestBillingCheckoutIntentByOrganizationIDRow:
		createdAt, err := requiredTime("billing_checkout_intents.created_at", typed.CreatedAt)
		if err != nil {
			return BillingCheckoutIntent{}, err
		}
		updatedAt, err := requiredTime("billing_checkout_intents.updated_at", typed.UpdatedAt)
		if err != nil {
			return BillingCheckoutIntent{}, err
		}
		return BillingCheckoutIntent{
			ID:               typed.ID,
			OrganizationID:   typed.OrganizationID,
			RequestedPlanKey: typed.RequestedPlanKey,
			BillingPeriod:    typed.BillingPeriod,
			SeatQuantity:     int(typed.SeatQuantity),
			ReturnURL:        typed.ReturnUrl,
			CheckoutURL:      typed.CheckoutUrl,
			DodoCheckoutID:   typed.DodoCheckoutSessionID,
			Status:           typed.Status,
			Metadata:         cloneJSON(typed.Metadata),
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
		}, nil
	default:
		return BillingCheckoutIntent{}, fmt.Errorf("unsupported billing checkout intent row %T", row)
	}
}

func mapBillingAccount(row repositorysqlc.BillingAccount) (BillingAccount, error) {
	createdAt, err := requiredTime("billing_accounts.created_at", row.CreatedAt)
	if err != nil {
		return BillingAccount{}, err
	}
	updatedAt, err := requiredTime("billing_accounts.updated_at", row.UpdatedAt)
	if err != nil {
		return BillingAccount{}, err
	}
	return BillingAccount{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		DodoCustomerID: row.DodoCustomerID,
		BillingEmail:   row.BillingEmail,
		Status:         row.Status,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

func mapBillingSubscription(row repositorysqlc.BillingSubscription) (BillingSubscription, error) {
	createdAt, err := requiredTime("billing_subscriptions.created_at", row.CreatedAt)
	if err != nil {
		return BillingSubscription{}, err
	}
	updatedAt, err := requiredTime("billing_subscriptions.updated_at", row.UpdatedAt)
	if err != nil {
		return BillingSubscription{}, err
	}
	return BillingSubscription{
		ID:                      row.ID,
		OrganizationID:          row.OrganizationID,
		DodoSubscriptionID:      row.DodoSubscriptionID,
		DodoCustomerID:          row.DodoCustomerID,
		DodoProductID:           row.DodoProductID,
		PlanKey:                 row.PlanKey,
		BillingPeriod:           row.BillingPeriod,
		Status:                  row.Status,
		NextBillingDate:         optionalTime(row.NextBillingDate),
		CancelAtNextBillingDate: row.CancelAtNextBillingDate,
		CancelledAt:             optionalTime(row.CancelledAt),
		ExpiresAt:               optionalTime(row.ExpiresAt),
		TrialPeriodDays:         intPtrFromInt32(row.TrialPeriodDays),
		SeatQuantity:            int(row.SeatQuantity),
		AddonQuantities:         cloneJSON(row.AddonQuantities),
		LatestDodoEventAt:       optionalTime(row.LatestDodoEventAt),
		CreatedAt:               createdAt,
		UpdatedAt:               updatedAt,
	}, nil
}

func mapBillingTrialGrant(row repositorysqlc.BillingTrialGrant) (BillingTrialGrant, error) {
	startedAt, err := requiredTime("billing_trial_grants.started_at", row.StartedAt)
	if err != nil {
		return BillingTrialGrant{}, err
	}
	expiresAt, err := requiredTime("billing_trial_grants.expires_at", row.ExpiresAt)
	if err != nil {
		return BillingTrialGrant{}, err
	}
	createdAt, err := requiredTime("billing_trial_grants.created_at", row.CreatedAt)
	if err != nil {
		return BillingTrialGrant{}, err
	}
	updatedAt, err := requiredTime("billing_trial_grants.updated_at", row.UpdatedAt)
	if err != nil {
		return BillingTrialGrant{}, err
	}
	return BillingTrialGrant{
		ID:              row.ID,
		OrganizationID:  row.OrganizationID,
		PlanKey:         row.PlanKey,
		BillingPeriod:   row.BillingPeriod,
		StartedByUserID: row.StartedByUserID,
		StartedAt:       startedAt,
		ExpiresAt:       expiresAt,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}, nil
}

func int32Ptr(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func intPtrFromInt32(value *int32) *int {
	if value == nil {
		return nil
	}
	converted := int(*value)
	return &converted
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

const activeWorkspaceRunsSQL = `
	SELECT count(*)
	FROM runs
	WHERE workspace_id = $1
	  AND status IN ('queued', 'provisioning', 'running', 'scoring')
`
