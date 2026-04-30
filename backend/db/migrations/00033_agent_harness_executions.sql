-- +goose Up
CREATE TABLE agent_harness_executions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    agent_harness_id uuid NOT NULL,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN (
        'queued',
        'provisioning',
        'running',
        'scoring',
        'completed',
        'failed',
        'cancelled'
    )),
    harness_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    execution_config_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    evaluation_config_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    error_message text,
    started_at timestamptz,
    completed_at timestamptz,
    cancelled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id, workspace_id),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (agent_harness_id, organization_id, workspace_id) REFERENCES agent_harnesses (id, organization_id, workspace_id) ON DELETE CASCADE
);

CREATE INDEX agent_harness_executions_workspace_created_idx
ON agent_harness_executions (workspace_id, created_at DESC, id DESC);

CREATE INDEX agent_harness_executions_harness_created_idx
ON agent_harness_executions (agent_harness_id, created_at DESC, id DESC);

CREATE INDEX agent_harness_executions_workspace_status_idx
ON agent_harness_executions (workspace_id, status, created_at DESC);

CREATE TRIGGER agent_harness_executions_set_updated_at
BEFORE UPDATE ON agent_harness_executions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS agent_harness_executions_set_updated_at ON agent_harness_executions;
DROP INDEX IF EXISTS agent_harness_executions_workspace_status_idx;
DROP INDEX IF EXISTS agent_harness_executions_harness_created_idx;
DROP INDEX IF EXISTS agent_harness_executions_workspace_created_idx;
DROP TABLE IF EXISTS agent_harness_executions;
