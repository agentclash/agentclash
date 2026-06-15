-- name: CreateVibeEvalConversation :one
INSERT INTO vibe_eval_conversations (
    organization_id,
    workspace_id,
    created_by_user_id,
    title,
    phase,
    status
)
VALUES (
    @organization_id,
    @workspace_id,
    @created_by_user_id,
    @title,
    @phase,
    @status
)
RETURNING *;

-- name: ListVibeEvalConversationsByWorkspaceID :many
SELECT *
FROM vibe_eval_conversations
WHERE workspace_id = @workspace_id
  AND archived_at IS NULL
ORDER BY updated_at DESC;

-- name: GetVibeEvalConversationByID :one
SELECT *
FROM vibe_eval_conversations
WHERE id = @id
  AND archived_at IS NULL
LIMIT 1;

-- name: SetVibeEvalConversationActiveDraft :one
UPDATE vibe_eval_conversations
SET active_draft_id = @active_draft_id
WHERE id = @id
RETURNING *;

-- name: CreateVibeEvalDraft :one
INSERT INTO vibe_eval_drafts (
    organization_id,
    workspace_id,
    conversation_id,
    draft_kind,
    content,
    validation_state,
    validation_errors,
    created_by_user_id,
    updated_by_user_id
)
VALUES (
    @organization_id,
    @workspace_id,
    @conversation_id,
    @draft_kind,
    @content,
    @validation_state,
    @validation_errors,
    @created_by_user_id,
    @updated_by_user_id
)
RETURNING *;

-- name: ListVibeEvalDraftsByConversationID :many
SELECT *
FROM vibe_eval_drafts
WHERE conversation_id = @conversation_id
ORDER BY updated_at DESC;

-- name: GetVibeEvalDraftByID :one
SELECT *
FROM vibe_eval_drafts
WHERE id = @id
LIMIT 1;

-- name: UpdateVibeEvalDraft :one
UPDATE vibe_eval_drafts
SET
    content = @content,
    validation_state = @validation_state,
    validation_errors = @validation_errors,
    updated_by_user_id = @updated_by_user_id
WHERE id = @id
RETURNING *;

-- name: LockVibeEvalConversationForAppend :one
-- Step 1 of a two-statement append (run inside a repository transaction). Locks the
-- conversation row FOR NO KEY UPDATE so concurrent appends to the same conversation serialize.
-- NO KEY UPDATE (not FOR UPDATE) still mutually excludes concurrent appenders while staying
-- compatible with the FOR KEY SHARE locks that FK checks from sibling inserts
-- (e.g. vibe_eval_drafts referencing this conversation) take on the same row.
-- The lock must be taken in its OWN statement: under READ COMMITTED, the following INSERT then
-- runs with a fresh snapshot taken AFTER the lock wait, so its MAX(seq) sees the prior
-- appender's committed row. A single CTE statement would compute MAX(seq) against the snapshot
-- taken before the lock wait and could still collide on seq.
SELECT
    vibe_eval_conversations.id,
    vibe_eval_conversations.organization_id,
    vibe_eval_conversations.workspace_id
FROM vibe_eval_conversations
WHERE vibe_eval_conversations.id = @conversation_id
FOR NO KEY UPDATE;

-- name: InsertVibeEvalMessage :one
-- Step 2 of the append: runs after LockVibeEvalConversationForAppend in the same transaction,
-- so MAX(seq) is computed against a post-lock snapshot. org/workspace come from the locked row.
INSERT INTO vibe_eval_messages (
    organization_id,
    workspace_id,
    conversation_id,
    seq,
    role,
    content,
    redaction_state,
    tool_call_id,
    tool_name,
    tool_args,
    tool_calls,
    usage
)
VALUES (
    @organization_id,
    @workspace_id,
    @conversation_id,
    COALESCE((SELECT MAX(m.seq) FROM vibe_eval_messages m WHERE m.conversation_id = @conversation_id), 0) + 1,
    @role,
    @content,
    @redaction_state,
    @tool_call_id,
    @tool_name,
    @tool_args,
    @tool_calls,
    @usage
)
RETURNING *;

-- name: ListVibeEvalMessagesByConversationID :many
SELECT *
FROM vibe_eval_messages
WHERE conversation_id = @conversation_id
ORDER BY seq ASC;

