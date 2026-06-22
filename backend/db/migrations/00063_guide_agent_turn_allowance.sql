-- +goose Up
-- +goose StatementBegin

-- 4e: guide-agent turn allowance — a monthly per-workspace QUOTA COUNTER, distinct from the $3 eval
-- credit wallet. The counter lives on the existing usage window (alongside race_count); the LIMIT is a
-- stored entitlement column (mirroring races_per_workspace_month, NULL ⇒ unlimited/custom).
ALTER TABLE workspace_usage_windows
    ADD COLUMN guide_agent_turn_count integer NOT NULL DEFAULT 0 CHECK (guide_agent_turn_count >= 0);

ALTER TABLE organization_entitlements
    ADD COLUMN guide_agent_turns_per_workspace_month integer;

-- Backfill existing entitlement rows from their plan, mirroring billing.materializeLimit / the §C numbers:
-- free → 50 (flat), pro → 1000·seats, team → 4000·seats, enterprise → NULL (custom/unlimited). New and
-- re-upserted rows get the materialized value through UpsertOrganizationEntitlements.
UPDATE organization_entitlements
SET guide_agent_turns_per_workspace_month = CASE plan_key
    WHEN 'free' THEN 50
    WHEN 'pro' THEN 1000 * seat_quantity
    WHEN 'team' THEN 4000 * seat_quantity
    ELSE NULL
END
WHERE guide_agent_turns_per_workspace_month IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE organization_entitlements DROP COLUMN guide_agent_turns_per_workspace_month;
ALTER TABLE workspace_usage_windows DROP COLUMN guide_agent_turn_count;

-- +goose StatementEnd
