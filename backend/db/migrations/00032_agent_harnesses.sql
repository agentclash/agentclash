-- +goose Up
CREATE TABLE agent_harnesses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    name text NOT NULL,
    slug text NOT NULL,
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived')),
    harness_kind text NOT NULL DEFAULT 'codex_e2b' CHECK (harness_kind IN ('codex_e2b')),
    task_prompt text NOT NULL,
    codex_template text NOT NULL DEFAULT 'codex',
    codex_model text,
    auth_mode text NOT NULL CHECK (auth_mode IN ('chatgpt_device', 'api_key_secret', 'bring_your_own_env')),
    openai_api_key_secret_name text,
    e2b_api_key_secret_name text,
    repository_url text,
    base_branch text,
    execution_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    evaluation_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (workspace_id, slug),
    UNIQUE (id, organization_id, workspace_id),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    CHECK (auth_mode <> 'api_key_secret' OR openai_api_key_secret_name IS NOT NULL)
);

CREATE INDEX agent_harnesses_workspace_status_updated_idx
ON agent_harnesses (workspace_id, status, updated_at DESC)
WHERE archived_at IS NULL;

CREATE TRIGGER agent_harnesses_set_updated_at
BEFORE UPDATE ON agent_harnesses
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS agent_harnesses_set_updated_at ON agent_harnesses;
DROP INDEX IF EXISTS agent_harnesses_workspace_status_updated_idx;
DROP TABLE IF EXISTS agent_harnesses;
