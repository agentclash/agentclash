-- +goose Up
CREATE TABLE vibe_saved_artifacts (
 session_id uuid NOT NULL REFERENCES vibe_sessions(id), artifact_id uuid NOT NULL,
 workspace_id uuid NOT NULL REFERENCES workspaces(id),
 draft_id uuid NOT NULL REFERENCES challenge_pack_drafts(id),
 build_id uuid NOT NULL REFERENCES agent_builds(id), build_version_id uuid NOT NULL REFERENCES agent_build_versions(id),
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(session_id,artifact_id)
);
CREATE TABLE vibe_credit_checkouts (
 id uuid PRIMARY KEY, organization_id uuid NOT NULL REFERENCES organizations(id), created_by uuid NOT NULL REFERENCES users(id),
 product_id text NOT NULL, credits bigint NOT NULL CHECK(credits>0), price_minor bigint NOT NULL CHECK(price_minor>0),
 currency text NOT NULL CHECK(currency='USD'), state text NOT NULL CHECK(state IN ('DISPATCHING','READY','UNCERTAIN','PAID')),
 checkout_url text, remote_id text, payment_id text UNIQUE, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE vibe_credit_payment_events (
 payment_id text PRIMARY KEY, checkout_id uuid REFERENCES vibe_credit_checkouts(id),
 organization_id uuid NOT NULL REFERENCES organizations(id), amount bigint NOT NULL, payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS vibe_credit_payment_events,vibe_credit_checkouts,vibe_saved_artifacts;
