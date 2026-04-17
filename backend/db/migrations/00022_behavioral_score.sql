-- +goose Up
ALTER TABLE run_agent_scorecards
    ADD COLUMN behavioral_score numeric(7,4);

-- +goose Down
ALTER TABLE run_agent_scorecards
    DROP COLUMN behavioral_score;
