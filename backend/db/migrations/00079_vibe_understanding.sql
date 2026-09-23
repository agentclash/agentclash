-- +goose Up
-- Observed understanding signals are never conversation facts or test policy.
-- Freeze the outcome before routing, including the decision to use the fallback.
ALTER TABLE vibe_operations ADD COLUMN understanding_outcome jsonb;

-- +goose Down
ALTER TABLE vibe_operations DROP COLUMN understanding_outcome;
