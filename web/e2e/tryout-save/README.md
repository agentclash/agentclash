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

Rolling deployment uses fresh `GET /v1/agent-tryout-templates` discovery with
`cache: no-store`, credentials omitted, and a 10-second discovery timeout before
every anonymous create/read/events/turns or claim operation. There is deliberately
no cached or global capability mode. Only `creator_access_protocol: "cookie-v1"`
selects credentialed transport. A valid catalog lacking the field selects legacy
transport, suppresses Save eligibility, and refuses claim before sending it.
Unknown/malformed discovery fails closed; failed requests never retry with weaker
credentials. The shipped frontend does not retry anonymous creation without judge
selection either.

New backend anonymous creation requires `cookie-v1` before invoking the service,
quota or proof generation. A stale legacy client meeting an updated backend gets
409 `creator_access_protocol_required` and must reload/update; there is no legacy
creation bypass. Workspace creates and authenticated saved/workspace reads retain
their token contracts. The UI also requires `save_eligible === true`, terminal
status and an explicitly selected authorized workspace.

`legacy-transport.spec.ts` serves the actual TypeScript transport source and a
separate loopback old-CORS API without Access-Control-Allow-Credentials. Chromium
first proves unconditional include fails preflight without a create, then executes
legacy create/read/events/turns through the production boundary, with Save false
and no claim request. The seven original journeys use the new backend catalog.

Backend deployment/promotion is separate from source merge. Release narrow CORS
preparation, migration and proof-aware backend through their actual staged release
process. No production deployment is asserted by this suite. Web/API must be
same-site for creator cookies. Legacy rows retain redacted public reads but cannot
be claimed or mutated on the new backend; earlier public data is not retroactively
private. Missing/evicted creator cookies require a new tryout; there is no
UUID-based recovery or cross-client transfer.

Creator-proof migration is `00097_agent_tryout_creator_proof.sql`; upstream owns
00095 device-auth consumption and 00096 tryout turn leases. The renumbering retains
its SQL bytes and must be applied with all current migrations in an owned test DB.
