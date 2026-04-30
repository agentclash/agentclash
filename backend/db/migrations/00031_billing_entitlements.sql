-- +goose Up
CREATE TABLE billing_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL UNIQUE REFERENCES organizations (id) ON DELETE CASCADE,
    dodo_customer_id text UNIQUE,
    billing_email citext,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE billing_subscriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    dodo_subscription_id text NOT NULL UNIQUE,
    dodo_customer_id text,
    dodo_product_id text NOT NULL,
    plan_key text NOT NULL CHECK (plan_key IN ('free', 'pro', 'team', 'enterprise')),
    billing_period text NOT NULL CHECK (billing_period IN ('monthly', 'yearly', 'custom')),
    status text NOT NULL,
    next_billing_date timestamptz,
    cancel_at_next_billing_date boolean NOT NULL DEFAULT false,
    cancelled_at timestamptz,
    expires_at timestamptz,
    trial_period_days integer,
    seat_quantity integer NOT NULL CHECK (seat_quantity > 0),
    addon_quantities jsonb NOT NULL DEFAULT '{}'::jsonb,
    latest_dodo_event_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX billing_subscriptions_organization_id_idx
    ON billing_subscriptions (organization_id, status);

CREATE TABLE billing_checkout_intents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    requested_plan_key text NOT NULL CHECK (requested_plan_key IN ('pro', 'team', 'enterprise')),
    billing_period text NOT NULL CHECK (billing_period IN ('monthly', 'yearly', 'custom')),
    seat_quantity integer NOT NULL CHECK (seat_quantity > 0),
    return_url text NOT NULL,
    dodo_checkout_session_id text,
    checkout_url text NOT NULL,
    status text NOT NULL DEFAULT 'created' CHECK (status IN ('created', 'pending', 'completed', 'cancelled', 'expired')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX billing_checkout_intents_organization_id_idx
    ON billing_checkout_intents (organization_id, created_at DESC);

CREATE TABLE billing_webhook_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id text NOT NULL UNIQUE,
    event_type text NOT NULL,
    dodo_business_id text,
    payload_type text,
    event_timestamp timestamptz,
    processed_at timestamptz,
    payload_hash text NOT NULL,
    status text NOT NULL DEFAULT 'processed' CHECK (status IN ('processed', 'duplicate', 'failed')),
    error text,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX billing_webhook_events_event_type_idx
    ON billing_webhook_events (event_type, created_at DESC);

CREATE TABLE organization_entitlements (
    organization_id uuid PRIMARY KEY REFERENCES organizations (id) ON DELETE CASCADE,
    plan_key text NOT NULL CHECK (plan_key IN ('free', 'pro', 'team', 'enterprise')),
    billing_period text NOT NULL CHECK (billing_period IN ('monthly', 'yearly', 'custom')),
    status text NOT NULL DEFAULT 'active',
    seat_quantity integer NOT NULL CHECK (seat_quantity > 0),
    seats_limit integer,
    workspaces_limit integer,
    races_per_workspace_month integer,
    max_models_per_race integer,
    replay_retention_days integer,
    concurrency_limit integer,
    feature_flags jsonb NOT NULL DEFAULT '{}'::jsonb,
    source_subscription_id uuid REFERENCES billing_subscriptions (id) ON DELETE SET NULL,
    effective_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspace_usage_windows (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    window_start timestamptz NOT NULL,
    window_end timestamptz NOT NULL,
    race_count integer NOT NULL DEFAULT 0 CHECK (race_count >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, window_start)
);

CREATE INDEX workspace_usage_windows_workspace_id_idx
    ON workspace_usage_windows (workspace_id, window_start DESC);

CREATE TRIGGER billing_accounts_set_updated_at
BEFORE UPDATE ON billing_accounts
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER billing_subscriptions_set_updated_at
BEFORE UPDATE ON billing_subscriptions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER billing_checkout_intents_set_updated_at
BEFORE UPDATE ON billing_checkout_intents
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER organization_entitlements_set_updated_at
BEFORE UPDATE ON organization_entitlements
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER workspace_usage_windows_set_updated_at
BEFORE UPDATE ON workspace_usage_windows
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

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
    feature_flags
)
SELECT
    id,
    'free',
    'monthly',
    'active',
    1,
    1,
    1,
    25,
    4,
    7,
    1,
    '{"byok_llm":true,"byok_e2b":true,"community_support":true}'::jsonb
FROM organizations
ON CONFLICT (organization_id) DO NOTHING;

-- +goose Down
DROP TRIGGER IF EXISTS workspace_usage_windows_set_updated_at ON workspace_usage_windows;
DROP TRIGGER IF EXISTS organization_entitlements_set_updated_at ON organization_entitlements;
DROP TRIGGER IF EXISTS billing_checkout_intents_set_updated_at ON billing_checkout_intents;
DROP TRIGGER IF EXISTS billing_subscriptions_set_updated_at ON billing_subscriptions;
DROP TRIGGER IF EXISTS billing_accounts_set_updated_at ON billing_accounts;

DROP TABLE IF EXISTS workspace_usage_windows;
DROP TABLE IF EXISTS organization_entitlements;
DROP TABLE IF EXISTS billing_webhook_events;
DROP TABLE IF EXISTS billing_checkout_intents;
DROP TABLE IF EXISTS billing_subscriptions;
DROP TABLE IF EXISTS billing_accounts;
