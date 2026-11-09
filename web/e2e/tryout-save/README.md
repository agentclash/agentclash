# Tryout signup and explicit save browser regression

Run from `web/`:

```sh
pnpm exec playwright test --config playwright.tryout-save.config.ts
```

Install the repository's Playwright Chromium first, or set `VIBE_TEST_CHROMIUM`
to an existing executable. The suite uses dedicated loopback ports 53818 (web),
55741 (API), and 55742 (fake WorkOS); `TRYOUT_BROWSER_WEB_PORT`,
`TRYOUT_BROWSER_API_PORT`, and `TRYOUT_BROWSER_AUTH_PORT` override them. It refuses
to reuse a running server. The existing frontend launcher copies source/config to
an owned temporary directory, links dependencies, and copies no `.env` files.
The API launcher compiles a temporary test binary and unsets `DATABASE_URL`.

This runs actual Next/AuthKit login, callback, dashboard and onboarding pages,
and actual Go tryout handlers/managers/CORS. A deterministic in-memory repository
and local identity provider replace persistence and external authentication.
There is no Playwright request interception, real signup, Temporal execution,
provider request or paid inference. Fixture onboarding maintains controlled
membership state; it is not a test of the production onboarding database manager.

The matrix covers new account + workspace creation, existing account + chosen
workspace, cancelled login, expiry, legacy unclaimable results, tampered workspace,
UUID substitution and wrong creator cookie. Tests verify no automatic claim,
human workspace receipts with exact result/workspace identity metadata, authenticated
reload and idempotent retry. The active-session case covers queued/running save
rejection, creator reply and end, then explicit terminal save. Historical private
active rows remain readable without anonymous reply/end controls.
Cookie attributes are inspected only in memory. Traces, videos, HAR and storage
state exports are disabled. Artifacts contain synthetic IDs/screenshots only;
never attach raw request/response headers or cookie values.

Real SQL ownership/expiry/concurrency is separately covered by
`TestRepositoryTryoutCreatorClaimAtomicity` using the explicit
`AGENTCLASH_TRYOUT_TEST_DATABASE_URL` opt-in. It accepts only loopback databases
named `tryout_test_*`. Create and migrate an owned disposable PostgreSQL cluster
before setting that variable; never point it at an existing application database.
The ordinary backend test suite skips this test without that explicit variable.

Rollout: deploy backend credentialed-CORS preparation before the credentialed
frontend. Deploy migration and proof-aware backend before enabling the new save
UI. The UI requires `save_eligible === true`; missing/false capability disables
save. Web/API must be same-site. Legacy results retain redacted public reads but
cannot be claimed or mutated. Earlier public data is not retroactively private.
Stale clients must reload the updated app before starting a new tryout. Missing
or evicted creator cookies require a new tryout; no UUID-based
ownership recovery or cross-client transfer is provided.
