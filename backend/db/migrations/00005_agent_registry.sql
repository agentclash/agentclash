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
    agent_build_id uuid NOT NULL REFERENCES agent_builds (id) ON DELETE CASCADE,
    current_build_version_id uuid NOT NULL,
    runtime_profile_id uuid NOT NULL REFERENCES runtime_profiles (id) ON DELETE RESTRICT,
    provider_account_id uuid REFERENCES provider_accounts (id) ON DELETE SET NULL,
    model_alias_id uuid REFERENCES model_aliases (id) ON DELETE SET NULL,
    routing_policy_id uuid REFERENCES routing_policies (id) ON DELETE SET NULL,
    spend_policy_id uuid REFERENCES spend_policies (id) ON DELETE SET NULL,
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
    UNIQUE (workspace_id, slug),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    CHECK ((deployment_type = 'hosted_external' AND endpoint_url IS NOT NULL) OR deployment_type = 'native')
);

ALTER TABLE agent_deployments
ADD CONSTRAINT agent_deployments_current_build_version_fk
FOREIGN KEY (current_build_version_id, agent_build_id) REFERENCES agent_build_versions (id, agent_build_id) ON DELETE RESTRICT;

CREATE TABLE agent_deployment_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_deployment_id uuid NOT NULL REFERENCES agent_deployments (id) ON DELETE CASCADE,
    source_agent_build_version_id uuid NOT NULL REFERENCES agent_build_versions (id) ON DELETE RESTRICT,
    source_runtime_profile_id uuid NOT NULL REFERENCES runtime_profiles (id) ON DELETE RESTRICT,
    source_provider_account_id uuid REFERENCES provider_accounts (id) ON DELETE SET NULL,
    source_model_alias_id uuid REFERENCES model_aliases (id) ON DELETE SET NULL,
    source_routing_policy_id uuid REFERENCES routing_policies (id) ON DELETE SET NULL,
    source_spend_policy_id uuid REFERENCES spend_policies (id) ON DELETE SET NULL,
    deployment_type text NOT NULL CHECK (deployment_type IN ('native', 'hosted_external')),
    endpoint_url text,
    snapshot_hash text NOT NULL,
    snapshot_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_deployment_id, snapshot_hash),
    UNIQUE (id, agent_deployment_id),
    CHECK ((deployment_type = 'hosted_external' AND endpoint_url IS NOT NULL) OR deployment_type = 'native')
);

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

-- +goose Down
DROP TRIGGER IF EXISTS agent_deployments_set_updated_at ON agent_deployments;
DROP TRIGGER IF EXISTS agent_builds_set_updated_at ON agent_builds;
DROP TRIGGER IF EXISTS knowledge_sources_set_updated_at ON knowledge_sources;
DROP TRIGGER IF EXISTS tools_set_updated_at ON tools;
DROP TRIGGER IF EXISTS runtime_profiles_set_updated_at ON runtime_profiles;

DROP TABLE IF EXISTS agent_deployment_snapshots;
DROP TABLE IF EXISTS agent_deployments;
DROP TABLE IF EXISTS agent_build_version_knowledge_sources;
DROP TABLE IF EXISTS agent_build_version_tools;
DROP TABLE IF EXISTS agent_build_versions;
DROP TABLE IF EXISTS agent_builds;
DROP TABLE IF EXISTS knowledge_sources;
DROP TABLE IF EXISTS tools;
DROP TABLE IF EXISTS runtime_profiles;
