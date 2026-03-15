-- name: InsertRunEvent :one
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

-- name: ListRunEventsByRunAgentID :many
SELECT id, run_id, run_agent_id, sequence_number, event_type, actor_type, occurred_at, artifact_id, payload
FROM run_events
WHERE run_agent_id = @run_agent_id
ORDER BY sequence_number ASC;
