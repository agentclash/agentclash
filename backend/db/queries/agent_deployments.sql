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
ORDER BY ad.id, ads.created_at DESC, ads.id DESC;
