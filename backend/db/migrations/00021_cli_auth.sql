-- +goose Up

-- CLI tokens: long-lived, separately revocable tokens for CLI/CI authentication.
-- Only the SHA-256 hash is persisted for auth lookup. The raw token is returned
-- exactly once at creation time and never stored server-side.
CREATE TABLE cli_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL,
    name text NOT NULL DEFAULT '',
    last_used_at timestamptz,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);

CREATE UNIQUE INDEX cli_tokens_token_hash_uq ON cli_tokens (token_hash);
CREATE INDEX cli_tokens_user_id_idx ON cli_tokens (user_id);

-- Device authorization codes for the RFC 8628-style device code flow.
-- The CLI gets a device_code (secret) and user_code (displayed). The user
-- visits the web app, enters the user_code, and approves. The CLI polls
-- until the status transitions from 'pending' to 'approved'.
--
-- raw_token is a temporary column that holds the raw CLI token after approval,
-- allowing the polling CLI to retrieve it. It is NULLed out after first retrieval.
CREATE TABLE device_auth_codes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    device_code text NOT NULL,
    user_code text NOT NULL,
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'denied', 'expired')),
    user_id uuid REFERENCES users (id),
    cli_token_id uuid REFERENCES cli_tokens (id) ON DELETE SET NULL,
    raw_token text,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX device_auth_codes_device_code_uq ON device_auth_codes (device_code);
CREATE UNIQUE INDEX device_auth_codes_user_code_pending_uq ON device_auth_codes (user_code)
    WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS device_auth_codes;
DROP TABLE IF EXISTS cli_tokens;
