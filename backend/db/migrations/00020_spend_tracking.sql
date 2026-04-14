-- +goose Up

-- Pricing columns on model catalog so we can compute cost at execution time.
ALTER TABLE model_catalog_entries
ADD COLUMN input_cost_per_million_tokens numeric(18,6) NOT NULL DEFAULT 0,
ADD COLUMN output_cost_per_million_tokens numeric(18,6) NOT NULL DEFAULT 0;

-- Per-run cost snapshot, written after scoring completes.
CREATE TABLE run_cost_summaries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL UNIQUE REFERENCES runs (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    total_cost_usd numeric(18,6) NOT NULL DEFAULT 0,
    total_input_tokens bigint NOT NULL DEFAULT 0,
    total_output_tokens bigint NOT NULL DEFAULT 0,
    cost_breakdown jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX run_cost_summaries_workspace_idx
ON run_cost_summaries (workspace_id, created_at);

-- Accumulates actual spend per (spend_policy, window). Upserted after each run.
CREATE TABLE workspace_spend_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    spend_policy_id uuid NOT NULL REFERENCES spend_policies (id) ON DELETE CASCADE,
    window_start timestamptz NOT NULL,
    window_end timestamptz NOT NULL,
    total_cost_usd numeric(18,6) NOT NULL DEFAULT 0,
    total_input_tokens bigint NOT NULL DEFAULT 0,
    total_output_tokens bigint NOT NULL DEFAULT 0,
    run_count int NOT NULL DEFAULT 0,
    last_run_id uuid REFERENCES runs (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    UNIQUE (spend_policy_id, window_start)
);

CREATE INDEX workspace_spend_ledger_workspace_window_idx
ON workspace_spend_ledger (workspace_id, window_start, window_end);

CREATE TRIGGER workspace_spend_ledger_set_updated_at
BEFORE UPDATE ON workspace_spend_ledger
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS workspace_spend_ledger_set_updated_at ON workspace_spend_ledger;
DROP TABLE IF EXISTS workspace_spend_ledger;
DROP TABLE IF EXISTS run_cost_summaries;
ALTER TABLE model_catalog_entries
DROP COLUMN IF EXISTS input_cost_per_million_tokens,
DROP COLUMN IF EXISTS output_cost_per_million_tokens;
