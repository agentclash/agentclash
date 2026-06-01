-- name: CreateDatasetTraceImport :one
INSERT INTO dataset_trace_imports (
    dataset_id,
    source_platform,
    artifact_id,
    candidate_count,
    status,
    created_by
) VALUES (
    @dataset_id,
    @source_platform,
    @artifact_id,
    @candidate_count,
    @status,
    @created_by
)
RETURNING *;

-- name: CreateDatasetTraceCandidate :one
INSERT INTO dataset_trace_candidates (
    dataset_id,
    import_id,
    source_platform,
    source_trace_id,
    source_run_id,
    source_run_agent_id,
    external_id,
    input,
    output,
    expected,
    metadata,
    tags,
    status
) VALUES (
    @dataset_id,
    @import_id,
    @source_platform,
    @source_trace_id,
    @source_run_id,
    @source_run_agent_id,
    @external_id,
    @input,
    @output,
    @expected,
    @metadata,
    @tags,
    @status
)
RETURNING *;

-- name: GetDatasetTraceCandidateByID :one
SELECT *
FROM dataset_trace_candidates
WHERE id = @id;

-- name: ListDatasetTraceCandidates :many
SELECT *
FROM dataset_trace_candidates
WHERE dataset_id = @dataset_id
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountDatasetTraceCandidates :one
SELECT COUNT(*)::bigint AS count
FROM dataset_trace_candidates
WHERE dataset_id = @dataset_id
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'));

-- name: UpdateDatasetTraceCandidatePromotion :one
UPDATE dataset_trace_candidates
SET status = 'promoted',
    promoted_example_id = @promoted_example_id,
    expected = COALESCE(@expected, expected),
    updated_at = now()
WHERE id = @id
  AND status = 'pending'
RETURNING *;

-- name: MarkDatasetTraceCandidatePromotedIfPending :one
UPDATE dataset_trace_candidates
SET status = 'promoted',
    promoted_example_id = @promoted_example_id,
    expected = COALESCE(@expected, expected),
    updated_at = now()
WHERE id = @id
RETURNING *;
