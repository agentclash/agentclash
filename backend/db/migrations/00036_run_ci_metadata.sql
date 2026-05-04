-- +goose Up
ALTER TABLE runs
ADD COLUMN ci_metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE runs
DROP COLUMN IF EXISTS ci_metadata;
