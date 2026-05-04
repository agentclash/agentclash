-- +goose Up
ALTER TABLE workspace_regression_cases
    DROP CONSTRAINT IF EXISTS workspace_regression_cases_status_check;

ALTER TABLE workspace_regression_cases
    ADD CONSTRAINT workspace_regression_cases_status_check
    CHECK (status IN ('proposed', 'active', 'archived', 'muted', 'rejected'));

-- +goose Down
UPDATE workspace_regression_cases
SET status = 'archived'
WHERE status IN ('proposed', 'rejected');

ALTER TABLE workspace_regression_cases
    DROP CONSTRAINT IF EXISTS workspace_regression_cases_status_check;

ALTER TABLE workspace_regression_cases
    ADD CONSTRAINT workspace_regression_cases_status_check
    CHECK (status IN ('active', 'archived', 'muted'));
