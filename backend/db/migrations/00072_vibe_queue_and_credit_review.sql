-- +goose Up
ALTER TABLE vibe_operations ADD COLUMN queued_at timestamptz;
CREATE TABLE vibe_credit_reviews (
 source text PRIMARY KEY, payment_id text NOT NULL, event_type text NOT NULL,
 payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX vibe_credit_reviews_payment_idx ON vibe_credit_reviews(payment_id);

-- +goose Down
DROP TABLE IF EXISTS vibe_credit_reviews;
ALTER TABLE vibe_operations DROP COLUMN IF EXISTS queued_at;
