-- +goose Up
ALTER TABLE runs
ADD COLUMN official_pack_mode text NOT NULL DEFAULT 'full'
CHECK (official_pack_mode IN ('full', 'suite_only'));

CREATE TABLE run_case_selections (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id                uuid        NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    challenge_identity_id uuid        NOT NULL REFERENCES challenge_identities (id) ON DELETE RESTRICT,
    selection_origin      text        NOT NULL CHECK (selection_origin IN ('official', 'regression_suite', 'regression_case')),
    regression_case_id    uuid        REFERENCES workspace_regression_cases (id) ON DELETE SET NULL,
    selection_rank        integer     NOT NULL CHECK (selection_rank > 0),
    created_at            timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (selection_origin = 'official' AND regression_case_id IS NULL) OR
        (selection_origin IN ('regression_suite', 'regression_case') AND regression_case_id IS NOT NULL)
    )
);

CREATE INDEX run_case_selections_run_id_idx
    ON run_case_selections (run_id, selection_rank);

CREATE INDEX run_case_selections_identity_idx
    ON run_case_selections (run_id, challenge_identity_id);

ALTER TABLE judge_results
ADD COLUMN regression_case_id uuid REFERENCES workspace_regression_cases (id) ON DELETE SET NULL;

CREATE INDEX judge_results_regression_case_id_idx
    ON judge_results (regression_case_id);

ALTER TABLE metric_results
ADD COLUMN regression_case_id uuid REFERENCES workspace_regression_cases (id) ON DELETE SET NULL;

CREATE INDEX metric_results_regression_case_id_idx
    ON metric_results (regression_case_id);

-- +goose Down
DROP INDEX IF EXISTS metric_results_regression_case_id_idx;
ALTER TABLE metric_results DROP COLUMN IF EXISTS regression_case_id;

DROP INDEX IF EXISTS judge_results_regression_case_id_idx;
ALTER TABLE judge_results DROP COLUMN IF EXISTS regression_case_id;

DROP INDEX IF EXISTS run_case_selections_identity_idx;
DROP INDEX IF EXISTS run_case_selections_run_id_idx;
DROP TABLE IF EXISTS run_case_selections;

ALTER TABLE runs DROP COLUMN IF EXISTS official_pack_mode;
