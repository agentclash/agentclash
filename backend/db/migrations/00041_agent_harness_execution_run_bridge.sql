-- +goose Up
ALTER TABLE runs
    ADD COLUMN source_type text NOT NULL DEFAULT 'eval_pack'
        CHECK (source_type IN ('eval_pack', 'agent_harness'));

ALTER TABLE runs
    ALTER COLUMN eval_pack_version_id DROP NOT NULL,
    ADD CONSTRAINT runs_source_shape_check
        CHECK (
            (source_type = 'eval_pack' AND eval_pack_version_id IS NOT NULL)
            OR
            (source_type = 'agent_harness' AND eval_pack_version_id IS NULL AND challenge_input_set_id IS NULL)
        );

ALTER TABLE run_agents
    ADD COLUMN source_type text NOT NULL DEFAULT 'agent_deployment'
        CHECK (source_type IN ('agent_deployment', 'agent_harness'));

ALTER TABLE run_agents
    ALTER COLUMN agent_deployment_id DROP NOT NULL,
    ALTER COLUMN agent_deployment_snapshot_id DROP NOT NULL,
    ADD CONSTRAINT run_agents_source_shape_check
        CHECK (
            (source_type = 'agent_deployment' AND agent_deployment_id IS NOT NULL AND agent_deployment_snapshot_id IS NOT NULL)
            OR
            (source_type = 'agent_harness' AND agent_deployment_id IS NULL AND agent_deployment_snapshot_id IS NULL)
        );

ALTER TABLE agent_harness_executions
    ADD COLUMN run_id uuid,
    ADD COLUMN run_agent_id uuid,
    ADD COLUMN evaluation_spec_id uuid,
    ADD CONSTRAINT agent_harness_executions_run_scope_fk
        FOREIGN KEY (run_id, organization_id, workspace_id)
        REFERENCES runs (id, organization_id, workspace_id),
    ADD CONSTRAINT agent_harness_executions_run_agent_fk
        FOREIGN KEY (run_agent_id, run_id)
        REFERENCES run_agents (id, run_id),
    ADD CONSTRAINT agent_harness_executions_evaluation_spec_fk
        FOREIGN KEY (evaluation_spec_id)
        REFERENCES evaluation_specs (id) ON DELETE SET NULL,
    ADD CONSTRAINT agent_harness_executions_run_agent_requires_run
        CHECK (run_agent_id IS NULL OR run_id IS NOT NULL);

CREATE UNIQUE INDEX agent_harness_executions_run_agent_unique
ON agent_harness_executions (run_agent_id)
WHERE run_agent_id IS NOT NULL;

CREATE INDEX agent_harness_executions_run_idx
ON agent_harness_executions (run_id)
WHERE run_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS agent_harness_executions_run_idx;
DROP INDEX IF EXISTS agent_harness_executions_run_agent_unique;

ALTER TABLE agent_harness_executions
    DROP CONSTRAINT IF EXISTS agent_harness_executions_run_agent_requires_run,
    DROP CONSTRAINT IF EXISTS agent_harness_executions_evaluation_spec_fk,
    DROP CONSTRAINT IF EXISTS agent_harness_executions_run_agent_fk,
    DROP CONSTRAINT IF EXISTS agent_harness_executions_run_scope_fk,
    DROP COLUMN IF EXISTS evaluation_spec_id,
    DROP COLUMN IF EXISTS run_agent_id,
    DROP COLUMN IF EXISTS run_id;

DELETE FROM runs
WHERE source_type = 'agent_harness';

ALTER TABLE run_agents
    DROP CONSTRAINT IF EXISTS run_agents_source_shape_check,
    ALTER COLUMN agent_deployment_snapshot_id SET NOT NULL,
    ALTER COLUMN agent_deployment_id SET NOT NULL,
    DROP COLUMN IF EXISTS source_type;

ALTER TABLE runs
    DROP CONSTRAINT IF EXISTS runs_source_shape_check,
    ALTER COLUMN eval_pack_version_id SET NOT NULL,
    DROP COLUMN IF EXISTS source_type;
