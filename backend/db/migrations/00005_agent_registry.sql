-- +goose Up
CREATE TABLE runtime_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid,
    name text NOT NULL,
    slug text NOT NULL,
    execution_target text NOT NULL CHECK (execution_target IN ('native', 'hosted_external')),
    trace_mode text NOT NULL DEFAULT 'required' CHECK (trace_mode IN ('required', 'preferred', 'disabled')),
    max_iterations integer NOT NULL DEFAULT 1 CHECK (max_iterations > 0),
    max_tool_calls integer NOT NULL DEFAULT 0 CHECK (max_tool_calls >= 0),
    step_timeout_seconds integer NOT NULL DEFAULT 60 CHECK (step_timeout_seconds > 0),
    run_timeout_seconds integer NOT NULL DEFAULT 300 CHECK (run_timeout_seconds > 0),
    profile_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, organization_id)
);

ALTER TABLE runtime_profiles
ADD CONSTRAINT runtime_profiles_workspace_fk
FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE;

CREATE UNIQUE INDEX runtime_profiles_org_slug_uq
ON runtime_profiles (organization_id, slug)
WHERE workspace_id IS NULL;

CREATE UNIQUE INDEX runtime_profiles_workspace_slug_uq
ON runtime_profiles (workspace_id, slug)
WHERE workspace_id IS NOT NULL;

CREATE TABLE tools (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid,
    name text NOT NULL,
    slug text NOT NULL,
    tool_kind text NOT NULL,
    capability_key text NOT NULL,
    definition jsonb NOT NULL DEFAULT '{}'::jsonb,
    lifecycle_status text NOT NULL DEFAULT 'active' CHECK (lifecycle_status IN ('active', 'disabled', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, organization_id)
);

ALTER TABLE tools
ADD CONSTRAINT tools_workspace_fk
FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE;

CREATE UNIQUE INDEX tools_org_slug_uq
ON tools (organization_id, slug)
WHERE workspace_id IS NULL;

CREATE UNIQUE INDEX tools_workspace_slug_uq
ON tools (workspace_id, slug)
WHERE workspace_id IS NOT NULL;

CREATE TABLE knowledge_sources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid,
    name text NOT NULL,
    slug text NOT NULL,
    source_kind text NOT NULL,
    connection_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    lifecycle_status text NOT NULL DEFAULT 'active' CHECK (lifecycle_status IN ('active', 'syncing', 'error', 'archived')),
    last_synced_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, organization_id)
);

ALTER TABLE knowledge_sources
ADD CONSTRAINT knowledge_sources_workspace_fk
FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE;

CREATE UNIQUE INDEX knowledge_sources_org_slug_uq
ON knowledge_sources (organization_id, slug)
WHERE workspace_id IS NULL;

CREATE UNIQUE INDEX knowledge_sources_workspace_slug_uq
ON knowledge_sources (workspace_id, slug)
WHERE workspace_id IS NOT NULL;

CREATE TABLE agent_builds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    name text NOT NULL,
    slug text NOT NULL,
    description text,
    build_kind text NOT NULL DEFAULT 'private' CHECK (build_kind IN ('private', 'template')),
    lifecycle_status text NOT NULL DEFAULT 'active' CHECK (lifecycle_status IN ('active', 'archived')),
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, organization_id),
    UNIQUE (id, organization_id, workspace_id),
    UNIQUE (workspace_id, slug),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE
);

CREATE TABLE agent_build_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_build_id uuid NOT NULL REFERENCES agent_builds (id) ON DELETE CASCADE,
    version_number integer NOT NULL CHECK (version_number > 0),
    version_status text NOT NULL DEFAULT 'draft' CHECK (version_status IN ('draft', 'ready', 'archived')),
    build_definition jsonb NOT NULL DEFAULT '{}'::jsonb,
    prompt_spec text,
    output_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    trace_contract jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_build_id, version_number),
    UNIQUE (id, agent_build_id)
);

CREATE TABLE agent_build_version_tools (
    agent_build_version_id uuid NOT NULL REFERENCES agent_build_versions (id) ON DELETE CASCADE,
    tool_id uuid NOT NULL REFERENCES tools (id) ON DELETE RESTRICT,
    binding_role text NOT NULL DEFAULT 'default',
    binding_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_build_version_id, tool_id)
);

CREATE TABLE agent_build_version_knowledge_sources (
    agent_build_version_id uuid NOT NULL REFERENCES agent_build_versions (id) ON DELETE CASCADE,
    knowledge_source_id uuid NOT NULL REFERENCES knowledge_sources (id) ON DELETE RESTRICT,
    binding_role text NOT NULL DEFAULT 'default',
    binding_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_build_version_id, knowledge_source_id)
);

