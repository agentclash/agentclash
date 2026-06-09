-- +goose Up

-- Track rerun lineage: a tryout created by "rerun with another model" points back
-- to the tryout it was derived from. Nullable for original (non-rerun) tryouts.
ALTER TABLE agent_tryouts
ADD COLUMN parent_tryout_id uuid REFERENCES agent_tryouts (id) ON DELETE SET NULL;

CREATE INDEX idx_agent_tryouts_parent_tryout_id
ON agent_tryouts (parent_tryout_id)
WHERE parent_tryout_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_agent_tryouts_parent_tryout_id;

ALTER TABLE agent_tryouts
DROP COLUMN IF EXISTS parent_tryout_id;
