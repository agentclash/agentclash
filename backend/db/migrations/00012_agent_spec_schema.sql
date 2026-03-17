-- +goose Up
ALTER TABLE agent_build_versions
ADD COLUMN agent_kind text NOT NULL DEFAULT 'llm_agent',
ADD COLUMN interface_spec jsonb NOT NULL DEFAULT '{}'::jsonb,
ADD COLUMN policy_spec jsonb NOT NULL DEFAULT '{}'::jsonb,
ADD COLUMN reasoning_spec jsonb NOT NULL DEFAULT '{}'::jsonb,
ADD COLUMN memory_spec jsonb NOT NULL DEFAULT '{}'::jsonb,
ADD COLUMN workflow_spec jsonb NOT NULL DEFAULT '{}'::jsonb,
ADD COLUMN guardrail_spec jsonb NOT NULL DEFAULT '{}'::jsonb,
ADD COLUMN model_spec jsonb NOT NULL DEFAULT '{}'::jsonb,
ADD COLUMN publication_spec jsonb NOT NULL DEFAULT '{}'::jsonb,
ADD CONSTRAINT agent_build_versions_agent_kind_check
CHECK (agent_kind IN ('llm_agent', 'workflow_agent', 'programmatic_agent', 'multi_agent_system', 'hosted_external'));

UPDATE agent_build_versions
SET policy_spec = jsonb_set(policy_spec, '{instructions}', to_jsonb(prompt_spec), true)
WHERE prompt_spec IS NOT NULL
  AND jsonb_typeof(policy_spec) = 'object'
  AND NOT (policy_spec ? 'instructions');

CREATE FUNCTION hydrate_agent_build_version_spec_fields()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF COALESCE(NULLIF(BTRIM(NEW.agent_kind), ''), '') = '' THEN
        NEW.agent_kind := 'llm_agent';
    END IF;

    IF NEW.policy_spec IS NULL THEN
        NEW.policy_spec := '{}'::jsonb;
    END IF;

    IF NEW.prompt_spec IS NOT NULL
       AND jsonb_typeof(NEW.policy_spec) = 'object'
       AND NOT (NEW.policy_spec ? 'instructions') THEN
        NEW.policy_spec := jsonb_set(NEW.policy_spec, '{instructions}', to_jsonb(NEW.prompt_spec), true);
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER agent_build_versions_hydrate_spec_fields
BEFORE INSERT OR UPDATE ON agent_build_versions
FOR EACH ROW
EXECUTE FUNCTION hydrate_agent_build_version_spec_fields();

ALTER TABLE agent_deployment_snapshots
ADD COLUMN source_agent_spec jsonb NOT NULL DEFAULT '{}'::jsonb;

CREATE FUNCTION build_agent_spec_snapshot(p_agent_build_version_id uuid)
RETURNS jsonb
LANGUAGE sql
STABLE
AS $$
    WITH tool_bindings AS (
        SELECT COALESCE(
            jsonb_agg(
                jsonb_build_object(
                    'tool_id', abvt.tool_id,
                    'binding_role', abvt.binding_role,
                    'binding_config', abvt.binding_config
                )
                ORDER BY abvt.tool_id
            ),
            '[]'::jsonb
        ) AS payload
        FROM agent_build_version_tools AS abvt
        WHERE abvt.agent_build_version_id = p_agent_build_version_id
    ),
    knowledge_bindings AS (
        SELECT COALESCE(
            jsonb_agg(
                jsonb_build_object(
                    'knowledge_source_id', abvks.knowledge_source_id,
                    'binding_role', abvks.binding_role,
                    'binding_config', abvks.binding_config
                )
                ORDER BY abvks.knowledge_source_id
            ),
            '[]'::jsonb
        ) AS payload
        FROM agent_build_version_knowledge_sources AS abvks
        WHERE abvks.agent_build_version_id = p_agent_build_version_id
    )
    SELECT jsonb_build_object(
        'agent_kind', abv.agent_kind,
        'interface_spec', abv.interface_spec,
        'policy_spec', abv.policy_spec,
        'reasoning_spec', abv.reasoning_spec,
        'memory_spec', abv.memory_spec,
        'workflow_spec', abv.workflow_spec,
        'guardrail_spec', abv.guardrail_spec,
        'model_spec', abv.model_spec,
        'output_schema', abv.output_schema,
        'trace_contract', abv.trace_contract,
        'publication_spec', abv.publication_spec,
        'tools', tool_bindings.payload,
        'knowledge_sources', knowledge_bindings.payload
    )
    FROM agent_build_versions AS abv
    CROSS JOIN tool_bindings
    CROSS JOIN knowledge_bindings
    WHERE abv.id = p_agent_build_version_id;
$$;

UPDATE agent_deployment_snapshots AS ads
SET source_agent_spec = build_agent_spec_snapshot(ads.source_agent_build_version_id)
WHERE source_agent_spec = '{}'::jsonb;

CREATE FUNCTION hydrate_agent_deployment_snapshot_spec()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.source_agent_spec IS NULL OR NEW.source_agent_spec = '{}'::jsonb THEN
        NEW.source_agent_spec := build_agent_spec_snapshot(NEW.source_agent_build_version_id);
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER agent_deployment_snapshots_hydrate_source_agent_spec
BEFORE INSERT OR UPDATE ON agent_deployment_snapshots
FOR EACH ROW
EXECUTE FUNCTION hydrate_agent_deployment_snapshot_spec();

-- +goose Down
DROP TRIGGER IF EXISTS agent_deployment_snapshots_hydrate_source_agent_spec ON agent_deployment_snapshots;
DROP FUNCTION IF EXISTS hydrate_agent_deployment_snapshot_spec();
DROP FUNCTION IF EXISTS build_agent_spec_snapshot(uuid);

ALTER TABLE agent_deployment_snapshots
DROP COLUMN IF EXISTS source_agent_spec;

DROP TRIGGER IF EXISTS agent_build_versions_hydrate_spec_fields ON agent_build_versions;
DROP FUNCTION IF EXISTS hydrate_agent_build_version_spec_fields();

ALTER TABLE agent_build_versions
DROP CONSTRAINT IF EXISTS agent_build_versions_agent_kind_check,
DROP COLUMN IF EXISTS publication_spec,
DROP COLUMN IF EXISTS model_spec,
DROP COLUMN IF EXISTS guardrail_spec,
DROP COLUMN IF EXISTS workflow_spec,
DROP COLUMN IF EXISTS memory_spec,
DROP COLUMN IF EXISTS reasoning_spec,
DROP COLUMN IF EXISTS policy_spec,
DROP COLUMN IF EXISTS interface_spec,
DROP COLUMN IF EXISTS agent_kind;
