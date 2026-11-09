-- +goose Up
-- Nullable only for legacy sources whose authority was never retained.
ALTER TABLE eval_sessions ADD COLUMN workspace_id uuid REFERENCES workspaces(id);
CREATE INDEX eval_sessions_workspace_created_idx ON eval_sessions(workspace_id, created_at DESC, id DESC);

CREATE TABLE eval_session_admissions (
 workspace_id uuid NOT NULL REFERENCES workspaces(id),
 request_id uuid NOT NULL,
 request_hash text NOT NULL,
 eval_session_id uuid NOT NULL UNIQUE REFERENCES eval_sessions(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(workspace_id,request_id)
);

-- +goose Down
-- Retain eval-session ownership and admission receipts on binary rollback.
SELECT 1;
