-- name: CreateRunAgent :one
INSERT INTO run_agents (
    organization_id,
    workspace_id,
    run_id,
    agent_deployment_id,
    agent_deployment_snapshot_id,
    lane_index,
    label,
    status,
    queued_at,
    started_at,
    finished_at,
    failure_reason
) VALUES (
    @organization_id,
    @workspace_id,
    @run_id,
    @agent_deployment_id,
    @agent_deployment_snapshot_id,
    @lane_index,
    @label,
    @status,
    sqlc.narg('queued_at'),
    sqlc.narg('started_at'),
    sqlc.narg('finished_at'),
    sqlc.narg('failure_reason')
)
RETURNING *;

-- name: ListRunAgentsByRunID :many
SELECT *
FROM run_agents
WHERE run_id = @run_id
ORDER BY lane_index ASC;

-- name: GetRunAgentByID :one
SELECT *
FROM run_agents
WHERE id = @id
LIMIT 1;

-- name: UpdateRunAgentStatus :one
UPDATE run_agents
SET status = @to_status,
    started_at = CASE
        WHEN @to_status::text = 'executing' AND started_at IS NULL THEN now()
        ELSE started_at
    END,
    finished_at = CASE
        WHEN @to_status::text IN ('completed', 'failed') AND finished_at IS NULL THEN now()
        ELSE finished_at
    END,
    failure_reason = CASE
        WHEN @to_status::text = 'failed' THEN sqlc.narg('failure_reason')
        ELSE failure_reason
    END
WHERE id = @id
  AND status = @from_status
RETURNING *;

-- name: InsertRunAgentStatusHistory :one
INSERT INTO run_agent_status_history (
    run_agent_id,
    from_status,
    to_status,
    reason
) VALUES (
    @run_agent_id,
    sqlc.narg('from_status'),
    @to_status,
    sqlc.narg('reason')
)
RETURNING *;

-- name: ListRunAgentStatusHistoryByRunAgentID :many
SELECT *
FROM run_agent_status_history
WHERE run_agent_id = @run_agent_id
ORDER BY changed_at ASC, id ASC;
