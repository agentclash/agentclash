-- name: ListEvalPacks :many
SELECT cp.id, cp.name, cp.description, cp.created_at, cp.updated_at
FROM eval_packs cp
WHERE cp.archived_at IS NULL
ORDER BY cp.name ASC;

-- name: ListRunnableChallengePVersionsByPackID :many
SELECT id, eval_pack_id, version_number, lifecycle_status, manifest, created_at, updated_at
FROM eval_pack_versions
WHERE eval_pack_id = @eval_pack_id
  AND lifecycle_status = 'runnable'
  AND archived_at IS NULL
ORDER BY created_at DESC;

-- name: GetWorkspaceEvalPackVersionBySlug :one
SELECT p.id AS eval_pack_id, v.id AS eval_pack_version_id
FROM eval_packs p
JOIN eval_pack_versions v ON v.eval_pack_id = p.id
WHERE p.workspace_id = sqlc.arg(workspace_id)::uuid
  AND p.slug = @slug
  AND p.archived_at IS NULL
  AND v.lifecycle_status = 'runnable'
  AND v.archived_at IS NULL
ORDER BY v.version_number DESC
LIMIT 1;
