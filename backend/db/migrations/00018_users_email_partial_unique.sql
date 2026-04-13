-- +goose Up

-- WorkOS AuthKit JWTs do not include an email claim by default. Users who
-- sign in without email in their token get email = ''. The old unconditional
-- UNIQUE(email) allowed only ONE such user; every subsequent sign-up collided.
-- Replace with a partial unique index that ignores empty emails.

ALTER TABLE users ALTER COLUMN email SET DEFAULT '';
ALTER TABLE users DROP CONSTRAINT users_email_key;
CREATE UNIQUE INDEX users_email_uq ON users (email) WHERE email != '';

-- +goose Down

DROP INDEX IF EXISTS users_email_uq;
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
ALTER TABLE users ALTER COLUMN email DROP DEFAULT;
