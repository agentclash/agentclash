-- name: CreateEvalSession :one
INSERT INTO eval_sessions (
    status,
    repetitions,
    aggregation_config,
    success_threshold_config,
    routing_task_snapshot,
    schema_version,
    started_at,
    finished_at
) VALUES (
    @status,
    @repetitions,
    @aggregation_config,
    @success_threshold_config,
    @routing_task_snapshot,
    @schema_version,
    sqlc.narg('started_at'),
    sqlc.narg('finished_at')
)
RETURNING *;

-- name: GetEvalSessionByID :one
SELECT *
FROM eval_sessions
WHERE id = @id
LIMIT 1;

-- name: ListEvalSessions :many
SELECT *
FROM eval_sessions
ORDER BY created_at DESC, id DESC
LIMIT @result_limit OFFSET @result_offset;

-- name: ListEvalSessionsByWorkspaceID :many
SELECT *
FROM eval_sessions
WHERE EXISTS (
    SELECT 1
    FROM runs
    WHERE runs.eval_session_id = eval_sessions.id
      AND runs.workspace_id = @workspace_id
)
ORDER BY created_at DESC, id DESC
LIMIT @result_limit OFFSET @result_offset;

-- name: UpdateEvalSessionStatus :one
UPDATE eval_sessions
SET status = @to_status,
    started_at = CASE
        WHEN @to_status::text = 'running' AND started_at IS NULL THEN now()
        ELSE started_at
    END,
    finished_at = CASE
        WHEN @to_status::text IN ('completed', 'failed', 'cancelled') AND finished_at IS NULL THEN now()
        ELSE finished_at
    END
WHERE id = @id
  AND status = @from_status
RETURNING *;

-- name: AttachRunToEvalSession :one
UPDATE runs
SET eval_session_id = @eval_session_id
WHERE id = @id
  AND eval_session_id IS NULL
RETURNING *;

-- name: ListRunsByEvalSessionID :many
SELECT *
FROM runs
WHERE eval_session_id = @eval_session_id
ORDER BY created_at ASC, id ASC;
