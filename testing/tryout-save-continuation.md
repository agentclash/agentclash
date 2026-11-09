# Tryout save continuation — Test Contract

## Functional Behavior

- Preserve the selected UUID through existing login/signup and workspace onboarding. Never claim from navigation, login, onboarding, or page load.
- New anonymous results use an independent random creator proof, stored only as a digest server-side and transported by a host-only, path-scoped HttpOnly API cookie. Production cookies are Secure. No proof in redirects, JSON, analytics, browser storage, or artifacts. Quota fingerprints retain their existing separate role.
- Require creator proof for new anonymous detail, events, turns and claim; enforce anonymous expiry. Deny missing/wrong proof and UUID substitution. Private workspace results remain inaccessible through anonymous endpoints. Explicit share tokens preserve existing authorization/redaction behavior.
- Legacy proofless anonymous results retain redacted public reads (subject to existing expiry); they cannot be claimed or changed. Previously public data is not retroactively private. Missing cookies and legacy results offer a new tryout, never ownership recovery from an ID.
- Claim requires authentication, authorized target workspace and independent creator proof. SQL checks ownership and expiry atomically. Competing targets cannot both claim. Identical successful retries preserve the original transition and receipt.
- After return, offer explicit workspace selection and save. Only a successful backend response displays saved result/workspace identity. Forbidden, expired, unavailable and cancellation states are recoverable. Saved reloads use authenticated authorization, never anonymous access to private data.
- Save is disabled when the server omits the new eligibility capability. Never send the new UI to an older ID-only claim handler.
- Deploy backend-only narrow credentialed-CORS preparation before credentialed frontend requests. Same-site web/API deployment is required; unsupported cross-site cookies fail closed. Preserve other endpoint CORS and existing auth paths.

## Unit Tests

- CORS exact tryout path boundary, production origin allowlist, credentialed preflight, forbidden origin, loopback-only dev fallback, unrelated endpoints unchanged.
- Cookie entropy/format/path/expiry/HttpOnly/Secure, proof hashing/persistence, no response proof leakage.
- Manager creator access, expiry, legacy read/write policy, private/share isolation, target authorization, claim identity/retry/error behavior.
- Frontend API credentials and eligibility gating; explicit save choices, success receipt, failure states; existing return-to/device/invite/Vibe/onboarding tests remain passing.

## Integration / Functional Tests

- Actual handlers/managers/CORS over HTTP and AuthKit callback against deterministic local fixtures.
- PostgreSQL migration/query round-trip; expired/missing/wrong proof; same-owner retry; simultaneous competing target claims. Must use an explicitly isolated test DB. Mock results are not evidence for SQL concurrency. If unavailable, retain this as a separate required CI gate.
- Regenerate sqlc normally; backend build/vet/focused race tests, frontend focused tests/lint/typecheck, OpenAPI validation.

## Smoke Tests

Local fixture readiness, real HTTP anonymous create/read and authenticated save. No real signup, provider/model execution, customer database or deployment. Deployed smoke verification belongs to rollout and is not claimed by local evidence.

## E2E Tests

Real browser + actual Next/AuthKit pages + local fake identity provider:
1. New account, workspace creation, exact result reopening, no automatic claim, explicit save receipt.
2. Existing account, explicit workspace selection, exact saved identity and reload.
3. Cancelled login preserves return intent and unsaved result; retry succeeds.
4. Expired result gives concrete recovery without saved state.
5. Wrong/tampered workspace rejects; UUID substitution/wrong creator cookie rejects; anonymous private access rejects.
6. Old server without eligibility does not enable save.

Browser traces/HAR are disabled because they contain raw cookie headers. Inspect cookies only in memory; screenshots and synthetic identity assertions may be retained.

## Manual / cURL Tests

Use only the isolated browser fixture: create two browser contexts, launch a synthetic result in each, substitute their UUIDs, and verify rejection. Login with fixture identities, select an authorized workspace, click Save and verify the exact receipt; repeat the same request and confirm unchanged claim identity/time. No production URLs or credentials are involved.
