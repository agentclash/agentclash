-- +goose Up
-- Vibe Eval tool-invocation audit log (Step 3 of the guide agent, #875 §6). Generalizes the
-- never-merged Phase 2 `vibe_eval_draft_events` (validate/publish only) into one append-only row
-- per draft+ tool call, regardless of outcome. read-tier calls are audited only when they touch
-- sensitive evidence (Phase 0 rule). Metadata and hashes only — NEVER secret values, raw
-- artifact contents, or provider keys.
CREATE TABLE vibe_eval_tool_invocations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    message_id uuid, -- the assistant tool-call message this records (nullable; SET NULL if pruned)
    actor_user_id uuid NOT NULL, -- the authenticated Caller, NEVER the model
    tool_name text NOT NULL,
    action text NOT NULL, -- api.Action string, validated by the api adapter
    risk_tier text NOT NULL CHECK (risk_tier IN ('read', 'draft', 'workspace_write', 'cost_incurring', 'admin_sensitive', 'destructive_external')),
    payload_hash text NOT NULL DEFAULT '', -- sha256 of canonical-JSON(tool, normalized args)
    -- Soft reference to vibe_eval_pending_confirmations.id (no FK): the audit log is append-only
    -- and must outlive transient/expired confirmation rows, so it must not be nulled or cascaded
    -- when a confirmation row is cleaned up.
    confirmation_id uuid,
    request_payload jsonb NOT NULL DEFAULT '{}'::jsonb, -- validated args, audit-scrubbed (metadata only)
    result_payload jsonb NOT NULL DEFAULT '{}'::jsonb, -- outcome metadata, audit-scrubbed (ids/hashes/state/counts)
    credit_reservation_id uuid, -- Step 4: set for cost_incurring tools (no FK yet; wallet tables land in Step 4)
    outcome text NOT NULL CHECK (outcome IN ('ok', 'denied', 'error', 'confirmation_required')),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (conversation_id, workspace_id) REFERENCES vibe_eval_conversations (id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (message_id) REFERENCES vibe_eval_messages (id) ON DELETE SET NULL,
    FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT
);

CREATE INDEX vibe_eval_tool_invocations_conversation_idx
    ON vibe_eval_tool_invocations (conversation_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS vibe_eval_tool_invocations;
