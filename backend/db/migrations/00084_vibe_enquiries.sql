-- +goose Up
CREATE TABLE vibe_enquiries (
 id uuid PRIMARY KEY,
 session_id uuid NOT NULL REFERENCES vibe_sessions(id),
 client_id uuid NOT NULL,
 request_hash text NOT NULL,
 content jsonb NOT NULL,
 transport_id text NOT NULL,
 status text NOT NULL DEFAULT 'received' CHECK(status IN ('received','sending','provider_accepted','needs_review','cancelled')),
 created_at timestamptz NOT NULL DEFAULT now(),
 first_attempt_at timestamptz,
 next_attempt_at timestamptz NOT NULL DEFAULT now(),
 lease_until timestamptz,
 attempts integer NOT NULL DEFAULT 0,
 provider_id text NOT NULL DEFAULT '',
 UNIQUE(session_id,client_id)
);
CREATE INDEX vibe_enquiries_pending ON vibe_enquiries(status,next_attempt_at);
-- +goose Down
DROP TABLE vibe_enquiries;
