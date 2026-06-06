-- +goose Up

ALTER TABLE public_share_links
DROP CONSTRAINT public_share_links_resource_type_check;

ALTER TABLE public_share_links
ADD CONSTRAINT public_share_links_resource_type_check
CHECK (resource_type IN ('challenge_pack_version', 'run_scorecard', 'run_agent_scorecard', 'run_agent_replay', 'agent_tryout'));

CREATE TABLE agent_tryouts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid,
    template_slug text NOT NULL,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    input_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    template_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    tool_policy_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    evaluation_spec_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    selected_model_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    summary jsonb NOT NULL DEFAULT '{}'::jsonb,
    redaction_status text NOT NULL DEFAULT 'pending' CHECK (redaction_status IN ('pending', 'passed', 'failed', 'not_required')),
    run_id uuid REFERENCES runs (id) ON DELETE SET NULL,
    cost_limit_usd numeric(12, 6) NOT NULL DEFAULT 0 CHECK (cost_limit_usd >= 0),
    actual_cost_usd numeric(12, 6) CHECK (actual_cost_usd IS NULL OR actual_cost_usd >= 0),
    latency_ms bigint CHECK (latency_ms IS NULL OR latency_ms >= 0),
    max_duration_seconds integer NOT NULL DEFAULT 0 CHECK (max_duration_seconds >= 0),
    anonymous_fingerprint_hash text,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    claimed_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    claimed_at timestamptz,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (organization_id IS NULL AND workspace_id IS NULL)
        OR (organization_id IS NOT NULL AND workspace_id IS NOT NULL)
    ),
    CHECK (
        (claimed_by_user_id IS NULL AND claimed_at IS NULL)
        OR (claimed_by_user_id IS NOT NULL AND claimed_at IS NOT NULL)
    ),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE
);

CREATE INDEX agent_tryouts_workspace_idx
ON agent_tryouts (workspace_id, created_at DESC)
WHERE workspace_id IS NOT NULL;

CREATE INDEX agent_tryouts_template_status_idx
ON agent_tryouts (template_slug, status, created_at DESC);

CREATE INDEX agent_tryouts_anonymous_fingerprint_idx
ON agent_tryouts (anonymous_fingerprint_hash, created_at DESC)
WHERE anonymous_fingerprint_hash IS NOT NULL;

CREATE INDEX agent_tryouts_run_id_idx
ON agent_tryouts (run_id)
WHERE run_id IS NOT NULL;

CREATE TRIGGER agent_tryouts_set_updated_at
BEFORE UPDATE ON agent_tryouts
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down

DROP TRIGGER IF EXISTS agent_tryouts_set_updated_at ON agent_tryouts;
DROP INDEX IF EXISTS agent_tryouts_run_id_idx;
DROP INDEX IF EXISTS agent_tryouts_anonymous_fingerprint_idx;
DROP INDEX IF EXISTS agent_tryouts_template_status_idx;
DROP INDEX IF EXISTS agent_tryouts_workspace_idx;
DROP TABLE IF EXISTS agent_tryouts;

ALTER TABLE public_share_links
DROP CONSTRAINT public_share_links_resource_type_check;

ALTER TABLE public_share_links
ADD CONSTRAINT public_share_links_resource_type_check
CHECK (resource_type IN ('challenge_pack_version', 'run_scorecard', 'run_agent_scorecard', 'run_agent_replay'));
