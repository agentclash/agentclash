-- name: CreatePlayground :one
INSERT INTO playgrounds (
    organization_id,
    workspace_id,
    name,
    prompt_template,
    system_prompt,
    evaluation_spec,
    created_by_user_id,
    updated_by_user_id
)
VALUES (
    @organization_id,
    @workspace_id,
    @name,
    @prompt_template,
    @system_prompt,
    @evaluation_spec,
    sqlc.narg('created_by_user_id'),
    sqlc.narg('updated_by_user_id')
)
RETURNING *;

-- name: ListPlaygroundsByWorkspaceID :many
SELECT *
FROM playgrounds
WHERE workspace_id = @workspace_id
ORDER BY updated_at DESC;

-- name: GetPlaygroundByID :one
SELECT *
FROM playgrounds
WHERE id = @id
LIMIT 1;

-- name: UpdatePlayground :one
UPDATE playgrounds
SET
    name = @name,
    prompt_template = @prompt_template,
    system_prompt = @system_prompt,
    evaluation_spec = @evaluation_spec,
    updated_by_user_id = sqlc.narg('updated_by_user_id')
WHERE id = @id
RETURNING *;

-- name: DeletePlayground :exec
DELETE FROM playgrounds
WHERE id = @id;

-- name: CreatePlaygroundTestCase :one
INSERT INTO playground_test_cases (
    playground_id,
    case_key,
    variables,
    expectations
)
VALUES (
    @playground_id,
    @case_key,
    @variables,
    @expectations
)
RETURNING *;

-- name: ListPlaygroundTestCasesByPlaygroundID :many
SELECT *
FROM playground_test_cases
WHERE playground_id = @playground_id
ORDER BY case_key;

-- name: GetPlaygroundTestCaseByID :one
SELECT *
FROM playground_test_cases
WHERE id = @id
LIMIT 1;

-- name: UpdatePlaygroundTestCase :one
UPDATE playground_test_cases
SET
    case_key = @case_key,
    variables = @variables,
    expectations = @expectations
WHERE id = @id
RETURNING *;

-- name: DeletePlaygroundTestCase :exec
DELETE FROM playground_test_cases
WHERE id = @id;

-- name: CreatePlaygroundExperiment :one
INSERT INTO playground_experiments (
    organization_id,
    workspace_id,
    playground_id,
    provider_account_id,
    model_alias_id,
    name,
    status,
    request_config,
    summary,
    queued_at,
    created_by_user_id
)
VALUES (
    @organization_id,
    @workspace_id,
    @playground_id,
    @provider_account_id,
    @model_alias_id,
    @name,
    @status,
    @request_config,
    @summary,
    sqlc.narg('queued_at'),
    sqlc.narg('created_by_user_id')
)
RETURNING *;

-- name: ListPlaygroundExperimentsByPlaygroundID :many
SELECT *
FROM playground_experiments
WHERE playground_id = @playground_id
ORDER BY created_at DESC;

-- name: GetPlaygroundExperimentByID :one
SELECT *
FROM playground_experiments
WHERE id = @id
LIMIT 1;

-- name: SetPlaygroundExperimentTemporalIDs :one
UPDATE playground_experiments
SET
    temporal_workflow_id = @temporal_workflow_id,
    temporal_run_id = @temporal_run_id
WHERE id = @id
RETURNING *;

-- name: UpdatePlaygroundExperimentStatus :one
UPDATE playground_experiments
SET
    status = @status,
    summary = @summary,
    started_at = sqlc.narg('started_at'),
    finished_at = sqlc.narg('finished_at'),
    failed_at = sqlc.narg('failed_at')
WHERE id = @id
RETURNING *;

-- name: UpsertPlaygroundExperimentResult :one
INSERT INTO playground_experiment_results (
    playground_experiment_id,
    playground_test_case_id,
    case_key,
    status,
    variables,
    expectations,
    rendered_prompt,
    actual_output,
    provider_key,
    provider_model_id,
    input_tokens,
    output_tokens,
    total_tokens,
    latency_ms,
    cost_usd,
    validator_results,
    llm_judge_results,
    dimension_results,
    dimension_scores,
    warnings,
    error_message
)
VALUES (
    @playground_experiment_id,
    @playground_test_case_id,
    @case_key,
    @status,
    @variables,
    @expectations,
    @rendered_prompt,
    @actual_output,
    @provider_key,
    @provider_model_id,
    @input_tokens,
    @output_tokens,
    @total_tokens,
    @latency_ms,
    @cost_usd,
    @validator_results,
    @llm_judge_results,
    @dimension_results,
    @dimension_scores,
    @warnings,
    sqlc.narg('error_message')
)
ON CONFLICT (playground_experiment_id, playground_test_case_id)
DO UPDATE SET
    case_key = EXCLUDED.case_key,
    status = EXCLUDED.status,
    variables = EXCLUDED.variables,
    expectations = EXCLUDED.expectations,
    rendered_prompt = EXCLUDED.rendered_prompt,
    actual_output = EXCLUDED.actual_output,
    provider_key = EXCLUDED.provider_key,
    provider_model_id = EXCLUDED.provider_model_id,
    input_tokens = EXCLUDED.input_tokens,
    output_tokens = EXCLUDED.output_tokens,
    total_tokens = EXCLUDED.total_tokens,
    latency_ms = EXCLUDED.latency_ms,
    cost_usd = EXCLUDED.cost_usd,
    validator_results = EXCLUDED.validator_results,
    llm_judge_results = EXCLUDED.llm_judge_results,
    dimension_results = EXCLUDED.dimension_results,
    dimension_scores = EXCLUDED.dimension_scores,
    warnings = EXCLUDED.warnings,
    error_message = EXCLUDED.error_message
RETURNING *;

-- name: ListPlaygroundExperimentResultsByExperimentID :many
SELECT *
FROM playground_experiment_results
WHERE playground_experiment_id = @playground_experiment_id
ORDER BY case_key;
