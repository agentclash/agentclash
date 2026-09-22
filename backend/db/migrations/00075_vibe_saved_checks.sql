-- +goose Up
CREATE TABLE vibe_saved_checks (
 id uuid PRIMARY KEY,
 session_id uuid NOT NULL REFERENCES vibe_sessions(id),
 artifact_id uuid NOT NULL,
 baseline_operation_id uuid NOT NULL REFERENCES vibe_operations(id),
 workspace_id uuid NOT NULL REFERENCES workspaces(id),
 created_by uuid NOT NULL REFERENCES users(id),
 title text NOT NULL,
 source jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(session_id,baseline_operation_id)
);
CREATE INDEX vibe_saved_checks_owner ON vibe_saved_checks(created_by,workspace_id,created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS vibe_saved_checks;
