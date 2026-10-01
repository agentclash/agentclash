-- +goose Up
CREATE TABLE vibe_inputs (
 id uuid PRIMARY KEY, session_id uuid NOT NULL REFERENCES vibe_sessions(id),
 client_id uuid NOT NULL, request_hash text NOT NULL, kind text NOT NULL CHECK(kind IN ('text','pdf')),
 name text NOT NULL, object_key text NOT NULL DEFAULT '', source_hash text NOT NULL,
 content_hash text NOT NULL DEFAULT '', size_bytes bigint NOT NULL,
 status text NOT NULL CHECK(status IN ('uploaded','extracting','ready','unreadable','failed','deleted','expired')),
 pages jsonb NOT NULL DEFAULT '[]', warnings jsonb NOT NULL DEFAULT '[]', parser_version text NOT NULL DEFAULT '',
 error text NOT NULL DEFAULT '', attempts integer NOT NULL DEFAULT 0, lease_until timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz, deleted_at timestamptz,
 UNIQUE(session_id,client_id)
);
CREATE INDEX vibe_inputs_pending ON vibe_inputs(status,lease_until);
CREATE INDEX vibe_inputs_expiry ON vibe_inputs(expires_at) WHERE expires_at IS NOT NULL;
CREATE TABLE vibe_input_workers (id uuid PRIMARY KEY, expires_at timestamptz NOT NULL);
CREATE TABLE vibe_input_staging (object_key text PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT now());
ALTER TABLE vibe_cycle_steps DROP CONSTRAINT vibe_cycle_steps_step_check;
ALTER TABLE vibe_cycle_steps ADD CONSTRAINT vibe_cycle_steps_step_check CHECK(step IN ('prepare','answer','check','initial_trial') OR step LIKE 'retry:%' OR step LIKE 'message:%');
-- +goose Down
DROP TABLE vibe_input_workers;
DROP TABLE vibe_input_staging;
ALTER TABLE vibe_cycle_steps DROP CONSTRAINT vibe_cycle_steps_step_check;
ALTER TABLE vibe_cycle_steps ADD CONSTRAINT vibe_cycle_steps_step_check CHECK(step IN ('prepare','answer','check') OR step LIKE 'retry:%' OR step LIKE 'message:%');
DROP TABLE vibe_inputs;
