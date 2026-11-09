-- +goose Up
-- Immutable intake identity survives a lost response and a later session revision.
CREATE TABLE vibe_evidence_receipts (
 session_id uuid NOT NULL REFERENCES vibe_sessions(id), client_id uuid NOT NULL,
 request_hash text NOT NULL, evidence_id uuid NOT NULL, artifact_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(session_id,client_id)
);

CREATE TABLE vibe_grade_reviews (
 id uuid PRIMARY KEY, session_id uuid NOT NULL REFERENCES vibe_sessions(id),
 client_id uuid NOT NULL, request_hash text NOT NULL, result_kind text NOT NULL
 CHECK(result_kind IN ('vibe_operation','run','eval_session','sdk_report')),
 result_id uuid NOT NULL, body jsonb NOT NULL CHECK(octet_length(body::text)<=1048576),
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(session_id,client_id)
);
CREATE INDEX vibe_grade_reviews_source_idx ON vibe_grade_reviews(session_id,result_kind,result_id,created_at,id);

CREATE TABLE vibe_validation_reviews (
 id uuid PRIMARY KEY, session_id uuid NOT NULL REFERENCES vibe_sessions(id),
 operation_id uuid NOT NULL REFERENCES vibe_operations(id), stage_key text NOT NULL,
 candidate_hash text NOT NULL, policy_hash text NOT NULL,
 body jsonb NOT NULL CHECK(octet_length(body::text)<=262144),
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(operation_id,stage_key,candidate_hash,policy_hash)
);
CREATE INDEX vibe_validation_reviews_session_idx ON vibe_validation_reviews(session_id,created_at,id);

-- +goose Down
DROP TABLE IF EXISTS vibe_validation_reviews,vibe_grade_reviews,vibe_evidence_receipts;
