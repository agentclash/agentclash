-- +goose Up
CREATE TABLE workspace_ci_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name text NOT NULL,
    repository_full_name text NOT NULL DEFAULT '',
    github_repository_id bigint,
    github_installation_id bigint,
    default_branch text NOT NULL DEFAULT 'main',
    manifest_path text NOT NULL DEFAULT '.agentclash/ci.yaml',
    workflow_path text NOT NULL DEFAULT '.github/workflows/agentclash.yml',
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

CREATE INDEX workspace_ci_profiles_workspace_updated_idx
ON workspace_ci_profiles (workspace_id, updated_at DESC);

CREATE TRIGGER workspace_ci_profiles_set_updated_at
BEFORE UPDATE ON workspace_ci_profiles
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS workspace_ci_profiles_set_updated_at ON workspace_ci_profiles;
DROP INDEX IF EXISTS workspace_ci_profiles_workspace_updated_idx;
DROP TABLE IF EXISTS workspace_ci_profiles;
