-- +goose Up
-- Rename the "challenge pack" product concept to "eval pack" across the schema.
--
-- Migrations 00001-00063 are immutable and still create the original
-- challenge_pack* objects. This forward migration renames them, so that BOTH a
-- freshly-created database (which runs 00001-00063 then this) and the already
-- deployed database (which has 00001-00063 applied and runs only this) converge
-- to the identical eval_pack* schema. No historical migration is rewritten.
--
-- The "challenge" concept *inside* a pack -- challenge_identities,
-- challenge_input_sets/items, challenge_key, challenge_definition,
-- challenge_pieces, challenge_input_set_id -- is intentionally preserved; only
-- the "pack" aggregate is renamed.

-- 1. Tables ------------------------------------------------------------------
ALTER TABLE challenge_packs                   RENAME TO eval_packs;
ALTER TABLE challenge_pack_versions           RENAME TO eval_pack_versions;
ALTER TABLE challenge_pack_version_challenges RENAME TO eval_pack_version_challenges;
ALTER TABLE challenge_pack_drafts             RENAME TO eval_pack_drafts;

-- 2. Columns (substring challenge_pack -> eval_pack) -------------------------
ALTER TABLE eval_pack_versions           RENAME COLUMN challenge_pack_id         TO eval_pack_id;
ALTER TABLE challenge_identities         RENAME COLUMN challenge_pack_id         TO eval_pack_id;
ALTER TABLE eval_pack_version_challenges RENAME COLUMN challenge_pack_version_id TO eval_pack_version_id;
ALTER TABLE eval_pack_version_challenges RENAME COLUMN challenge_pack_id         TO eval_pack_id;
ALTER TABLE eval_pack_drafts             RENAME COLUMN challenge_pack_id         TO eval_pack_id;

ALTER TABLE challenge_input_sets         RENAME COLUMN challenge_pack_version_id TO eval_pack_version_id;
ALTER TABLE challenge_input_items        RENAME COLUMN challenge_pack_version_id TO eval_pack_version_id;
ALTER TABLE runs                         RENAME COLUMN challenge_pack_version_id TO eval_pack_version_id;
ALTER TABLE evaluation_specs             RENAME COLUMN challenge_pack_version_id TO eval_pack_version_id;
ALTER TABLE arena_submissions            RENAME COLUMN challenge_pack_version_id TO eval_pack_version_id;
ALTER TABLE public_run_snapshots         RENAME COLUMN challenge_pack_version_id TO eval_pack_version_id;
ALTER TABLE leaderboard_entries          RENAME COLUMN challenge_pack_version_id TO eval_pack_version_id;
ALTER TABLE dataset_version_input_sets   RENAME COLUMN challenge_pack_version_id TO eval_pack_version_id;
ALTER TABLE dataset_baselines            RENAME COLUMN challenge_pack_version_id TO eval_pack_version_id;

ALTER TABLE workspace_regression_suites  RENAME COLUMN source_challenge_pack_id         TO source_eval_pack_id;
ALTER TABLE workspace_regression_cases   RENAME COLUMN source_challenge_pack_version_id TO source_eval_pack_version_id;
ALTER TABLE vibe_eval_drafts             RENAME COLUMN published_challenge_pack_id         TO published_eval_pack_id;
ALTER TABLE vibe_eval_drafts             RENAME COLUMN published_challenge_pack_version_id TO published_eval_pack_version_id;
ALTER TABLE datasets                     RENAME COLUMN default_challenge_pack_version_id  TO default_eval_pack_version_id;

-- 3. Indexes -----------------------------------------------------------------
ALTER INDEX challenge_packs_global_slug_uq                              RENAME TO eval_packs_global_slug_uq;
ALTER INDEX challenge_packs_workspace_slug_uq                           RENAME TO eval_packs_workspace_slug_uq;
ALTER INDEX challenge_packs_workspace_id_idx                            RENAME TO eval_packs_workspace_id_idx;
ALTER INDEX challenge_pack_versions_pack_id_idx                         RENAME TO eval_pack_versions_pack_id_idx;
ALTER INDEX challenge_pack_version_challenges_pack_version_idx          RENAME TO eval_pack_version_challenges_pack_version_idx;
ALTER INDEX challenge_pack_drafts_workspace_idx                         RENAME TO eval_pack_drafts_workspace_idx;
ALTER INDEX runs_challenge_pack_version_id_idx                          RENAME TO runs_eval_pack_version_id_idx;
ALTER INDEX evaluation_specs_challenge_pack_version_name_version_uq     RENAME TO evaluation_specs_eval_pack_version_name_version_uq;
ALTER INDEX workspace_regression_cases_source_challenge_pack_version_idx RENAME TO workspace_regression_cases_source_eval_pack_version_idx;

