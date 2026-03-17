-- +goose Up
ALTER TABLE agent_build_versions
ADD CONSTRAINT agent_build_versions_interface_spec_object_check CHECK (jsonb_typeof(interface_spec) = 'object'),
ADD CONSTRAINT agent_build_versions_policy_spec_object_check CHECK (jsonb_typeof(policy_spec) = 'object'),
ADD CONSTRAINT agent_build_versions_reasoning_spec_object_check CHECK (jsonb_typeof(reasoning_spec) = 'object'),
ADD CONSTRAINT agent_build_versions_memory_spec_object_check CHECK (jsonb_typeof(memory_spec) = 'object'),
ADD CONSTRAINT agent_build_versions_workflow_spec_object_check CHECK (jsonb_typeof(workflow_spec) = 'object'),
ADD CONSTRAINT agent_build_versions_guardrail_spec_object_check CHECK (jsonb_typeof(guardrail_spec) = 'object'),
ADD CONSTRAINT agent_build_versions_model_spec_object_check CHECK (jsonb_typeof(model_spec) = 'object'),
ADD CONSTRAINT agent_build_versions_publication_spec_object_check CHECK (jsonb_typeof(publication_spec) = 'object');

ALTER TABLE agent_deployment_snapshots
ADD CONSTRAINT agent_deployment_snapshots_source_agent_spec_object_check CHECK (jsonb_typeof(source_agent_spec) = 'object');

-- +goose Down
ALTER TABLE agent_deployment_snapshots
DROP CONSTRAINT IF EXISTS agent_deployment_snapshots_source_agent_spec_object_check;

ALTER TABLE agent_build_versions
DROP CONSTRAINT IF EXISTS agent_build_versions_publication_spec_object_check,
DROP CONSTRAINT IF EXISTS agent_build_versions_model_spec_object_check,
DROP CONSTRAINT IF EXISTS agent_build_versions_guardrail_spec_object_check,
DROP CONSTRAINT IF EXISTS agent_build_versions_workflow_spec_object_check,
DROP CONSTRAINT IF EXISTS agent_build_versions_memory_spec_object_check,
DROP CONSTRAINT IF EXISTS agent_build_versions_reasoning_spec_object_check,
DROP CONSTRAINT IF EXISTS agent_build_versions_policy_spec_object_check,
DROP CONSTRAINT IF EXISTS agent_build_versions_interface_spec_object_check;
