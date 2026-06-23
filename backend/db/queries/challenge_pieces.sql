-- name: CreateChallengePiece :one
INSERT INTO challenge_pieces (
    workspace_id,
    kind,
    slug,
    name,
    description,
    definition,
    created_by_user_id
) VALUES (
    @workspace_id,
    @kind,
    @slug,
    @name,
    @description,
    @definition,
    sqlc.narg('created_by_user_id')
)
RETURNING *;

-- name: GetChallengePieceByID :one
SELECT *
FROM challenge_pieces
WHERE id = @id
  AND lifecycle_status = 'active'
LIMIT 1;

-- name: ListChallengePiecesByWorkspace :many
SELECT *
FROM challenge_pieces
WHERE workspace_id = @workspace_id
  AND lifecycle_status = 'active'
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind')::text)
ORDER BY created_at DESC, id DESC;

-- name: ListChallengePiecesByIDs :many
SELECT *
FROM challenge_pieces
WHERE id = ANY (@ids::uuid[])
  AND workspace_id = @workspace_id
  AND lifecycle_status = 'active';

-- name: PatchChallengePiece :one
UPDATE challenge_pieces
SET name = COALESCE(sqlc.narg('name'), name),
    slug = COALESCE(sqlc.narg('slug'), slug),
    description = COALESCE(sqlc.narg('description'), description),
    definition = COALESCE(sqlc.narg('definition')::jsonb, definition)
WHERE id = @id
  AND workspace_id = @workspace_id
  AND lifecycle_status = 'active'
RETURNING *;

-- name: ArchiveChallengePiece :one
UPDATE challenge_pieces
SET lifecycle_status = 'archived',
    archived_at = now()
WHERE id = @id
  AND workspace_id = @workspace_id
  AND lifecycle_status = 'active'
RETURNING *;
