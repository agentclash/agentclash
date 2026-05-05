-- name: ResolveWorkspaceOrganization :one
SELECT organization_id
FROM workspaces
WHERE id = @workspace_id
  AND status = 'active';

-- name: GetOrganizationEntitlements :one
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
    source_subscription_id,
    effective_at,
    expires_at,
    updated_at
FROM organization_entitlements
WHERE organization_id = @organization_id;

-- name: UpsertOrganizationEntitlements :exec
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
VALUES (
    @organization_id,
    @plan_key,
    @billing_period,
    @status,
    @seat_quantity,
    @seats_limit,
    @workspaces_limit,
    @races_per_workspace_month,
    @max_models_per_race,
    @replay_retention_days,
    @concurrency_limit,
    @feature_flags::jsonb,
    sqlc.narg('source_subscription_id'),
    now(),
    sqlc.narg('expires_at')
)
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
    expires_at = EXCLUDED.expires_at;

-- name: CountActiveOrgMembers :one
SELECT count(*)::integer
FROM organization_memberships
WHERE organization_id = @organization_id
  AND membership_status = 'active';

-- name: CountActiveWorkspaces :one
SELECT count(*)::integer
FROM workspaces
WHERE organization_id = @organization_id
  AND status = 'active';

-- name: CountActiveWorkspaceRuns :one
SELECT count(*)::integer
FROM runs
WHERE workspace_id = @workspace_id
  AND status IN ('queued', 'provisioning', 'running', 'scoring');

-- name: GetWorkspaceUsageWindowRaceCount :one
SELECT race_count
FROM workspace_usage_windows
WHERE workspace_id = @workspace_id
  AND window_start = @window_start;

-- name: CreateBillingCheckoutIntent :one
INSERT INTO billing_checkout_intents (
    id,
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
VALUES (
    @id,
    @organization_id,
    @created_by_user_id,
    @requested_plan_key,
    @billing_period,
    @seat_quantity,
    @return_url,
    NULLIF(@dodo_checkout_session_id, ''),
    @checkout_url,
    'created',
    @metadata::jsonb
)
RETURNING
    id,
    organization_id,
    requested_plan_key,
    billing_period,
    seat_quantity,
    return_url,
    checkout_url,
    dodo_checkout_session_id,
    status,
    metadata,
    created_at,
    updated_at;

-- name: MarkBillingCheckoutIntentCompleted :execrows
UPDATE billing_checkout_intents
SET status = 'completed'
WHERE id = @id
  AND status <> 'completed';

-- name: GetLatestBillingCheckoutIntentByOrganizationID :one
SELECT
    id,
    organization_id,
    requested_plan_key,
    billing_period,
    seat_quantity,
    return_url,
    checkout_url,
    dodo_checkout_session_id,
    status,
    metadata,
    created_at,
    updated_at
FROM billing_checkout_intents
WHERE organization_id = @organization_id
ORDER BY created_at DESC
LIMIT 1;

-- name: UpsertBillingAccount :exec
INSERT INTO billing_accounts (organization_id, dodo_customer_id, billing_email, status)
VALUES (
    @organization_id,
    NULLIF(@dodo_customer_id, ''),
    NULLIF(@billing_email, ''),
    @status
)
ON CONFLICT (organization_id) DO UPDATE SET
    dodo_customer_id = COALESCE(EXCLUDED.dodo_customer_id, billing_accounts.dodo_customer_id),
    billing_email = COALESCE(EXCLUDED.billing_email, billing_accounts.billing_email),
    status = EXCLUDED.status;

-- name: GetBillingAccountByOrganizationID :one
SELECT
    id,
    organization_id,
    dodo_customer_id,
    billing_email,
    status,
    created_at,
    updated_at
FROM billing_accounts
WHERE organization_id = @organization_id;

-- name: UpsertBillingSubscription :one
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
VALUES (
    @organization_id,
    @dodo_subscription_id,
    NULLIF(@dodo_customer_id, ''),
    @dodo_product_id,
    @plan_key,
    @billing_period,
    @status,
    sqlc.narg('next_billing_date'),
    @cancel_at_next_billing_date,
    sqlc.narg('cancelled_at'),
    sqlc.narg('expires_at'),
    sqlc.narg('trial_period_days'),
    @seat_quantity,
    @addon_quantities::jsonb,
    sqlc.narg('latest_dodo_event_at')
)
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
RETURNING
    id,
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
    latest_dodo_event_at,
    created_at,
    updated_at;

-- name: GetBillingSubscriptionByDodoID :one
SELECT
    id,
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
    latest_dodo_event_at,
    created_at,
    updated_at
FROM billing_subscriptions
WHERE dodo_subscription_id = @dodo_subscription_id;

-- name: GetLatestBillingSubscriptionByOrganizationID :one
SELECT
    id,
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
    latest_dodo_event_at,
    created_at,
    updated_at
FROM billing_subscriptions
WHERE organization_id = @organization_id
ORDER BY latest_dodo_event_at DESC NULLS LAST, updated_at DESC
LIMIT 1;

-- name: FindOrganizationByDodoSubscriptionOrCustomer :one
SELECT organization_id FROM (
    SELECT organization_id, 1 AS priority, latest_dodo_event_at AS event_at
    FROM billing_subscriptions
    WHERE dodo_subscription_id = @dodo_subscription_id
    UNION ALL
    SELECT organization_id, 2 AS priority, updated_at AS event_at
    FROM billing_accounts
    WHERE dodo_customer_id = NULLIF(@dodo_customer_id, '')
) candidates
ORDER BY priority, event_at DESC NULLS LAST
LIMIT 1;

-- name: BeginBillingWebhookEvent :one
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
VALUES (
    @webhook_id,
    @event_type,
    sqlc.narg('dodo_business_id'),
    sqlc.narg('payload_type'),
    sqlc.narg('event_timestamp'),
    NULL,
    @payload_hash,
    'failed',
    NULL,
    @payload::jsonb
)
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
RETURNING status;

-- name: FinishBillingWebhookEvent :execrows
UPDATE billing_webhook_events
SET status = @status,
    error = sqlc.narg('error'),
    processed_at = now()
WHERE webhook_id = @webhook_id;

-- name: CreateBillingTrialGrant :one
INSERT INTO billing_trial_grants (
    organization_id,
    plan_key,
    billing_period,
    started_by_user_id,
    started_at,
    expires_at
)
VALUES (
    @organization_id,
    @plan_key,
    @billing_period,
    @started_by_user_id,
    @started_at,
    @expires_at
)
RETURNING
    id,
    organization_id,
    plan_key,
    billing_period,
    started_by_user_id,
    started_at,
    expires_at,
    created_at,
    updated_at;
