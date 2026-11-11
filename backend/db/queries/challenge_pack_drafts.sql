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
-- Only a draft in status draft takes edits. A published draft is final. The
-- publish path writes the status (see challenge_pack_draft_status.go).
UPDATE challenge_pack_drafts
SET name = COALESCE(sqlc.narg('name'), name),
    execution_mode = COALESCE(sqlc.narg('execution_mode'), execution_mode),
    composition = COALESCE(sqlc.narg('composition')::jsonb, composition)
WHERE id = @id
  AND status = 'draft'
RETURNING *;

-- name: DeleteChallengePackDraft :exec
DELETE FROM challenge_pack_drafts
WHERE id = @id;