-- name: AppendVibeEvalToolInvocation :one
-- Append-only audit row for one draft+ tool call (#875 §6). request_payload/result_payload are
-- audit-scrubbed metadata only. confirmation_id is a soft reference (no FK).
INSERT INTO vibe_eval_tool_invocations (
    organization_id,
    workspace_id,
    conversation_id,
    message_id,
    actor_user_id,
    tool_name,
    action,
    risk_tier,
    payload_hash,
    confirmation_id,
    request_payload,
    result_payload,
    credit_reservation_id,
    outcome
)
VALUES (
    @organization_id,
    @workspace_id,
    @conversation_id,
    sqlc.narg('message_id'),
    @actor_user_id,
    @tool_name,
    @action,
    @risk_tier,
    @payload_hash,
    sqlc.narg('confirmation_id'),
    @request_payload,
    @result_payload,
    sqlc.narg('credit_reservation_id'),
    @outcome
)
RETURNING *;

-- name: ListVibeEvalToolInvocationsByConversationID :many
SELECT *
FROM vibe_eval_tool_invocations
WHERE conversation_id = @conversation_id
ORDER BY created_at DESC, id DESC;

-- name: CreateVibeEvalPendingConfirmation :one
-- Propose half of the confirmation engine (#875 §5.3). bound_args is the verbatim args to
-- execute on approve; payload_hash binds the confirmation to exactly those args.
INSERT INTO vibe_eval_pending_confirmations (
    organization_id,
    workspace_id,
    conversation_id,
    message_id,
    proposed_by_user_id,
    tool_name,
    tool_call_id,
    action,
    risk_tier,
    payload_hash,
    bound_args,
    summary,
    estimate,
    expires_at
)
VALUES (
    @organization_id,
    @workspace_id,
    @conversation_id,
    sqlc.narg('message_id'),
    @proposed_by_user_id,
    @tool_name,
    @tool_call_id,
    @action,
    @risk_tier,
    @payload_hash,
    @bound_args,
    @summary,
    sqlc.narg('estimate'),
    @expires_at
)
RETURNING *;

-- name: ExpireStaleVibeEvalPendingConfirmations :many
-- Transition lapsed 'pending' rows for a conversation to 'expired' so they leave the active
-- partial unique index and stop blocking re-proposal (#875 §5.3). The create path runs this in
-- the same tx before inserting.
UPDATE vibe_eval_pending_confirmations
SET status = 'expired', resolved_at = now()
WHERE conversation_id = @conversation_id
  AND status = 'pending'
  AND expires_at <= now()
RETURNING *;

-- name: GetVibeEvalPendingConfirmationForResume :one
-- Crash-safe re-entry: returns the row only if it is still 'executing' and the presented hash
-- matches, so a retried POST can resume effect execution exactly once. 0 rows => not resumable.
SELECT *
FROM vibe_eval_pending_confirmations
WHERE id = @id
  AND status = 'executing'
  AND payload_hash = @payload_hash
LIMIT 1;

-- name: GetVibeEvalPendingConfirmationByID :one
SELECT *
FROM vibe_eval_pending_confirmations
WHERE id = @id
LIMIT 1;

-- name: ResolveVibeEvalPendingConfirmation :one
-- Atomic, single-use transition out of 'pending' (#875 §5.3). Only the request that matches a
-- still-pending, unexpired row with the presented payload_hash wins (0 rows => already resolved,
-- expired, or hash mismatch => reject). @new_status is 'executing' (approve) or 'denied' (deny).
UPDATE vibe_eval_pending_confirmations
SET
    status = @new_status,
    resolved_by_user_id = @resolved_by_user_id,
    resolved_at = now()
WHERE id = @id
  AND status = 'pending'
  AND expires_at > now()
  AND payload_hash = @payload_hash
RETURNING *;

-- name: MarkVibeEvalPendingConfirmationResult :one
-- Terminal transition after the bound effect runs: 'executing' -> 'succeeded' | 'failed'.
-- Conditioned on status='executing' so a crashed/retried effect transitions exactly once.
UPDATE vibe_eval_pending_confirmations
SET status = @status
WHERE id = @id
  AND status = 'executing'
RETURNING *;
