-- +goose Up
ALTER TABLE vibe_attempts ADD COLUMN reconciliation_evidence jsonb;
ALTER TABLE vibe_attempts ADD CONSTRAINT vibe_reconciliation_evidence_size CHECK (reconciliation_evidence IS NULL OR octet_length(reconciliation_evidence::text)<=262144);
-- +goose Down
ALTER TABLE vibe_attempts DROP COLUMN reconciliation_evidence;
