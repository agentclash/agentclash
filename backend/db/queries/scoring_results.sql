-- name: UpsertJudgeResult :one
INSERT INTO judge_results (
    run_agent_id,
    evaluation_spec_id,
    challenge_identity_id,
    regression_case_id,
    judge_key,
    verdict,
    normalized_score,
    raw_output
) VALUES (
    @run_agent_id,
    @evaluation_spec_id,
    @challenge_identity_id,
    sqlc.narg('regression_case_id'),
    @judge_key,
    @verdict,
    @normalized_score,
    @raw_output
)
ON CONFLICT (run_agent_id, evaluation_spec_id, judge_key)
DO UPDATE SET
    challenge_identity_id = EXCLUDED.challenge_identity_id,
    regression_case_id = EXCLUDED.regression_case_id,
    verdict = EXCLUDED.verdict,
    normalized_score = EXCLUDED.normalized_score,
    raw_output = EXCLUDED.raw_output
RETURNING id, run_agent_id, evaluation_spec_id, challenge_identity_id, regression_case_id, judge_key, verdict, normalized_score, raw_output, created_at;

-- name: ListJudgeResultsByRunAgentAndEvaluationSpec :many
SELECT id, run_agent_id, evaluation_spec_id, challenge_identity_id, regression_case_id, judge_key, verdict, normalized_score, raw_output, created_at
FROM judge_results
WHERE run_agent_id = @run_agent_id
  AND evaluation_spec_id = @evaluation_spec_id
ORDER BY judge_key ASC;

-- name: UpsertMetricResult :one
INSERT INTO metric_results (
    run_agent_id,
    evaluation_spec_id,
    challenge_identity_id,
    regression_case_id,
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
    sqlc.narg('regression_case_id'),
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
    regression_case_id = EXCLUDED.regression_case_id,
    metric_type = EXCLUDED.metric_type,
    numeric_value = EXCLUDED.numeric_value,
    text_value = EXCLUDED.text_value,
    boolean_value = EXCLUDED.boolean_value,
    unit = EXCLUDED.unit,
    metadata = EXCLUDED.metadata
RETURNING id, run_agent_id, evaluation_spec_id, challenge_identity_id, regression_case_id, metric_key, metric_type, numeric_value, text_value, boolean_value, unit, metadata, created_at;

-- name: ListMetricResultsByRunAgentAndEvaluationSpec :many
SELECT id, run_agent_id, evaluation_spec_id, challenge_identity_id, regression_case_id, metric_key, metric_type, numeric_value, text_value, boolean_value, unit, metadata, created_at
FROM metric_results
WHERE run_agent_id = @run_agent_id
  AND evaluation_spec_id = @evaluation_spec_id
ORDER BY metric_key ASC;

-- name: UpsertRunAgentScorecard :one
INSERT INTO run_agent_scorecards (
    run_agent_id,
    evaluation_spec_id,
    overall_score,
    correctness_score,
    reliability_score,
    latency_score,
    cost_score,
    behavioral_score,
    scorecard_passed,
    scorecard
) VALUES (
    @run_agent_id,
    @evaluation_spec_id,
    @overall_score,
    @correctness_score,
    @reliability_score,
    @latency_score,
    @cost_score,
    @behavioral_score,
    @scorecard_passed,
    @scorecard
)
ON CONFLICT (run_agent_id)
DO UPDATE SET
    evaluation_spec_id = EXCLUDED.evaluation_spec_id,
    overall_score = EXCLUDED.overall_score,
    correctness_score = EXCLUDED.correctness_score,
    reliability_score = EXCLUDED.reliability_score,
    latency_score = EXCLUDED.latency_score,
    cost_score = EXCLUDED.cost_score,
    behavioral_score = EXCLUDED.behavioral_score,
    scorecard_passed = EXCLUDED.scorecard_passed,
    scorecard = EXCLUDED.scorecard
RETURNING id, run_agent_id, evaluation_spec_id, overall_score, correctness_score, reliability_score, latency_score, cost_score, behavioral_score, scorecard_passed, scorecard, created_at, updated_at;

-- name: GetRunScorecardByRunID :one
SELECT id, run_id, evaluation_spec_id, winning_run_agent_id, scorecard, created_at, updated_at
FROM run_scorecards
WHERE run_id = @run_id;

-- name: UpsertRunScorecard :one
INSERT INTO run_scorecards (
    run_id,
    evaluation_spec_id,
    winning_run_agent_id,
    scorecard
) VALUES (
    @run_id,
    @evaluation_spec_id,
    @winning_run_agent_id,
    @scorecard
)
ON CONFLICT (run_id)
DO UPDATE SET
    evaluation_spec_id = EXCLUDED.evaluation_spec_id,
    winning_run_agent_id = EXCLUDED.winning_run_agent_id,
    scorecard = EXCLUDED.scorecard
RETURNING id, run_id, evaluation_spec_id, winning_run_agent_id, scorecard, created_at, updated_at;
