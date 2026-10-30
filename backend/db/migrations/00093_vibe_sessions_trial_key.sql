-- +goose Up
CREATE INDEX vibe_sessions_trial_key_idx ON vibe_sessions(trial_key) WHERE trial_key IS NOT NULL;

-- +goose Down
DROP INDEX vibe_sessions_trial_key_idx;