-- 4. Triggers ----------------------------------------------------------------
ALTER TRIGGER challenge_packs_set_updated_at         ON eval_packs         RENAME TO eval_packs_set_updated_at;
ALTER TRIGGER challenge_pack_versions_set_updated_at ON eval_pack_versions RENAME TO eval_pack_versions_set_updated_at;
ALTER TRIGGER challenge_pack_drafts_set_updated_at   ON eval_pack_drafts   RENAME TO eval_pack_drafts_set_updated_at;

-- 5. Persisted enum VALUES + their CHECK constraints -------------------------
-- public_share_links.resource_type: 'challenge_pack_version' -> 'eval_pack_version'
ALTER TABLE public_share_links DROP CONSTRAINT IF EXISTS public_share_links_resource_type_check;
UPDATE public_share_links SET resource_type = 'eval_pack_version' WHERE resource_type = 'challenge_pack_version';
ALTER TABLE public_share_links ADD CONSTRAINT public_share_links_resource_type_check
    CHECK (resource_type IN ('eval_pack_version', 'run_scorecard', 'run_agent_scorecard', 'run_agent_replay', 'agent_tryout'));

-- runs.source_type: 'challenge_pack' -> 'eval_pack'. Drop every CHECK that still
-- references the old value (regardless of its generated name), migrate the data,
-- then recreate the value check and the source-shape check.
-- +goose StatementBegin
DO $$
DECLARE c text;
BEGIN
    FOR c IN
        SELECT conname FROM pg_constraint
        WHERE conrelid = 'runs'::regclass AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%challenge_pack%'
    LOOP
        EXECUTE format('ALTER TABLE runs DROP CONSTRAINT %I', c);
    END LOOP;
END $$;
-- +goose StatementEnd
UPDATE runs SET source_type = 'eval_pack' WHERE source_type = 'challenge_pack';
ALTER TABLE runs ALTER COLUMN source_type SET DEFAULT 'eval_pack';
ALTER TABLE runs ADD CONSTRAINT runs_source_type_check
    CHECK (source_type IN ('eval_pack', 'agent_harness'));
ALTER TABLE runs ADD CONSTRAINT runs_source_shape_check
    CHECK (
        (source_type = 'eval_pack' AND eval_pack_version_id IS NOT NULL)
        OR
        (source_type = 'agent_harness' AND eval_pack_version_id IS NULL AND challenge_input_set_id IS NULL)
    );

-- vibe_eval_drafts.draft_kind: 'challenge_pack' -> 'eval_pack'
-- +goose StatementBegin
DO $$
DECLARE c text;
BEGIN
    FOR c IN
        SELECT conname FROM pg_constraint
        WHERE conrelid = 'vibe_eval_drafts'::regclass AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%challenge_pack%'
    LOOP
        EXECUTE format('ALTER TABLE vibe_eval_drafts DROP CONSTRAINT %I', c);
    END LOOP;
END $$;
-- +goose StatementEnd
UPDATE vibe_eval_drafts SET draft_kind = 'eval_pack' WHERE draft_kind = 'challenge_pack';
ALTER TABLE vibe_eval_drafts ADD CONSTRAINT vibe_eval_drafts_draft_kind_check
    CHECK (draft_kind IN ('eval_plan', 'eval_pack', 'input_cases', 'scoring', 'runtime'));

-- 5b. Auto-generated PK / UNIQUE / FK constraint names left behind by the table
-- and column renames above (e.g. challenge_packs_pkey on eval_packs). Renaming a
-- constraint also renames its backing index, so this clears the remaining
-- challenge_pack residue. Done dynamically because the names are Postgres-
-- generated (and some are truncated to 63 chars), so they cannot be hand-written
-- reliably across environments.
-- +goose StatementBegin
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT conrelid::regclass::text AS tbl, conname AS oldname,
               replace(conname, 'challenge_pack', 'eval_pack') AS newname
        FROM pg_constraint
        WHERE connamespace = 'public'::regnamespace
          AND conname LIKE '%challenge_pack%'
    LOOP
        EXECUTE format('ALTER TABLE %s RENAME CONSTRAINT %I TO %I', r.tbl, r.oldname, r.newname);
    END LOOP;
