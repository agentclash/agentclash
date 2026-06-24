-- name: InsertDatasetBaseline :one
INSERT INTO dataset_baselines (
    dataset_id,
    dataset_version_id,
    dataset_version_input_set_id,
    challenge_pack_version_id,
    challenge_key,
    agent_deployment_id,
    run_id,
    pass_rate,
    metrics,
    example_outcomes,
    label,
    created_by_user_id
) VALUES (
    @dataset_id,
    @dataset_version_id,
    @dataset_version_input_set_id,
    @challenge_pack_version_id,
    @challenge_key,
    @agent_deployment_id,
    @run_id,
    @pass_rate,
    @metrics,
    @example_outcomes,
    @label,
    @created_by_user_id
)
RETURNING *;

-- name: ListDatasetBaselinesByDatasetID :many
SELECT *
FROM dataset_baselines
WHERE dataset_id = @dataset_id
ORDER BY created_at DESC
LIMIT @result_limit OFFSET @result_offset;

-- name: CountDatasetBaselinesByDatasetID :one
SELECT count(*)
FROM dataset_baselines
WHERE dataset_id = @dataset_id;

-- name: GetDatasetBaselineByID :one
SELECT *
FROM dataset_baselines
WHERE id = @id;

-- name: GetDatasetEvalRunByRunID :one
SELECT *
FROM dataset_eval_runs
WHERE run_id = @run_id;

-- name: GetDatasetVersionInputSetByID :one
SELECT *
FROM dataset_version_input_sets
WHERE id = @id;

-- name: ListDatasetEvalResultsForRun :many
SELECT DISTINCT ON (dil.dataset_example_id)
    dil.dataset_example_id,
    jr.verdict,
    jr.normalized_score
FROM dataset_eval_runs der
JOIN dataset_version_input_sets dvis
  ON dvis.id = der.dataset_version_input_set_id
JOIN dataset_input_item_links dil
  ON dil.dataset_version_input_set_id = dvis.id
JOIN run_agents ra
  ON ra.run_id = der.run_id
LEFT JOIN judge_results jr
  ON jr.run_agent_id = ra.id
 AND jr.challenge_identity_id = dvis.challenge_identity_id
WHERE der.run_id = @run_id
  AND (
    sqlc.narg('agent_deployment_id')::uuid IS NULL
    OR ra.agent_deployment_id = sqlc.narg('agent_deployment_id')::uuid
  )
ORDER BY dil.dataset_example_id, ra.lane_index ASC, jr.created_at DESC NULLS LAST;

-- name: UpsertDatasetRegressionSuiteLink :one
INSERT INTO dataset_regression_suite_links (
    dataset_id,
    regression_suite_id,
    synced_version_id
) VALUES (
    @dataset_id,
    @regression_suite_id,
    @synced_version_id
)
ON CONFLICT (dataset_id) DO UPDATE SET
    regression_suite_id = EXCLUDED.regression_suite_id,
    synced_version_id = EXCLUDED.synced_version_id,
    updated_at = now()
RETURNING *;

-- name: GetDatasetRegressionSuiteLinkByDatasetID :one
SELECT *
FROM dataset_regression_suite_links
WHERE dataset_id = @dataset_id;
