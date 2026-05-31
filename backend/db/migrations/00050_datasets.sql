-- +goose Up
CREATE TABLE datasets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    input_schema jsonb,
    input_schema_enforced boolean NOT NULL DEFAULT false,
    default_challenge_pack_version_id uuid REFERENCES challenge_pack_versions (id) ON DELETE SET NULL,
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id) ON DELETE CASCADE,
    UNIQUE (workspace_id, slug)
);

CREATE INDEX datasets_workspace_id_archived_at_idx
    ON datasets (workspace_id, archived_at);

CREATE TABLE dataset_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id uuid NOT NULL REFERENCES datasets (id) ON DELETE CASCADE,
    version_number integer NOT NULL CHECK (version_number > 0),
    label text,
    example_count integer NOT NULL DEFAULT 0 CHECK (example_count >= 0),
    manifest_checksum text NOT NULL,
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (dataset_id, version_number)
);

CREATE TABLE dataset_examples (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id uuid NOT NULL REFERENCES datasets (id) ON DELETE CASCADE,
    external_id text,
    input jsonb NOT NULL,
    expected jsonb,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    tags text[] NOT NULL DEFAULT '{}'::text[],
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived', 'muted')),
    source text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'import', 'trace', 'synthetic', 'promotion')),
    source_run_id uuid REFERENCES runs (id) ON DELETE SET NULL,
    source_trace_id text,
    source_platform text,
    artifact_id uuid REFERENCES artifacts (id) ON DELETE SET NULL,
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX dataset_examples_dataset_id_status_idx
    ON dataset_examples (dataset_id, status);

CREATE UNIQUE INDEX dataset_examples_dataset_id_external_id_uq
    ON dataset_examples (dataset_id, external_id)
    WHERE external_id IS NOT NULL;

CREATE INDEX dataset_examples_tags_gin_idx
    ON dataset_examples USING gin (tags);

CREATE INDEX dataset_examples_metadata_gin_idx
    ON dataset_examples USING gin (metadata);

CREATE TABLE dataset_example_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id uuid NOT NULL REFERENCES datasets (id) ON DELETE CASCADE,
    example_id uuid REFERENCES dataset_examples (id) ON DELETE SET NULL,
    version_id uuid REFERENCES dataset_versions (id) ON DELETE SET NULL,
    operation text NOT NULL CHECK (operation IN ('insert', 'update', 'delete')),
    before jsonb,
    after jsonb,
    actor uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX dataset_example_revisions_example_id_created_at_idx
    ON dataset_example_revisions (example_id, created_at DESC);

CREATE TABLE dataset_version_examples (
    version_id uuid NOT NULL REFERENCES dataset_versions (id) ON DELETE CASCADE,
    example_id uuid NOT NULL REFERENCES dataset_examples (id) ON DELETE CASCADE,
    revision_id uuid NOT NULL REFERENCES dataset_example_revisions (id) ON DELETE RESTRICT,
    PRIMARY KEY (version_id, example_id)
);

CREATE TRIGGER datasets_set_updated_at
BEFORE UPDATE ON datasets
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER dataset_examples_set_updated_at
BEFORE UPDATE ON dataset_examples
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS dataset_examples_set_updated_at ON dataset_examples;
DROP TRIGGER IF EXISTS datasets_set_updated_at ON datasets;
DROP TABLE IF EXISTS dataset_version_examples;
DROP INDEX IF EXISTS dataset_example_revisions_example_id_created_at_idx;
DROP TABLE IF EXISTS dataset_example_revisions;
DROP INDEX IF EXISTS dataset_examples_metadata_gin_idx;
DROP INDEX IF EXISTS dataset_examples_tags_gin_idx;
DROP INDEX IF EXISTS dataset_examples_dataset_id_external_id_uq;
DROP INDEX IF EXISTS dataset_examples_dataset_id_status_idx;
DROP TABLE IF EXISTS dataset_examples;
DROP TABLE IF EXISTS dataset_versions;
DROP INDEX IF EXISTS datasets_workspace_id_archived_at_idx;
DROP TABLE IF EXISTS datasets;
