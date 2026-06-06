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
    summary,
    redaction_status,
    run_id,
    cost_limit_usd,
    actual_cost_usd,
    latency_ms,
    max_duration_seconds,
    anonymous_fingerprint_hash,
    created_by_user_id,
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
    @summary,
    @redaction_status,
    sqlc.narg('run_id'),
    @cost_limit_usd,
    sqlc.narg('actual_cost_usd'),
    sqlc.narg('latency_ms'),
    @max_duration_seconds,
    sqlc.narg('anonymous_fingerprint_hash'),
    sqlc.narg('created_by_user_id'),
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
