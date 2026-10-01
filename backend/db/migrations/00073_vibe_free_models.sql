-- +goose Up
-- Explicit free routes retain the same durable journal and reservations.
ALTER TABLE vibe_reservations DROP CONSTRAINT vibe_reservations_amount_check;
ALTER TABLE vibe_reservations ADD CONSTRAINT vibe_reservations_amount_check CHECK(amount>=0);
ALTER TABLE vibe_attempts DROP CONSTRAINT vibe_attempts_max_cost_check;
ALTER TABLE vibe_attempts ADD CONSTRAINT vibe_attempts_max_cost_check
 CHECK(max_cost>0 OR (max_cost=0 AND COALESCE(policy->'profile'->>'free'='true',false)));
CREATE INDEX vibe_attempts_free_daily_idx ON vibe_attempts(created_at) WHERE max_cost=0;

-- +goose Down
-- Retain free-call evidence; rollback refuses while zero-cost rows exist.
DROP INDEX vibe_attempts_free_daily_idx;
ALTER TABLE vibe_reservations DROP CONSTRAINT vibe_reservations_amount_check;
ALTER TABLE vibe_reservations ADD CONSTRAINT vibe_reservations_amount_check CHECK(amount>0);
ALTER TABLE vibe_attempts DROP CONSTRAINT vibe_attempts_max_cost_check;
ALTER TABLE vibe_attempts ADD CONSTRAINT vibe_attempts_max_cost_check CHECK(max_cost>0);
