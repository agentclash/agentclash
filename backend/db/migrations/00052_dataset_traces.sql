-- +goose Up
CREATE TABLE dataset_trace_imports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id uuid NOT NULL REFERENCES datasets (id) ON DELETE CASCADE,
    source_platform text NOT NULL,
    artifact_id uuid REFERENCES artifacts (id) ON DELETE SET NULL,
    candidate_count integer NOT NULL DEFAULT 0 CHECK (candidate_count >= 0),
    status text NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'failed')),
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX dataset_trace_imports_dataset_id_created_at_idx
    ON dataset_trace_imports (dataset_id, created_at DESC);

CREATE TABLE dataset_trace_candidates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id uuid NOT NULL REFERENCES datasets (id) ON DELETE CASCADE,
    import_id uuid NOT NULL REFERENCES dataset_trace_imports (id) ON DELETE CASCADE,
    source_platform text NOT NULL,
    source_trace_id text,
    source_run_id uuid REFERENCES runs (id) ON DELETE SET NULL,
    source_run_agent_id uuid REFERENCES run_agents (id) ON DELETE SET NULL,
    external_id text,
    input jsonb NOT NULL,
    output jsonb,
    expected jsonb,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    tags text[] NOT NULL DEFAULT '{}'::text[],
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'promoted', 'dismissed')),
    promoted_example_id uuid REFERENCES dataset_examples (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX dataset_trace_candidates_dataset_id_status_idx
    ON dataset_trace_candidates (dataset_id, status, created_at DESC);

CREATE INDEX dataset_trace_candidates_import_id_idx
    ON dataset_trace_candidates (import_id);

CREATE UNIQUE INDEX dataset_trace_candidates_dataset_promotion_source_uq
    ON dataset_trace_candidates (dataset_id, source_platform, source_trace_id, source_run_id, source_run_agent_id)
    WHERE source_trace_id IS NOT NULL;

CREATE TRIGGER dataset_trace_candidates_set_updated_at
BEFORE UPDATE ON dataset_trace_candidates
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS dataset_trace_candidates_set_updated_at ON dataset_trace_candidates;
DROP INDEX IF EXISTS dataset_trace_candidates_dataset_promotion_source_uq;
DROP INDEX IF EXISTS dataset_trace_candidates_import_id_idx;
DROP INDEX IF EXISTS dataset_trace_candidates_dataset_id_status_idx;
DROP TABLE IF EXISTS dataset_trace_candidates;
DROP INDEX IF EXISTS dataset_trace_imports_dataset_id_created_at_idx;
DROP TABLE IF EXISTS dataset_trace_imports;
