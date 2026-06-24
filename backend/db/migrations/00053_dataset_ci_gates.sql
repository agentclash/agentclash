-- +goose Up
CREATE TABLE dataset_baselines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id uuid NOT NULL REFERENCES datasets (id) ON DELETE CASCADE,
    dataset_version_id uuid NOT NULL REFERENCES dataset_versions (id) ON DELETE CASCADE,
    dataset_version_input_set_id uuid REFERENCES dataset_version_input_sets (id) ON DELETE SET NULL,
    eval_pack_version_id uuid NOT NULL REFERENCES eval_pack_versions (id) ON DELETE RESTRICT,
    challenge_key text NOT NULL,
    agent_deployment_id uuid,
    run_id uuid NOT NULL REFERENCES runs (id) ON DELETE RESTRICT,
    pass_rate numeric(7, 4),
    metrics jsonb NOT NULL DEFAULT '{}'::jsonb,
    example_outcomes jsonb NOT NULL DEFAULT '[]'::jsonb,
    label text,
    created_by_user_id uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX dataset_baselines_dataset_idx
    ON dataset_baselines (dataset_id, created_at DESC);

CREATE TABLE dataset_regression_suite_links (
    dataset_id uuid PRIMARY KEY REFERENCES datasets (id) ON DELETE CASCADE,
    regression_suite_id uuid NOT NULL REFERENCES workspace_regression_suites (id) ON DELETE CASCADE,
    synced_version_id uuid REFERENCES dataset_versions (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER dataset_regression_suite_links_set_updated_at
BEFORE UPDATE ON dataset_regression_suite_links
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS dataset_regression_suite_links_set_updated_at ON dataset_regression_suite_links;
DROP TABLE IF EXISTS dataset_regression_suite_links;
DROP INDEX IF EXISTS dataset_baselines_dataset_idx;
DROP TABLE IF EXISTS dataset_baselines;
