-- +goose Up
-- Adds opt-in race-context mode to runs. When enabled, the executor injects
-- live peer standings into each agent's message stream during the run. See
-- issue #400 for design. Phase 1 of the feature lands these columns, the
-- event type, the API/CLI plumbing, and the backend injection path.
ALTER TABLE runs
ADD COLUMN race_context boolean NOT NULL DEFAULT false;

ALTER TABLE runs
ADD COLUMN race_context_min_step_gap integer
CHECK (race_context_min_step_gap IS NULL OR (race_context_min_step_gap BETWEEN 1 AND 10));

-- +goose Down
ALTER TABLE runs DROP COLUMN IF EXISTS race_context_min_step_gap;
ALTER TABLE runs DROP COLUMN IF EXISTS race_context;
