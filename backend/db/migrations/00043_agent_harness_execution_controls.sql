-- +goose Up
ALTER TABLE agent_harness_executions
ADD COLUMN temporal_workflow_id text,
ADD COLUMN temporal_run_id text,
ADD COLUMN retry_of_execution_id uuid REFERENCES agent_harness_executions (id) ON DELETE SET NULL,
ADD COLUMN retry_idempotency_key text;

CREATE UNIQUE INDEX agent_harness_executions_retry_idempotency_idx
ON agent_harness_executions (workspace_id, retry_of_execution_id, retry_idempotency_key)
WHERE retry_of_execution_id IS NOT NULL AND retry_idempotency_key IS NOT NULL;

CREATE INDEX agent_harness_executions_workspace_active_idx
ON agent_harness_executions (workspace_id, status)
WHERE status IN ('queued', 'provisioning', 'running', 'scoring');

-- +goose Down
DROP INDEX IF EXISTS agent_harness_executions_workspace_active_idx;
DROP INDEX IF EXISTS agent_harness_executions_retry_idempotency_idx;
ALTER TABLE agent_harness_executions
DROP COLUMN IF EXISTS retry_idempotency_key,
DROP COLUMN IF EXISTS retry_of_execution_id,
DROP COLUMN IF EXISTS temporal_run_id,
DROP COLUMN IF EXISTS temporal_workflow_id;
