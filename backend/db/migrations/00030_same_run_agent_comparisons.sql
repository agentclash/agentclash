-- +goose Up
DO $$
DECLARE
    constraint_name text;
BEGIN
    SELECT conname INTO constraint_name
    FROM pg_constraint
    WHERE conrelid = 'run_comparisons'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) = 'CHECK ((baseline_run_id <> candidate_run_id))'
    LIMIT 1;

    IF constraint_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE run_comparisons DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;

ALTER TABLE run_comparisons
ADD CONSTRAINT run_comparisons_distinct_runs_or_agents_check
CHECK (
    baseline_run_id <> candidate_run_id
    OR (
        baseline_run_agent_id IS NOT NULL
        AND candidate_run_agent_id IS NOT NULL
        AND baseline_run_agent_id <> candidate_run_agent_id
    )
);

-- +goose Down
ALTER TABLE run_comparisons
DROP CONSTRAINT IF EXISTS run_comparisons_distinct_runs_or_agents_check;

ALTER TABLE run_comparisons
ADD CONSTRAINT run_comparisons_check
CHECK (baseline_run_id <> candidate_run_id);
