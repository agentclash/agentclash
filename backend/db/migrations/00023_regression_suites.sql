-- +goose Up
CREATE TABLE workspace_regression_suites (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    source_challenge_pack_id uuid     NOT NULL REFERENCES challenge_packs (id) ON DELETE RESTRICT,
    name                  text        NOT NULL,
    description           text        NOT NULL DEFAULT '',
    status                text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    source_mode           text        NOT NULL DEFAULT 'derived_only' CHECK (source_mode IN ('derived_only', 'mixed_manual')),
    default_gate_severity text        NOT NULL DEFAULT 'warning' CHECK (default_gate_severity IN ('info', 'warning', 'blocking')),
    created_by_user_id    uuid        NOT NULL REFERENCES users (id),
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX workspace_regression_suites_workspace_name_active_idx
    ON workspace_regression_suites (workspace_id, name)
    WHERE status = 'active';

CREATE TABLE workspace_regression_cases (
    id                               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    suite_id                         uuid        NOT NULL REFERENCES workspace_regression_suites (id) ON DELETE CASCADE,
    title                            text        NOT NULL,
    description                      text        NOT NULL DEFAULT '',
    status                           text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived', 'muted')),
    severity                         text        NOT NULL CHECK (severity IN ('info', 'warning', 'blocking')),
    promotion_mode                   text        NOT NULL CHECK (promotion_mode IN ('full_executable', 'manual', 'output_only')),
    source_run_id                    uuid        REFERENCES runs (id) ON DELETE SET NULL,
    source_run_agent_id              uuid        REFERENCES run_agents (id) ON DELETE SET NULL,
    source_replay_id                 uuid        REFERENCES run_agent_replays (id) ON DELETE SET NULL,
    source_challenge_pack_version_id uuid        NOT NULL REFERENCES challenge_pack_versions (id) ON DELETE RESTRICT,
    source_challenge_input_set_id    uuid        REFERENCES challenge_input_sets (id) ON DELETE SET NULL,
    source_challenge_identity_id     uuid        NOT NULL REFERENCES challenge_identities (id) ON DELETE RESTRICT,
    source_case_key                  text        NOT NULL,
    source_item_key                  text,
    evidence_tier                    text        NOT NULL,
    failure_class                    text        NOT NULL,
    failure_summary                  text        NOT NULL DEFAULT '',
    payload_snapshot                 jsonb       NOT NULL DEFAULT '{}'::jsonb,
    expected_contract                jsonb       NOT NULL DEFAULT '{}'::jsonb,
    validator_overrides              jsonb,
    metadata                         jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at                       timestamptz NOT NULL DEFAULT now(),
    updated_at                       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX workspace_regression_cases_suite_id_status_idx
    ON workspace_regression_cases (suite_id, status);

CREATE INDEX workspace_regression_cases_source_challenge_pack_version_idx
    ON workspace_regression_cases (source_challenge_pack_version_id);

CREATE INDEX workspace_regression_cases_source_run_id_idx
    ON workspace_regression_cases (source_run_id);

CREATE TABLE workspace_regression_promotions (
    id                          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_regression_case_id uuid       NOT NULL REFERENCES workspace_regression_cases (id) ON DELETE CASCADE,
    source_run_id               uuid        NOT NULL REFERENCES runs (id) ON DELETE RESTRICT,
    source_run_agent_id         uuid        NOT NULL REFERENCES run_agents (id) ON DELETE RESTRICT,
    source_event_refs           jsonb       NOT NULL DEFAULT '[]'::jsonb,
    promoted_by_user_id         uuid        NOT NULL REFERENCES users (id),
    promotion_reason            text        NOT NULL DEFAULT '',
    promotion_snapshot          jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at                  timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS workspace_regression_promotions;
DROP INDEX IF EXISTS workspace_regression_cases_source_run_id_idx;
DROP INDEX IF EXISTS workspace_regression_cases_source_challenge_pack_version_idx;
DROP INDEX IF EXISTS workspace_regression_cases_suite_id_status_idx;
DROP TABLE IF EXISTS workspace_regression_cases;
DROP INDEX IF EXISTS workspace_regression_suites_workspace_name_active_idx;
DROP TABLE IF EXISTS workspace_regression_suites;
