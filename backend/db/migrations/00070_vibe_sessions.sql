-- +goose Up
-- Private Vibe sessions are intentionally separate from public tryouts and the
-- discontinued guide-agent schema. All USD values are integer nano-USD.
CREATE TABLE vibe_sessions (
 id uuid PRIMARY KEY, actor text NOT NULL, trial_key text,
 workspace_id uuid REFERENCES workspaces(id), revision bigint NOT NULL DEFAULT 0,
 title text NOT NULL DEFAULT 'New conversation', document jsonb NOT NULL,
 saved_draft_id uuid REFERENCES challenge_pack_drafts(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK (octet_length(document::text) <= 8388608)
);
CREATE INDEX vibe_sessions_actor_idx ON vibe_sessions(actor,updated_at DESC);
CREATE TABLE vibe_accounts (
 id text PRIMARY KEY, balance bigint NOT NULL DEFAULT 0 CHECK(balance>=0),
 held bigint NOT NULL DEFAULT 0 CHECK(held>=0), disabled boolean NOT NULL DEFAULT false,
 CHECK(held<=balance)
);
CREATE TABLE vibe_grants (
 source text PRIMARY KEY, account_id text NOT NULL REFERENCES vibe_accounts(id),
 amount bigint NOT NULL CHECK(amount>0), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE vibe_operations (
 id uuid PRIMARY KEY, session_id uuid NOT NULL REFERENCES vibe_sessions(id), actor text NOT NULL,
 client_id uuid NOT NULL, request_hash text NOT NULL, kind text NOT NULL,
 state text NOT NULL CHECK(state IN ('CREATED','VALIDATING','AWAITING_INPUT','AWAITING_APPROVAL','RESERVED','QUEUED','RUNNING','FINALIZING','CANCELLING','COMPLETED','PARTIAL','FAILED','CANCELLED','EXPIRED')),
 billing text NOT NULL CHECK(billing IN ('UNRESERVED','RESERVED','RECONCILING','SETTLED','RELEASED')),
 models jsonb NOT NULL, input jsonb NOT NULL, max_cost bigint NOT NULL CHECK(max_cost>=0),
 actual_cost bigint CHECK(actual_cost>=0), model_calls integer NOT NULL DEFAULT 0,
 error jsonb, created_at timestamptz NOT NULL DEFAULT now(), deadline timestamptz NOT NULL,
 dispatch_started_at timestamptz, completed_at timestamptz,
 UNIQUE(session_id,client_id), CHECK(octet_length(input::text)<=8388608)
);
CREATE INDEX vibe_operations_pending_idx ON vibe_operations(state,created_at);
CREATE TABLE vibe_reservations (
 operation_id uuid NOT NULL REFERENCES vibe_operations(id), account_id text NOT NULL REFERENCES vibe_accounts(id),
 amount bigint NOT NULL CHECK(amount>0), settled_amount bigint CHECK(settled_amount>=0),
 PRIMARY KEY(operation_id,account_id)
);
CREATE TABLE vibe_attempts (
 id uuid PRIMARY KEY, operation_id uuid NOT NULL REFERENCES vibe_operations(id), step_key text NOT NULL,
 role text NOT NULL CHECK(role IN ('assistant','target','evaluator')),
 model text NOT NULL, provider text NOT NULL, policy jsonb NOT NULL,
 request_hash text NOT NULL, input_bound integer NOT NULL, max_output integer NOT NULL,
 max_cost bigint NOT NULL CHECK(max_cost>0), actual_cost bigint,
 state text NOT NULL CHECK(state IN ('DISPATCHING','SUCCEEDED','UNCERTAIN','RECONCILED')),
 generation_id text, output text NOT NULL DEFAULT '', usage jsonb, error jsonb,
 created_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz,
 UNIQUE(operation_id,step_key), CHECK(octet_length(output)<=1048576)
);
CREATE TABLE vibe_case_results (
 operation_id uuid NOT NULL REFERENCES vibe_operations(id), case_key text NOT NULL, version text NOT NULL,
 result jsonb NOT NULL, PRIMARY KEY(operation_id,case_key,version)
);
CREATE TABLE vibe_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, session_id uuid NOT NULL REFERENCES vibe_sessions(id),
 operation_id uuid REFERENCES vibe_operations(id), kind text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX vibe_events_session_idx ON vibe_events(session_id,id);
CREATE TABLE vibe_outbox (
 operation_id uuid PRIMARY KEY REFERENCES vibe_operations(id), delivered_at timestamptz
);
CREATE TABLE vibe_disabled_profiles (model text PRIMARY KEY, reason text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());

-- +goose Down
DROP TABLE IF EXISTS vibe_disabled_profiles,vibe_outbox,vibe_events,vibe_case_results,vibe_attempts,vibe_reservations,vibe_operations,vibe_grants,vibe_accounts,vibe_sessions;
