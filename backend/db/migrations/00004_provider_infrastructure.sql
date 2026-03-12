-- +goose Up
CREATE TABLE provider_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid,
    provider_key text NOT NULL,
    name text NOT NULL,
    credential_reference text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'error', 'archived')),
    limits_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, organization_id)
);

ALTER TABLE provider_accounts
ADD CONSTRAINT provider_accounts_workspace_fk
FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE;

CREATE UNIQUE INDEX provider_accounts_org_slug_uq
ON provider_accounts (organization_id, provider_key, name)
WHERE workspace_id IS NULL;

CREATE UNIQUE INDEX provider_accounts_workspace_slug_uq
ON provider_accounts (workspace_id, provider_key, name)
WHERE workspace_id IS NOT NULL;

CREATE TABLE model_catalog_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_key text NOT NULL,
    provider_model_id text NOT NULL,
    display_name text NOT NULL,
    model_family text NOT NULL,
    modality text NOT NULL DEFAULT 'text' CHECK (modality IN ('text', 'multimodal', 'embedding', 'speech')),
    lifecycle_status text NOT NULL DEFAULT 'active' CHECK (lifecycle_status IN ('active', 'preview', 'deprecated', 'archived')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_key, provider_model_id)
);

CREATE TABLE model_aliases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid,
    provider_account_id uuid REFERENCES provider_accounts (id) ON DELETE SET NULL,
    model_catalog_entry_id uuid NOT NULL REFERENCES model_catalog_entries (id) ON DELETE RESTRICT,
    alias_key text NOT NULL,
    display_name text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, organization_id)
);

ALTER TABLE model_aliases
ADD CONSTRAINT model_aliases_workspace_fk
FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE;

CREATE UNIQUE INDEX model_aliases_org_alias_uq
ON model_aliases (organization_id, alias_key)
WHERE workspace_id IS NULL;

CREATE UNIQUE INDEX model_aliases_workspace_alias_uq
ON model_aliases (workspace_id, alias_key)
WHERE workspace_id IS NOT NULL;

CREATE TABLE routing_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid,
    name text NOT NULL,
    policy_kind text NOT NULL CHECK (policy_kind IN ('single_model', 'fallback', 'budget_aware', 'latency_aware')),
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, organization_id)
);

ALTER TABLE routing_policies
ADD CONSTRAINT routing_policies_workspace_fk
FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE;

CREATE UNIQUE INDEX routing_policies_org_name_uq
ON routing_policies (organization_id, name)
WHERE workspace_id IS NULL;

CREATE UNIQUE INDEX routing_policies_workspace_name_uq
ON routing_policies (workspace_id, name)
WHERE workspace_id IS NOT NULL;

CREATE TABLE spend_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid,
    name text NOT NULL,
    currency_code text NOT NULL DEFAULT 'USD',
    window_kind text NOT NULL CHECK (window_kind IN ('run', 'day', 'week', 'month')),
    soft_limit numeric(18,6),
    hard_limit numeric(18,6),
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, organization_id),
    CHECK (soft_limit IS NULL OR soft_limit >= 0),
    CHECK (hard_limit IS NULL OR hard_limit >= 0),
    CHECK (soft_limit IS NULL OR hard_limit IS NULL OR soft_limit <= hard_limit)
);

ALTER TABLE spend_policies
ADD CONSTRAINT spend_policies_workspace_fk
FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE;

CREATE UNIQUE INDEX spend_policies_org_name_uq
ON spend_policies (organization_id, name)
WHERE workspace_id IS NULL;

CREATE UNIQUE INDEX spend_policies_workspace_name_uq
ON spend_policies (workspace_id, name)
WHERE workspace_id IS NOT NULL;

CREATE TRIGGER provider_accounts_set_updated_at
BEFORE UPDATE ON provider_accounts
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER model_catalog_entries_set_updated_at
BEFORE UPDATE ON model_catalog_entries
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER model_aliases_set_updated_at
BEFORE UPDATE ON model_aliases
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER routing_policies_set_updated_at
BEFORE UPDATE ON routing_policies
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER spend_policies_set_updated_at
BEFORE UPDATE ON spend_policies
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS spend_policies_set_updated_at ON spend_policies;
DROP TRIGGER IF EXISTS routing_policies_set_updated_at ON routing_policies;
DROP TRIGGER IF EXISTS model_aliases_set_updated_at ON model_aliases;
DROP TRIGGER IF EXISTS model_catalog_entries_set_updated_at ON model_catalog_entries;
DROP TRIGGER IF EXISTS provider_accounts_set_updated_at ON provider_accounts;

DROP TABLE IF EXISTS spend_policies;
DROP TABLE IF EXISTS routing_policies;
DROP TABLE IF EXISTS model_aliases;
DROP TABLE IF EXISTS model_catalog_entries;
DROP TABLE IF EXISTS provider_accounts;
