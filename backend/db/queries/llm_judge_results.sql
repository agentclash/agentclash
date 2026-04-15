-- name: UpsertLLMJudgeResult :one
INSERT INTO llm_judge_results (
    run_agent_id,
    evaluation_spec_id,
    judge_key,
    mode,
    normalized_score,
    payload,
    confidence,
    variance,
    sample_count,
    model_count
) VALUES (
    @run_agent_id,
    @evaluation_spec_id,
    @judge_key,
    @mode,
    @normalized_score,
    @payload,
    @confidence,
    @variance,
    @sample_count,
    @model_count
)
ON CONFLICT (run_agent_id, evaluation_spec_id, judge_key)
DO UPDATE SET
    mode             = EXCLUDED.mode,
    normalized_score = EXCLUDED.normalized_score,
    payload          = EXCLUDED.payload,
    confidence       = EXCLUDED.confidence,
    variance         = EXCLUDED.variance,
    sample_count     = EXCLUDED.sample_count,
    model_count      = EXCLUDED.model_count,
    updated_at       = now()
RETURNING id, run_agent_id, evaluation_spec_id, judge_key, mode, normalized_score, payload, confidence, variance, sample_count, model_count, created_at, updated_at;

-- name: ListLLMJudgeResultsByRunAgentAndEvaluationSpec :many
SELECT id, run_agent_id, evaluation_spec_id, judge_key, mode, normalized_score, payload, confidence, variance, sample_count, model_count, created_at, updated_at
FROM llm_judge_results
WHERE run_agent_id = @run_agent_id
  AND evaluation_spec_id = @evaluation_spec_id
ORDER BY judge_key ASC;
