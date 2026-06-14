-- +goose Up
-- Vibe Eval conversation transcript (Step 2 of the guide agent, #875 §4.4 / §11.6).
-- Append-only; deleted on conversation/workspace/org delete (cascade); no TTL in v1.
CREATE TABLE vibe_eval_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    seq bigint NOT NULL, -- monotonic per conversation
    role text NOT NULL CHECK (role IN ('user', 'assistant', 'tool')),
    content text NOT NULL DEFAULT '',
    -- user/assistant: verbatim minus narrow secret scrub; tool: redacted evidence only (§11.6).
    redaction_state text NOT NULL DEFAULT 'none' CHECK (redaction_state IN ('none', 'applied', 'not_applicable')),
    tool_call_id text NOT NULL DEFAULT '', -- links a tool result to the assistant tool call
    tool_name text NOT NULL DEFAULT '',
    tool_args jsonb NOT NULL DEFAULT '{}'::jsonb, -- validated args for a single assistant tool-call (legacy/unused for multi-call)
    tool_calls jsonb NOT NULL DEFAULT '[]'::jsonb, -- assistant rows: the full provider tool-call array, so cross-turn replay preserves tool_use/tool_result pairing
    usage jsonb NOT NULL DEFAULT '{}'::jsonb, -- token usage on assistant rows (observational, §11.3)
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (conversation_id, seq),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (conversation_id, workspace_id) REFERENCES vibe_eval_conversations (id, workspace_id) ON DELETE CASCADE
);

CREATE INDEX vibe_eval_messages_conversation_idx
    ON vibe_eval_messages (conversation_id, seq);

-- +goose Down
DROP TABLE IF EXISTS vibe_eval_messages;
