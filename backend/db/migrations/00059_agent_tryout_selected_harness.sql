-- +goose Up
ALTER TABLE agent_tryouts
    ADD COLUMN selected_harness_kind text;

-- +goose Down
ALTER TABLE agent_tryouts
    DROP COLUMN IF EXISTS selected_harness_kind;
