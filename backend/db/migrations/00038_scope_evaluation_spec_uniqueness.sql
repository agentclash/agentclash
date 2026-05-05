-- +goose Up
ALTER TABLE evaluation_specs
DROP CONSTRAINT IF EXISTS evaluation_specs_name_version_number_key;

CREATE UNIQUE INDEX IF NOT EXISTS evaluation_specs_challenge_pack_version_name_version_uq
ON evaluation_specs (challenge_pack_version_id, name, version_number)
WHERE challenge_pack_version_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS evaluation_specs_global_name_version_uq
ON evaluation_specs (name, version_number)
WHERE challenge_pack_version_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS evaluation_specs_global_name_version_uq;
DROP INDEX IF EXISTS evaluation_specs_challenge_pack_version_name_version_uq;

-- This down migration can fail if duplicate name/version pairs were written
-- for different challenge pack versions after the Up migration ran.
ALTER TABLE evaluation_specs
ADD CONSTRAINT evaluation_specs_name_version_number_key UNIQUE (name, version_number);
