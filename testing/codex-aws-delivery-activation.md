# Protected delivery activation — test contract

## Functional behavior

- An explicitly dispatched verification job runs behind the same required environment review and main-only branch policy as delivery.
- An expiring private environment secret selects claim capture or bounded permission verification. No runtime credentials, real infrastructure identifiers, JWTs, claims, command output or reports are printed, summarized or uploaded as public artifacts.
- Claim capture observes the actual GitHub-signed job token context. It checks repository, main ref, workflow, commit, audience, issuer and environment, discards the JWT after use, and uploads only the selected claim evidence directly to a fixed, expiring private object destination. Decoding is not signature verification; STS verifies tokens during permission checks.
- Permission verification uses the existing protection snapshot, exact subject and scoped delivery role. Build can read/publish its allowed test artifact; production can read the immutable candidate and invoke only the pinned documents against an operator-verified closed host with a nonexistent sentinel release.
- Both roles must reject operator-approval writes, runtime-secret access using a deliberately nonexistent secret version, arbitrary SSM shell and unrelated resource operations. Wrong-audience and other-environment role exchange must be rejected. Expected authorization errors are distinct from connection, missing-resource or validation errors.
- The verifier never deploys an application candidate, promotes intake, changes databases or provisions infrastructure. There is no staging role/host creation in this change.

## Unit tests

- Reject missing activation approval, expired or overlong windows, wrong main/repository/workflow/environment/source, wildcard subjects and unsafe receipt URLs.
- Decode and validate only the expected token context; malformed tokens or changed audience/issuer/ref/environment fail.
- Upload receipts only to the configured private S3 object with conditional creation and encryption headers; redirects are refused.
- Authorization probe success or non-authorization failure cannot count as a passing denial. Secret probes use a nonexistent version and never persist returned secret data.
- Success and failure paths emit generic text only; private values do not enter stdout/stderr, job outputs or artifacts.

## Integration / functional tests

- Workflow syntax and existing delivery/platform tests remain passing.
- Live main jobs wait for required owner review. A non-main dispatch is denied by environment branch policy; an unapproved job does not receive environment secrets or start its steps. Administrator bypass is disabled and unavailable on the waiting job.
- Captured real subjects are reviewed privately before creating exact scoped AWS trust. Successful and negative live probes retain private receipts. The real release build uses the normal required checks and scanner path.
- Pinned SSM sentinel refusals leave the closed host and running platform containers unchanged; no production release approval is minted.

## Smoke tests

- Personal operator validates the private receipts and current host state. Delivery remains closed to app deployment while rehearsal and cutover are still unapproved.

## E2E tests

- Application/data/browser cutover is outside this change and remains a later gated step. Live OIDC and permission checks above are the end-to-end scope here.

## Manual checks

- Verify only the approved personal CLI identity is used for publication/configuration. Inspect exact diff, all unpublished history, public workflow/log paths and configured secret scan before commit or push.
- Preserve all existing worktree edits; operational configuration and evidence stay outside every checkout.