CREATE TABLE agent_deployments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    agent_build_id uuid NOT NULL,
    current_build_version_id uuid NOT NULL,
    runtime_profile_id uuid NOT NULL,
    provider_account_id uuid,
    model_alias_id uuid,
    routing_policy_id uuid,
    spend_policy_id uuid,
    name text NOT NULL,
    slug text NOT NULL,
    deployment_type text NOT NULL CHECK (deployment_type IN ('native', 'hosted_external')),
    endpoint_url text,
    healthcheck_url text,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'archived')),
    deployment_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, organization_id),
    UNIQUE (id, organization_id, workspace_id),
    UNIQUE (id, agent_build_id),
    UNIQUE (workspace_id, slug),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    CHECK ((deployment_type = 'hosted_external' AND endpoint_url IS NOT NULL) OR deployment_type = 'native')
);

ALTER TABLE agent_deployments
ADD CONSTRAINT agent_deployments_agent_build_fk
FOREIGN KEY (agent_build_id, organization_id, workspace_id) REFERENCES agent_builds (id, organization_id, workspace_id) ON DELETE CASCADE;

ALTER TABLE agent_deployments
ADD CONSTRAINT agent_deployments_current_build_version_fk
FOREIGN KEY (current_build_version_id, agent_build_id) REFERENCES agent_build_versions (id, agent_build_id) ON DELETE RESTRICT;

ALTER TABLE agent_deployments
ADD CONSTRAINT agent_deployments_runtime_profile_fk
FOREIGN KEY (runtime_profile_id, organization_id) REFERENCES runtime_profiles (id, organization_id) ON DELETE RESTRICT;

ALTER TABLE agent_deployments
ADD CONSTRAINT agent_deployments_provider_account_fk
FOREIGN KEY (provider_account_id, organization_id) REFERENCES provider_accounts (id, organization_id) ON DELETE RESTRICT;

ALTER TABLE agent_deployments
ADD CONSTRAINT agent_deployments_model_alias_fk
FOREIGN KEY (model_alias_id, organization_id) REFERENCES model_aliases (id, organization_id) ON DELETE RESTRICT;

ALTER TABLE agent_deployments
ADD CONSTRAINT agent_deployments_routing_policy_fk
FOREIGN KEY (routing_policy_id, organization_id) REFERENCES routing_policies (id, organization_id) ON DELETE RESTRICT;

ALTER TABLE agent_deployments
ADD CONSTRAINT agent_deployments_spend_policy_fk
FOREIGN KEY (spend_policy_id, organization_id) REFERENCES spend_policies (id, organization_id) ON DELETE RESTRICT;

CREATE TABLE agent_deployment_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    agent_build_id uuid NOT NULL,
    agent_deployment_id uuid NOT NULL,
    source_agent_build_version_id uuid NOT NULL,
    source_runtime_profile_id uuid NOT NULL,
    source_provider_account_id uuid,
    source_model_alias_id uuid,
    source_routing_policy_id uuid,
    source_spend_policy_id uuid,
    deployment_type text NOT NULL CHECK (deployment_type IN ('native', 'hosted_external')),
    endpoint_url text,
    snapshot_hash text NOT NULL,
    snapshot_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_deployment_id, snapshot_hash),
    UNIQUE (id, agent_deployment_id),
    UNIQUE (id, organization_id, workspace_id),
    CHECK ((deployment_type = 'hosted_external' AND endpoint_url IS NOT NULL) OR deployment_type = 'native')
);

ALTER TABLE agent_deployment_snapshots
ADD CONSTRAINT agent_deployment_snapshots_workspace_fk
FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE;

ALTER TABLE agent_deployment_snapshots
ADD CONSTRAINT agent_deployment_snapshots_deployment_tenant_fk
FOREIGN KEY (agent_deployment_id, organization_id, workspace_id) REFERENCES agent_deployments (id, organization_id, workspace_id) ON DELETE CASCADE;

ALTER TABLE agent_deployment_snapshots
ADD CONSTRAINT agent_deployment_snapshots_deployment_build_fk
FOREIGN KEY (agent_deployment_id, agent_build_id) REFERENCES agent_deployments (id, agent_build_id) ON DELETE CASCADE;

ALTER TABLE agent_deployment_snapshots
ADD CONSTRAINT agent_deployment_snapshots_build_version_fk
FOREIGN KEY (source_agent_build_version_id, agent_build_id) REFERENCES agent_build_versions (id, agent_build_id) ON DELETE RESTRICT;

