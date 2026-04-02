-- +goose Up
CREATE TABLE run_comparison_release_gates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_comparison_id uuid NOT NULL REFERENCES run_comparisons (id) ON DELETE CASCADE,
    policy_key text NOT NULL,
    policy_version integer NOT NULL,
    policy_fingerprint text NOT NULL,
    policy_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    verdict text NOT NULL CHECK (verdict IN ('pass', 'warn', 'fail', 'insufficient_evidence')),
    reason_code text NOT NULL,
    summary text NOT NULL,
    evidence_status text NOT NULL CHECK (evidence_status IN ('sufficient', 'insufficient')),
    evaluation_details jsonb NOT NULL DEFAULT '{}'::jsonb,
    source_fingerprint text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_comparison_id, policy_key, policy_version, policy_fingerprint)
);

CREATE INDEX run_comparison_release_gates_run_comparison_idx
    ON run_comparison_release_gates (run_comparison_id, updated_at DESC);

CREATE TRIGGER run_comparison_release_gates_set_updated_at
BEFORE UPDATE ON run_comparison_release_gates
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS run_comparison_release_gates_set_updated_at ON run_comparison_release_gates;
DROP TABLE IF EXISTS run_comparison_release_gates;
