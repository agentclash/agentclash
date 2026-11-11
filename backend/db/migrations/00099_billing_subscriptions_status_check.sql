-- +goose Up
-- billing_subscriptions.status had no CHECK, so each Dodo string went into the
-- column. The set is the same as billing.SubscriptionStatuses() in
-- backend/internal/billing/subscription_status.go. Add the CHECK as NOT VALID,
-- map the old values into the set, and then validate the CHECK.
ALTER TABLE billing_subscriptions ADD CONSTRAINT billing_subscriptions_status_check
    CHECK (status IN ('pending', 'active', 'trialing', 'on_hold', 'paused', 'past_due', 'failed', 'cancelled', 'expired', 'unknown')) NOT VALID;

-- 'renewed' and 'canceled' are Dodo aliases. 'succeeded' came from the body of
-- a payment.succeeded event, so that subscription was active. Each other value
-- becomes 'unknown'.
UPDATE billing_subscriptions
SET status = CASE
        WHEN lower(btrim(status)) IN ('renewed', 'succeeded') THEN 'active'
        WHEN lower(btrim(status)) = 'canceled' THEN 'cancelled'
        WHEN lower(btrim(status)) IN ('pending', 'active', 'trialing', 'on_hold', 'paused', 'past_due', 'failed', 'cancelled', 'expired') THEN lower(btrim(status))
        ELSE 'unknown'
    END
WHERE status NOT IN ('pending', 'active', 'trialing', 'on_hold', 'paused', 'past_due', 'failed', 'cancelled', 'expired', 'unknown');

ALTER TABLE billing_subscriptions VALIDATE CONSTRAINT billing_subscriptions_status_check;

-- +goose Down
-- The Down section does not restore the old values. They were not in the set.
ALTER TABLE billing_subscriptions DROP CONSTRAINT IF EXISTS billing_subscriptions_status_check;
