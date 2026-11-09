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
SET run_agent_id = EXCLUDED.run_agent_id
RETURNING *;

-- name: GetHostedRunExecutionByRunAgentID :one
SELECT *
FROM hosted_run_executions
WHERE run_agent_id = @run_agent_id
LIMIT 1;

-- name: MarkHostedRunExecutionAccepted :one
UPDATE hosted_run_executions
SET external_run_id = @external_run_id,
    status = CASE WHEN status IN ('starting','unknown') THEN 'accepted' ELSE status END,
    accepted_response = @accepted_response,
    accepted_at = COALESCE(accepted_at, now())
WHERE run_agent_id = @run_agent_id AND (external_run_id IS NULL OR external_run_id = @external_run_id)
RETURNING *;

-- name: MarkHostedRunExecutionFailed :one
UPDATE hosted_run_executions
SET status = 'failed',
    error_message = @error_message,
    finished_at = COALESCE(finished_at, now()),
    last_event_type = sqlc.narg('last_event_type'),
    last_event_payload = CASE
        WHEN sqlc.narg('last_event_payload')::jsonb IS NULL THEN last_event_payload
        ELSE sqlc.narg('last_event_payload')
    END,
    result_payload = CASE
        WHEN sqlc.narg('result_payload')::jsonb IS NULL THEN result_payload
        ELSE sqlc.narg('result_payload')
    END
WHERE run_agent_id = @run_agent_id AND status IN ('starting','unknown')
RETURNING *;

-- name: MarkHostedRunExecutionTimedOut :one
UPDATE hosted_run_executions
SET status = CASE WHEN status IN ('completed','failed') THEN status ELSE 'unknown' END,
    error_message = @error_message,
    finished_at = COALESCE(finished_at, now())
WHERE run_agent_id = @run_agent_id AND status NOT IN ('completed','failed')
RETURNING *;

-- name: ApplyHostedRunEvent :one
UPDATE hosted_run_executions
SET status = @status,
    external_run_id = COALESCE(external_run_id, sqlc.narg('external_run_id')),
    last_event_type = @last_event_type,
    last_event_payload = @last_event_payload,
    result_payload = CASE
        WHEN sqlc.narg('result_payload')::jsonb IS NULL THEN result_payload
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
