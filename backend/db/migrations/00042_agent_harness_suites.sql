-- +goose Up
CREATE TABLE agent_harness_suites (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    name text NOT NULL,
    slug text NOT NULL,
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    current_version_number integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, slug),
    UNIQUE (id, organization_id, workspace_id),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE
);

CREATE TABLE agent_harness_suite_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    agent_harness_suite_id uuid NOT NULL,
    version_number integer NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_harness_suite_id, version_number),
    UNIQUE (id, organization_id, workspace_id),
    FOREIGN KEY (agent_harness_suite_id, organization_id, workspace_id) REFERENCES agent_harness_suites (id, organization_id, workspace_id) ON DELETE CASCADE
);

CREATE TABLE agent_harness_suite_tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    agent_harness_suite_version_id uuid NOT NULL,
    task_order integer NOT NULL DEFAULT 0,
    title text NOT NULL,
    public_prompt text NOT NULL DEFAULT '',
    task_prompt text NOT NULL,
    source_type text NOT NULL CHECK (source_type IN ('manual', 'github_issue', 'upload', 'prior_harness_run')),
    source_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    repository_url text,
    base_branch text,
    execution_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    evaluation_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (agent_harness_suite_version_id, organization_id, workspace_id) REFERENCES agent_harness_suite_versions (id, organization_id, workspace_id) ON DELETE CASCADE
);

CREATE INDEX agent_harness_suites_workspace_status_idx
ON agent_harness_suites (workspace_id, status, updated_at DESC);

CREATE INDEX agent_harness_suite_tasks_version_order_idx
ON agent_harness_suite_tasks (agent_harness_suite_version_id, task_order, id);

CREATE TRIGGER agent_harness_suites_set_updated_at
BEFORE UPDATE ON agent_harness_suites
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER agent_harness_suite_tasks_set_updated_at
BEFORE UPDATE ON agent_harness_suite_tasks
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS agent_harness_suite_tasks_set_updated_at ON agent_harness_suite_tasks;
DROP TRIGGER IF EXISTS agent_harness_suites_set_updated_at ON agent_harness_suites;
DROP INDEX IF EXISTS agent_harness_suite_tasks_version_order_idx;
DROP INDEX IF EXISTS agent_harness_suites_workspace_status_idx;
DROP TABLE IF EXISTS agent_harness_suite_tasks;
DROP TABLE IF EXISTS agent_harness_suite_versions;
DROP TABLE IF EXISTS agent_harness_suites;
