# codex/fix-cli-login-identity — Test Contract

## Functional Behavior
- `agentclash auth login` must never print a blank identity after a successful login.
- Human-readable auth success messages must use this fallback order: `display_name`, then `email`, then `user_id`.
- The same fallback must apply to both fresh login and already-authenticated login paths.
- When a valid WorkOS-authenticated request includes a non-empty email and the stored user record for that same WorkOS identity has a blank email, the backend must backfill the stored email before returning the caller/session identity.
- Existing non-empty stored emails must not be overwritten by the WorkOS authenticator.
- Auth structured output and `auth status` shape stay unchanged apart from any expected email now being present because of backend backfill.

## Unit Tests
- `TestWorkOSAuthenticator_BackfillsMissingEmailForExistingUser` — existing WorkOS-linked user with blank stored email gets email backfilled from JWT claims.
- `TestWorkOSAuthenticator_DoesNotOverwriteExistingEmail` — existing non-empty stored email is preserved even when JWT includes a different email.
- `TestWorkOSAuthenticator_NoEmailClaimLeavesExistingUserUnchanged` — no claim means no backfill call and current behavior stays intact.
- `TestAuthLoginSkipsDeviceFlowWhenStoredTokenValid` — success output still works for the already-authenticated path.
- `TestAuthLoginSkipsDeviceFlowWhenEnvTokenValid` — env-token path still works.
- `TestAuthLoginFallsBackToEmailWhenDisplayNameMissing` — fresh login prints email when display name is blank.
- `TestAuthLoginFallsBackToUserIDWhenIdentityMissing` — fresh login prints user ID when display name and email are blank.
- `TestAuthLoginAlreadyAuthenticatedFallsBackToUserIDWhenIdentityMissing` — cached-token path prints user ID when display name and email are blank.

## Integration / Functional Tests
- `go test ./backend/internal/api -run 'TestWorkOSAuthenticator_(BackfillsMissingEmailForExistingUser|DoesNotOverwriteExistingEmail|NoEmailClaimLeavesExistingUserUnchanged)'`
- `go test ./cli/cmd -run 'TestAuthLogin(SkipsDeviceFlowWhenStoredTokenValid|SkipsDeviceFlowWhenEnvTokenValid|FallsBackToEmailWhenDisplayNameMissing|FallsBackToUserIDWhenIdentityMissing|AlreadyAuthenticatedFallsBackToUserIDWhenIdentityMissing)'`

## Smoke Tests
- `go test ./backend/internal/api ./cli/cmd`
- `go test ./backend/internal/repository ./cli/internal/auth`

## E2E Tests
- N/A — not applicable for this fix; coverage is backend auth plus CLI command tests.

## Manual / cURL Tests
- Manual:
  - Run `go run . auth login --device` from `cli/` against staging or local services and confirm the success line shows a name, email, or user ID instead of `Logged in as`.
  - Run `go run . auth status` afterward and confirm the command still prints the same fields/format as before.
- cURL:
  - N/A — this fix is best verified through existing Go auth tests and the CLI login flow rather than standalone curl commands.
