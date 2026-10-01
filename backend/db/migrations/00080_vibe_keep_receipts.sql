-- +goose Up
-- A kept pack points to the run selected when saving, never a later rerun.
ALTER TABLE vibe_saved_artifacts ADD COLUMN baseline_operation_id uuid REFERENCES vibe_operations(id);
-- Older receipts have no known selected run. Keep that association unknown.
CREATE TABLE vibe_saved_briefs (
 id uuid PRIMARY KEY,
 session_id uuid NOT NULL REFERENCES vibe_sessions(id),
 artifact_id uuid NOT NULL,
 workspace_id uuid NOT NULL REFERENCES workspaces(id),
 created_by uuid NOT NULL REFERENCES users(id),
 artifact jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(session_id,artifact_id)
);
CREATE INDEX vibe_saved_briefs_owner ON vibe_saved_briefs(created_by,workspace_id,created_at DESC);

-- +goose Down
DROP TABLE vibe_saved_briefs;
ALTER TABLE vibe_saved_artifacts DROP COLUMN baseline_operation_id;
