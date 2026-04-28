# Issue 423 Contract: CLI Auth Login Human Identity Label

## Scope

Fix issue #423 so normal WorkOS-backed CLI sessions expose a human-readable identity label to the CLI. The preferred label source is `display_name`, then `email`, with `user_id` remaining only a last-resort fallback.

## Functional Expectations

- WorkOS authentication should populate `Caller.Email` from a verified WorkOS JWT email claim or `/oauth2/userinfo` when the stored user row is missing email.
- WorkOS authentication should populate `Caller.DisplayName` from verified WorkOS identity/profile data when available.
- New auto-created WorkOS users should store any available display name and email.
- Existing active WorkOS users with missing email and/or display name should be backfilled opportunistically from verified WorkOS identity/profile data.
- Stub-user link and existing-user relink flows should preserve existing profile fields and should backfill missing email/display name when verified identity/profile data is available.
- `/v1/auth/session` should continue to return `user_id` for automation and should include `email`/`display_name` when known.
- CLI fallback order remains display name -> email -> user ID.

## Tests To Add Or Run

- Add backend auth unit tests covering:
  - WorkOS userinfo containing email and name for an existing user missing both fields.
  - New WorkOS user creation stores email and display name from userinfo/profile data.
  - Existing user with display name already set keeps that display name.
  - CLI-token session path returns the stored display name/email after backfill.
- Run targeted backend auth tests:
  - `go test ./internal/api -run 'TestWorkOS|TestCLIToken|TestSession'`
- Run targeted CLI auth tests:
  - `cd cli && go test ./cmd ./internal/auth -run 'Auth|Login|Status'`
- Run broader validation if targeted tests pass:
  - `go test ./internal/api ./internal/repository`
  - `cd cli && go test -short ./...`

## Manual Verification

- With a logged-in CLI token, `agentclash auth status --json` should include at least one of `email` or `display_name` for a normal WorkOS account once backend data has been backfilled.
- `agentclash auth login --device` should print a human-readable account label when the backend session provides one.

## Out Of Scope

- Changing CLI auth fallback behavior beyond preserving display name -> email -> user ID.
- Reworking WorkOS token issuance or CLI device-login UX.
- Data migration for historical users beyond opportunistic backfill during authenticated requests.
