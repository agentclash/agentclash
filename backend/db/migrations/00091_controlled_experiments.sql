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
CREATE TABLE eval_set_admissions (
 workspace_id uuid NOT NULL REFERENCES workspaces(id),
 request_id uuid NOT NULL,
 request_hash text NOT NULL,
 eval_set_id uuid NOT NULL UNIQUE REFERENCES eval_sets(id),
 PRIMARY KEY(workspace_id,request_id)
);
CREATE TABLE run_agent_controlled_settings (
 run_agent_id uuid PRIMARY KEY REFERENCES run_agents(id),
 snapshot_id uuid NOT NULL REFERENCES agent_deployment_snapshots(id),
 settings_hash text NOT NULL,
 settings jsonb NOT NULL,
 admitted_at timestamptz NOT NULL DEFAULT now(),
 dispatch_validated_at timestamptz,
 dispatch_guard_version integer
);
-- +goose StatementBegin
CREATE FUNCTION retain_controlled_settings() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.run_agent_id,NEW.snapshot_id,NEW.settings_hash,NEW.settings,NEW.admitted_at) IS DISTINCT FROM (OLD.run_agent_id,OLD.snapshot_id,OLD.settings_hash,OLD.settings,OLD.admitted_at) THEN
 RAISE EXCEPTION 'admitted controlled settings are immutable';
 END IF;
 RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER retain_controlled_settings BEFORE UPDATE ON run_agent_controlled_settings FOR EACH ROW EXECUTE FUNCTION retain_controlled_settings();

-- +goose Down
-- Retain admitted ownership and experiment evidence on binary rollback.
SELECT 1;
