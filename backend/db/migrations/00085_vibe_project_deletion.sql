-- +goose Up
ALTER TABLE vibe_sessions ADD COLUMN deleted_at timestamptz;
ALTER TABLE vibe_sessions ADD COLUMN cleanup_finished_at timestamptz;
-- +goose Down
ALTER TABLE vibe_sessions DROP COLUMN cleanup_finished_at;
ALTER TABLE vibe_sessions DROP COLUMN deleted_at;
