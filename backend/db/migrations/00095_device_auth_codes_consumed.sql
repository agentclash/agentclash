-- +goose Up
-- A device code moves to 'consumed' when the CLI receives its token. The
-- stale-code sweeper only expires pending and approved codes and only
-- revokes tokens of expired codes, so a token that the CLI holds but has not
-- used yet stays valid.
ALTER TABLE device_auth_codes DROP CONSTRAINT device_auth_codes_status_check;
ALTER TABLE device_auth_codes ADD CONSTRAINT device_auth_codes_status_check
    CHECK (status IN ('pending', 'approved', 'denied', 'expired', 'consumed'));

-- Before this migration, a consumed code stayed 'approved' with its raw token
-- cleared. Approval always stores a raw token, so these rows are consumed.
UPDATE device_auth_codes
SET status = 'consumed'
WHERE status = 'approved' AND raw_token IS NULL;

-- +goose Down
-- Restore the old representation of a consumed code: approved, no raw token.
UPDATE device_auth_codes
SET status = 'approved'
WHERE status = 'consumed';

ALTER TABLE device_auth_codes DROP CONSTRAINT device_auth_codes_status_check;
ALTER TABLE device_auth_codes ADD CONSTRAINT device_auth_codes_status_check
    CHECK (status IN ('pending', 'approved', 'denied', 'expired'));
