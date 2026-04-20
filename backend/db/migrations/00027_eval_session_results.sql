-- +goose Up
CREATE TABLE eval_session_results (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    eval_session_id uuid NOT NULL UNIQUE REFERENCES eval_sessions (id) ON DELETE CASCADE,
    schema_version integer NOT NULL CHECK (schema_version >= 1),
    child_run_count integer NOT NULL CHECK (child_run_count >= 0),
    scored_child_count integer NOT NULL CHECK (scored_child_count >= 0),
    aggregate jsonb NOT NULL DEFAULT '{}'::jsonb,
    evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
    computed_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX eval_session_results_computed_at_idx
ON eval_session_results (computed_at DESC);

CREATE TRIGGER eval_session_results_set_updated_at
BEFORE UPDATE ON eval_session_results
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS eval_session_results_set_updated_at ON eval_session_results;
DROP INDEX IF EXISTS eval_session_results_computed_at_idx;
DROP TABLE IF EXISTS eval_session_results;
