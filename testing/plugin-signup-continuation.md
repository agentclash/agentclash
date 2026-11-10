# Plugin signup continuation — Test Contract

## Functional Behavior
- `/tryouts?tryout=<UUID>` survives login, WorkOS signed cancellation retry, dashboard and workspace onboarding with exactly the selected ID. Bare `/tryouts` remains valid.
- `/try/<bundled catalog slug>` survives the same boundaries, with no query fields. Unknown demos and unsafe/external paths fall back to dashboard.
- Invalid tryout IDs are discarded; unknown query fields and fragments are stripped. Navigation carries no claim authorization and never invents saved results.
- Device, invite, GitHub, dashboard plan and Vibe return behavior remains intact.
- New users complete workspace setup before continuing; existing users retain continuation. Unavailable session retries retain intent.

## Unit Tests
- `sanitizeReturnTo`: exact builders, invalid IDs, unknown demos, unsafe paths, unknown arguments, fragments and existing routes.
- Bundled demo slug allowlist matches catalog.
- Signed callback intents preserve tryout intent and expire safely.

## Integration / Functional Tests
- Mocked dashboard and onboard server pages verify login, new-account setup, existing-account redirect, retry and wizard propagation with tryout intent.
- Existing auth focused tests remain passing.

## Smoke Tests
- Focused ESLint and TypeScript check; disclose unrelated baseline failures.

## E2E Tests
- Credentialed browser acceptance required: new account/workspace creation; existing account; cancelled login; expired result; wrong workspace attempt. Do not mark these complete from mocks.
- Explicit claim must authorize selected workspace and reject another user's result. No backend authorization changes are in scope; report missing claim support or acceptance inputs honestly.

## Manual / cURL Tests
- In configured WorkOS environment, open a real anonymous tryout result, sign up, create workspace, verify identical result ID. Repeat with existing account and cancelled login retry.
- Open expired result and verify honest unavailable state; attempt claim in unauthorized workspace and verify denial. Requires result fixtures, two accounts/workspaces, and claim endpoint support.
