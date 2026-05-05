-- +goose Up
CREATE TABLE billing_trial_grants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL UNIQUE REFERENCES organizations (id) ON DELETE CASCADE,
    plan_key text NOT NULL CHECK (plan_key IN ('pro', 'team')),
    billing_period text NOT NULL CHECK (billing_period IN ('monthly', 'yearly')),
    started_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER billing_trial_grants_set_updated_at
BEFORE UPDATE ON billing_trial_grants
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS billing_trial_grants_set_updated_at ON billing_trial_grants;
DROP TABLE IF EXISTS billing_trial_grants;
