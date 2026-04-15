-- +goose Up
-- Phase 2 of issue #148: dedicated table for LLM-as-judge results.
--
-- DELIBERATE DEVIATION from the original Phase 2 plan in
-- backend/.claude/analysis/issue-148-deep-analysis.md: that plan
-- proposed extending the existing judge_results table. Auditing the
-- repository write path (repository.go StoreRunAgentEvaluationResults)
-- revealed judge_results is misnamed — it actually stores deterministic
-- validator results, not LLM judge results. Rather than muddy that
-- table with mode/payload/confidence columns that would all be NULL
-- for the validator-result rows, this migration creates a dedicated
-- table for the LLM-as-judge pipeline. The existing judge_results
-- table stays as-is; renaming it to validator_results is deferred to
-- a future housekeeping issue.
CREATE TABLE llm_judge_results (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    run_agent_id       uuid        NOT NULL REFERENCES run_agents      (id) ON DELETE CASCADE,
    evaluation_spec_id uuid        NOT NULL REFERENCES evaluation_specs (id) ON DELETE CASCADE,
    judge_key          text        NOT NULL,
    mode               text        NOT NULL CHECK (mode IN ('rubric', 'assertion', 'n_wise', 'reference')),
    normalized_score   numeric(7,4),
    payload            jsonb       NOT NULL DEFAULT '{}'::jsonb,
    confidence         text        CHECK (confidence IS NULL OR confidence IN ('high', 'medium', 'low')),
    variance           numeric(7,4),
    sample_count       integer     NOT NULL DEFAULT 0 CHECK (sample_count >= 0),
    model_count        integer     NOT NULL DEFAULT 0 CHECK (model_count >= 0),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_agent_id, evaluation_spec_id, judge_key)
);

CREATE INDEX llm_judge_results_run_agent_id_idx
    ON llm_judge_results (run_agent_id);

-- +goose Down
DROP INDEX IF EXISTS llm_judge_results_run_agent_id_idx;
DROP TABLE IF EXISTS llm_judge_results;
