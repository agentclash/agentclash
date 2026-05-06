-- +goose Up

ALTER TABLE organization_memberships
    ADD COLUMN invite_token text,
    ADD COLUMN invite_token_expires_at timestamptz;

CREATE UNIQUE INDEX organization_memberships_invite_token_uq
    ON organization_memberships (invite_token)
    WHERE invite_token IS NOT NULL;

ALTER TABLE workspace_memberships
    ADD COLUMN invite_token text,
    ADD COLUMN invite_token_expires_at timestamptz;

CREATE UNIQUE INDEX workspace_memberships_invite_token_uq
    ON workspace_memberships (invite_token)
    WHERE invite_token IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS workspace_memberships_invite_token_uq;
ALTER TABLE workspace_memberships
    DROP COLUMN IF EXISTS invite_token_expires_at,
    DROP COLUMN IF EXISTS invite_token;

DROP INDEX IF EXISTS organization_memberships_invite_token_uq;
ALTER TABLE organization_memberships
    DROP COLUMN IF EXISTS invite_token_expires_at,
    DROP COLUMN IF EXISTS invite_token;
