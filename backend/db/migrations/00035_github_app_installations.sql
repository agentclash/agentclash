-- +goose Up
CREATE TABLE organization_github_installations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    github_installation_id bigint NOT NULL UNIQUE,
    github_account_id bigint NOT NULL,
    github_account_login text NOT NULL,
    github_account_type text NOT NULL CHECK (github_account_type IN ('User', 'Organization')),
    repository_selection text NOT NULL DEFAULT 'selected' CHECK (repository_selection IN ('all', 'selected')),
    installed_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id),
    UNIQUE (organization_id, github_installation_id)
);

CREATE INDEX organization_github_installations_org_status_idx
ON organization_github_installations (organization_id, status, updated_at DESC);

CREATE TRIGGER organization_github_installations_set_updated_at
BEFORE UPDATE ON organization_github_installations
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE workspace_github_installation_bindings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    organization_github_installation_id uuid NOT NULL REFERENCES organization_github_installations (id) ON DELETE CASCADE,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, organization_github_installation_id),
    FOREIGN KEY (organization_github_installation_id, organization_id) REFERENCES organization_github_installations (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE
);

CREATE INDEX workspace_github_installation_bindings_workspace_idx
ON workspace_github_installation_bindings (workspace_id, created_at DESC);

CREATE TABLE github_installation_repositories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_github_installation_id uuid NOT NULL REFERENCES organization_github_installations (id) ON DELETE CASCADE,
    github_repository_id bigint NOT NULL,
    full_name text NOT NULL,
    owner_login text NOT NULL,
    name text NOT NULL,
    private boolean NOT NULL DEFAULT false,
    default_branch text NOT NULL DEFAULT 'main',
    html_url text NOT NULL DEFAULT '',
    clone_url text NOT NULL DEFAULT '',
    archived boolean NOT NULL DEFAULT false,
    permissions jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'removed')),
    last_synced_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_github_installation_id, github_repository_id)
);

CREATE INDEX github_installation_repositories_installation_status_idx
ON github_installation_repositories (organization_github_installation_id, status, full_name);

CREATE TRIGGER github_installation_repositories_set_updated_at
BEFORE UPDATE ON github_installation_repositories
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

ALTER TABLE agent_harnesses
    ADD COLUMN repository_provider text CHECK (repository_provider IS NULL OR repository_provider IN ('github')),
    ADD COLUMN github_repository_id bigint,
    ADD COLUMN github_installation_id bigint,
    ADD COLUMN repository_full_name text,
    ADD COLUMN repository_clone_url text;

CREATE INDEX agent_harnesses_github_repository_idx
ON agent_harnesses (workspace_id, github_repository_id)
WHERE github_repository_id IS NOT NULL AND archived_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS agent_harnesses_github_repository_idx;
ALTER TABLE agent_harnesses
    DROP COLUMN IF EXISTS repository_clone_url,
    DROP COLUMN IF EXISTS repository_full_name,
    DROP COLUMN IF EXISTS github_installation_id,
    DROP COLUMN IF EXISTS github_repository_id,
    DROP COLUMN IF EXISTS repository_provider;

DROP TRIGGER IF EXISTS github_installation_repositories_set_updated_at ON github_installation_repositories;
DROP INDEX IF EXISTS github_installation_repositories_installation_status_idx;
DROP TABLE IF EXISTS github_installation_repositories;

DROP INDEX IF EXISTS workspace_github_installation_bindings_workspace_idx;
DROP TABLE IF EXISTS workspace_github_installation_bindings;

DROP TRIGGER IF EXISTS organization_github_installations_set_updated_at ON organization_github_installations;
DROP INDEX IF EXISTS organization_github_installations_org_status_idx;
DROP TABLE IF EXISTS organization_github_installations;
