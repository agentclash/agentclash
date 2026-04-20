-- +goose Up
CREATE TABLE eval_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'aggregating', 'completed', 'failed', 'cancelled')),
    repetitions integer NOT NULL CHECK (repetitions >= 1),
    aggregation_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    success_threshold_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    routing_task_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    schema_version integer NOT NULL CHECK (schema_version >= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX eval_sessions_status_created_at_idx ON eval_sessions (status, created_at DESC);
CREATE INDEX eval_sessions_created_at_idx ON eval_sessions (created_at DESC);

ALTER TABLE runs
ADD COLUMN eval_session_id uuid REFERENCES eval_sessions (id) ON DELETE SET NULL;

CREATE INDEX runs_eval_session_id_created_at_idx ON runs (eval_session_id, created_at ASC, id ASC)
WHERE eval_session_id IS NOT NULL;

CREATE TRIGGER eval_sessions_set_updated_at
BEFORE UPDATE ON eval_sessions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS eval_sessions_set_updated_at ON eval_sessions;

DROP INDEX IF EXISTS runs_eval_session_id_created_at_idx;

ALTER TABLE runs
DROP COLUMN IF EXISTS eval_session_id;

DROP INDEX IF EXISTS eval_sessions_created_at_idx;
DROP INDEX IF EXISTS eval_sessions_status_created_at_idx;

DROP TABLE IF EXISTS eval_sessions;
