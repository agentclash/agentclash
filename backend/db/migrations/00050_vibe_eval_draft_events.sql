-- +goose Up
CREATE TABLE vibe_eval_draft_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    draft_id uuid NOT NULL,
    actor_user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    action text NOT NULL CHECK (action IN ('validate_challenge_pack', 'publish_challenge_pack')),
    payload_hash text NOT NULL,
    request_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    result_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (conversation_id, workspace_id) REFERENCES vibe_eval_conversations (id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (draft_id) REFERENCES vibe_eval_drafts (id) ON DELETE CASCADE
);

CREATE INDEX vibe_eval_draft_events_draft_idx
    ON vibe_eval_draft_events (draft_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS vibe_eval_draft_events;
