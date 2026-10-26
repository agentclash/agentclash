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
-- Workspace receipts outlive both the private project and the editable canonical draft.
-- draft_id is the immutable originating draft UUID, not a live draft foreign key.
CREATE TABLE vibe_pack_sources (
 draft_id uuid PRIMARY KEY,
 session_id uuid NOT NULL REFERENCES vibe_sessions(id), artifact_id uuid NOT NULL, workspace_id uuid NOT NULL REFERENCES workspaces(id),
 composition_hash text NOT NULL, case_keys jsonb NOT NULL DEFAULT '[]',
 original_filename text, original_media_type text, original_hash text, original_content bytea CHECK(octet_length(original_content)<=4194304),
 advanced boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now()
);
-- Save idempotency is serialized by the session row lock and checks live drafts.
-- A deleted draft can be saved again with a new independent receipt.
CREATE INDEX vibe_pack_sources_session_idx ON vibe_pack_sources(session_id,artifact_id,workspace_id,advanced);
CREATE TABLE vibe_published_case_bindings (
 -- Published mappings reference the retained receipt, not the deletable editor draft.
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
