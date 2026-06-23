-- name: CreateChallengePackDraft :one
INSERT INTO challenge_pack_drafts (
    workspace_id,
    name,
    execution_mode,
    challenge_pack_id,
    composition,
    created_by_user_id
) VALUES (
    @workspace_id,
    @name,
    @execution_mode,
    sqlc.narg('challenge_pack_id'),
    @composition,
    sqlc.narg('created_by_user_id')
)
RETURNING *;

-- name: GetChallengePackDraftByID :one
SELECT *
FROM challenge_pack_drafts
WHERE id = @id
LIMIT 1;

-- name: ListChallengePackDraftsByWorkspace :many
SELECT *
FROM challenge_pack_drafts
WHERE workspace_id = @workspace_id
  AND status = 'draft'
ORDER BY updated_at DESC, id DESC;

-- name: PatchChallengePackDraft :one
UPDATE challenge_pack_drafts
SET name = COALESCE(sqlc.narg('name'), name),
    execution_mode = COALESCE(sqlc.narg('execution_mode'), execution_mode),
    composition = COALESCE(sqlc.narg('composition')::jsonb, composition),
    challenge_pack_id = COALESCE(sqlc.narg('challenge_pack_id'), challenge_pack_id),
    status = COALESCE(sqlc.narg('to_status')::text, status),
    last_published_version_id = COALESCE(sqlc.narg('last_published_version_id'), last_published_version_id)
WHERE id = @id
RETURNING *;

-- name: DeleteChallengePackDraft :exec
DELETE FROM challenge_pack_drafts
WHERE id = @id;
