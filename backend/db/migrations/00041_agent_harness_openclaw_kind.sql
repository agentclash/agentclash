-- +goose Up
ALTER TABLE agent_harnesses
DROP CONSTRAINT IF EXISTS agent_harnesses_harness_kind_check;

ALTER TABLE agent_harnesses
ADD CONSTRAINT agent_harnesses_harness_kind_check
CHECK (harness_kind IN ('codex_e2b', 'claude_e2b', 'hermes_e2b', 'openclaw_e2b'));

-- +goose Down
ALTER TABLE agent_harnesses
DROP CONSTRAINT IF EXISTS agent_harnesses_harness_kind_check;

ALTER TABLE agent_harnesses
ADD CONSTRAINT agent_harnesses_harness_kind_check
CHECK (harness_kind IN ('codex_e2b'));