ALTER TABLE agent_deployment_snapshots
ADD CONSTRAINT agent_deployment_snapshots_runtime_profile_fk
FOREIGN KEY (source_runtime_profile_id, organization_id) REFERENCES runtime_profiles (id, organization_id) ON DELETE RESTRICT;

ALTER TABLE agent_deployment_snapshots
ADD CONSTRAINT agent_deployment_snapshots_provider_account_fk
FOREIGN KEY (source_provider_account_id, organization_id) REFERENCES provider_accounts (id, organization_id) ON DELETE RESTRICT;

ALTER TABLE agent_deployment_snapshots
ADD CONSTRAINT agent_deployment_snapshots_model_alias_fk
FOREIGN KEY (source_model_alias_id, organization_id) REFERENCES model_aliases (id, organization_id) ON DELETE RESTRICT;

ALTER TABLE agent_deployment_snapshots
ADD CONSTRAINT agent_deployment_snapshots_routing_policy_fk
FOREIGN KEY (source_routing_policy_id, organization_id) REFERENCES routing_policies (id, organization_id) ON DELETE RESTRICT;

ALTER TABLE agent_deployment_snapshots
ADD CONSTRAINT agent_deployment_snapshots_spend_policy_fk
FOREIGN KEY (source_spend_policy_id, organization_id) REFERENCES spend_policies (id, organization_id) ON DELETE RESTRICT;

CREATE FUNCTION validate_agent_deployment_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM 1
    FROM runtime_profiles
    WHERE id = NEW.runtime_profile_id
      AND organization_id = NEW.organization_id
      AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
    IF NOT FOUND THEN
        RAISE EXCEPTION 'runtime profile % is not visible to workspace %', NEW.runtime_profile_id, NEW.workspace_id;
    END IF;

    IF NEW.provider_account_id IS NOT NULL THEN
        PERFORM 1
        FROM provider_accounts
        WHERE id = NEW.provider_account_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'provider account % is not visible to workspace %', NEW.provider_account_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.model_alias_id IS NOT NULL THEN
        PERFORM 1
        FROM model_aliases
        WHERE id = NEW.model_alias_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'model alias % is not visible to workspace %', NEW.model_alias_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.routing_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM routing_policies
        WHERE id = NEW.routing_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'routing policy % is not visible to workspace %', NEW.routing_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.spend_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM spend_policies
        WHERE id = NEW.spend_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'spend policy % is not visible to workspace %', NEW.spend_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

CREATE FUNCTION validate_agent_build_version_tool_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    build_org_id uuid;
    build_workspace_id uuid;
BEGIN
    SELECT ab.organization_id, ab.workspace_id
    INTO build_org_id, build_workspace_id
    FROM agent_build_versions abv
    JOIN agent_builds ab ON ab.id = abv.agent_build_id
    WHERE abv.id = NEW.agent_build_version_id;

    IF build_org_id IS NULL THEN
        RAISE EXCEPTION 'agent build version % not found', NEW.agent_build_version_id;
    END IF;

    PERFORM 1
    FROM tools
    WHERE id = NEW.tool_id
      AND organization_id = build_org_id
      AND (workspace_id IS NULL OR workspace_id = build_workspace_id);
    IF NOT FOUND THEN
        RAISE EXCEPTION 'tool % is not visible to build workspace %', NEW.tool_id, build_workspace_id;
    END IF;

    RETURN NEW;
END;
$$;

CREATE FUNCTION validate_agent_build_version_knowledge_source_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    build_org_id uuid;
    build_workspace_id uuid;
BEGIN
    SELECT ab.organization_id, ab.workspace_id
    INTO build_org_id, build_workspace_id
    FROM agent_build_versions abv
    JOIN agent_builds ab ON ab.id = abv.agent_build_id
    WHERE abv.id = NEW.agent_build_version_id;

    IF build_org_id IS NULL THEN
        RAISE EXCEPTION 'agent build version % not found', NEW.agent_build_version_id;
    END IF;

    PERFORM 1
    FROM knowledge_sources
    WHERE id = NEW.knowledge_source_id
      AND organization_id = build_org_id
      AND (workspace_id IS NULL OR workspace_id = build_workspace_id);
    IF NOT FOUND THEN
        RAISE EXCEPTION 'knowledge source % is not visible to build workspace %', NEW.knowledge_source_id, build_workspace_id;
    END IF;

    RETURN NEW;
END;
$$;

