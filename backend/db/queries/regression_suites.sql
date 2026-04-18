-- name: CreateRegressionSuite :one
INSERT INTO workspace_regression_suites (
    workspace_id,
    source_challenge_pack_id,
    name,
    description,
    status,
    source_mode,
    default_gate_severity,
    created_by_user_id
) VALUES (
    @workspace_id,
    @source_challenge_pack_id,
    @name,
    @description,
    @status,
    @source_mode,
    @default_gate_severity,
    @created_by_user_id
)
RETURNING
    id,
    workspace_id,
    source_challenge_pack_id,
    name,
    description,
    status,
    source_mode,
    default_gate_severity,
    created_by_user_id,
    created_at,
    updated_at;

-- name: GetRegressionSuiteByID :one
SELECT *
FROM workspace_regression_suites
WHERE id = @id
LIMIT 1;

-- name: ListRegressionSuitesByWorkspaceID :many
SELECT *
FROM workspace_regression_suites
WHERE workspace_id = @workspace_id
ORDER BY created_at DESC, id DESC
LIMIT @result_limit OFFSET @result_offset;

-- name: CountRegressionSuitesByWorkspaceID :one
SELECT count(*)
FROM workspace_regression_suites
WHERE workspace_id = @workspace_id;

-- name: PatchRegressionSuite :one
UPDATE workspace_regression_suites
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    status = COALESCE(sqlc.narg('to_status')::text, status),
    default_gate_severity = COALESCE(sqlc.narg('default_gate_severity'), default_gate_severity),
    updated_at = now()
WHERE id = @id
  AND (sqlc.narg('from_status')::text IS NULL OR status = sqlc.narg('from_status')::text)
RETURNING *;

-- name: CreateRegressionCase :one
INSERT INTO workspace_regression_cases (
    suite_id,
    title,
    description,
    status,
    severity,
    promotion_mode,
    source_run_id,
    source_run_agent_id,
    source_replay_id,
    source_challenge_pack_version_id,
    source_challenge_input_set_id,
    source_challenge_identity_id,
    source_case_key,
    source_item_key,
    evidence_tier,
    failure_class,
    failure_summary,
    payload_snapshot,
    expected_contract,
    validator_overrides,
    metadata
) VALUES (
    @suite_id,
    @title,
    @description,
    @status,
    @severity,
    @promotion_mode,
    sqlc.narg('source_run_id'),
    sqlc.narg('source_run_agent_id'),
    sqlc.narg('source_replay_id'),
    @source_challenge_pack_version_id,
    sqlc.narg('source_challenge_input_set_id'),
    @source_challenge_identity_id,
    @source_case_key,
    sqlc.narg('source_item_key'),
    @evidence_tier,
    @failure_class,
    @failure_summary,
    @payload_snapshot,
    @expected_contract,
    sqlc.narg('validator_overrides'),
    @metadata
)
RETURNING *;

-- name: GetRegressionCaseByID :one
SELECT
    c.id,
    c.suite_id,
    s.workspace_id,
    c.title,
    c.description,
    c.status,
    c.severity,
    c.promotion_mode,
    c.source_run_id,
    c.source_run_agent_id,
    c.source_replay_id,
    c.source_challenge_pack_version_id,
    c.source_challenge_input_set_id,
    c.source_challenge_identity_id,
    c.source_case_key,
    c.source_item_key,
    c.evidence_tier,
    c.failure_class,
    c.failure_summary,
    c.payload_snapshot,
    c.expected_contract,
    c.validator_overrides,
    c.metadata,
    c.created_at,
    c.updated_at
FROM workspace_regression_cases c
JOIN workspace_regression_suites s ON s.id = c.suite_id
WHERE c.id = @id
LIMIT 1;

-- name: ListRegressionCasesBySuiteID :many
SELECT
    c.id,
    c.suite_id,
    s.workspace_id,
    c.title,
    c.description,
    c.status,
    c.severity,
    c.promotion_mode,
    c.source_run_id,
    c.source_run_agent_id,
    c.source_replay_id,
    c.source_challenge_pack_version_id,
    c.source_challenge_input_set_id,
    c.source_challenge_identity_id,
    c.source_case_key,
    c.source_item_key,
    c.evidence_tier,
    c.failure_class,
    c.failure_summary,
    c.payload_snapshot,
    c.expected_contract,
    c.validator_overrides,
    c.metadata,
    c.created_at,
    c.updated_at
FROM workspace_regression_cases c
JOIN workspace_regression_suites s ON s.id = c.suite_id
WHERE c.suite_id = @suite_id
ORDER BY c.created_at DESC, c.id DESC;

-- name: PatchRegressionCase :one
UPDATE workspace_regression_cases
SET title = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    status = COALESCE(sqlc.narg('to_status')::text, status),
    severity = COALESCE(sqlc.narg('severity'), severity),
    updated_at = now()
WHERE id = @id
  AND (sqlc.narg('from_status')::text IS NULL OR status = sqlc.narg('from_status')::text)
RETURNING *;

-- name: CreateRegressionPromotion :one
INSERT INTO workspace_regression_promotions (
    workspace_regression_case_id,
    source_run_id,
    source_run_agent_id,
    source_event_refs,
    promoted_by_user_id,
    promotion_reason,
    promotion_snapshot
) VALUES (
    @workspace_regression_case_id,
    @source_run_id,
    @source_run_agent_id,
    @source_event_refs,
    @promoted_by_user_id,
    @promotion_reason,
    @promotion_snapshot
)
RETURNING *;
