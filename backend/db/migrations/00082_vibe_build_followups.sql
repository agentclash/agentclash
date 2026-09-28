-- +goose Up
-- A clarification may be interrupted by chat. Each submitted message has its
-- own idempotency identity; only the initial check remains a singleton step.
ALTER TABLE vibe_cycle_steps DROP CONSTRAINT vibe_cycle_steps_step_check;
ALTER TABLE vibe_cycle_steps ADD CONSTRAINT vibe_cycle_steps_step_check
 CHECK(step IN ('prepare','answer','check') OR step LIKE 'retry:%' OR step LIKE 'message:%');

-- +goose Down
-- Refuse a downgrade with newer follow-ups rather than discarding their receipts.
ALTER TABLE vibe_cycle_steps DROP CONSTRAINT vibe_cycle_steps_step_check;
ALTER TABLE vibe_cycle_steps ADD CONSTRAINT vibe_cycle_steps_step_check
 CHECK(step IN ('prepare','answer','check') OR step LIKE 'retry:%');
