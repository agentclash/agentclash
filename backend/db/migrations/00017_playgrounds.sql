-- +goose Up
CREATE TABLE playgrounds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    name text NOT NULL,
    prompt_template text NOT NULL,
    system_prompt text NOT NULL DEFAULT '',
    evaluation_spec jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    updated_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id, workspace_id),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE
);

CREATE TABLE playground_test_cases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    playground_id uuid NOT NULL REFERENCES playgrounds (id) ON DELETE CASCADE,
    case_key text NOT NULL,
    variables jsonb NOT NULL DEFAULT '{}'::jsonb,
    expectations jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (playground_id, case_key)
);

CREATE TABLE playground_experiments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    playground_id uuid NOT NULL,
    provider_account_id uuid NOT NULL REFERENCES provider_accounts (id) ON DELETE RESTRICT,
    model_alias_id uuid NOT NULL REFERENCES model_aliases (id) ON DELETE RESTRICT,
    name text NOT NULL,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    request_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    summary jsonb NOT NULL DEFAULT '{}'::jsonb,
    temporal_workflow_id text,
    temporal_run_id text,
    queued_at timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    failed_at timestamptz,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id, workspace_id),
    UNIQUE (temporal_workflow_id),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (playground_id, organization_id, workspace_id) REFERENCES playgrounds (id, organization_id, workspace_id) ON DELETE CASCADE
);

CREATE TABLE playground_experiment_results (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    playground_experiment_id uuid NOT NULL REFERENCES playground_experiments (id) ON DELETE CASCADE,
    playground_test_case_id uuid NOT NULL REFERENCES playground_test_cases (id) ON DELETE CASCADE,
    case_key text NOT NULL,
    status text NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'failed')),
    variables jsonb NOT NULL DEFAULT '{}'::jsonb,
    expectations jsonb NOT NULL DEFAULT '{}'::jsonb,
    rendered_prompt text NOT NULL DEFAULT '',
    actual_output text NOT NULL DEFAULT '',
    provider_key text NOT NULL DEFAULT '',
    provider_model_id text NOT NULL DEFAULT '',
    input_tokens bigint NOT NULL DEFAULT 0,
    output_tokens bigint NOT NULL DEFAULT 0,
    total_tokens bigint NOT NULL DEFAULT 0,
    latency_ms bigint NOT NULL DEFAULT 0,
    cost_usd numeric(12, 6) NOT NULL DEFAULT 0,
    validator_results jsonb NOT NULL DEFAULT '[]'::jsonb,
    llm_judge_results jsonb NOT NULL DEFAULT '[]'::jsonb,
    dimension_results jsonb NOT NULL DEFAULT '[]'::jsonb,
    dimension_scores jsonb NOT NULL DEFAULT '{}'::jsonb,
    warnings jsonb NOT NULL DEFAULT '[]'::jsonb,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (playground_experiment_id, playground_test_case_id)
);

CREATE INDEX playgrounds_workspace_id_idx
ON playgrounds (workspace_id, updated_at DESC);

CREATE INDEX playground_test_cases_playground_id_idx
ON playground_test_cases (playground_id, case_key);

CREATE INDEX playground_experiments_playground_id_idx
ON playground_experiments (playground_id, created_at DESC);

CREATE INDEX playground_experiment_results_experiment_id_idx
ON playground_experiment_results (playground_experiment_id, case_key);

CREATE TRIGGER playgrounds_set_updated_at
BEFORE UPDATE ON playgrounds
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER playground_test_cases_set_updated_at
BEFORE UPDATE ON playground_test_cases
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER playground_experiments_set_updated_at
BEFORE UPDATE ON playground_experiments
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER playground_experiment_results_set_updated_at
BEFORE UPDATE ON playground_experiment_results
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS playground_experiment_results_set_updated_at ON playground_experiment_results;
DROP TRIGGER IF EXISTS playground_experiments_set_updated_at ON playground_experiments;
DROP TRIGGER IF EXISTS playground_test_cases_set_updated_at ON playground_test_cases;
DROP TRIGGER IF EXISTS playgrounds_set_updated_at ON playgrounds;

DROP INDEX IF EXISTS playground_experiment_results_experiment_id_idx;
DROP INDEX IF EXISTS playground_experiments_playground_id_idx;
DROP INDEX IF EXISTS playground_test_cases_playground_id_idx;
DROP INDEX IF EXISTS playgrounds_workspace_id_idx;

DROP TABLE IF EXISTS playground_experiment_results;
DROP TABLE IF EXISTS playground_experiments;
DROP TABLE IF EXISTS playground_test_cases;
DROP TABLE IF EXISTS playgrounds;
