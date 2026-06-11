-- name: CreateAgentTryout :one
INSERT INTO agent_tryouts (
    organization_id,
    workspace_id,
    template_slug,
    status,
    input_snapshot,
    template_snapshot,
    tool_policy_snapshot,
    evaluation_spec_snapshot,
    selected_model_policy,
    selected_harness_kind,
    summary,
    redaction_status,
    run_id,
    cost_limit_usd,
    actual_cost_usd,
    latency_ms,
    max_duration_seconds,
    anonymous_fingerprint_hash,
    created_by_user_id,
    parent_tryout_id,
    expires_at
)
VALUES (
    sqlc.narg('organization_id'),
    sqlc.narg('workspace_id'),
    @template_slug,
    @status,
    @input_snapshot,
    @template_snapshot,
    @tool_policy_snapshot,
    @evaluation_spec_snapshot,
    @selected_model_policy,
    sqlc.narg('selected_harness_kind'),
    @summary,
    @redaction_status,
    sqlc.narg('run_id'),
    @cost_limit_usd,
    sqlc.narg('actual_cost_usd'),
    sqlc.narg('latency_ms'),
    @max_duration_seconds,
    sqlc.narg('anonymous_fingerprint_hash'),
    sqlc.narg('created_by_user_id'),
    sqlc.narg('parent_tryout_id'),
    sqlc.narg('expires_at')
)
RETURNING *;

-- name: GetAgentTryoutByID :one
SELECT *
FROM agent_tryouts
WHERE id = @id
LIMIT 1;

-- name: ListAgentTryoutsByWorkspaceID :many
SELECT *
FROM agent_tryouts
WHERE workspace_id = @workspace_id
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: ClaimAgentTryout :one
UPDATE agent_tryouts
SET
    organization_id = @organization_id,
    workspace_id = @workspace_id,
    claimed_by_user_id = @claimed_by_user_id,
    claimed_at = @claimed_at,
    created_by_user_id = COALESCE(created_by_user_id, @claimed_by_user_id),
    expires_at = NULL
WHERE id = @id
  AND organization_id IS NULL
  AND workspace_id IS NULL
  AND claimed_by_user_id IS NULL
RETURNING *;

-- name: UpdateAgentTryoutStatus :one
UPDATE agent_tryouts
SET
    status = @status,
    summary = COALESCE(sqlc.narg('summary'), summary),
    actual_cost_usd = COALESCE(sqlc.narg('actual_cost_usd'), actual_cost_usd),
    latency_ms = COALESCE(sqlc.narg('latency_ms'), latency_ms),
    redaction_status = COALESCE(sqlc.narg('redaction_status'), redaction_status)
WHERE id = @id
RETURNING *;

-- name: SetAgentTryoutRunID :one
UPDATE agent_tryouts
SET run_id = @run_id
WHERE id = @id
RETURNING *;

-- name: RecordAgentTryoutEvent :one
WITH next_sequence AS (
    SELECT COALESCE(MAX(sequence_number), 0) + 1 AS sequence_number
    FROM agent_tryout_events
    WHERE agent_tryout_id = @agent_tryout_id
)
INSERT INTO agent_tryout_events (
    agent_tryout_id,
    sequence_number,
    event_type,
    actor_type,
    payload
)
SELECT
    @agent_tryout_id,
    next_sequence.sequence_number,
    @event_type,
    @actor_type,
    @payload
FROM next_sequence
RETURNING *;

-- name: ListAgentTryoutEventsAfter :many
SELECT *
FROM agent_tryout_events
WHERE agent_tryout_id = @agent_tryout_id
  AND id > @after_id
ORDER BY id ASC
LIMIT @limit_count;

-- name: AppendAgentTryoutTurn :one
WITH next_turn AS (
    SELECT COALESCE(MAX(turn_index), -1) + 1 AS turn_index
    FROM agent_tryout_turns
    WHERE agent_tryout_id = @agent_tryout_id
)
INSERT INTO agent_tryout_turns (
    agent_tryout_id,
    turn_index,
    role,
    message,
    status
)
SELECT
    @agent_tryout_id,
    next_turn.turn_index,
    @role,
    @message,
    'pending'
FROM next_turn
RETURNING *;

-- name: ClaimNextPendingAgentTryoutTurn :one
UPDATE agent_tryout_turns
SET status = 'processing'
WHERE id = (
    SELECT pending.id
    FROM agent_tryout_turns AS pending
    WHERE pending.agent_tryout_id = @agent_tryout_id
      AND pending.status = 'pending'
    ORDER BY pending.id ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkAgentTryoutTurnProcessed :exec
UPDATE agent_tryout_turns
SET status = 'done', processed_at = now()
WHERE id = @id;

-- name: CountPendingAgentTryoutTurns :one
SELECT COUNT(*)
FROM agent_tryout_turns
WHERE agent_tryout_id = @agent_tryout_id
  AND status = 'pending';
