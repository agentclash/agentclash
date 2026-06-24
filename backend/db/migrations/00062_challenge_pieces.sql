-- +goose Up
-- challenge_pieces is the reusable, workspace-scoped library of authoring
-- "pieces" that the visual pack builder composes into a challenge pack.
-- It is polymorphic: a piece's typed shape lives in `definition` (a
-- scoring.ValidatorDeclaration, scoring.LLMJudgeDeclaration, a
-- challengepack.ChallengeDefinition, or an input-set {name, cases[]}),
-- discriminated by `kind`. Multi-turn user simulators are not a separate
-- kind; they ride inside input_set cases (CaseDefinition.user_simulator).
--
-- Pieces are mutable. They are resolved and SNAPSHOTTED into the immutable
-- challenge_pack_versions.manifest at publish time (see
-- challengepack.ComposeBundle + repository.PublishChallengePackBundle), so
-- editing a piece never mutates an already-published pack version. This
-- mirrors the workspace_regression_cases snapshot model (00023).
CREATE TABLE challenge_pieces (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('validator', 'judge', 'input_set', 'challenge')),
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    definition jsonb NOT NULL DEFAULT '{}'::jsonb,
    lifecycle_status text NOT NULL DEFAULT 'active' CHECK (lifecycle_status IN ('active', 'archived')),
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);

-- A slug is unique per (workspace, kind) among active pieces; archiving frees
-- the slug for reuse, matching the regression-suite active-uniqueness pattern.
CREATE UNIQUE INDEX challenge_pieces_workspace_kind_slug_active_idx
    ON challenge_pieces (workspace_id, kind, slug)
    WHERE lifecycle_status = 'active';

CREATE INDEX challenge_pieces_workspace_kind_idx
    ON challenge_pieces (workspace_id, kind, created_at DESC)
    WHERE lifecycle_status = 'active';

CREATE TRIGGER challenge_pieces_set_updated_at
BEFORE UPDATE ON challenge_pieces
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS challenge_pieces_set_updated_at ON challenge_pieces;
DROP INDEX IF EXISTS challenge_pieces_workspace_kind_idx;
DROP INDEX IF EXISTS challenge_pieces_workspace_kind_slug_active_idx;
DROP TABLE IF EXISTS challenge_pieces;
