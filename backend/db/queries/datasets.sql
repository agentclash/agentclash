-- name: CreateDataset :one
INSERT INTO datasets (
    organization_id,
    workspace_id,
    slug,
    name,
    description,
    input_schema,
    input_schema_enforced,
    default_challenge_pack_version_id,
    created_by
)
SELECT
    w.organization_id,
    w.id,
    @slug,
    @name,
    @description,
    sqlc.narg('input_schema'),
    @input_schema_enforced,
    sqlc.narg('default_challenge_pack_version_id'),
    @created_by
FROM workspaces w
WHERE w.id = @workspace_id
RETURNING *;

-- name: GetDatasetByID :one
SELECT
    d.id, d.organization_id, d.workspace_id, d.slug, d.name, d.description, d.input_schema, d.input_schema_enforced, d.default_challenge_pack_version_id, d.created_by, d.created_at, d.updated_at, d.archived_at,
    count(DISTINCT e.id) FILTER (WHERE e.status = 'active')::bigint AS active_example_count,
    count(DISTINCT v.id)::bigint AS version_count
FROM datasets d
LEFT JOIN dataset_examples e ON e.dataset_id = d.id
LEFT JOIN dataset_versions v ON v.dataset_id = d.id
WHERE d.id = @id
GROUP BY d.id
LIMIT 1;

-- name: LockActiveDatasetForVersion :one
SELECT id
FROM datasets
WHERE id = @id
  AND archived_at IS NULL
FOR UPDATE;

-- name: ListDatasetsByWorkspaceID :many
SELECT
    d.*,
    count(DISTINCT e.id) FILTER (WHERE e.status = 'active')::bigint AS active_example_count,
    count(DISTINCT v.id)::bigint AS version_count
FROM datasets d
LEFT JOIN dataset_examples e ON e.dataset_id = d.id
LEFT JOIN dataset_versions v ON v.dataset_id = d.id
WHERE d.workspace_id = @workspace_id
  AND d.archived_at IS NULL
GROUP BY d.id
ORDER BY d.created_at DESC, d.id DESC
LIMIT @result_limit OFFSET @result_offset;

-- name: CountDatasetsByWorkspaceID :one
SELECT count(*)
FROM datasets
WHERE workspace_id = @workspace_id
  AND archived_at IS NULL;

-- name: PatchDataset :one
UPDATE datasets
SET slug = COALESCE(sqlc.narg('slug'), slug),
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    input_schema = COALESCE(sqlc.narg('input_schema'), input_schema),
    input_schema_enforced = COALESCE(sqlc.narg('input_schema_enforced'), input_schema_enforced),
    default_challenge_pack_version_id = COALESCE(sqlc.narg('default_challenge_pack_version_id'), default_challenge_pack_version_id)
WHERE id = @id
  AND archived_at IS NULL
RETURNING *;

-- name: ArchiveDataset :one
UPDATE datasets
SET archived_at = now()
WHERE id = @id
  AND archived_at IS NULL
RETURNING *;

-- name: InsertDatasetExample :one
INSERT INTO dataset_examples (
    dataset_id,
    external_id,
    input,
    expected,
    metadata,
    tags,
    status,
    source,
    source_run_id,
    source_trace_id,
    source_platform,
    artifact_id,
    created_by
) VALUES (
    @dataset_id,
    sqlc.narg('external_id'),
    @input,
    sqlc.narg('expected'),
    @metadata,
    @tags,
    @status,
    @source,
    sqlc.narg('source_run_id'),
    sqlc.narg('source_trace_id'),
    sqlc.narg('source_platform'),
    sqlc.narg('artifact_id'),
    @created_by
)
RETURNING *;

-- name: UpdateDatasetExample :one
UPDATE dataset_examples
SET input = @input,
    expected = sqlc.narg('expected'),
    metadata = @metadata,
    tags = @tags,
    status = @status,
    source = @source,
    source_run_id = sqlc.narg('source_run_id'),
    source_trace_id = sqlc.narg('source_trace_id'),
    source_platform = sqlc.narg('source_platform'),
    artifact_id = sqlc.narg('artifact_id')
