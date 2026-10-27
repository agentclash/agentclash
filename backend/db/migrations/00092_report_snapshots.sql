-- +goose Up
CREATE TABLE report_snapshots (
 id uuid PRIMARY KEY, workspace_id uuid NOT NULL REFERENCES workspaces(id),
 organization_id uuid NOT NULL REFERENCES organizations(id), actor text NOT NULL,
 client_id uuid NOT NULL, request_hash text NOT NULL, content_hash text NOT NULL,
 body jsonb NOT NULL, created_at timestamptz NOT NULL,
 UNIQUE(workspace_id,actor,client_id),
 FOREIGN KEY (workspace_id,organization_id) REFERENCES workspaces(id,organization_id)
);
CREATE INDEX report_snapshots_workspace ON report_snapshots(workspace_id,created_at DESC,id);
CREATE TABLE report_snapshot_notes (
 id uuid PRIMARY KEY, snapshot_id uuid NOT NULL REFERENCES report_snapshots(id),
 author_id uuid NOT NULL REFERENCES users(id), client_id uuid NOT NULL,
 target_id text NOT NULL, text text NOT NULL CHECK(octet_length(text) BETWEEN 1 AND 4096),
 created_at timestamptz NOT NULL, UNIQUE(snapshot_id,author_id,client_id)
);
CREATE INDEX report_snapshot_notes_report ON report_snapshot_notes(snapshot_id,created_at,id);
-- +goose StatementBegin
CREATE FUNCTION report_snapshot_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Selected report content and annotations are immutable'; END;
$$;
-- +goose StatementEnd
CREATE TRIGGER report_snapshots_immutable BEFORE UPDATE ON report_snapshots FOR EACH ROW EXECUTE FUNCTION report_snapshot_immutable();
CREATE TRIGGER report_snapshot_notes_immutable BEFORE UPDATE ON report_snapshot_notes FOR EACH ROW EXECUTE FUNCTION report_snapshot_immutable();
ALTER TABLE public_share_links DROP CONSTRAINT public_share_links_resource_type_check;
ALTER TABLE public_share_links ADD CONSTRAINT public_share_links_resource_type_check CHECK(resource_type IN ('challenge_pack_version','run_scorecard','run_agent_scorecard','run_agent_replay','agent_tryout','report_snapshot'));
-- +goose Down
-- Preserve independent evidence and existing capability references on rollback.
SELECT 1;
