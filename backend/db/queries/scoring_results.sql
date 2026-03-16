-- name: UpsertJudgeResult :one
INSERT INTO judge_results (
    run_agent_id,
    evaluation_spec_id,
    challenge_identity_id,
    judge_key,
    verdict,
    normalized_score,
    raw_output
) VALUES (
    @run_agent_id,
    @evaluation_spec_id,
    @challenge_identity_id,
    @judge_key,
    @verdict,
    @normalized_score,
    @raw_output
)
ON CONFLICT (run_agent_id, evaluation_spec_id, judge_key)
DO UPDATE SET
    challenge_identity_id = EXCLUDED.challenge_identity_id,
    verdict = EXCLUDED.verdict,
    normalized_score = EXCLUDED.normalized_score,
    raw_output = EXCLUDED.raw_output
RETURNING id, run_agent_id, evaluation_spec_id, challenge_identity_id, judge_key, verdict, normalized_score, raw_output, created_at;

-- name: ListJudgeResultsByRunAgentAndEvaluationSpec :many
SELECT id, run_agent_id, evaluation_spec_id, challenge_identity_id, judge_key, verdict, normalized_score, raw_output, created_at
FROM judge_results
WHERE run_agent_id = @run_agent_id
  AND evaluation_spec_id = @evaluation_spec_id
ORDER BY judge_key ASC;

-- name: UpsertMetricResult :one
INSERT INTO metric_results (
    run_agent_id,
    evaluation_spec_id,
    challenge_identity_id,
    metric_key,
    metric_type,
    numeric_value,
    text_value,
    boolean_value,
    unit,
    metadata
) VALUES (
    @run_agent_id,
    @evaluation_spec_id,
    @challenge_identity_id,
    @metric_key,
    @metric_type,
    @numeric_value,
    @text_value,
    @boolean_value,
    @unit,
    @metadata
)
ON CONFLICT (run_agent_id, evaluation_spec_id, metric_key)
DO UPDATE SET
    challenge_identity_id = EXCLUDED.challenge_identity_id,
    metric_type = EXCLUDED.metric_type,
    numeric_value = EXCLUDED.numeric_value,
    text_value = EXCLUDED.text_value,
    boolean_value = EXCLUDED.boolean_value,
    unit = EXCLUDED.unit,
    metadata = EXCLUDED.metadata
RETURNING id, run_agent_id, evaluation_spec_id, challenge_identity_id, metric_key, metric_type, numeric_value, text_value, boolean_value, unit, metadata, created_at;

-- name: ListMetricResultsByRunAgentAndEvaluationSpec :many
SELECT id, run_agent_id, evaluation_spec_id, challenge_identity_id, metric_key, metric_type, numeric_value, text_value, boolean_value, unit, metadata, created_at
FROM metric_results
WHERE run_agent_id = @run_agent_id
  AND evaluation_spec_id = @evaluation_spec_id
ORDER BY metric_key ASC;
