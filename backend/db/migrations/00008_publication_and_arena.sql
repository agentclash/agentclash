-- +goose Up
CREATE TABLE arenas (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_kind text NOT NULL CHECK (arena_kind IN ('official', 'community')),
    slug text NOT NULL,
    title text NOT NULL,
    description text,
    lifecycle_status text NOT NULL DEFAULT 'active' CHECK (lifecycle_status IN ('active', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (arena_kind, slug)
);

CREATE TABLE public_agent_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_kind text NOT NULL CHECK (profile_kind IN ('organization', 'community')),
    organization_id uuid REFERENCES organizations (id) ON DELETE CASCADE,
    owner_user_id uuid REFERENCES users (id) ON DELETE CASCADE,
    source_workspace_id uuid REFERENCES workspaces (id) ON DELETE SET NULL,
    display_name text NOT NULL,
    slug text NOT NULL,
    description text,
    visibility_status text NOT NULL DEFAULT 'draft' CHECK (visibility_status IN ('draft', 'published', 'archived')),
    is_verified boolean NOT NULL DEFAULT false,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (slug),
    CHECK (
        (profile_kind = 'organization' AND organization_id IS NOT NULL AND owner_user_id IS NULL) OR
        (profile_kind = 'community' AND organization_id IS NULL AND owner_user_id IS NOT NULL)
    )
);

CREATE TABLE publications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    run_id uuid NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    run_agent_id uuid REFERENCES run_agents (id) ON DELETE CASCADE,
    public_agent_profile_id uuid NOT NULL REFERENCES public_agent_profiles (id) ON DELETE RESTRICT,
    published_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    publication_status text NOT NULL DEFAULT 'draft' CHECK (publication_status IN ('draft', 'pending_review', 'published', 'rejected', 'withdrawn')),
    redaction_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE
);

ALTER TABLE publications
ADD CONSTRAINT publications_run_agent_fk
FOREIGN KEY (run_agent_id, run_id) REFERENCES run_agents (id, run_id) ON DELETE CASCADE;

CREATE TABLE arena_submissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES arenas (id) ON DELETE CASCADE,
    public_agent_profile_id uuid NOT NULL REFERENCES public_agent_profiles (id) ON DELETE CASCADE,
    source_publication_id uuid REFERENCES publications (id) ON DELETE SET NULL,
    challenge_pack_version_id uuid NOT NULL REFERENCES challenge_pack_versions (id) ON DELETE CASCADE,
    submission_status text NOT NULL DEFAULT 'pending_review' CHECK (submission_status IN ('pending_review', 'accepted', 'rejected', 'queued', 'published')),
    submitted_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    review_notes text,
    submitted_at timestamptz NOT NULL DEFAULT now(),
    reviewed_at timestamptz
);

CREATE TABLE public_run_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    publication_id uuid NOT NULL UNIQUE REFERENCES publications (id) ON DELETE CASCADE,
    arena_submission_id uuid REFERENCES arena_submissions (id) ON DELETE SET NULL,
    public_agent_profile_id uuid NOT NULL REFERENCES public_agent_profiles (id) ON DELETE CASCADE,
    challenge_pack_version_id uuid NOT NULL REFERENCES challenge_pack_versions (id) ON DELETE CASCADE,
    challenge_identity_id uuid REFERENCES challenge_identities (id) ON DELETE SET NULL,
    replay_artifact_id uuid REFERENCES artifacts (id) ON DELETE SET NULL,
    summary jsonb NOT NULL DEFAULT '{}'::jsonb,
    score_summary jsonb NOT NULL DEFAULT '{}'::jsonb,
    published_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE leaderboard_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES arenas (id) ON DELETE CASCADE,
    public_run_snapshot_id uuid NOT NULL REFERENCES public_run_snapshots (id) ON DELETE CASCADE,
    challenge_pack_version_id uuid NOT NULL REFERENCES challenge_pack_versions (id) ON DELETE CASCADE,
    challenge_identity_id uuid REFERENCES challenge_identities (id) ON DELETE SET NULL,
    ranking_scope text NOT NULL CHECK (ranking_scope IN ('pack', 'challenge')),
    rank integer NOT NULL CHECK (rank > 0),
    score numeric(7,4) NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (arena_id, ranking_scope, public_run_snapshot_id, challenge_identity_id),
    UNIQUE (arena_id, challenge_pack_version_id, challenge_identity_id, ranking_scope, rank)
);

CREATE INDEX arena_submissions_arena_id_idx ON arena_submissions (arena_id, submission_status);
CREATE INDEX public_run_snapshots_profile_id_idx ON public_run_snapshots (public_agent_profile_id, published_at DESC);
CREATE INDEX leaderboard_entries_arena_rank_idx ON leaderboard_entries (arena_id, ranking_scope, rank);

CREATE TRIGGER arenas_set_updated_at
BEFORE UPDATE ON arenas
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER public_agent_profiles_set_updated_at
BEFORE UPDATE ON public_agent_profiles
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER publications_set_updated_at
BEFORE UPDATE ON publications
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER public_run_snapshots_set_updated_at
BEFORE UPDATE ON public_run_snapshots
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS public_run_snapshots_set_updated_at ON public_run_snapshots;
DROP TRIGGER IF EXISTS publications_set_updated_at ON publications;
DROP TRIGGER IF EXISTS public_agent_profiles_set_updated_at ON public_agent_profiles;
DROP TRIGGER IF EXISTS arenas_set_updated_at ON arenas;

DROP TABLE IF EXISTS leaderboard_entries;
DROP TABLE IF EXISTS public_run_snapshots;
DROP TABLE IF EXISTS arena_submissions;
DROP TABLE IF EXISTS publications;
DROP TABLE IF EXISTS public_agent_profiles;
DROP TABLE IF EXISTS arenas;
