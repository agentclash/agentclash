-- +goose Up
CREATE TABLE runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    challenge_pack_version_id uuid NOT NULL REFERENCES challenge_pack_versions (id) ON DELETE RESTRICT,
    challenge_input_set_id uuid,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    name text NOT NULL,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'queued', 'provisioning', 'running', 'scoring', 'completed', 'failed', 'cancelled')),
    execution_mode text NOT NULL DEFAULT 'comparison' CHECK (execution_mode IN ('single_agent', 'comparison')),
    temporal_workflow_id text,
    temporal_run_id text,
    execution_plan jsonb NOT NULL DEFAULT '{}'::jsonb,
    queued_at timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    cancelled_at timestamptz,
    failed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id),
    UNIQUE (id, organization_id, workspace_id),
    UNIQUE (temporal_workflow_id),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE
);

ALTER TABLE runs
ADD CONSTRAINT runs_challenge_input_set_fk
FOREIGN KEY (challenge_input_set_id, challenge_pack_version_id) REFERENCES challenge_input_sets (id, challenge_pack_version_id) ON DELETE RESTRICT;

CREATE TABLE run_status_history (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    from_status text,
    to_status text NOT NULL CHECK (to_status IN ('draft', 'queued', 'provisioning', 'running', 'scoring', 'completed', 'failed', 'cancelled')),
    reason text,
    changed_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    changed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE run_agents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    run_id uuid NOT NULL,
    agent_deployment_id uuid NOT NULL,
    agent_deployment_snapshot_id uuid NOT NULL,
    lane_index integer NOT NULL CHECK (lane_index >= 0),
    label text NOT NULL,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'ready', 'executing', 'evaluating', 'completed', 'failed')),
    queued_at timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    failure_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id, lane_index),
    UNIQUE (id, run_id),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (run_id, organization_id, workspace_id) REFERENCES runs (id, organization_id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (agent_deployment_id, organization_id, workspace_id) REFERENCES agent_deployments (id, organization_id, workspace_id) ON DELETE RESTRICT
);

ALTER TABLE run_agents
ADD CONSTRAINT run_agents_snapshot_fk
FOREIGN KEY (agent_deployment_snapshot_id, agent_deployment_id) REFERENCES agent_deployment_snapshots (id, agent_deployment_id) ON DELETE RESTRICT;

CREATE TABLE run_agent_status_history (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_agent_id uuid NOT NULL REFERENCES run_agents (id) ON DELETE CASCADE,
    from_status text,
    to_status text NOT NULL CHECK (to_status IN ('queued', 'ready', 'executing', 'evaluating', 'completed', 'failed')),
    reason text,
    changed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX runs_workspace_id_idx ON runs (workspace_id, created_at DESC);
CREATE INDEX runs_challenge_pack_version_id_idx ON runs (challenge_pack_version_id);
CREATE INDEX run_agents_run_id_idx ON run_agents (run_id);
CREATE INDEX run_agents_snapshot_id_idx ON run_agents (agent_deployment_snapshot_id);

CREATE TRIGGER runs_set_updated_at
BEFORE UPDATE ON runs
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER run_agents_set_updated_at
BEFORE UPDATE ON run_agents
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS run_agents_set_updated_at ON run_agents;
DROP TRIGGER IF EXISTS runs_set_updated_at ON runs;

DROP TABLE IF EXISTS run_agent_status_history;
DROP TABLE IF EXISTS run_agents;
DROP TABLE IF EXISTS run_status_history;
DROP TABLE IF EXISTS runs;
