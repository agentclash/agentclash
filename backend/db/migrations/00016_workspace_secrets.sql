-- +goose Up
CREATE TABLE workspace_secrets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    key text NOT NULL,
    encrypted_value bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid REFERENCES users (id),
    updated_by uuid REFERENCES users (id),
    CONSTRAINT workspace_secrets_key_format CHECK (key ~ '^[A-Za-z_][A-Za-z0-9_]*$'),
    CONSTRAINT workspace_secrets_key_length CHECK (char_length(key) BETWEEN 1 AND 128)
);

CREATE UNIQUE INDEX workspace_secrets_workspace_key_uq
ON workspace_secrets (workspace_id, key);

CREATE TRIGGER workspace_secrets_set_updated_at
BEFORE UPDATE ON workspace_secrets
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS workspace_secrets_set_updated_at ON workspace_secrets;
DROP INDEX IF EXISTS workspace_secrets_workspace_key_uq;
DROP TABLE IF EXISTS workspace_secrets;