END $$;
-- +goose StatementEnd

-- 6. Persisted free-text artifact values (no CHECK constraint) ---------------
UPDATE artifacts SET artifact_type = 'eval_pack_bundle' WHERE artifact_type = 'challenge_pack_bundle';
UPDATE artifacts
   SET metadata = jsonb_set(metadata, '{artifact_role}', '"eval_pack_bundle"'::jsonb)
 WHERE metadata ->> 'artifact_role' = 'challenge_pack_bundle';

-- +goose Down
-- Reverse the rename. (Never executed by scripts/migrate.sh, which applies only
-- the Up section; provided for goose-based local rollback and documentation.)

-- 6. Artifact values
UPDATE artifacts SET artifact_type = 'challenge_pack_bundle' WHERE artifact_type = 'eval_pack_bundle';
UPDATE artifacts
   SET metadata = jsonb_set(metadata, '{artifact_role}', '"challenge_pack_bundle"'::jsonb)
 WHERE metadata ->> 'artifact_role' = 'eval_pack_bundle';

-- 4. Triggers
ALTER TRIGGER eval_packs_set_updated_at         ON eval_packs         RENAME TO challenge_packs_set_updated_at;
ALTER TRIGGER eval_pack_versions_set_updated_at ON eval_pack_versions RENAME TO challenge_pack_versions_set_updated_at;
ALTER TRIGGER eval_pack_drafts_set_updated_at   ON eval_pack_drafts   RENAME TO challenge_pack_drafts_set_updated_at;

-- 3. Indexes
ALTER INDEX eval_packs_global_slug_uq                              RENAME TO challenge_packs_global_slug_uq;
ALTER INDEX eval_packs_workspace_slug_uq                           RENAME TO challenge_packs_workspace_slug_uq;
ALTER INDEX eval_packs_workspace_id_idx                            RENAME TO challenge_packs_workspace_id_idx;
ALTER INDEX eval_pack_versions_pack_id_idx                         RENAME TO challenge_pack_versions_pack_id_idx;
ALTER INDEX eval_pack_version_challenges_pack_version_idx          RENAME TO challenge_pack_version_challenges_pack_version_idx;
ALTER INDEX eval_pack_drafts_workspace_idx                         RENAME TO challenge_pack_drafts_workspace_idx;
ALTER INDEX runs_eval_pack_version_id_idx                          RENAME TO runs_challenge_pack_version_id_idx;
ALTER INDEX evaluation_specs_eval_pack_version_name_version_uq     RENAME TO evaluation_specs_challenge_pack_version_name_version_uq;
ALTER INDEX workspace_regression_cases_source_eval_pack_version_idx RENAME TO workspace_regression_cases_source_challenge_pack_version_idx;

-- 2. Columns
ALTER TABLE datasets                     RENAME COLUMN default_eval_pack_version_id  TO default_challenge_pack_version_id;
ALTER TABLE vibe_eval_drafts             RENAME COLUMN published_eval_pack_version_id TO published_challenge_pack_version_id;
ALTER TABLE vibe_eval_drafts             RENAME COLUMN published_eval_pack_id         TO published_challenge_pack_id;
ALTER TABLE workspace_regression_cases   RENAME COLUMN source_eval_pack_version_id    TO source_challenge_pack_version_id;
ALTER TABLE workspace_regression_suites  RENAME COLUMN source_eval_pack_id            TO source_challenge_pack_id;

ALTER TABLE dataset_baselines            RENAME COLUMN eval_pack_version_id TO challenge_pack_version_id;
ALTER TABLE dataset_version_input_sets   RENAME COLUMN eval_pack_version_id TO challenge_pack_version_id;
ALTER TABLE leaderboard_entries          RENAME COLUMN eval_pack_version_id TO challenge_pack_version_id;
ALTER TABLE public_run_snapshots         RENAME COLUMN eval_pack_version_id TO challenge_pack_version_id;
ALTER TABLE arena_submissions            RENAME COLUMN eval_pack_version_id TO challenge_pack_version_id;
ALTER TABLE evaluation_specs             RENAME COLUMN eval_pack_version_id TO challenge_pack_version_id;
ALTER TABLE runs                         RENAME COLUMN eval_pack_version_id TO challenge_pack_version_id;
ALTER TABLE challenge_input_items        RENAME COLUMN eval_pack_version_id TO challenge_pack_version_id;
ALTER TABLE challenge_input_sets         RENAME COLUMN eval_pack_version_id TO challenge_pack_version_id;

