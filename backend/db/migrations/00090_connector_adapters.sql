-- +goose Up
-- Secrets retain their existing encryption format. Revision changes on every value write.
ALTER TABLE workspace_secrets ADD COLUMN revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0);
-- +goose StatementBegin
CREATE FUNCTION bump_workspace_secret_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 NEW.revision := OLD.revision + 1;
 RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER workspace_secret_revision BEFORE UPDATE OF encrypted_value ON workspace_secrets FOR EACH ROW EXECUTE FUNCTION bump_workspace_secret_revision();

-- Additive extension of canonical immutable deployment snapshots; native rows need none.
CREATE TABLE connector_snapshots (
 snapshot_id uuid PRIMARY KEY REFERENCES agent_deployment_snapshots(id) ON DELETE CASCADE,
 workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 adapter text NOT NULL CHECK (adapter IN ('http_json','hosted')),
 config jsonb NOT NULL,
 target_origin text NOT NULL,
 secret_id uuid NOT NULL,
 secret_revision bigint NOT NULL,
 approved_by uuid REFERENCES users(id),
 approved_at timestamptz,
 revoked_at timestamptz,
 FOREIGN KEY(snapshot_id,organization_id,workspace_id) REFERENCES agent_deployment_snapshots(id,organization_id,workspace_id)
);
CREATE TABLE connector_run_deadlines(run_agent_id uuid PRIMARY KEY REFERENCES run_agents(id) ON DELETE CASCADE,deadline_at timestamptz NOT NULL);
CREATE TABLE connector_dispatches (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 run_agent_id uuid NOT NULL REFERENCES run_agents(id) ON DELETE CASCADE,
 source_case_id uuid NOT NULL,
 case_key text NOT NULL,
 challenge_key text NOT NULL,
 snapshot_id uuid NOT NULL REFERENCES connector_snapshots(snapshot_id) ON DELETE CASCADE,
 input_hash text NOT NULL,
 state text NOT NULL CHECK (state IN ('sending','accepted','completed','rejected','unknown')),
 deadline_at timestamptz NOT NULL,
 response jsonb,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(run_agent_id,source_case_id)
);
ALTER TABLE hosted_run_executions DROP CONSTRAINT hosted_run_executions_status_check;
ALTER TABLE hosted_run_executions ADD CONSTRAINT hosted_run_executions_status_check CHECK(status IN ('starting','accepted','running','completed','failed','timed_out','unknown'));
-- Remote IDs are meaningful only inside the authenticated run-agent dispatch.
DROP INDEX hosted_run_executions_external_run_id_uq;
CREATE TABLE hosted_event_receipts (
 run_agent_id uuid NOT NULL REFERENCES run_agents(id) ON DELETE CASCADE,
 source_event_id text NOT NULL,
 payload_hash text NOT NULL,
 original_event jsonb NOT NULL,
 sequence_number bigint NOT NULL,
 PRIMARY KEY(run_agent_id,source_event_id)
);
CREATE TABLE hosted_terminal_notifications (
 run_agent_id uuid PRIMARY KEY REFERENCES run_agents(id) ON DELETE CASCADE,
 run_id uuid NOT NULL REFERENCES runs(id),
 event jsonb NOT NULL,
 delivered_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE hosted_run_executions ADD COLUMN dispatch_claimed_at timestamptz;

-- +goose StatementBegin
CREATE FUNCTION immutable_connector_binding() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.snapshot_id,NEW.workspace_id,NEW.organization_id,NEW.adapter,NEW.config,NEW.target_origin,NEW.secret_id,NEW.secret_revision)
 IS DISTINCT FROM ROW(OLD.snapshot_id,OLD.workspace_id,OLD.organization_id,OLD.adapter,OLD.config,OLD.target_origin,OLD.secret_id,OLD.secret_revision) THEN
  RAISE EXCEPTION 'connector binding is immutable; create a new snapshot';
 END IF;
 RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER immutable_connector_binding BEFORE UPDATE ON connector_snapshots FOR EACH ROW EXECUTE FUNCTION immutable_connector_binding();

-- +goose Down
-- Rollback deliberately retains dispatch/evidence/secret revision data for reconciliation.
-- Disable connector writers before deploying an older binary; no destructive down migration.
