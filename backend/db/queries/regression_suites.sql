-- name: CreateRegressionSuite :one
INSERT INTO workspace_regression_suites (
    workspace_id,
    source_eval_pack_id,
    name,
    description,
    status,
    source_mode,
    default_gate_severity,
    created_by_user_id
) VALUES (
    @workspace_id,
    @source_eval_pack_id,
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
    source_eval_pack_id,
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

-- name: CountRegressionCasesBySuiteID :one
SELECT count(*)
FROM workspace_regression_cases
WHERE suite_id = @suite_id;

-- name: CountRegressionCasesBySuiteIDs :many
SELECT
    suite_id,
    count(*)::bigint AS case_count
FROM workspace_regression_cases
WHERE suite_id = ANY(@suite_ids::uuid[])
GROUP BY suite_id;

-- name: ListRegressionCasesByWorkspaceID :many
SELECT
    c.id,
    c.suite_id,
    s.workspace_id,
    s.name AS suite_name,
    c.title,
    c.description,
    c.status,
    c.severity,
    c.promotion_mode,
    c.source_run_id,
    c.source_run_agent_id,
    c.source_replay_id,
    c.source_eval_pack_version_id,
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
WHERE s.workspace_id = @workspace_id
  AND (sqlc.narg('status')::text IS NULL OR c.status = sqlc.narg('status')::text)
ORDER BY c.created_at DESC, c.id DESC
LIMIT @result_limit OFFSET @result_offset;

-- name: CountRegressionCasesByWorkspaceID :one
SELECT count(*)
FROM workspace_regression_cases c
JOIN workspace_regression_suites s ON s.id = c.suite_id
WHERE s.workspace_id = @workspace_id
  AND (sqlc.narg('status')::text IS NULL OR c.status = sqlc.narg('status')::text);

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
    source_eval_pack_version_id,
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
    @source_eval_pack_version_id,
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
    s.name AS suite_name,
    c.title,
    c.description,
    c.status,
    c.severity,
    c.promotion_mode,
    c.source_run_id,
    c.source_run_agent_id,
    c.source_replay_id,
    c.source_eval_pack_version_id,
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

-- name: GetRegressionCaseIDByPromotionSource :one
SELECT c.id
FROM workspace_regression_cases c
WHERE c.suite_id = @suite_id
  AND c.source_run_agent_id = @source_run_agent_id
  AND c.source_challenge_identity_id = @source_challenge_identity_id
LIMIT 1;

-- name: ListRegressionCasesBySuiteID :many
SELECT
    c.id,
    c.suite_id,
    s.workspace_id,
    s.name AS suite_name,
    c.title,
    c.description,
    c.status,
    c.severity,
    c.promotion_mode,
    c.source_run_id,
    c.source_run_agent_id,
    c.source_replay_id,
    c.source_eval_pack_version_id,
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

-- name: ListRegressionCaseValidationStatsBySuiteID :many
WITH selected_run_cases AS (
    SELECT DISTINCT
        c.id AS regression_case_id,
        rcs.run_id,
        rcs.challenge_identity_id
    FROM run_case_selections AS rcs
    JOIN workspace_regression_cases AS c
      ON c.id = rcs.regression_case_id
    WHERE c.suite_id = @suite_id
      AND rcs.regression_case_id IS NOT NULL
),
case_run_outcomes AS (
    SELECT
        src.regression_case_id,
        src.run_id,
        COALESCE(r.finished_at, r.started_at, r.created_at)::timestamptz AS validated_at,
        -- Treat any failed winning-agent judge verdict as a reproduced failure.
        CASE
            WHEN bool_or(jr.verdict = 'fail') THEN 'fail'
            WHEN bool_or(jr.verdict = 'pass') THEN 'pass'
            ELSE 'pending'
        END AS outcome
    FROM selected_run_cases AS src
    JOIN runs AS r
      ON r.id = src.run_id
     AND r.status = 'completed'
    JOIN run_scorecards AS rs
      ON rs.run_id = r.id
     AND rs.winning_run_agent_id IS NOT NULL
    LEFT JOIN judge_results AS jr
      ON jr.run_agent_id = rs.winning_run_agent_id
     AND (
        jr.regression_case_id = src.regression_case_id
        OR (
            jr.regression_case_id IS NULL
            -- Older judge rows only carry the challenge identity; use it as a fallback.
            AND jr.challenge_identity_id = src.challenge_identity_id
        )
     )
    GROUP BY src.regression_case_id, src.run_id, validated_at
),
scored_case_runs AS (
    SELECT *
    FROM case_run_outcomes
    WHERE outcome IN ('pass', 'fail')
)
SELECT
    regression_case_id,
    count(*)::bigint AS validation_run_count,
    count(*) FILTER (WHERE outcome = 'fail')::bigint AS validation_failure_count,
    count(*) FILTER (WHERE outcome = 'pass')::bigint AS validation_pass_count,
    ((count(*) FILTER (WHERE outcome = 'fail'))::float8 / count(*)::float8)::float8 AS reproduction_rate,
    (array_agg(outcome ORDER BY validated_at DESC, run_id DESC))[1]::text AS last_validation_outcome,
    max(validated_at)::timestamptz AS last_validated_at
FROM scored_case_runs
GROUP BY regression_case_id
ORDER BY regression_case_id;

-- name: ListRegressionCaseValidationStatsByCaseIDs :many
WITH selected_run_cases AS (
    SELECT DISTINCT
        rcs.regression_case_id,
        rcs.run_id,
        rcs.challenge_identity_id
    FROM run_case_selections AS rcs
    WHERE rcs.regression_case_id = ANY(@regression_case_ids::uuid[])
),
case_run_outcomes AS (
    SELECT
        src.regression_case_id,
        src.run_id,
        COALESCE(r.finished_at, r.started_at, r.created_at)::timestamptz AS validated_at,
        -- Treat any failed winning-agent judge verdict as a reproduced failure.
        CASE
            WHEN bool_or(jr.verdict = 'fail') THEN 'fail'
            WHEN bool_or(jr.verdict = 'pass') THEN 'pass'
            ELSE 'pending'
        END AS outcome
    FROM selected_run_cases AS src
    JOIN runs AS r
      ON r.id = src.run_id
     AND r.status = 'completed'
    JOIN run_scorecards AS rs
      ON rs.run_id = r.id
     AND rs.winning_run_agent_id IS NOT NULL
    LEFT JOIN judge_results AS jr
      ON jr.run_agent_id = rs.winning_run_agent_id
     AND (
        jr.regression_case_id = src.regression_case_id
        OR (
            jr.regression_case_id IS NULL
            -- Older judge rows only carry the challenge identity; use it as a fallback.
            AND jr.challenge_identity_id = src.challenge_identity_id
        )
     )
    GROUP BY src.regression_case_id, src.run_id, validated_at
),
scored_case_runs AS (
    SELECT *
    FROM case_run_outcomes
    WHERE outcome IN ('pass', 'fail')
)
SELECT
    regression_case_id,
    count(*)::bigint AS validation_run_count,
    count(*) FILTER (WHERE outcome = 'fail')::bigint AS validation_failure_count,
    count(*) FILTER (WHERE outcome = 'pass')::bigint AS validation_pass_count,
    ((count(*) FILTER (WHERE outcome = 'fail'))::float8 / count(*)::float8)::float8 AS reproduction_rate,
    (array_agg(outcome ORDER BY validated_at DESC, run_id DESC))[1]::text AS last_validation_outcome,
    max(validated_at)::timestamptz AS last_validated_at
FROM scored_case_runs
GROUP BY regression_case_id
ORDER BY regression_case_id;

-- name: GetRegressionCaseValidationStatsByCaseID :one
WITH selected_run_cases AS (
    SELECT DISTINCT
        @regression_case_id::uuid AS regression_case_id,
        rcs.run_id,
        rcs.challenge_identity_id
    FROM run_case_selections AS rcs
    WHERE rcs.regression_case_id = @regression_case_id::uuid
),
case_run_outcomes AS (
    SELECT
        src.regression_case_id,
        src.run_id,
        COALESCE(r.finished_at, r.started_at, r.created_at)::timestamptz AS validated_at,
        -- Treat any failed winning-agent judge verdict as a reproduced failure.
        CASE
            WHEN bool_or(jr.verdict = 'fail') THEN 'fail'
            WHEN bool_or(jr.verdict = 'pass') THEN 'pass'
            ELSE 'pending'
        END AS outcome
    FROM selected_run_cases AS src
    JOIN runs AS r
      ON r.id = src.run_id
     AND r.status = 'completed'
    JOIN run_scorecards AS rs
      ON rs.run_id = r.id
     AND rs.winning_run_agent_id IS NOT NULL
    LEFT JOIN judge_results AS jr
      ON jr.run_agent_id = rs.winning_run_agent_id
     AND (
        jr.regression_case_id = src.regression_case_id
        OR (
            jr.regression_case_id IS NULL
            -- Older judge rows only carry the challenge identity; use it as a fallback.
            AND jr.challenge_identity_id = src.challenge_identity_id
        )
     )
    GROUP BY src.regression_case_id, src.run_id, validated_at
),
scored_case_runs AS (
    SELECT *
    FROM case_run_outcomes
    WHERE outcome IN ('pass', 'fail')
)
SELECT
    regression_case_id,
    count(*)::bigint AS validation_run_count,
    count(*) FILTER (WHERE outcome = 'fail')::bigint AS validation_failure_count,
    count(*) FILTER (WHERE outcome = 'pass')::bigint AS validation_pass_count,
    ((count(*) FILTER (WHERE outcome = 'fail'))::float8 / count(*)::float8)::float8 AS reproduction_rate,
    (array_agg(outcome ORDER BY validated_at DESC, run_id DESC))[1]::text AS last_validation_outcome,
    max(validated_at)::timestamptz AS last_validated_at
FROM scored_case_runs
GROUP BY regression_case_id;

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

-- name: GetLatestRegressionPromotionByCaseID :one
SELECT *
FROM workspace_regression_promotions
WHERE workspace_regression_case_id = @workspace_regression_case_id
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListLatestRegressionPromotionsByCaseIDs :many
SELECT DISTINCT ON (workspace_regression_case_id)
    *
FROM workspace_regression_promotions
WHERE workspace_regression_case_id = ANY(@workspace_regression_case_ids::uuid[])
ORDER BY workspace_regression_case_id, created_at DESC, id DESC;
