-- name: CreateEvaluationSpec :one
INSERT INTO evaluation_specs (
    challenge_pack_version_id,
    name,
    version_number,
    judge_mode,
    definition
) VALUES (
    @challenge_pack_version_id,
    @name,
    @version_number,
    @judge_mode,
    @definition
)
RETURNING id, challenge_pack_version_id, name, version_number, judge_mode, definition, created_at, updated_at;

-- name: GetEvaluationSpecByID :one
SELECT id, challenge_pack_version_id, name, version_number, judge_mode, definition, created_at, updated_at
FROM evaluation_specs
WHERE id = @id
LIMIT 1;

-- name: GetEvaluationSpecByChallengePackVersionAndVersion :one
SELECT id, challenge_pack_version_id, name, version_number, judge_mode, definition, created_at, updated_at
FROM evaluation_specs
WHERE challenge_pack_version_id = @challenge_pack_version_id
  AND name = @name
  AND version_number = @version_number
LIMIT 1;
