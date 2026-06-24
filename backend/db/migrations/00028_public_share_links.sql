-- +goose Up
CREATE TABLE public_share_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key text NOT NULL UNIQUE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    resource_type text NOT NULL CHECK (resource_type IN ('challenge_pack_version', 'run_scorecard', 'run_agent_scorecard', 'run_agent_replay')),
    resource_id uuid NOT NULL,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    is_active boolean NOT NULL DEFAULT true,
    search_indexing boolean NOT NULL DEFAULT false,
    view_count bigint NOT NULL DEFAULT 0 CHECK (view_count >= 0),
    last_accessed_at timestamptz,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX public_share_links_active_resource_unique
ON public_share_links (resource_type, resource_id)
WHERE is_active;

CREATE INDEX public_share_links_resource_idx ON public_share_links (resource_type, resource_id) WHERE is_active;
CREATE INDEX public_share_links_workspace_idx ON public_share_links (workspace_id, created_at DESC);

CREATE TRIGGER public_share_links_set_updated_at
BEFORE UPDATE ON public_share_links
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS public_share_links_set_updated_at ON public_share_links;
DROP TABLE IF EXISTS public_share_links;
