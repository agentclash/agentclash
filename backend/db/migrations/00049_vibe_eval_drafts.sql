-- +goose Up
CREATE TABLE vibe_eval_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    created_by_user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    title text NOT NULL,
    phase text NOT NULL DEFAULT 'plan' CHECK (phase IN ('plan', 'author', 'validate', 'publish', 'run', 'analyze', 'regress', 'admin')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    active_draft_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, workspace_id),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE
);

CREATE TABLE vibe_eval_drafts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    draft_kind text NOT NULL CHECK (draft_kind IN ('eval_plan', 'eval_pack', 'input_cases', 'scoring', 'runtime')),
    content jsonb NOT NULL DEFAULT '{}'::jsonb,
    validation_state text NOT NULL DEFAULT 'unknown' CHECK (validation_state IN ('unknown', 'valid', 'invalid')),
    validation_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    published_eval_pack_id uuid,
    published_eval_pack_version_id uuid,
    created_by_user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    updated_by_user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (conversation_id, workspace_id) REFERENCES vibe_eval_conversations (id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (published_eval_pack_id) REFERENCES eval_packs (id) ON DELETE SET NULL,
    FOREIGN KEY (published_eval_pack_version_id) REFERENCES eval_pack_versions (id) ON DELETE SET NULL
);

ALTER TABLE vibe_eval_conversations
    ADD CONSTRAINT vibe_eval_conversations_active_draft_fk
    FOREIGN KEY (active_draft_id) REFERENCES vibe_eval_drafts (id) ON DELETE SET NULL;

CREATE INDEX vibe_eval_conversations_workspace_idx
    ON vibe_eval_conversations (workspace_id, updated_at DESC)
    WHERE archived_at IS NULL;

CREATE INDEX vibe_eval_drafts_conversation_idx
    ON vibe_eval_drafts (conversation_id, updated_at DESC);

CREATE TRIGGER vibe_eval_conversations_set_updated_at
BEFORE UPDATE ON vibe_eval_conversations
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER vibe_eval_drafts_set_updated_at
BEFORE UPDATE ON vibe_eval_drafts
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS vibe_eval_drafts_set_updated_at ON vibe_eval_drafts;
DROP TRIGGER IF EXISTS vibe_eval_conversations_set_updated_at ON vibe_eval_conversations;
ALTER TABLE vibe_eval_conversations
    DROP CONSTRAINT IF EXISTS vibe_eval_conversations_active_draft_fk;
DROP TABLE IF EXISTS vibe_eval_drafts;
DROP TABLE IF EXISTS vibe_eval_conversations;
