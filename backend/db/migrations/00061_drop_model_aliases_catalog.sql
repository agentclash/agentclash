-- +goose Up

-- Collapse the model_aliases + model_catalog_entries indirection. Deployments,
-- deployment snapshots, and playground experiments now store the provider model
-- id directly. Dataset generation already stores its model in the job config
-- JSON (migrated in application code), so it needs no schema change here.

-- 1. Add the new model_id columns (nullable-by-default empty string).
ALTER TABLE agent_deployments
ADD COLUMN model_id text NOT NULL DEFAULT '';

ALTER TABLE agent_deployment_snapshots
ADD COLUMN source_model_id text NOT NULL DEFAULT '';

ALTER TABLE playground_experiments
ADD COLUMN model_id text NOT NULL DEFAULT '';

-- 2. Backfill from the alias -> catalog provider_model_id. The alias FKs were
--    ON DELETE RESTRICT, so no dangling references should exist.
UPDATE agent_deployments d
SET model_id = mce.provider_model_id
FROM model_aliases ma
JOIN model_catalog_entries mce ON mce.id = ma.model_catalog_entry_id
WHERE ma.id = d.model_alias_id;

UPDATE agent_deployment_snapshots ads
SET source_model_id = mce.provider_model_id
FROM model_aliases ma
JOIN model_catalog_entries mce ON mce.id = ma.model_catalog_entry_id
WHERE ma.id = ads.source_model_alias_id;

UPDATE playground_experiments pe
SET model_id = mce.provider_model_id
FROM model_aliases ma
JOIN model_catalog_entries mce ON mce.id = ma.model_catalog_entry_id
WHERE ma.id = pe.model_alias_id;

-- 3. Drop the alias visibility checks from the deployment/snapshot scope
--    validation triggers (reproduced from migration 00005 minus the alias block).
CREATE OR REPLACE FUNCTION validate_agent_deployment_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM 1
    FROM runtime_profiles
    WHERE id = NEW.runtime_profile_id
      AND organization_id = NEW.organization_id
      AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
    IF NOT FOUND THEN
        RAISE EXCEPTION 'runtime profile % is not visible to workspace %', NEW.runtime_profile_id, NEW.workspace_id;
    END IF;

    IF NEW.provider_account_id IS NOT NULL THEN
        PERFORM 1
        FROM provider_accounts
        WHERE id = NEW.provider_account_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'provider account % is not visible to workspace %', NEW.provider_account_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.routing_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM routing_policies
        WHERE id = NEW.routing_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'routing policy % is not visible to workspace %', NEW.routing_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.spend_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM spend_policies
        WHERE id = NEW.spend_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'spend policy % is not visible to workspace %', NEW.spend_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION validate_agent_deployment_snapshot_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM 1
    FROM runtime_profiles
    WHERE id = NEW.source_runtime_profile_id
      AND organization_id = NEW.organization_id
      AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
    IF NOT FOUND THEN
        RAISE EXCEPTION 'snapshot runtime profile % is not visible to workspace %', NEW.source_runtime_profile_id, NEW.workspace_id;
    END IF;

    IF NEW.source_provider_account_id IS NOT NULL THEN
        PERFORM 1
        FROM provider_accounts
        WHERE id = NEW.source_provider_account_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'snapshot provider account % is not visible to workspace %', NEW.source_provider_account_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.source_routing_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM routing_policies
        WHERE id = NEW.source_routing_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'snapshot routing policy % is not visible to workspace %', NEW.source_routing_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.source_spend_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM spend_policies
        WHERE id = NEW.source_spend_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'snapshot spend policy % is not visible to workspace %', NEW.source_spend_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

-- 4. Drop alias FK constraints and columns.
ALTER TABLE agent_deployments DROP CONSTRAINT IF EXISTS agent_deployments_model_alias_fk;
ALTER TABLE agent_deployments DROP COLUMN model_alias_id;

ALTER TABLE agent_deployment_snapshots DROP CONSTRAINT IF EXISTS agent_deployment_snapshots_model_alias_fk;
ALTER TABLE agent_deployment_snapshots DROP COLUMN source_model_alias_id;

-- playground_experiments.model_alias_id has an inline (auto-named) FK that is
-- dropped together with the column.
ALTER TABLE playground_experiments DROP COLUMN model_alias_id;

-- 5. Drop the alias/catalog tables (alias references catalog, so alias first).
DROP TABLE IF EXISTS model_aliases;
DROP TABLE IF EXISTS model_catalog_entries;
DROP FUNCTION IF EXISTS validate_model_alias_scope();

-- +goose Down

-- NOTE: this restores the schema structure only. Row data in model_aliases and
-- model_catalog_entries is NOT recoverable (the up migration backfilled a raw
-- model id and dropped the source rows).

CREATE TABLE model_catalog_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_key text NOT NULL,
    provider_model_id text NOT NULL,
    display_name text NOT NULL,
    model_family text NOT NULL,
    modality text NOT NULL DEFAULT 'text' CHECK (modality IN ('text', 'multimodal', 'embedding', 'speech')),
    lifecycle_status text NOT NULL DEFAULT 'active' CHECK (lifecycle_status IN ('active', 'preview', 'deprecated', 'archived')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    input_cost_per_million_tokens numeric(18,6) NOT NULL DEFAULT 0,
    output_cost_per_million_tokens numeric(18,6) NOT NULL DEFAULT 0,
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
    input_cost_per_million_tokens numeric(18,6) NOT NULL DEFAULT 0,
    output_cost_per_million_tokens numeric(18,6) NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, organization_id)
);

