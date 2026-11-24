-- name: ListActiveAgentDeploymentsByWorkspaceID :many
SELECT DISTINCT ON (ad.id)
    ad.id,
    ad.organization_id,
    ad.workspace_id,
    ad.current_build_version_id,
    ad.name,
    ad.status,
    ad.created_at,
    ad.updated_at,
    ads.id AS latest_snapshot_id
FROM agent_deployments ad
LEFT JOIN agent_deployment_snapshots ads
  ON ads.agent_deployment_id = ad.id
 AND ads.organization_id = ad.organization_id
 AND ads.workspace_id = ad.workspace_id
WHERE ad.workspace_id = @workspace_id
  AND ad.status = 'active'
  AND ad.archived_at IS NULL
ORDER BY ad.id, ads.created_at DESC, ads.id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountActiveAgentDeploymentsByWorkspaceID :one
SELECT COUNT(*)
FROM agent_deployments
WHERE workspace_id = @workspace_id
  AND status = 'active'
  AND archived_at IS NULL;

-- name: AgentDeploymentSlugExists :one
SELECT EXISTS (
    SELECT 1
    FROM agent_deployments
    WHERE workspace_id = @workspace_id
      AND slug = @slug
);

-- name: AgentDeploymentIsConnector :one
SELECT EXISTS (
    SELECT 1
    FROM agent_deployment_snapshots s
    JOIN connector_snapshots c ON c.snapshot_id = s.id
    WHERE s.agent_deployment_id = @id
      AND s.workspace_id = @workspace_id
);

-- name: ArchiveAgentDeployment :execrows
UPDATE agent_deployments
SET status = 'archived',
    archived_at = now(),
    updated_at = now()
WHERE workspace_id = @workspace_id
  AND id = @id
  AND status IN ('active', 'paused')
  AND archived_at IS NULL;
