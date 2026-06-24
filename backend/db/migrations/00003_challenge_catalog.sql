-- +goose Up
CREATE TABLE eval_packs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug text NOT NULL,
    name text NOT NULL,
    family text NOT NULL,
    description text,
    lifecycle_status text NOT NULL DEFAULT 'draft' CHECK (lifecycle_status IN ('draft', 'active', 'deprecated', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (slug)
);

CREATE TABLE eval_pack_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    eval_pack_id uuid NOT NULL REFERENCES eval_packs (id) ON DELETE CASCADE,
    version_number integer NOT NULL CHECK (version_number > 0),
    lifecycle_status text NOT NULL DEFAULT 'draft' CHECK (lifecycle_status IN ('draft', 'runnable', 'deprecated', 'archived')),
    manifest_checksum text NOT NULL,
    manifest jsonb NOT NULL DEFAULT '{}'::jsonb,
    published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (eval_pack_id, version_number),
    UNIQUE (id, eval_pack_id)
);

CREATE TABLE challenge_identities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    eval_pack_id uuid NOT NULL REFERENCES eval_packs (id) ON DELETE CASCADE,
    challenge_key text NOT NULL,
    name text NOT NULL,
    category text NOT NULL,
    difficulty text NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard', 'expert')),
    description text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (eval_pack_id, challenge_key),
    UNIQUE (id, eval_pack_id)
);

CREATE TABLE eval_pack_version_challenges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    eval_pack_version_id uuid NOT NULL,
    eval_pack_id uuid NOT NULL,
    challenge_identity_id uuid NOT NULL,
    execution_order integer NOT NULL CHECK (execution_order >= 0),
    title_snapshot text NOT NULL,
    category_snapshot text NOT NULL,
    difficulty_snapshot text NOT NULL CHECK (difficulty_snapshot IN ('easy', 'medium', 'hard', 'expert')),
    challenge_definition jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (eval_pack_version_id, challenge_identity_id),
    UNIQUE (eval_pack_version_id, execution_order),
    FOREIGN KEY (eval_pack_version_id, eval_pack_id) REFERENCES eval_pack_versions (id, eval_pack_id) ON DELETE CASCADE,
    FOREIGN KEY (challenge_identity_id, eval_pack_id) REFERENCES challenge_identities (id, eval_pack_id) ON DELETE CASCADE
);

CREATE TABLE challenge_input_sets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    eval_pack_version_id uuid NOT NULL REFERENCES eval_pack_versions (id) ON DELETE CASCADE,
    input_key text NOT NULL,
    name text NOT NULL,
    description text,
    input_checksum text NOT NULL,
    generated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (eval_pack_version_id, input_key),
    UNIQUE (id, eval_pack_version_id)
);

CREATE TABLE challenge_input_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    challenge_input_set_id uuid NOT NULL,
    eval_pack_version_id uuid NOT NULL,
    challenge_identity_id uuid NOT NULL,
    item_key text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (challenge_input_set_id, challenge_identity_id, item_key),
    FOREIGN KEY (challenge_input_set_id, eval_pack_version_id) REFERENCES challenge_input_sets (id, eval_pack_version_id) ON DELETE CASCADE,
    FOREIGN KEY (eval_pack_version_id, challenge_identity_id) REFERENCES eval_pack_version_challenges (eval_pack_version_id, challenge_identity_id) ON DELETE CASCADE
);

CREATE INDEX eval_pack_versions_pack_id_idx ON eval_pack_versions (eval_pack_id);
CREATE INDEX eval_pack_version_challenges_pack_version_idx ON eval_pack_version_challenges (eval_pack_version_id);
CREATE INDEX challenge_input_items_input_set_idx ON challenge_input_items (challenge_input_set_id);

CREATE TRIGGER eval_packs_set_updated_at
BEFORE UPDATE ON eval_packs
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER eval_pack_versions_set_updated_at
BEFORE UPDATE ON eval_pack_versions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER challenge_identities_set_updated_at
BEFORE UPDATE ON challenge_identities
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER challenge_input_sets_set_updated_at
BEFORE UPDATE ON challenge_input_sets
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS challenge_input_sets_set_updated_at ON challenge_input_sets;
DROP TRIGGER IF EXISTS challenge_identities_set_updated_at ON challenge_identities;
DROP TRIGGER IF EXISTS eval_pack_versions_set_updated_at ON eval_pack_versions;
DROP TRIGGER IF EXISTS eval_packs_set_updated_at ON eval_packs;

DROP TABLE IF EXISTS challenge_input_items;
DROP TABLE IF EXISTS challenge_input_sets;
DROP TABLE IF EXISTS eval_pack_version_challenges;
DROP TABLE IF EXISTS challenge_identities;
DROP TABLE IF EXISTS eval_pack_versions;
DROP TABLE IF EXISTS eval_packs;
