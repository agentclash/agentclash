-- +goose Up
ALTER TABLE challenge_packs
ADD COLUMN workspace_id uuid REFERENCES workspaces (id) ON DELETE CASCADE;

ALTER TABLE challenge_packs
DROP CONSTRAINT IF EXISTS challenge_packs_slug_key;

CREATE UNIQUE INDEX challenge_packs_global_slug_uq
ON challenge_packs (slug)
WHERE workspace_id IS NULL;

CREATE UNIQUE INDEX challenge_packs_workspace_slug_uq
ON challenge_packs (workspace_id, slug)
WHERE workspace_id IS NOT NULL;

CREATE INDEX challenge_packs_workspace_id_idx
ON challenge_packs (workspace_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS challenge_packs_workspace_id_idx;
DROP INDEX IF EXISTS challenge_packs_workspace_slug_uq;
DROP INDEX IF EXISTS challenge_packs_global_slug_uq;

ALTER TABLE challenge_packs
ADD CONSTRAINT challenge_packs_slug_key UNIQUE (slug);

ALTER TABLE challenge_packs
DROP COLUMN IF EXISTS workspace_id;
