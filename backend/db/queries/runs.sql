-- name: CreateRun :one
INSERT INTO runs (
    organization_id,
    workspace_id,
    challenge_pack_version_id,
    challenge_input_set_id,
    created_by_user_id,
    name,
    status,
    execution_mode,
    temporal_workflow_id,
    temporal_run_id,
    execution_plan,
    queued_at,
    started_at,
    finished_at,
    cancelled_at,
    failed_at
) VALUES (
    @organization_id,
    @workspace_id,
    @challenge_pack_version_id,
    sqlc.narg('challenge_input_set_id'),
    sqlc.narg('created_by_user_id'),
    @name,
    @status,
    @execution_mode,
    sqlc.narg('temporal_workflow_id'),
    sqlc.narg('temporal_run_id'),
    @execution_plan,
    sqlc.narg('queued_at'),
    sqlc.narg('started_at'),
    sqlc.narg('finished_at'),
    sqlc.narg('cancelled_at'),
    sqlc.narg('failed_at')
)
RETURNING *;

-- name: GetRunByID :one
SELECT *
FROM runs
WHERE id = @id
LIMIT 1;

-- name: SetRunTemporalIDs :one
UPDATE runs
SET temporal_workflow_id = @temporal_workflow_id,
    temporal_run_id = @temporal_run_id
WHERE id = @id
  AND temporal_workflow_id IS NULL
  AND temporal_run_id IS NULL
RETURNING *;

-- name: UpdateRunStatus :one
UPDATE runs
SET status = @to_status,
    queued_at = CASE
        WHEN @to_status::text = 'queued' AND queued_at IS NULL THEN now()
        ELSE queued_at
    END,
    started_at = CASE
        WHEN @to_status::text = 'provisioning' AND started_at IS NULL THEN now()
        ELSE started_at
    END,
    finished_at = CASE
        WHEN @to_status::text IN ('completed', 'failed', 'cancelled') AND finished_at IS NULL THEN now()
        ELSE finished_at
    END,
    cancelled_at = CASE
        WHEN @to_status::text = 'cancelled' AND cancelled_at IS NULL THEN now()
        ELSE cancelled_at
    END,
    failed_at = CASE
        WHEN @to_status::text = 'failed' AND failed_at IS NULL THEN now()
        ELSE failed_at
    END
WHERE id = @id
  AND status = @from_status
RETURNING *;

-- name: InsertRunStatusHistory :one
INSERT INTO run_status_history (
    run_id,
    from_status,
    to_status,
    reason,
    changed_by_user_id
) VALUES (
    @run_id,
    sqlc.narg('from_status'),
    @to_status,
    sqlc.narg('reason'),
    sqlc.narg('changed_by_user_id')
)
RETURNING *;

-- name: ListRunStatusHistoryByRunID :many
SELECT *
FROM run_status_history
WHERE run_id = @run_id
ORDER BY changed_at ASC, id ASC;
