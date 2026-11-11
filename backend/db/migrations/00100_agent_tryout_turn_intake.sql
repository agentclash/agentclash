-- +goose Up
-- The worker closes the turn intake of a public tryout when its turn loop
-- ends. After that, the API accepts no new turn. A turn that was still
-- pending at the close becomes 'dropped': no session will answer it.
ALTER TABLE agent_tryouts ADD COLUMN turns_closed_at timestamptz;

ALTER TABLE agent_tryout_turns DROP CONSTRAINT agent_tryout_turns_status_check;
ALTER TABLE agent_tryout_turns ADD CONSTRAINT agent_tryout_turns_status_check
    CHECK (status IN ('pending', 'processing', 'done', 'dropped'));

-- +goose Down
-- Before this migration, a turn that no session answered stayed pending.
UPDATE agent_tryout_turns
SET status = 'pending', processed_at = NULL
WHERE status = 'dropped';

ALTER TABLE agent_tryout_turns DROP CONSTRAINT agent_tryout_turns_status_check;
ALTER TABLE agent_tryout_turns ADD CONSTRAINT agent_tryout_turns_status_check
    CHECK (status IN ('pending', 'processing', 'done'));

ALTER TABLE agent_tryouts DROP COLUMN turns_closed_at;
