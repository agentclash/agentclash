-- name: CreateAgentBuild :one
INSERT INTO agent_builds (organization_id, workspace_id, name, slug, description, created_by_user_id)
VALUES (@organization_id, @workspace_id, @name, @slug, @description, @created_by_user_id)
RETURNING *;

-- name: GetAgentBuildByID :one
SELECT * FROM agent_builds WHERE id = @id AND archived_at IS NULL;

-- name: ListAgentBuildsByWorkspaceID :many
SELECT * FROM agent_builds
WHERE workspace_id = @workspace_id AND lifecycle_status = 'active' AND archived_at IS NULL
ORDER BY updated_at DESC;

-- name: CreateAgentBuildVersion :one
INSERT INTO agent_build_versions (
    agent_build_id, version_number, version_status,
    agent_kind, interface_spec, policy_spec, reasoning_spec,
    memory_spec, workflow_spec, guardrail_spec, model_spec,
    output_schema, trace_contract, publication_spec, created_by_user_id
) VALUES (
    @agent_build_id, @version_number, 'draft',
    @agent_kind, @interface_spec, @policy_spec, @reasoning_spec,
    @memory_spec, @workflow_spec, @guardrail_spec, @model_spec,
    @output_schema, @trace_contract, @publication_spec, @created_by_user_id
) RETURNING *;

-- name: GetAgentBuildVersionByID :one
SELECT * FROM agent_build_versions WHERE id = @id;

-- name: GetLatestVersionNumberForBuild :one
SELECT COALESCE(MAX(version_number), 0)::integer AS max_version
FROM agent_build_versions WHERE agent_build_id = @agent_build_id;

-- name: ListAgentBuildVersionsByBuildID :many
SELECT * FROM agent_build_versions
WHERE agent_build_id = @agent_build_id
ORDER BY version_number DESC;

-- name: UpdateAgentBuildVersionDraft :exec
UPDATE agent_build_versions SET
    agent_kind = @agent_kind,
    interface_spec = @interface_spec,
    policy_spec = @policy_spec,
    reasoning_spec = @reasoning_spec,
    memory_spec = @memory_spec,
    workflow_spec = @workflow_spec,
    guardrail_spec = @guardrail_spec,
    model_spec = @model_spec,
    output_schema = @output_schema,
    trace_contract = @trace_contract,
    publication_spec = @publication_spec
WHERE id = @id AND version_status = 'draft';

-- name: MarkAgentBuildVersionReady :exec
UPDATE agent_build_versions SET version_status = 'ready'
WHERE id = @id AND version_status = 'draft';

-- name: CreateAgentDeployment :one
INSERT INTO agent_deployments (
    organization_id, workspace_id, agent_build_id, current_build_version_id,
    runtime_profile_id, provider_account_id, model_alias_id,
    name, slug, deployment_type, deployment_config
) VALUES (
    @organization_id, @workspace_id, @agent_build_id, @current_build_version_id,
    @runtime_profile_id, @provider_account_id, @model_alias_id,
    @name, @slug, 'native', @deployment_config
) RETURNING *;
