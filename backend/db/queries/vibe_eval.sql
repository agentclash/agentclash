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
