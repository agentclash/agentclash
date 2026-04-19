-- name: CreateRun :one
INSERT INTO runs (
    organization_id,
    workspace_id,
    challenge_pack_version_id,
    challenge_input_set_id,
    official_pack_mode,
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
    @official_pack_mode,
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

-- name: CreateRunCaseSelection :one
INSERT INTO run_case_selections (
    run_id,
    challenge_identity_id,
    selection_origin,
    regression_case_id,
    selection_rank
) VALUES (
    @run_id,
    @challenge_identity_id,
    @selection_origin,
    sqlc.narg('regression_case_id'),
    @selection_rank
)
RETURNING *;

-- name: ListRunCaseSelectionsByRunID :many
SELECT *
FROM run_case_selections
WHERE run_id = @run_id
ORDER BY selection_rank ASC, created_at ASC, challenge_identity_id ASC;

-- name: ListRunRegressionCoverageCasesByRunID :many
WITH winning_run_agent AS (
    SELECT winning_run_agent_id
    FROM run_scorecards
    WHERE run_scorecards.run_id = @run_id
    ORDER BY created_at DESC
    LIMIT 1
),
selected_regression_cases AS (
    SELECT DISTINCT ON (rcs.regression_case_id)
        rcs.regression_case_id,
        c.title AS regression_case_title,
        s.id AS suite_id,
        s.name AS suite_name
    FROM run_case_selections AS rcs
    LEFT JOIN workspace_regression_cases AS c
      ON c.id = rcs.regression_case_id
    LEFT JOIN workspace_regression_suites AS s
      ON s.id = c.suite_id
    WHERE rcs.run_id = @run_id
      AND rcs.regression_case_id IS NOT NULL
    ORDER BY rcs.regression_case_id, rcs.selection_rank ASC, rcs.created_at ASC
),
winning_case_outcomes AS (
    SELECT
        jr.regression_case_id,
        CASE
            WHEN bool_or(jr.verdict = 'fail') THEN 'fail'
            WHEN bool_or(jr.verdict = 'pass') THEN 'pass'
            ELSE 'pending'
        END AS outcome
    FROM judge_results AS jr
    JOIN winning_run_agent AS wra
      ON jr.run_agent_id = wra.winning_run_agent_id
    WHERE jr.regression_case_id IS NOT NULL
    GROUP BY jr.regression_case_id
)
SELECT
    src.regression_case_id,
    src.regression_case_title,
    src.suite_id,
    src.suite_name,
    COALESCE(wco.outcome, 'pending') AS outcome
FROM selected_regression_cases AS src
LEFT JOIN winning_case_outcomes AS wco
  ON wco.regression_case_id = src.regression_case_id
ORDER BY src.suite_name ASC NULLS LAST, src.regression_case_title ASC, src.regression_case_id ASC;

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

-- name: ListRunsByWorkspaceID :many
SELECT *
FROM runs
WHERE workspace_id = @workspace_id
ORDER BY created_at DESC
LIMIT @result_limit OFFSET @result_offset;

-- name: CountRunsByWorkspaceID :one
SELECT count(*)
FROM runs
WHERE workspace_id = @workspace_id;
