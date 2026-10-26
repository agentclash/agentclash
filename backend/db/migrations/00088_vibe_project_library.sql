-- +goose Up
ALTER TABLE vibe_sessions ADD COLUMN archived_at timestamptz;
UPDATE vibe_sessions SET title=CASE WHEN document#>>'{evaluation,door}'='build' THEN 'New prototype ' ELSE 'New evaluation ' END||left(id::text,8) WHERE title IN ('New conversation','New evaluation');
CREATE INDEX vibe_sessions_library_idx ON vibe_sessions(actor,archived_at,updated_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE vibe_import_originals (
 id uuid PRIMARY KEY, session_id uuid NOT NULL REFERENCES vibe_sessions(id), artifact_id uuid NOT NULL,
 filename text NOT NULL, media_type text NOT NULL, content_sha256 text NOT NULL,
 content bytea NOT NULL CHECK(octet_length(content)<=4194304), created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(session_id,artifact_id)
);
-- Workspace save receipts own an independent original copy, retained on private project deletion.
CREATE TABLE vibe_pack_sources (
 draft_id uuid PRIMARY KEY REFERENCES challenge_pack_drafts(id) ON DELETE CASCADE,
 session_id uuid NOT NULL REFERENCES vibe_sessions(id), artifact_id uuid NOT NULL, workspace_id uuid NOT NULL REFERENCES workspaces(id),
 composition_hash text NOT NULL, case_keys jsonb NOT NULL DEFAULT '[]',
 original_filename text, original_media_type text, original_hash text, original_content bytea CHECK(octet_length(original_content)<=4194304),
 advanced boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(session_id,artifact_id,workspace_id,advanced)
);
CREATE TABLE vibe_published_case_bindings (
 -- A published binding protects its source receipt from later draft deletion.
 draft_id uuid NOT NULL REFERENCES vibe_pack_sources(draft_id),
 artifact_id uuid NOT NULL, challenge_pack_version_id uuid NOT NULL REFERENCES challenge_pack_versions(id),
 case_key text NOT NULL, input_set_key text NOT NULL, challenge_key text NOT NULL,
 published_case_id uuid NOT NULL REFERENCES challenge_input_items(id),
 PRIMARY KEY(draft_id,challenge_pack_version_id,input_set_key,challenge_key,case_key)
);
-- +goose Down
DROP TABLE vibe_published_case_bindings,vibe_pack_sources,vibe_import_originals;
DROP INDEX vibe_sessions_library_idx;
ALTER TABLE vibe_sessions DROP COLUMN archived_at;