CREATE FUNCTION validate_agent_deployment_snapshot_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM 1
    FROM runtime_profiles
    WHERE id = NEW.source_runtime_profile_id
      AND organization_id = NEW.organization_id
      AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
    IF NOT FOUND THEN
        RAISE EXCEPTION 'snapshot runtime profile % is not visible to workspace %', NEW.source_runtime_profile_id, NEW.workspace_id;
    END IF;

    IF NEW.source_provider_account_id IS NOT NULL THEN
        PERFORM 1
        FROM provider_accounts
        WHERE id = NEW.source_provider_account_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'snapshot provider account % is not visible to workspace %', NEW.source_provider_account_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.source_model_alias_id IS NOT NULL THEN
        PERFORM 1
        FROM model_aliases
        WHERE id = NEW.source_model_alias_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'snapshot model alias % is not visible to workspace %', NEW.source_model_alias_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.source_routing_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM routing_policies
        WHERE id = NEW.source_routing_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'snapshot routing policy % is not visible to workspace %', NEW.source_routing_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.source_spend_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM spend_policies
        WHERE id = NEW.source_spend_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'snapshot spend policy % is not visible to workspace %', NEW.source_spend_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

CREATE INDEX agent_build_versions_build_id_idx ON agent_build_versions (agent_build_id);
CREATE INDEX agent_build_version_tools_tool_id_idx ON agent_build_version_tools (tool_id);
CREATE INDEX agent_build_version_knowledge_sources_source_id_idx ON agent_build_version_knowledge_sources (knowledge_source_id);
CREATE INDEX agent_deployments_build_id_idx ON agent_deployments (agent_build_id);
CREATE INDEX agent_deployment_snapshots_deployment_id_idx ON agent_deployment_snapshots (agent_deployment_id);

CREATE TRIGGER runtime_profiles_set_updated_at
BEFORE UPDATE ON runtime_profiles
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER tools_set_updated_at
BEFORE UPDATE ON tools
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER knowledge_sources_set_updated_at
BEFORE UPDATE ON knowledge_sources
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER agent_builds_set_updated_at
BEFORE UPDATE ON agent_builds
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER agent_deployments_set_updated_at
BEFORE UPDATE ON agent_deployments
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER agent_deployments_validate_scope
BEFORE INSERT OR UPDATE ON agent_deployments
FOR EACH ROW
EXECUTE FUNCTION validate_agent_deployment_scope();

CREATE TRIGGER agent_build_version_tools_validate_scope
BEFORE INSERT OR UPDATE ON agent_build_version_tools
FOR EACH ROW
EXECUTE FUNCTION validate_agent_build_version_tool_scope();

CREATE TRIGGER agent_build_version_knowledge_sources_validate_scope
BEFORE INSERT OR UPDATE ON agent_build_version_knowledge_sources
FOR EACH ROW
EXECUTE FUNCTION validate_agent_build_version_knowledge_source_scope();

CREATE TRIGGER agent_deployment_snapshots_validate_scope
BEFORE INSERT OR UPDATE ON agent_deployment_snapshots
FOR EACH ROW
EXECUTE FUNCTION validate_agent_deployment_snapshot_scope();

-- +goose Down
DROP TRIGGER IF EXISTS agent_build_version_knowledge_sources_validate_scope ON agent_build_version_knowledge_sources;
DROP TRIGGER IF EXISTS agent_build_version_tools_validate_scope ON agent_build_version_tools;
DROP TRIGGER IF EXISTS agent_deployment_snapshots_validate_scope ON agent_deployment_snapshots;
DROP TRIGGER IF EXISTS agent_deployments_validate_scope ON agent_deployments;
DROP TRIGGER IF EXISTS agent_deployments_set_updated_at ON agent_deployments;
DROP TRIGGER IF EXISTS agent_builds_set_updated_at ON agent_builds;
DROP TRIGGER IF EXISTS knowledge_sources_set_updated_at ON knowledge_sources;
DROP TRIGGER IF EXISTS tools_set_updated_at ON tools;
DROP TRIGGER IF EXISTS runtime_profiles_set_updated_at ON runtime_profiles;

DROP FUNCTION IF EXISTS validate_agent_build_version_knowledge_source_scope();
DROP FUNCTION IF EXISTS validate_agent_build_version_tool_scope();
DROP FUNCTION IF EXISTS validate_agent_deployment_snapshot_scope();
DROP FUNCTION IF EXISTS validate_agent_deployment_scope();

DROP TABLE IF EXISTS agent_deployment_snapshots;
DROP TABLE IF EXISTS agent_deployments;
DROP TABLE IF EXISTS agent_build_version_knowledge_sources;
DROP TABLE IF EXISTS agent_build_version_tools;
DROP TABLE IF EXISTS agent_build_versions;
DROP TABLE IF EXISTS agent_builds;
DROP TABLE IF EXISTS knowledge_sources;
DROP TABLE IF EXISTS tools;
DROP TABLE IF EXISTS runtime_profiles;
