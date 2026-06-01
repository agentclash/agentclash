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
