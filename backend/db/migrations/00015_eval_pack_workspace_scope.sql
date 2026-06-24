-- +goose Up
ALTER TABLE eval_packs
ADD COLUMN workspace_id uuid REFERENCES workspaces (id) ON DELETE CASCADE;

ALTER TABLE eval_packs
DROP CONSTRAINT IF EXISTS eval_packs_slug_key;

CREATE UNIQUE INDEX eval_packs_global_slug_uq
ON eval_packs (slug)
WHERE workspace_id IS NULL;

CREATE UNIQUE INDEX eval_packs_workspace_slug_uq
ON eval_packs (workspace_id, slug)
WHERE workspace_id IS NOT NULL;

CREATE INDEX eval_packs_workspace_id_idx
ON eval_packs (workspace_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS eval_packs_workspace_id_idx;
DROP INDEX IF EXISTS eval_packs_workspace_slug_uq;
DROP INDEX IF EXISTS eval_packs_global_slug_uq;

ALTER TABLE eval_packs
ADD CONSTRAINT eval_packs_slug_key UNIQUE (slug);

ALTER TABLE eval_packs
DROP COLUMN IF EXISTS workspace_id;
