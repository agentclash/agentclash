-- +goose Up
-- Domain completion is distinct from provider completion and billing settlement.
-- The receipt is written in the same transaction as the document it describes.
ALTER TABLE vibe_operations ADD COLUMN completion_receipt jsonb;
ALTER TABLE vibe_attempts ADD COLUMN domain_outcome jsonb;

-- +goose Down
ALTER TABLE vibe_attempts DROP COLUMN domain_outcome;
ALTER TABLE vibe_operations DROP COLUMN completion_receipt;
