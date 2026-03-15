-- name: CreateHostedRunExecution :one
INSERT INTO hosted_run_executions (
    run_id,
    run_agent_id,
    endpoint_url,
    trace_level,
    status,
    deadline_at
)
VALUES (
    @run_id,
    @run_agent_id,
    @endpoint_url,
    @trace_level,
    'starting',
    @deadline_at
)
ON CONFLICT (run_agent_id) DO UPDATE
SET endpoint_url = EXCLUDED.endpoint_url,
    trace_level = EXCLUDED.trace_level,
    status = EXCLUDED.status,
    external_run_id = NULL,
    accepted_response = '{}'::jsonb,
    last_event_type = NULL,
    last_event_payload = '{}'::jsonb,
    result_payload = '{}'::jsonb,
    error_message = NULL,
    deadline_at = EXCLUDED.deadline_at,
    accepted_at = NULL,
    started_at = NULL,
    finished_at = NULL
RETURNING *;

-- name: GetHostedRunExecutionByRunAgentID :one
SELECT *
FROM hosted_run_executions
WHERE run_agent_id = @run_agent_id
LIMIT 1;

-- name: MarkHostedRunExecutionAccepted :one
UPDATE hosted_run_executions
SET external_run_id = @external_run_id,
    status = 'accepted',
    accepted_response = @accepted_response,
    accepted_at = COALESCE(accepted_at, now())
WHERE run_agent_id = @run_agent_id
RETURNING *;

-- name: MarkHostedRunExecutionFailed :one
UPDATE hosted_run_executions
SET status = 'failed',
    error_message = @error_message,
    finished_at = COALESCE(finished_at, now()),
    last_event_type = sqlc.narg('last_event_type'),
    last_event_payload = CASE
        WHEN sqlc.narg('last_event_payload') IS NULL THEN last_event_payload
        ELSE sqlc.narg('last_event_payload')
    END,
    result_payload = CASE
        WHEN sqlc.narg('result_payload') IS NULL THEN result_payload
        ELSE sqlc.narg('result_payload')
    END
WHERE run_agent_id = @run_agent_id
RETURNING *;

-- name: MarkHostedRunExecutionTimedOut :one
UPDATE hosted_run_executions
SET status = 'timed_out',
    error_message = @error_message,
    finished_at = COALESCE(finished_at, now())
WHERE run_agent_id = @run_agent_id
RETURNING *;

-- name: ApplyHostedRunEvent :one
UPDATE hosted_run_executions
SET status = @status,
    external_run_id = COALESCE(external_run_id, sqlc.narg('external_run_id')),
    last_event_type = @last_event_type,
    last_event_payload = @last_event_payload,
    result_payload = CASE
        WHEN sqlc.narg('result_payload') IS NULL THEN result_payload
        ELSE sqlc.narg('result_payload')
    END,
    error_message = CASE
        WHEN sqlc.narg('error_message') IS NULL THEN error_message
        ELSE sqlc.narg('error_message')
    END,
    started_at = CASE
        WHEN @status IN ('running', 'completed', 'failed') AND started_at IS NULL THEN @occurred_at
        ELSE started_at
    END,
    finished_at = CASE
        WHEN @status IN ('completed', 'failed') AND finished_at IS NULL THEN @occurred_at
        ELSE finished_at
    END
WHERE run_agent_id = @run_agent_id
RETURNING *;

-- name: InsertHostedRunEvent :one
WITH next_sequence AS (
    SELECT COALESCE(MAX(sequence_number), 0) + 1 AS sequence_number
    FROM run_events
    WHERE run_agent_id = @run_agent_id
)
INSERT INTO run_events (
    run_id,
    run_agent_id,
    sequence_number,
    event_type,
    actor_type,
    occurred_at,
    payload
)
SELECT
    @run_id,
    @run_agent_id,
    next_sequence.sequence_number,
    @event_type,
    @actor_type,
    @occurred_at,
    @payload
FROM next_sequence
RETURNING id, run_id, run_agent_id, sequence_number, event_type, actor_type, occurred_at, artifact_id, payload;

-- name: UpsertRunAgentReplaySummary :one
INSERT INTO run_agent_replays (
    run_agent_id,
    summary,
    latest_sequence_number,
    event_count
)
VALUES (
    @run_agent_id,
    @summary,
    @latest_sequence_number,
    @event_count
)
ON CONFLICT (run_agent_id) DO UPDATE
SET summary = EXCLUDED.summary,
    latest_sequence_number = EXCLUDED.latest_sequence_number,
    event_count = EXCLUDED.event_count
RETURNING id, run_agent_id, artifact_id, summary, latest_sequence_number, event_count, created_at, updated_at;
