-- +goose Up
-- The input/request receipt stays immutable. A resolved conversation decision
-- is journaled separately so retries and repairs cannot change its action.
ALTER TABLE vibe_operations ADD COLUMN conversation_decision jsonb;

-- +goose Down
ALTER TABLE vibe_operations DROP COLUMN conversation_decision;
