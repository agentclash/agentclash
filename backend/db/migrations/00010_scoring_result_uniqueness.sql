-- +goose Up
CREATE UNIQUE INDEX judge_results_run_agent_spec_judge_key_idx
    ON judge_results (run_agent_id, evaluation_spec_id, judge_key);

CREATE UNIQUE INDEX metric_results_run_agent_spec_metric_key_idx
    ON metric_results (run_agent_id, evaluation_spec_id, metric_key);

-- +goose Down
DROP INDEX IF EXISTS metric_results_run_agent_spec_metric_key_idx;
DROP INDEX IF EXISTS judge_results_run_agent_spec_judge_key_idx;
