-- +goose Up
-- Evaluation sessions reuse the existing ownership, source and operation journals.
-- Their parent is a chat container, never a source of inherited requirements.
CREATE TABLE vibe_evaluation_contexts (
 chat_id uuid NOT NULL REFERENCES vibe_sessions(id) ON DELETE CASCADE,
 evaluation_id uuid PRIMARY KEY REFERENCES vibe_sessions(id) ON DELETE CASCADE,
 client_id uuid NOT NULL,
 door text NOT NULL CHECK (door IN ('build','test')),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(chat_id,client_id), CHECK(chat_id<>evaluation_id)
);
CREATE INDEX vibe_evaluation_contexts_chat_idx ON vibe_evaluation_contexts(chat_id,created_at);
CREATE TABLE vibe_cycle_quotes (
 id uuid PRIMARY KEY,
 session_id uuid NOT NULL REFERENCES vibe_sessions(id) ON DELETE CASCADE,
 request_hash text NOT NULL,
 specification jsonb NOT NULL,
 max_cost bigint NOT NULL CHECK(max_cost>=0),
 expires_at timestamptz NOT NULL,
 authorized_at timestamptz,
 stopped_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX vibe_cycle_quotes_session_idx ON vibe_cycle_quotes(session_id,created_at);
CREATE TABLE vibe_cycle_steps (
 cycle_id uuid NOT NULL REFERENCES vibe_cycle_quotes(id) ON DELETE CASCADE,
 step text NOT NULL CHECK(step IN ('prepare','answer','check') OR step LIKE 'retry:%'),
 operation_id uuid UNIQUE NOT NULL REFERENCES vibe_operations(id) ON DELETE CASCADE,
 PRIMARY KEY(cycle_id,step)
);
-- +goose Down
DROP TABLE IF EXISTS vibe_cycle_steps,vibe_cycle_quotes,vibe_evaluation_contexts;
