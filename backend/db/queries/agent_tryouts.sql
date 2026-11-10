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
    anonymous_owner_proof_hash,
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
    sqlc.narg('anonymous_owner_proof_hash'),
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
  AND anonymous_owner_proof_hash = @owner_proof_hash::text
  AND expires_at > CURRENT_TIMESTAMP
  AND status IN ('completed', 'failed', 'cancelled')
RETURNING *;

-- name: GetClaimedAgentTryoutRetry :one
SELECT * FROM agent_tryouts
WHERE id = @id
  AND workspace_id = @workspace_id
  AND organization_id = @organization_id
  AND claimed_by_user_id = @claimed_by_user_id
  AND anonymous_owner_proof_hash = @owner_proof_hash::text;

-- name: UpdateAgentTryoutStatus :one
UPDATE agent_tryouts
SET
    status = @status,
    summary = COALESCE(sqlc.narg('summary'), summary),
    actual_cost_usd = COALESCE(sqlc.narg('actual_cost_usd'), actual_cost_usd),
    latency_ms = COALESCE(sqlc.narg('latency_ms'), latency_ms),
    redaction_status = COALESCE(sqlc.narg('redaction_status'), redaction_status)
WHERE id = @id
  -- from_status is the status the caller read; a concurrent change matches
  -- no row.
  AND status = @from_status
  -- A terminal tryout keeps its status. A write to the same status stays
  -- allowed so that retries and summary refreshes remain idempotent.
  AND (status NOT IN ('completed', 'failed', 'cancelled') OR status = @status)
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
WITH owned_tryout AS (
    SELECT id FROM agent_tryouts
    WHERE id = @agent_tryout_id AND workspace_id IS NULL
      AND organization_id IS NULL AND claimed_by_user_id IS NULL
      AND anonymous_owner_proof_hash = @owner_proof_hash::text
      AND expires_at > CURRENT_TIMESTAMP AND status IN ('queued', 'running')
    FOR UPDATE
), next_turn AS (
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
FROM next_turn, owned_tryout
RETURNING *;

-- name: ClaimNextPendingAgentTryoutTurn :one
-- Claims the oldest pending turn, or a processing turn whose lease ran out
-- because the session that claimed it is gone. A processing turn without a
-- lease predates leases and counts as expired.
UPDATE agent_tryout_turns
SET status = 'processing',
    lease_until = now() + make_interval(secs => sqlc.arg('lease_seconds')::integer)
WHERE id = (
    SELECT candidate.id
    FROM agent_tryout_turns AS candidate
    WHERE candidate.agent_tryout_id = @agent_tryout_id
      AND (
          candidate.status = 'pending'
          OR (
              candidate.status = 'processing'
              AND (candidate.lease_until IS NULL OR candidate.lease_until < now())
          )
      )
    ORDER BY candidate.id ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkAgentTryoutTurnProcessed :execrows
-- Only a processing turn can become done.
UPDATE agent_tryout_turns
SET status = 'done', processed_at = now(), lease_until = NULL
WHERE id = @id
  AND status = 'processing';

-- name: GetAgentTryoutTurnStatus :one
SELECT status
FROM agent_tryout_turns
WHERE id = @id;

-- name: CountPendingAgentTryoutTurns :one
SELECT COUNT(*)
FROM agent_tryout_turns
WHERE agent_tryout_id = @agent_tryout_id
  AND status = 'pending';
