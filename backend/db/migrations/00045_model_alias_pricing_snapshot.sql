-- +goose Up

ALTER TABLE model_aliases
ADD COLUMN input_cost_per_million_tokens numeric(18,6),
ADD COLUMN output_cost_per_million_tokens numeric(18,6);

UPDATE model_aliases ma
SET input_cost_per_million_tokens = mce.input_cost_per_million_tokens,
    output_cost_per_million_tokens = mce.output_cost_per_million_tokens
FROM model_catalog_entries mce
WHERE ma.model_catalog_entry_id = mce.id;

ALTER TABLE model_aliases
ALTER COLUMN input_cost_per_million_tokens SET DEFAULT 0,
ALTER COLUMN input_cost_per_million_tokens SET NOT NULL,
ALTER COLUMN output_cost_per_million_tokens SET DEFAULT 0,
ALTER COLUMN output_cost_per_million_tokens SET NOT NULL;

-- +goose Down

ALTER TABLE model_aliases
DROP COLUMN IF EXISTS output_cost_per_million_tokens,
DROP COLUMN IF EXISTS input_cost_per_million_tokens;
