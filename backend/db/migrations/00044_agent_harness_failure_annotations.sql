-- +goose Up
CREATE TABLE agent_harness_failure_annotations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_harness_execution_id uuid NOT NULL REFERENCES agent_harness_executions (id) ON DELETE CASCADE,
    suggested_class text CHECK (suggested_class IS NULL OR suggested_class IN (
        'setup',
        'auth',
        'tool_misuse',
        'incomplete_implementation',
        'no_op_diff',
        'test_failure',
        'overbroad_diff',
        'no_pr',
        'judge_failure',
        'timeout',
        'policy_privacy',
        'infrastructure',
        'none',
        'unknown'
    )),
    suggested_summary text NOT NULL DEFAULT '',
    suggested_source text NOT NULL DEFAULT 'rules' CHECK (suggested_source IN ('rules', 'llm')),
    suggested_confidence numeric(5,4),
    suggested_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    human_class text CHECK (human_class IS NULL OR human_class IN (
        'setup',
        'auth',
        'tool_misuse',
        'incomplete_implementation',
        'no_op_diff',
        'test_failure',
        'overbroad_diff',
        'no_pr',
        'judge_failure',
        'timeout',
        'policy_privacy',
        'infrastructure',
        'none',
        'unknown'
    )),
    human_summary text NOT NULL DEFAULT '',
    human_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    edited_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_harness_execution_id)
);

CREATE INDEX agent_harness_failure_annotations_execution_idx
ON agent_harness_failure_annotations (agent_harness_execution_id);

CREATE TRIGGER agent_harness_failure_annotations_set_updated_at
BEFORE UPDATE ON agent_harness_failure_annotations
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS agent_harness_failure_annotations_set_updated_at ON agent_harness_failure_annotations;
DROP INDEX IF EXISTS agent_harness_failure_annotations_execution_idx;
DROP TABLE IF EXISTS agent_harness_failure_annotations;
