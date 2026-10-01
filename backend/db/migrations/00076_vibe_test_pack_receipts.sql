-- +goose Up
-- Tests are reusable independently of creating an agent build. The session's
-- immutable artifact and model receipt retain the separately supplied target.
ALTER TABLE vibe_saved_artifacts ALTER COLUMN build_id DROP NOT NULL;
ALTER TABLE vibe_saved_artifacts ALTER COLUMN build_version_id DROP NOT NULL;
ALTER TABLE vibe_saved_artifacts ADD CONSTRAINT vibe_saved_build_pair CHECK (
 (build_id IS NULL) = (build_version_id IS NULL)
);

-- +goose Down
-- Refuse rollback while tests-only receipts exist; never delete saved work.
ALTER TABLE vibe_saved_artifacts ALTER COLUMN build_id SET NOT NULL;
ALTER TABLE vibe_saved_artifacts ALTER COLUMN build_version_id SET NOT NULL;
ALTER TABLE vibe_saved_artifacts DROP CONSTRAINT vibe_saved_build_pair;
