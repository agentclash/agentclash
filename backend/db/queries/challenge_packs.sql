-- name: ListChallengePacks :many
SELECT cp.id, cp.name, cp.description, cp.created_at, cp.updated_at
FROM challenge_packs cp
WHERE cp.archived_at IS NULL
ORDER BY cp.name ASC;

-- name: ListRunnableChallengePVersionsByPackID :many
SELECT id, challenge_pack_id, version_number, lifecycle_status, created_at, updated_at
FROM challenge_pack_versions
WHERE challenge_pack_id = @challenge_pack_id
  AND lifecycle_status = 'runnable'
  AND archived_at IS NULL
ORDER BY created_at DESC;
