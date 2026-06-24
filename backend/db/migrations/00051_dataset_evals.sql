-- +goose Up
CREATE TABLE dataset_version_input_sets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id uuid NOT NULL REFERENCES datasets (id) ON DELETE CASCADE,
    dataset_version_id uuid NOT NULL REFERENCES dataset_versions (id) ON DELETE CASCADE,
    challenge_pack_version_id uuid NOT NULL REFERENCES challenge_pack_versions (id) ON DELETE CASCADE,
    challenge_identity_id uuid NOT NULL REFERENCES challenge_identities (id) ON DELETE RESTRICT,
    challenge_key text NOT NULL,
    challenge_input_set_id uuid NOT NULL,
    input_key text NOT NULL,
    input_checksum text NOT NULL,
    mapping jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (dataset_version_id, challenge_pack_version_id, challenge_key),
    UNIQUE (challenge_input_set_id),
    FOREIGN KEY (challenge_input_set_id, challenge_pack_version_id)
        REFERENCES challenge_input_sets (id, challenge_pack_version_id) ON DELETE CASCADE
);

CREATE TABLE dataset_input_item_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_version_input_set_id uuid NOT NULL REFERENCES dataset_version_input_sets (id) ON DELETE CASCADE,
    dataset_example_id uuid NOT NULL REFERENCES dataset_examples (id) ON DELETE CASCADE,
    challenge_input_item_id uuid NOT NULL REFERENCES challenge_input_items (id) ON DELETE CASCADE,
    item_key text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (dataset_version_input_set_id, dataset_example_id),
    UNIQUE (challenge_input_item_id)
);

CREATE TABLE dataset_eval_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id uuid NOT NULL REFERENCES datasets (id) ON DELETE CASCADE,
    dataset_version_id uuid NOT NULL REFERENCES dataset_versions (id) ON DELETE CASCADE,
    dataset_version_input_set_id uuid NOT NULL REFERENCES dataset_version_input_sets (id) ON DELETE CASCADE,
    run_id uuid NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id)
);

CREATE INDEX dataset_version_input_sets_dataset_idx
    ON dataset_version_input_sets (dataset_id, dataset_version_id);

CREATE INDEX dataset_input_item_links_example_idx
    ON dataset_input_item_links (dataset_example_id);

CREATE INDEX dataset_eval_runs_dataset_idx
    ON dataset_eval_runs (dataset_id, dataset_version_id, created_at DESC);

CREATE TRIGGER dataset_version_input_sets_set_updated_at
BEFORE UPDATE ON dataset_version_input_sets
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS dataset_version_input_sets_set_updated_at ON dataset_version_input_sets;
DROP INDEX IF EXISTS dataset_eval_runs_dataset_idx;
DROP INDEX IF EXISTS dataset_input_item_links_example_idx;
DROP INDEX IF EXISTS dataset_version_input_sets_dataset_idx;
DROP TABLE IF EXISTS dataset_eval_runs;
DROP TABLE IF EXISTS dataset_input_item_links;
DROP TABLE IF EXISTS dataset_version_input_sets;
