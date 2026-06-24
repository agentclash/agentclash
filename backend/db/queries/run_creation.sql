-- name: GetRunnableChallengePackVersionByID :one
SELECT *
FROM challenge_pack_versions
WHERE id = @id
  AND lifecycle_status = 'runnable'
  AND archived_at IS NULL
LIMIT 1;

-- name: GetChallengeInputSetByID :one
SELECT *
FROM challenge_input_sets
WHERE id = @id
  AND archived_at IS NULL
LIMIT 1;

-- name: ListChallengeInputSetsByVersionID :many
SELECT id, challenge_pack_version_id, input_key, name
FROM challenge_input_sets
WHERE challenge_pack_version_id = @challenge_pack_version_id
  AND archived_at IS NULL
ORDER BY created_at ASC;

-- name: ListChallengeIdentityIDsByPackVersionID :many
SELECT challenge_identity_id
FROM challenge_pack_version_challenges
WHERE challenge_pack_version_id = @challenge_pack_version_id
ORDER BY execution_order ASC, id ASC;

-- name: ListRunnableDeploymentsWithLatestSnapshot :many
SELECT DISTINCT ON (agent_deployments.id)
    agent_deployments.id,
    agent_deployments.organization_id,
    agent_deployments.workspace_id,
    agent_deployments.name,
    agent_deployment_snapshots.id AS agent_deployment_snapshot_id,
    agent_deployments.spend_policy_id,
    agent_deployments.routing_policy_id
FROM agent_deployments
JOIN agent_deployment_snapshots
  ON agent_deployment_snapshots.agent_deployment_id = agent_deployments.id
 AND agent_deployment_snapshots.organization_id = agent_deployments.organization_id
 AND agent_deployment_snapshots.workspace_id = agent_deployments.workspace_id
WHERE agent_deployments.workspace_id = @workspace_id
  AND agent_deployments.id = ANY(@deployment_ids::uuid[])
  AND agent_deployments.status = 'active'
  AND agent_deployments.archived_at IS NULL
ORDER BY agent_deployments.id, agent_deployment_snapshots.created_at DESC, agent_deployment_snapshots.id DESC;