ALTER TABLE model_aliases
ADD CONSTRAINT model_aliases_workspace_fk
FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE;

CREATE FUNCTION validate_model_alias_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.provider_account_id IS NOT NULL THEN
        PERFORM 1
        FROM provider_accounts
        WHERE id = NEW.provider_account_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'provider account % is not visible to model alias workspace %', NEW.provider_account_id, NEW.workspace_id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

CREATE UNIQUE INDEX model_aliases_org_alias_uq
ON model_aliases (organization_id, alias_key)
WHERE workspace_id IS NULL AND archived_at IS NULL;

CREATE UNIQUE INDEX model_aliases_workspace_alias_uq
ON model_aliases (workspace_id, alias_key)
WHERE workspace_id IS NOT NULL AND archived_at IS NULL;

CREATE TRIGGER model_catalog_entries_set_updated_at
BEFORE UPDATE ON model_catalog_entries
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER model_aliases_set_updated_at
BEFORE UPDATE ON model_aliases
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER model_aliases_validate_scope
BEFORE INSERT OR UPDATE ON model_aliases
FOR EACH ROW
EXECUTE FUNCTION validate_model_alias_scope();

-- Re-add the alias columns and FKs, then drop the model_id columns.
ALTER TABLE agent_deployments ADD COLUMN model_alias_id uuid;
ALTER TABLE agent_deployments
ADD CONSTRAINT agent_deployments_model_alias_fk
FOREIGN KEY (model_alias_id, organization_id) REFERENCES model_aliases (id, organization_id) ON DELETE RESTRICT;
ALTER TABLE agent_deployments DROP COLUMN model_id;

ALTER TABLE agent_deployment_snapshots ADD COLUMN source_model_alias_id uuid;
ALTER TABLE agent_deployment_snapshots
ADD CONSTRAINT agent_deployment_snapshots_model_alias_fk
FOREIGN KEY (source_model_alias_id, organization_id) REFERENCES model_aliases (id, organization_id) ON DELETE RESTRICT;
ALTER TABLE agent_deployment_snapshots DROP COLUMN source_model_id;

ALTER TABLE playground_experiments
ADD COLUMN model_alias_id uuid REFERENCES model_aliases (id) ON DELETE RESTRICT;
ALTER TABLE playground_experiments DROP COLUMN model_id;

-- Restore the alias visibility blocks in the deployment/snapshot scope triggers.
CREATE OR REPLACE FUNCTION validate_agent_deployment_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM 1
    FROM runtime_profiles
    WHERE id = NEW.runtime_profile_id
      AND organization_id = NEW.organization_id
      AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
    IF NOT FOUND THEN
        RAISE EXCEPTION 'runtime profile % is not visible to workspace %', NEW.runtime_profile_id, NEW.workspace_id;
    END IF;

    IF NEW.provider_account_id IS NOT NULL THEN
        PERFORM 1
        FROM provider_accounts
        WHERE id = NEW.provider_account_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'provider account % is not visible to workspace %', NEW.provider_account_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.model_alias_id IS NOT NULL THEN
        PERFORM 1
        FROM model_aliases
        WHERE id = NEW.model_alias_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'model alias % is not visible to workspace %', NEW.model_alias_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.routing_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM routing_policies
        WHERE id = NEW.routing_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'routing policy % is not visible to workspace %', NEW.routing_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.spend_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM spend_policies
        WHERE id = NEW.spend_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'spend policy % is not visible to workspace %', NEW.spend_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION validate_agent_deployment_snapshot_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM 1
    FROM runtime_profiles
    WHERE id = NEW.source_runtime_profile_id
      AND organization_id = NEW.organization_id
      AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
    IF NOT FOUND THEN
        RAISE EXCEPTION 'snapshot runtime profile % is not visible to workspace %', NEW.source_runtime_profile_id, NEW.workspace_id;
    END IF;

    IF NEW.source_provider_account_id IS NOT NULL THEN
        PERFORM 1
        FROM provider_accounts
        WHERE id = NEW.source_provider_account_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'snapshot provider account % is not visible to workspace %', NEW.source_provider_account_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.source_model_alias_id IS NOT NULL THEN
        PERFORM 1
        FROM model_aliases
        WHERE id = NEW.source_model_alias_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'snapshot model alias % is not visible to workspace %', NEW.source_model_alias_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.source_routing_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM routing_policies
        WHERE id = NEW.source_routing_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'snapshot routing policy % is not visible to workspace %', NEW.source_routing_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    IF NEW.source_spend_policy_id IS NOT NULL THEN
        PERFORM 1
        FROM spend_policies
        WHERE id = NEW.source_spend_policy_id
          AND organization_id = NEW.organization_id
          AND (workspace_id IS NULL OR workspace_id = NEW.workspace_id);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'snapshot spend policy % is not visible to workspace %', NEW.source_spend_policy_id, NEW.workspace_id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$;
