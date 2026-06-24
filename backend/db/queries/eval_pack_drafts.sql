-- name: CreateEvalPackDraft :one
INSERT INTO eval_pack_drafts (
    workspace_id,
    name,
    execution_mode,
    eval_pack_id,
    composition,
    created_by_user_id
) VALUES (
    @workspace_id,
    @name,
    @execution_mode,
    sqlc.narg('eval_pack_id'),
    @composition,
    sqlc.narg('created_by_user_id')
)
RETURNING *;

-- name: GetEvalPackDraftByID :one
SELECT *
FROM eval_pack_drafts
WHERE id = @id
LIMIT 1;

-- name: ListEvalPackDraftsByWorkspace :many
SELECT *
FROM eval_pack_drafts
WHERE workspace_id = @workspace_id
  AND status = 'draft'
ORDER BY updated_at DESC, id DESC;

-- name: PatchEvalPackDraft :one
UPDATE eval_pack_drafts
SET name = COALESCE(sqlc.narg('name'), name),
    execution_mode = COALESCE(sqlc.narg('execution_mode'), execution_mode),
    composition = COALESCE(sqlc.narg('composition')::jsonb, composition),
    eval_pack_id = COALESCE(sqlc.narg('eval_pack_id'), eval_pack_id),
    status = COALESCE(sqlc.narg('to_status')::text, status),
    last_published_version_id = COALESCE(sqlc.narg('last_published_version_id'), last_published_version_id)
WHERE id = @id
RETURNING *;

-- name: DeleteEvalPackDraft :exec
DELETE FROM eval_pack_drafts
WHERE id = @id;
