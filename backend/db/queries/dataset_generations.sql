-- name: CreateDatasetGenerationJob :one
INSERT INTO dataset_generation_jobs (
    dataset_id,
    workspace_id,
    strategy,
    status,
    config,
    summary,
    target_count,
    created_by,
    queued_at
) VALUES (
    @dataset_id,
    @workspace_id,
    @strategy,
    @status,
    @config,
    @summary,
    @target_count,
    @created_by,
    @queued_at
)
RETURNING *;

-- name: GetDatasetGenerationJobByID :one
SELECT *
FROM dataset_generation_jobs
WHERE id = @id
LIMIT 1;

-- name: SetDatasetGenerationJobTemporalIDs :one
UPDATE dataset_generation_jobs
SET temporal_workflow_id = @temporal_workflow_id,
    temporal_run_id = @temporal_run_id,
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: UpdateDatasetGenerationJobStatus :one
UPDATE dataset_generation_jobs
SET status = @status,
    summary = COALESCE(sqlc.narg('summary'), summary),
    version_id = COALESCE(sqlc.narg('version_id'), version_id),
    error_message = sqlc.narg('error_message'),
    started_at = COALESCE(sqlc.narg('started_at'), started_at),
    finished_at = COALESCE(sqlc.narg('finished_at'), finished_at),
    failed_at = COALESCE(sqlc.narg('failed_at'), failed_at),
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: UpdateDatasetGenerationJobProgress :one
UPDATE dataset_generation_jobs
SET generated_count = @generated_count,
    accepted_count = @accepted_count,
    rejected_count = @rejected_count,
    total_input_tokens = @total_input_tokens,
    total_output_tokens = @total_output_tokens,
    total_cost_usd = @total_cost_usd,
    summary = COALESCE(sqlc.narg('summary'), summary),
    version_id = COALESCE(sqlc.narg('version_id'), version_id),
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: CreateDatasetGenerationRejection :one
INSERT INTO dataset_generation_rejections (
    job_id,
    reason_code,
    reason_detail,
    candidate_input,
    candidate_expected,
    metadata
) VALUES (
    @job_id,
    @reason_code,
    @reason_detail,
    @candidate_input,
    @candidate_expected,
    @metadata
)
RETURNING *;

-- name: ListDatasetGenerationRejectionsByJobID :many
SELECT *
FROM dataset_generation_rejections
WHERE job_id = @job_id
ORDER BY created_at ASC, id ASC
LIMIT @result_limit OFFSET @result_offset;

-- name: CountDatasetGenerationRejectionsByJobID :one
SELECT count(*)
FROM dataset_generation_rejections
WHERE job_id = @job_id;
