-- name: GetRunComparisonByRunIDs :one
SELECT
    id,
    baseline_run_id,
    candidate_run_id,
    baseline_run_agent_id,
    candidate_run_agent_id,
    status,
    reason_code,
    source_fingerprint,
    summary,
    created_at,
    updated_at
FROM run_comparisons
WHERE baseline_run_id = @baseline_run_id
  AND candidate_run_id = @candidate_run_id
LIMIT 1;

-- name: UpsertRunComparison :one
INSERT INTO run_comparisons (
    baseline_run_id,
    candidate_run_id,
    baseline_run_agent_id,
    candidate_run_agent_id,
    status,
    reason_code,
    source_fingerprint,
    summary
)
VALUES (
    @baseline_run_id,
    @candidate_run_id,
    sqlc.narg('baseline_run_agent_id'),
    sqlc.narg('candidate_run_agent_id'),
    @status,
    sqlc.narg('reason_code'),
    @source_fingerprint,
    @summary
)
ON CONFLICT (baseline_run_id, candidate_run_id)
DO UPDATE SET
    baseline_run_agent_id = EXCLUDED.baseline_run_agent_id,
    candidate_run_agent_id = EXCLUDED.candidate_run_agent_id,
    status = EXCLUDED.status,
    reason_code = EXCLUDED.reason_code,
    source_fingerprint = EXCLUDED.source_fingerprint,
    summary = EXCLUDED.summary
RETURNING
    id,
    baseline_run_id,
    candidate_run_id,
    baseline_run_agent_id,
    candidate_run_agent_id,
    status,
    reason_code,
    source_fingerprint,
    summary,
    created_at,
    updated_at;
