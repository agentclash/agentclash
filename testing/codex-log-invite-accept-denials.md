# codex/log-invite-accept-denials - Test Contract

## Functional Behavior
- Token invite accept should keep the #620 security behavior: a caller with an empty email may accept a valid invite token when the invite row has an email.
- A caller with a non-empty email that differs from the invite row email should still be rejected.
- A malformed invite row with no invite email should still be rejected.
- Rejections from token invite acceptance should carry a private, structured reason that server logs can report without logging the token or raw email addresses.
- HTTP responses should remain unchanged: forbidden invite acceptance still returns `403` with `access denied`.

## Unit Tests
- `TestInviteTokenAcceptError_AllowsEmptyCallerEmail` - empty caller email and non-empty invite email returns no error.
- `TestInviteTokenAcceptError_RejectsEmptyInviteEmail` - empty invite email returns `ErrForbidden` and reason metadata.
- `TestInviteTokenAcceptError_RejectsMismatchedCallerEmail` - mismatched non-empty caller email returns `ErrForbidden` and reason metadata.
- Existing org and workspace token accept tests continue to pass.

## Integration / Functional Tests
- `go test ./internal/api` from `backend/` should pass.

## Smoke Tests
- N/A - this change is backend-only diagnostic behavior and does not start services.

## E2E Tests
- N/A - production verification requires a fresh failed invite click after deployment and checking backend logs for the structured denial reason.

## Manual / cURL Tests
- N/A locally - invite acceptance needs seeded membership data and authenticated WorkOS sessions.
