-- name: ListChallengePacks :many
SELECT cp.id, cp.name, cp.description, cp.created_at, cp.updated_at
FROM challenge_packs cp
WHERE cp.archived_at IS NULL
ORDER BY cp.name ASC;

-- name: ListRunnableChallengePVersionsByPackID :many
SELECT id, challenge_pack_id, version_number, lifecycle_status, manifest, created_at, updated_at
FROM challenge_pack_versions
WHERE challenge_pack_id = @challenge_pack_id
  AND lifecycle_status = 'runnable'
  AND archived_at IS NULL
ORDER BY created_at DESC;

-- name: GetWorkspaceChallengePackVersionBySlug :one
SELECT p.id AS challenge_pack_id, v.id AS challenge_pack_version_id
FROM challenge_packs p
JOIN challenge_pack_versions v ON v.challenge_pack_id = p.id
WHERE p.workspace_id = sqlc.arg(workspace_id)::uuid
  AND p.slug = @slug
  AND p.archived_at IS NULL
  AND v.lifecycle_status = 'runnable'
  AND v.archived_at IS NULL
ORDER BY v.version_number DESC
LIMIT 1;
