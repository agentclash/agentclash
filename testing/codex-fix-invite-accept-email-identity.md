# codex/fix-invite-accept-email-identity - Test Contract

## Functional Behavior
- WorkOS userinfo fallback uses an HTTP method accepted by production AuthKit userinfo.
- Token invite acceptance succeeds when the caller has the invite token and WorkOS returns an empty caller email.
- Token invite acceptance stays forbidden when WorkOS returns a non-empty caller email that differs from the invited email.
- Legacy membership-id invite acceptance still requires the caller email to match the invited email.
- Organization and workspace invite tokens are cleared after successful acceptance.

## Unit Tests
- `TestWorkOSAuthenticator_FallsBackToUserInfoEmailWhenClaimMissing` verifies the fallback userinfo request method and profile backfill.
- `TestWorkOSAuthenticator_BackfillsMissingIdentityFromUserInfo` verifies missing profile backfill via userinfo.
- Organization invite manager tests cover empty-email token acceptance and mismatched non-empty email rejection.
- Workspace invite manager tests cover empty-email token acceptance and mismatched non-empty email rejection.

## Integration / Functional Tests
- `go test ./internal/api` validates auth and invite manager behavior together at the API package level.

## Smoke Tests
- N/A - production smoke should be creating an invite, opening the emailed `/invites/.../invite_...` URL while logged in, and seeing the accept request return `200` instead of `403`.

## E2E Tests
- N/A - no browser E2E harness is part of this narrowly scoped backend fix.

## Manual / cURL Tests
```bash
curl -X PATCH "$AGENTCLASH_API_URL/v1/invites/organization/invite_<token>" \
  -H "Authorization: Bearer <authkit-session-token>"
# Expected: 200 when token is valid and the caller email is either empty or matches the invite email.
```
