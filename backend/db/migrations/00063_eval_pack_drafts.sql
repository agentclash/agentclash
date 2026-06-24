-- +goose Up
-- eval_pack_drafts holds an in-progress, resumable pack composition for
-- the visual builder. `composition` is the builder's working document: pack +
-- version metadata, ordered references to challenge_pieces (by id) or inline
-- piece definitions, and the per-pack scorecard wiring (dimensions referencing
-- validator/judge piece keys). It is NOT a runnable manifest; on publish the
-- composition is resolved + snapshotted into eval_pack_versions via the
-- existing publish path. eval_pack_id is set when a draft edits an
-- already-published pack (its publishes become new versions of that pack).
CREATE TABLE eval_pack_drafts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name text NOT NULL,
    execution_mode text NOT NULL DEFAULT 'native' CHECK (execution_mode IN ('native', 'prompt_eval', 'responses', 'multi_turn')),
    eval_pack_id uuid REFERENCES eval_packs (id) ON DELETE SET NULL,
    composition jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'discarded')),
    last_published_version_id uuid REFERENCES eval_pack_versions (id) ON DELETE SET NULL,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX eval_pack_drafts_workspace_idx
    ON eval_pack_drafts (workspace_id, updated_at DESC)
    WHERE status = 'draft';

CREATE TRIGGER eval_pack_drafts_set_updated_at
BEFORE UPDATE ON eval_pack_drafts
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS eval_pack_drafts_set_updated_at ON eval_pack_drafts;
DROP INDEX IF EXISTS eval_pack_drafts_workspace_idx;
DROP TABLE IF EXISTS eval_pack_drafts;
