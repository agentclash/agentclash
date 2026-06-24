-- name: CreateEvaluationSpec :one
INSERT INTO evaluation_specs (
    eval_pack_version_id,
    name,
    version_number,
    judge_mode,
    definition
) VALUES (
    @eval_pack_version_id,
    @name,
    @version_number,
    @judge_mode,
    @definition
)
RETURNING id, eval_pack_version_id, name, version_number, judge_mode, definition, created_at, updated_at;

-- name: GetEvaluationSpecByID :one
SELECT id, eval_pack_version_id, name, version_number, judge_mode, definition, created_at, updated_at
FROM evaluation_specs
WHERE id = @id
LIMIT 1;

-- name: GetEvaluationSpecByEvalPackVersionAndVersion :one
SELECT id, eval_pack_version_id, name, version_number, judge_mode, definition, created_at, updated_at
FROM evaluation_specs
WHERE eval_pack_version_id = @eval_pack_version_id
  AND name = @name
  AND version_number = @version_number
LIMIT 1;
