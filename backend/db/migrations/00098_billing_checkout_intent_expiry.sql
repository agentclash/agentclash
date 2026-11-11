-- +goose Up
-- The checkout intent lifecycle is created -> expired -> completed, and
-- created -> completed (repository/billing_checkout_intent_status.go). No
-- code writes 'pending' or 'cancelled'. If a row has one of them, it is an
-- abandoned checkout, so it becomes 'expired'.
UPDATE billing_checkout_intents
SET status = 'expired'
WHERE status IN ('pending', 'cancelled');

ALTER TABLE billing_checkout_intents DROP CONSTRAINT billing_checkout_intents_status_check;
ALTER TABLE billing_checkout_intents ADD CONSTRAINT billing_checkout_intents_status_check
    CHECK (status IN ('created', 'completed', 'expired'));

-- The worker sweep finds created intents by age.
CREATE INDEX billing_checkout_intents_open_created_at_idx
    ON billing_checkout_intents (created_at)
    WHERE status = 'created';

-- +goose Down
-- The old constraint permits each current status, so no row changes.
DROP INDEX IF EXISTS billing_checkout_intents_open_created_at_idx;

ALTER TABLE billing_checkout_intents DROP CONSTRAINT billing_checkout_intents_status_check;
ALTER TABLE billing_checkout_intents ADD CONSTRAINT billing_checkout_intents_status_check
    CHECK (status IN ('created', 'pending', 'completed', 'cancelled', 'expired'));