ALTER TABLE eval_pack_drafts             RENAME COLUMN eval_pack_id         TO challenge_pack_id;
ALTER TABLE eval_pack_version_challenges RENAME COLUMN eval_pack_id         TO challenge_pack_id;
ALTER TABLE eval_pack_version_challenges RENAME COLUMN eval_pack_version_id TO challenge_pack_version_id;
ALTER TABLE challenge_identities         RENAME COLUMN eval_pack_id         TO challenge_pack_id;
ALTER TABLE eval_pack_versions           RENAME COLUMN eval_pack_id         TO challenge_pack_id;

-- 1. Tables
ALTER TABLE eval_pack_drafts             RENAME TO challenge_pack_drafts;
ALTER TABLE eval_pack_version_challenges RENAME TO challenge_pack_version_challenges;
ALTER TABLE eval_pack_versions           RENAME TO challenge_pack_versions;
ALTER TABLE eval_packs                   RENAME TO challenge_packs;

-- 5b. Restore auto-generated constraint names.
-- +goose StatementBegin
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT conrelid::regclass::text AS tbl, conname AS oldname,
               replace(conname, 'eval_pack', 'challenge_pack') AS newname
        FROM pg_constraint
        WHERE connamespace = 'public'::regnamespace
          AND conname LIKE '%eval_pack%'
    LOOP
        EXECUTE format('ALTER TABLE %s RENAME CONSTRAINT %I TO %I', r.tbl, r.oldname, r.newname);
    END LOOP;
END $$;
-- +goose StatementEnd

-- 5. Persisted enum VALUES + their CHECK constraints (restore old values)
ALTER TABLE public_share_links DROP CONSTRAINT IF EXISTS public_share_links_resource_type_check;
UPDATE public_share_links SET resource_type = 'challenge_pack_version' WHERE resource_type = 'eval_pack_version';
ALTER TABLE public_share_links ADD CONSTRAINT public_share_links_resource_type_check
    CHECK (resource_type IN ('challenge_pack_version', 'run_scorecard', 'run_agent_scorecard', 'run_agent_replay', 'agent_tryout'));

-- +goose StatementBegin
DO $$
DECLARE c text;
BEGIN
    FOR c IN
        SELECT conname FROM pg_constraint
        WHERE conrelid = 'runs'::regclass AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%eval_pack%'
    LOOP
        EXECUTE format('ALTER TABLE runs DROP CONSTRAINT %I', c);
    END LOOP;
END $$;
-- +goose StatementEnd
UPDATE runs SET source_type = 'challenge_pack' WHERE source_type = 'eval_pack';
ALTER TABLE runs ALTER COLUMN source_type SET DEFAULT 'challenge_pack';
ALTER TABLE runs ADD CONSTRAINT runs_source_type_check
    CHECK (source_type IN ('challenge_pack', 'agent_harness'));
ALTER TABLE runs ADD CONSTRAINT runs_source_shape_check
    CHECK (
        (source_type = 'challenge_pack' AND challenge_pack_version_id IS NOT NULL)
        OR
        (source_type = 'agent_harness' AND challenge_pack_version_id IS NULL AND challenge_input_set_id IS NULL)
    );

-- +goose StatementBegin
DO $$
DECLARE c text;
BEGIN
    FOR c IN
        SELECT conname FROM pg_constraint
        WHERE conrelid = 'vibe_eval_drafts'::regclass AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%eval_pack%'
    LOOP
        EXECUTE format('ALTER TABLE vibe_eval_drafts DROP CONSTRAINT %I', c);
    END LOOP;
END $$;
-- +goose StatementEnd
UPDATE vibe_eval_drafts SET draft_kind = 'challenge_pack' WHERE draft_kind = 'eval_pack';
ALTER TABLE vibe_eval_drafts ADD CONSTRAINT vibe_eval_drafts_draft_kind_check
    CHECK (draft_kind IN ('eval_plan', 'challenge_pack', 'input_cases', 'scoring', 'runtime'));
