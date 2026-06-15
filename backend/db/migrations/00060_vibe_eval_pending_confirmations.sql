-- +goose Up
-- Vibe Eval pending confirmations (Step 3, #875 §5.3/§5.4). The propose→confirm half of the
-- confirmation engine: a workspace_write+ tool proposes an action, the turn ends, and the user
-- resolves it later via POST .../confirmations/{id}, which streams a continuation turn.
--
-- payload_hash binds the confirmation to EXACTLY the args shown to the user (sha256 of
-- canonical-JSON); resolve echoes it, and a mismatch is rejected (anti bait-and-switch).
-- bound_args is the verbatim validated args to execute on approve (internal state, not audit).
--
-- Crash-safe status machine (single-use): pending → executing → succeeded | failed (approve
-- path), pending → denied (deny path), pending → expired (lapsed). The atomic resolve claims the
-- row (pending → executing RETURNING) so two concurrent POSTs can't both execute; an `executing`
-- row left by a crash lets a retry re-enter effect execution idempotently.
CREATE TABLE vibe_eval_pending_confirmations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    message_id uuid, -- the assistant tool-call message that proposed this (for resume pairing)
    proposed_by_user_id uuid NOT NULL, -- the authenticated Caller who proposed (provenance; NEVER the model)
    tool_name text NOT NULL,
    tool_call_id text NOT NULL DEFAULT '', -- provider tool_use id, to pair the resumed tool-result
    action text NOT NULL, -- api.Action string, validated by the api adapter on resolve
    risk_tier text NOT NULL CHECK (risk_tier IN ('workspace_write', 'cost_incurring', 'admin_sensitive', 'destructive_external')),
    payload_hash text NOT NULL, -- sha256 of canonical-JSON(tool, normalized args); echoed on resolve
    bound_args jsonb NOT NULL DEFAULT '{}'::jsonb, -- exact validated args to execute on approve
    summary text NOT NULL DEFAULT '', -- human-readable confirmation card body
    estimate jsonb, -- Step 4: CostEstimate for cost_incurring tools (nullable)
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'executing', 'succeeded', 'failed', 'denied', 'expired')),
    resolved_by_user_id uuid, -- who approved/denied (the authenticated Caller)
    resolved_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (conversation_id, workspace_id) REFERENCES vibe_eval_conversations (id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (message_id) REFERENCES vibe_eval_messages (id) ON DELETE CASCADE,
    FOREIGN KEY (proposed_by_user_id) REFERENCES users (id) ON DELETE RESTRICT,
    FOREIGN KEY (resolved_by_user_id) REFERENCES users (id) ON DELETE RESTRICT
);

-- At most one ACTIVE confirmation per (conversation, tool, payload_hash): blocks duplicate
-- proposals while one is in flight, but allows a fresh attempt after a terminal state. Expiry is
-- an explicit status transition (pending -> expired), NOT a time predicate, precisely so a lapsed
-- row leaves this partial index instead of blocking re-proposal forever (partial indexes can't
-- use now()). The create path expires stale rows in-tx before inserting.
CREATE UNIQUE INDEX vibe_eval_pending_confirmations_active_idx
    ON vibe_eval_pending_confirmations (conversation_id, tool_name, payload_hash)
    WHERE status IN ('pending', 'executing');

CREATE INDEX vibe_eval_pending_confirmations_conversation_idx
    ON vibe_eval_pending_confirmations (conversation_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS vibe_eval_pending_confirmations;
