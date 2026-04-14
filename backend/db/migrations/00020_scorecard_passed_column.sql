-- +goose Up
-- Phase 5 of issue #147: expose scorecard-level pass/fail as a typed column so
-- downstream consumers (leaderboards, release gate, filters) can read the
-- verdict without decoding the JSONB payload on every query. The column stays
-- nullable because pre-Phase-5 scorecards may not have persisted a verdict,
-- and because "evaluation partial" runs legitimately have no pass decision.
ALTER TABLE run_agent_scorecards
    ADD COLUMN scorecard_passed boolean;

-- Backfill from the JSONB payload so historical rows become queryable without
-- a rescore pass. Rows that never had a `passed` field stay NULL.
UPDATE run_agent_scorecards
   SET scorecard_passed = (scorecard ->> 'passed')::boolean
 WHERE scorecard ? 'passed'
   AND scorecard ->> 'passed' IN ('true', 'false');

-- +goose Down
ALTER TABLE run_agent_scorecards DROP COLUMN scorecard_passed;
