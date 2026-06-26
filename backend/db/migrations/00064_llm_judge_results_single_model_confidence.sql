-- +goose Up
-- Fix: the llm_judge_results.confidence CHECK constraint was out of sync with
-- the application. deriveJudgeConfidence (backend/internal/workflow/judges.go)
-- emits "single-model" for single-model judges, and the scorecard UI renders it
-- as a first-class confidence chip, but the original constraint from migration
-- 00021 only permitted ('high','medium','low'). Persisting a single-model judge
-- result therefore violated llm_judge_results_confidence_check (SQLSTATE 23514),
-- which failed scoring and, in turn, the entire run. Widen the allowed set to
-- include 'single-model'.
ALTER TABLE llm_judge_results
DROP CONSTRAINT IF EXISTS llm_judge_results_confidence_check;

ALTER TABLE llm_judge_results
ADD CONSTRAINT llm_judge_results_confidence_check
CHECK (confidence IS NULL OR confidence IN ('high', 'medium', 'low', 'single-model'));

-- +goose Down
ALTER TABLE llm_judge_results
DROP CONSTRAINT IF EXISTS llm_judge_results_confidence_check;

ALTER TABLE llm_judge_results
ADD CONSTRAINT llm_judge_results_confidence_check
CHECK (confidence IS NULL OR confidence IN ('high', 'medium', 'low'));
