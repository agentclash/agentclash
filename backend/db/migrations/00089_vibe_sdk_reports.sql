-- +goose Up
CREATE TABLE vibe_sdk_reports (
 id uuid PRIMARY KEY,
 session_id uuid NOT NULL REFERENCES vibe_sessions(id),
 actor text NOT NULL,
 workspace_id uuid NOT NULL REFERENCES workspaces(id),
 artifact_id uuid NOT NULL,
 external_report_id text NOT NULL CHECK(octet_length(external_report_id)<=256),
 request_hash text NOT NULL,
 source_json bytea NOT NULL CHECK(octet_length(source_json)<=1048576),
 content_sha256 text NOT NULL,
 body jsonb NOT NULL,
 storage_bytes integer NOT NULL CHECK(storage_bytes>0),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(session_id,external_report_id)
);
CREATE INDEX vibe_sdk_reports_artifact ON vibe_sdk_reports(session_id,artifact_id,created_at,id);
-- +goose StatementBegin
CREATE FUNCTION vibe_sdk_reports_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'SDK report originals and metadata are immutable';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER vibe_sdk_reports_immutable BEFORE UPDATE ON vibe_sdk_reports FOR EACH ROW EXECUTE FUNCTION vibe_sdk_reports_immutable();
CREATE TABLE vibe_sdk_promotions (
 session_id uuid NOT NULL REFERENCES vibe_sessions(id), client_id uuid NOT NULL,
 report_id uuid NOT NULL REFERENCES vibe_sdk_reports(id), source_case_id text NOT NULL,
 source_evidence_hash text NOT NULL, request_hash text NOT NULL, artifact jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(session_id,client_id)
);
CREATE TRIGGER vibe_sdk_promotions_immutable BEFORE UPDATE ON vibe_sdk_promotions FOR EACH ROW EXECUTE FUNCTION vibe_sdk_reports_immutable();
-- +goose Down
DROP TABLE vibe_sdk_promotions;
DROP TABLE vibe_sdk_reports;
DROP FUNCTION vibe_sdk_reports_immutable();
