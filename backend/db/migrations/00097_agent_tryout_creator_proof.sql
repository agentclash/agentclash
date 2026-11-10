-- +goose Up
ALTER TABLE agent_tryouts ADD COLUMN anonymous_owner_proof_hash text;
ALTER TABLE agent_tryouts ADD CONSTRAINT agent_tryout_owner_proof_hash_format
    CHECK (anonymous_owner_proof_hash IS NULL OR anonymous_owner_proof_hash ~ '^[0-9a-f]{64}$');

-- +goose Down
ALTER TABLE agent_tryouts DROP COLUMN anonymous_owner_proof_hash;
