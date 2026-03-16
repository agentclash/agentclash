-- +goose Up
CREATE TABLE run_comparisons (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    baseline_run_id uuid NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    candidate_run_id uuid NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    baseline_run_agent_id uuid,
    candidate_run_agent_id uuid,
    status text NOT NULL CHECK (status IN ('comparable', 'not_comparable')),
    reason_code text,
    source_fingerprint text NOT NULL,
    summary jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (baseline_run_id <> candidate_run_id),
    UNIQUE (baseline_run_id, candidate_run_id)
);

ALTER TABLE run_comparisons
ADD CONSTRAINT run_comparisons_baseline_run_agent_fk
FOREIGN KEY (baseline_run_agent_id, baseline_run_id) REFERENCES run_agents (id, run_id) ON DELETE CASCADE;

ALTER TABLE run_comparisons
ADD CONSTRAINT run_comparisons_candidate_run_agent_fk
FOREIGN KEY (candidate_run_agent_id, candidate_run_id) REFERENCES run_agents (id, run_id) ON DELETE CASCADE;

CREATE TRIGGER run_comparisons_set_updated_at
BEFORE UPDATE ON run_comparisons
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS run_comparisons_set_updated_at ON run_comparisons;
DROP TABLE IF EXISTS run_comparisons;