WHERE id = @id
RETURNING *;

-- name: GetDatasetExampleByID :one
SELECT *
FROM dataset_examples
WHERE id = @id
LIMIT 1;

-- name: GetDatasetExampleByExternalID :one
SELECT *
FROM dataset_examples
WHERE dataset_id = @dataset_id
  AND external_id = @external_id
LIMIT 1;

-- name: ListDatasetExamplesByDatasetID :many
SELECT *
FROM dataset_examples
WHERE dataset_id = @dataset_id
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC, id DESC
LIMIT @result_limit OFFSET @result_offset;

-- name: CountDatasetExamplesByDatasetID :one
SELECT count(*)
FROM dataset_examples
WHERE dataset_id = @dataset_id
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text);

-- name: PatchDatasetExample :one
UPDATE dataset_examples
SET input = COALESCE(sqlc.narg('input'), input),
    expected = COALESCE(sqlc.narg('expected'), expected),
    metadata = COALESCE(sqlc.narg('metadata'), metadata),
    tags = COALESCE(sqlc.narg('tags'), tags),
    status = COALESCE(sqlc.narg('status')::text, status),
    source = COALESCE(sqlc.narg('source')::text, source),
    source_run_id = COALESCE(sqlc.narg('source_run_id'), source_run_id),
    source_trace_id = COALESCE(sqlc.narg('source_trace_id'), source_trace_id),
    source_platform = COALESCE(sqlc.narg('source_platform'), source_platform),
    artifact_id = COALESCE(sqlc.narg('artifact_id'), artifact_id)
WHERE id = @id
RETURNING *;

-- name: InsertDatasetExampleRevision :one
INSERT INTO dataset_example_revisions (
    dataset_id,
    example_id,
    version_id,
    operation,
    before,
    after,
    actor
) VALUES (
    @dataset_id,
    sqlc.narg('example_id'),
    sqlc.narg('version_id'),
    @operation,
    sqlc.narg('before'),
    sqlc.narg('after'),
    @actor
)
RETURNING *;

-- name: NextDatasetVersionNumber :one
SELECT COALESCE(max(version_number), 0)::int + 1
FROM dataset_versions
WHERE dataset_id = @dataset_id;

-- name: CreateDatasetVersion :one
INSERT INTO dataset_versions (
    dataset_id,
    version_number,
    label,
    example_count,
    manifest_checksum,
    created_by
) VALUES (
    @dataset_id,
    @version_number,
    sqlc.narg('label'),
    @example_count,
    @manifest_checksum,
    @created_by
)
RETURNING *;

-- name: ListLatestDatasetExampleRevisions :many
SELECT DISTINCT ON (dataset_example_revisions.example_id) dataset_example_revisions.*
FROM dataset_example_revisions
JOIN dataset_examples e ON e.id = dataset_example_revisions.example_id
WHERE dataset_example_revisions.dataset_id = @dataset_id
  AND dataset_example_revisions.example_id IS NOT NULL
  AND e.status <> 'archived'
ORDER BY example_id, dataset_example_revisions.created_at DESC, dataset_example_revisions.id DESC;

-- name: InsertDatasetVersionExample :exec
INSERT INTO dataset_version_examples (
    version_id,
    example_id,
    revision_id
) VALUES (
    @version_id,
    @example_id,
    @revision_id
);

-- name: ListDatasetVersionsByDatasetID :many
SELECT *
FROM dataset_versions
WHERE dataset_id = @dataset_id
ORDER BY version_number DESC;

-- name: GetDatasetVersionByID :one
SELECT *
FROM dataset_versions
WHERE id = @id
LIMIT 1;

-- name: ListDatasetVersionExamples :many
SELECT e.*
FROM dataset_version_examples ve
JOIN dataset_examples e ON e.id = ve.example_id
WHERE ve.version_id = @version_id
ORDER BY e.created_at DESC, e.id DESC;
