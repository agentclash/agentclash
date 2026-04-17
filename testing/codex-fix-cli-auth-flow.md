# codex/fix-cli-auth-flow - Test Contract

## Functional Behavior
- `agentclash auth login` validates an existing resolved token before starting a new browser/device login.
- A valid stored token exits successfully with an "Already logged in as ..." message and does not create another device code.
- A valid `AGENTCLASH_TOKEN` exits successfully, reports that authentication is coming from the environment, and does not write credentials.
- `agentclash auth login --force` always starts a new device-code login and overwrites stored credentials only after the issued token validates.
- The CLI always prints a browser URL and user code, tries to open the browser unless `--device` is used or the environment is headless, and supports a fallback URL from `verification_uri` plus `user_code`.
- The polling loop only continues for expected pending states, handles `slow_down`, and fails clearly for denied, expired, malformed, repeated network, and unexpected API errors.
- Backend CLI auth stays under `/v1/cli-auth/*`, returns absolute verification URLs, and adds `POST /v1/cli-auth/device/deny`.
- Browser device approval asks the user to confirm the terminal code, supports cancel/deny, and preserves sign-in return to `/auth/device?user_code=...`.
- Raw CLI tokens remain one-time terminal-only values: they are never placed in browser URLs, are consumed once by polling, and remain hash-only in `cli_tokens`.

## Unit Tests
- `cli/internal/auth`: login tests cover browser open, `--device`, URL fallback, missing URL/code, pending, slow_down, denied, expired, malformed token response, unexpected API errors, and repeated network failures.
- `cli/cmd`: command tests cover valid stored token no-op, valid `AGENTCLASH_TOKEN` no-op, invalid stored token continuing into login, and `--force`.
- `backend/internal/api`: route/manager tests cover absolute verification URLs and deny route behavior.
- `web/src/lib/auth`: return-to tests preserve normalized device codes and reject unsafe paths.
- Web component/action tests cover signed-out, signed-in, approved, denied/error states, plus approve/deny error mapping where practical.

## Integration / Functional Tests
- Backend repository integration tests cover approve -> first poll consumes raw token -> second poll cannot retrieve it, deny transitions, expired code rejection, and stale-code cleanup semantics. These may skip when `DATABASE_URL` is unset, matching existing repository tests.
- Backend route tests continue proving `/v1/auth/session` is not shadowed by CLI auth routes.

## Smoke Tests
- `cd cli && go test ./...`
- `cd backend && go test ./internal/api ./internal/repository`
- `cd web && npm ci && npm test && npm run build`

## E2E Tests
- N/A as an automated browser-to-terminal E2E in this change. Manual smoke covers the user journey until a dedicated E2E harness exists.

## Manual / cURL Tests
- Fresh login: run `agentclash auth login`, verify the link opens, approve in browser, and confirm CLI saves credentials.
- Already signed-in browser: run `agentclash auth login --force`, approve without another browser sign-in.
- Headless flow: run `agentclash auth login --device`, manually open the printed URL, approve, and confirm completion.
- Deny flow: run `agentclash auth login --force`, click cancel/deny in browser, and confirm CLI exits with authorization denied.
- Expiry flow: start login, let the code expire, and confirm CLI reports expiry.
- Session/token smoke: run `agentclash auth status`, `agentclash auth tokens list`, revoke the new token, and verify dashboard `/v1/auth/session` still works.
