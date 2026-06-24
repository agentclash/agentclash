-- name: GetChallengeIdentityForDatasetEval :one
SELECT ci.id
FROM eval_pack_version_challenges cpvc
JOIN challenge_identities ci
  ON ci.id = cpvc.challenge_identity_id
 AND ci.eval_pack_id = cpvc.eval_pack_id
WHERE cpvc.eval_pack_version_id = @eval_pack_version_id
  AND ci.challenge_key = @challenge_key
  AND ci.archived_at IS NULL
LIMIT 1;

-- name: GetDatasetVersionInputSetByBinding :one
SELECT *
FROM dataset_version_input_sets
WHERE dataset_version_id = @dataset_version_id
  AND eval_pack_version_id = @eval_pack_version_id
  AND challenge_key = @challenge_key
LIMIT 1;

-- name: CreateDatasetEvalChallengeInputSet :one
INSERT INTO challenge_input_sets (
    eval_pack_version_id,
    input_key,
    name,
    description,
    input_checksum,
    generated_at
) VALUES (
    @eval_pack_version_id,
    @input_key,
    @name,
    @description,
    @input_checksum,
    now()
)
ON CONFLICT (eval_pack_version_id, input_key) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    input_checksum = EXCLUDED.input_checksum
RETURNING *;

-- name: CreateDatasetVersionInputSet :one
INSERT INTO dataset_version_input_sets (
    dataset_id,
    dataset_version_id,
    eval_pack_version_id,
    challenge_identity_id,
    challenge_key,
    challenge_input_set_id,
    input_key,
    input_checksum,
    mapping
) VALUES (
    @dataset_id,
    @dataset_version_id,
    @eval_pack_version_id,
    @challenge_identity_id,
    @challenge_key,
    @challenge_input_set_id,
    @input_key,
    @input_checksum,
    @mapping
)
ON CONFLICT (dataset_version_id, eval_pack_version_id, challenge_key) DO UPDATE
SET challenge_input_set_id = EXCLUDED.challenge_input_set_id,
    input_key = EXCLUDED.input_key,
    input_checksum = EXCLUDED.input_checksum,
    mapping = EXCLUDED.mapping
RETURNING *;

-- name: UpsertDatasetChallengeInputItem :one
INSERT INTO challenge_input_items (
    challenge_input_set_id,
    eval_pack_version_id,
    challenge_identity_id,
    item_key,
    payload
) VALUES (
    @challenge_input_set_id,
    @eval_pack_version_id,
    @challenge_identity_id,
    @item_key,
    @payload
)
ON CONFLICT (challenge_input_set_id, challenge_identity_id, item_key) DO UPDATE
SET payload = EXCLUDED.payload
RETURNING *;

-- name: UpsertDatasetInputItemLink :one
INSERT INTO dataset_input_item_links (
    dataset_version_input_set_id,
    dataset_example_id,
    challenge_input_item_id,
    item_key
) VALUES (
    @dataset_version_input_set_id,
    @dataset_example_id,
    @challenge_input_item_id,
    @item_key
)
ON CONFLICT (dataset_version_input_set_id, dataset_example_id) DO UPDATE
SET challenge_input_item_id = EXCLUDED.challenge_input_item_id,
    item_key = EXCLUDED.item_key
RETURNING *;

-- name: ListDatasetInputItemLinksByVersionInputSet :many
SELECT *
FROM dataset_input_item_links
WHERE dataset_version_input_set_id = @dataset_version_input_set_id
ORDER BY item_key ASC;

-- name: RecordDatasetEvalRun :one
INSERT INTO dataset_eval_runs (
    dataset_id,
    dataset_version_id,
    dataset_version_input_set_id,
    run_id
) VALUES (
    @dataset_id,
    @dataset_version_id,
    @dataset_version_input_set_id,
    @run_id
)
ON CONFLICT (run_id) DO UPDATE
SET dataset_id = EXCLUDED.dataset_id,
    dataset_version_id = EXCLUDED.dataset_version_id,
    dataset_version_input_set_id = EXCLUDED.dataset_version_input_set_id
RETURNING *;

-- name: ListDatasetEvalResults :many
SELECT
    dil.dataset_example_id,
    dvis.dataset_version_id,
    dvis.challenge_input_set_id,
    der.run_id,
    ra.id AS run_agent_id,
    jr.verdict,
    jr.normalized_score,
    jr.created_at AS judged_at
FROM dataset_version_input_sets dvis
JOIN dataset_input_item_links dil
  ON dil.dataset_version_input_set_id = dvis.id
LEFT JOIN dataset_eval_runs der
  ON der.dataset_version_input_set_id = dvis.id
LEFT JOIN run_agents ra
  ON ra.run_id = der.run_id
LEFT JOIN judge_results jr
  ON jr.run_agent_id = ra.id
 AND jr.challenge_identity_id = dvis.challenge_identity_id
WHERE dvis.dataset_id = @dataset_id
  AND (sqlc.narg('dataset_version_id')::uuid IS NULL OR dvis.dataset_version_id = sqlc.narg('dataset_version_id')::uuid)
ORDER BY der.created_at DESC NULLS LAST, dil.item_key ASC, ra.lane_index ASC, jr.created_at DESC NULLS LAST
LIMIT @result_limit OFFSET @result_offset;

-- name: CountDatasetEvalResults :one
SELECT count(*)
FROM dataset_version_input_sets dvis
JOIN dataset_input_item_links dil
  ON dil.dataset_version_input_set_id = dvis.id
LEFT JOIN dataset_eval_runs der
  ON der.dataset_version_input_set_id = dvis.id
LEFT JOIN run_agents ra
  ON ra.run_id = der.run_id
LEFT JOIN judge_results jr
  ON jr.run_agent_id = ra.id
 AND jr.challenge_identity_id = dvis.challenge_identity_id
WHERE dvis.dataset_id = @dataset_id
  AND (sqlc.narg('dataset_version_id')::uuid IS NULL OR dvis.dataset_version_id = sqlc.narg('dataset_version_id')::uuid);
