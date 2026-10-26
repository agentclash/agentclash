# Vibe Evals on Ubuntu: local continuation

## Checkout and safety

The original `/home/tenxcoder/agentclash` checkout was dirty on `main`. It was left untouched. The linked Git worktree `/home/tenxcoder/agentclash-vibe-evals` continues `codex/vibe-evals` from `281f1f9c6d19668130ed6a617de5a2893d3bf2cb`; it is the same repository, not a replacement implementation.

This is a localhost-only development setup. It is not production readiness, real WorkOS sign-in, or a renewed free-provider trial. No provider inference was performed in this continuation. The former pilot database and its two unresolved attempts were not available here and were not modified, reconciled, or replayed.

## Tools and dependencies

Use Go 1.25.5, Node 22, Docker/Compose (supporting `!override`), PostgreSQL client tools, Python 3 and GCC. Read `CONTRIBUTING.md`, `scripts/dev/bootstrap.sh` and `scripts/dev/local-stack.sh` for the wider repository setup. This machine uses Docker Desktop's Linux context.

```bash
cd /home/tenxcoder/agentclash-vibe-evals
export PATH="$HOME/.nvm/versions/node/v22.14.0/bin:/usr/local/go/bin:$PATH"
export GOTOOLCHAIN=go1.25.5
go version
node --version
docker compose version
```

`npm ci` completed in `web`. `npx playwright install --with-deps chromium` did not complete its dependency-install step on this machine. `npx playwright install chromium` succeeded, and installed Chromium launched headlessly with the existing system libraries. Do not set a macOS `VIBE_TEST_CHROMIUM` path.

## Isolated services

The ignored file `testing/vibe-local/compose.override.yml` contains this override for the existing root Compose stack:

```yaml
services:
  postgres:
    container_name: vibe-evals-postgres
    environment:
      POSTGRES_DB: vibe_local
    ports: !override
      - "127.0.0.1:55439:5432"
  redis:
    container_name: vibe-evals-redis
    command: ["redis-server", "--appendonly", "yes"]
    ports: !override
      - "127.0.0.1:56379:6379"
  temporal:
    container_name: vibe-evals-temporal
    command: ["server", "start-dev", "--ip", "0.0.0.0", "--ui-ip", "0.0.0.0", "--db-filename", "/tmp/vibe-temporal/temporal.db", "--log-level", "warn"]
    ports: !override
      - "127.0.0.1:57233:7233"
      - "127.0.0.1:58233:8233"
    volumes:
      - vibe-temporal-data:/tmp/vibe-temporal
volumes:
  vibe-temporal-data:
```

Start/reuse this project only; do not reset volumes:

```bash
docker compose -p agentclash-vibe -f docker-compose.yml \
  -f testing/vibe-local/compose.override.yml up -d postgres redis temporal
```

Temporal initially failed because its UID 1000 process could not write the new root-owned named volume. The verified fix was to change ownership of **only this project's Temporal volume**, then check readiness:

```bash
docker run --rm --user root --entrypoint chown \
  -v agentclash-vibe_vibe-temporal-data:/tmp/vibe-temporal \
  temporalio/temporal:1.7.2 1000:1000 /tmp/vibe-temporal
docker compose -p agentclash-vibe -f docker-compose.yml \
  -f testing/vibe-local/compose.override.yml up -d --wait --wait-timeout 120 postgres redis temporal
docker exec vibe-evals-temporal temporal operator cluster health --address localhost:7233
```

The health check returned `SERVING`. PostgreSQL 17 and Redis 7 also report healthy. Only the stated localhost ports are published. Temporal UI is at <http://127.0.0.1:58233>.

## Databases

Both `vibe_local` and separate `vibe_test` exist. The existing migration script applied all 79 migration files, including `00074_vibe_saved_model_receipts.sql`. Rerunning it skipped all recorded versions; `schema_migrations` (not `goose_db_version`) contains 79 rows in each database. Historical saved-model receipts remain unknown; the migration does not backfill from editable canonical fields.

For a fresh local stack, create `vibe_test` once if absent, then run:

```bash
scripts/db/apply-goose-migrations.sh \
  'postgres://agentclash:agentclash@127.0.0.1:55439/vibe_local?sslmode=disable'
scripts/db/apply-goose-migrations.sh \
  'postgres://agentclash:agentclash@127.0.0.1:55439/vibe_test?sslmode=disable'
```

The credentials above are the root Compose file's local development credentials, not provider credentials. Never point failure-injection tests at a pilot/application database. Fixture attempt cleanup is restricted to the fixture's IDs in `vibe_test`; it is not a procedure for resetting live quotas.

## API, worker and frontend

`backend/.env` and `web/.env.local` are ignored, mode 0600, and already configured for these ports. **Use `NEXT_PUBLIC_API_URL=http://localhost:55440` when opening the frontend at `http://localhost:3000`.** The API still binds to `127.0.0.1:55440`. Mixing browser hostnames (`localhost` frontend with `127.0.0.1` API) made Chrome reject the Strict SameSite trial cookie: session creation returned 201, then message/reload returned 403. Align hosts; do not weaken cookie attributes to work around local configuration.

Run the following in **three separate terminals**, using the toolchain PATH above:

```bash
# Terminal 1
cd /home/tenxcoder/agentclash-vibe-evals/backend
set -a; source .env; set +a
go run ./cmd/api-server
```

```bash
# Terminal 2
cd /home/tenxcoder/agentclash-vibe-evals/backend
set -a; source .env; set +a
go run ./cmd/worker
```

```bash
# Terminal 3
cd /home/tenxcoder/agentclash-vibe-evals/web
npm run dev -- --hostname 127.0.0.1 --port 3000
```

Open <http://localhost:3000/vibe-evals>. API health is <http://127.0.0.1:55440/healthz>; it returned `{"ok":true,"service":"api-server"}`. The worker connected to Temporal. **Vibe inference is intentionally disabled**, so a Vibe execution poller/model catalog is not claimed operational. The frontend can render the anonymous preview; sending requires safe live setup below. WorkOS values are development placeholders, not verified credentials for actual sign-in or workspace saving.

## Live enablement: blocked, not bypassed

Current backend configuration retains `VIBE_FREE_ONLY=true` but uses `VIBE_ENABLED=false` and an empty `VIBE_MODELS_JSON`. The OpenRouter key is absent. Put a key only in the ignored mode-0600 backend env file; never paste it into logs, source, or chat. Keep inference disabled until:

1. The previous pilot accounting/session ledger is restored, or its remaining authorized allowance is established. A fresh DB does not reset the lifetime/day/account limit.
2. Current exact endpoint, zero pricing, role capabilities, assembled context, reported accounting, and conformance expiry have been checked with a deliberately bounded free-only plan.
3. An unexpired verified profile is supplied for each selected role. Do not mark a profile conformed from public catalog metadata alone.
4. API and worker receive the same config and are restarted only while idle.

Public metadata on September 9 still listed Dots at `atlas-cloud/fp8` with zero token pricing and `structured_outputs`, but no live accounting/quality claim follows from that response. There was **no paid fallback, quota reset, new identity to force a test, provider call, public deployment, or payment**.

## Validation commands

The authoritative CI selection is `.github/workflows/vibe-evals.yml`. Stop the foreground Next dev server before Playwright starts its own server; never compete over one `.next` directory.

```bash
cd /home/tenxcoder/agentclash-vibe-evals/backend
export VIBE_TEST_DATABASE_URL='postgres://agentclash:agentclash@127.0.0.1:55439/vibe_test?sslmode=disable'
go test -race ./internal/vibe ./internal/api \
  -run 'Test(Vibe|Integration|Structured|YAML|Fanout|Money|FullContext|Unknown|Coverage|Requirement|StateMachine|MissingPricing|TrialEconomics|PaidTemporal|InvalidJudge|CORSMiddleware|BillingManager.*Webhook|AgentTryoutPromot|Generate)' -count=1
go build ./...
go vet ./...
cd ../runtime
go test ./provider ./scoring -count=1
cd ../web
npx tsc --noEmit --incremental false
npx vitest run src/app/vibe-evals src/components/vibe
npx eslint src/app/vibe-evals src/components/vibe src/lib/vibe.ts
npx playwright test --config playwright.vibe.config.ts
```

Pre-change baseline passed: 125 CI-selected Go/database tests (including subtests) under race detection, backend build/vet, provider/scoring tests, TypeScript, two safe-Markdown tests, focused lint and five mocked Playwright journeys.

Integrated verification on September 9, 2026:

- **95 top-level Go/database tests, 255 including subtests**, passed under the exact CI race selection; zero failures and zero skips. Both package runs passed. Backend `go build ./...` and `go vet ./...` passed.
- Shared runtime provider and scoring suites passed.
- **42 Vibe component tests** passed across three files; TypeScript and focused ESLint passed.
- **10 mocked Playwright journeys** passed, including lost acknowledgement/SSE retry, dirty instructions, refreshed evidence, casual draft-null replies, and the accept/check/improve/retest journey.
- The final output-only recovery bug was reproduced in both component and browser tests before fixing it. Open rows now invalidate evidence on the persisted event cursor, including recovery after cancellation with unchanged operation state/verdict metadata. Identical snapshots do not refetch, closed rows do not fetch, and refresh never submits inference.
- The real localhost page returned HTTP 200 and fetched `/v1/vibe/config` from port 55440 with HTTP 200. After hydration it displayed the configured free default. This demonstrates rendering/config transport, not an available/conformed provider.
- An **unmocked Chromium transport probe** first reproduced the mixed-host 403 failure. After aligning hostnames, session creation returned 201, the HttpOnly/Strict cookie persisted, SSE and reload returned 200, and sending returned the expected disabled-inference 503 rather than an auth failure. There were zero browser page errors. The application database still contained **zero operations and zero provider attempts**; these offline transport sessions did not execute inference or consume an authoring submission.
- The final ten-test Playwright rerun used its own server while the local Next dev server was stopped; TypeScript passed again afterward. When stopping a background npm wrapper, verify its child `next-server` listener actually stopped.
- Real PostgreSQL fixture saving verified selected canonical model configuration, exact save receipt, repeat idempotency, explicit model-conflict errors, legacy behavior, and revoked-membership denial. Real WorkOS sign-in remains untested.

Broader checks are not claimed green: the first `make check` attempt incorrectly shared `DATABASE_URL` with `VIBE_TEST_DATABASE_URL`; generic repository fixtures truncate tables and interfered with Vibe fixtures. After unsetting generic `DATABASE_URL`/`TEST_DATABASE_URL`, backend build/vet/short-race passed (generic repository DB tests skipped; Vibe DB tests ran). The CLI target then stopped on a `modernc.org/libc@v1.66.3` download connection reset. Never point these destructive suites at `vibe_local`. `sqlc` 1.31.1 generation completed without generated-file changes. `npm ci` reported 49 existing dependency vulnerabilities, including one critical; this scoped change does not upgrade dependencies or claim remediation.

Local evidence logs are ignored under `testing/vibe-local/`. Do not interpret passing synthetic fixtures as a successful live cohort. Independent backend review approved the staged backend. The frontend review's remaining output-only evidence finding was fixed parent-side with observed RED→GREEN tests at the user's request, without another delegation cycle. Remote CI is reported separately from local execution.

See [the reliability test contract](../testing/codex-vibe-reliability.md). Synthetic onboarding replies and fixture scorecards are test data, not measured live model outcomes.

## Workspace readiness and safe operator handoff (26 October 2026)

The earlier continuation above is historical evidence. For a new headless Linux
setup, source/build/test processes run on **THINKBOOK**; open the forwarded browser
on **MACBOOK**. Keep API and frontend listeners on `127.0.0.1` and forward them
through SSH/VS Code. Container listeners inside a Kubernetes pod must be reachable
by the pod probe; the Helm template sets the API listener to the service port.
That does not authorize publishing a development port or deploying this change.

There are three independent signals:

- `GET /healthz` is cheap process liveness. It never calls a dependency or provider.
- `GET /healthz/ready` checks PostgreSQL connectivity and the required migrated
  Vibe/accounting columns, Temporal health, and usable Redis budget operations.
  Redis must allow `EVAL`, `INCR`, `EXPIRE`, `GET`, `DEL`, and worker heartbeat
  `SET` with expiry. A successful `PING` alone is insufficient. The response
  uses fixed dependency statuses, never connection errors or credential values.
  The Helm readiness probe uses this endpoint with a three-second timeout; its
  separate liveness probe stays on `/healthz`.
- `GET /v1/vibe/config` always describes inference separately. `enabled` remains
  the configured feature flag. Additive `availability.guest` and
  `availability.workspace` each contain `available` and an optional stable
  `reason_code`: `hosted_disabled`, `provider_unavailable`, `pricing_unavailable`,
  `dependencies_unavailable`, `worker_unavailable`, `outbox_unavailable`,
  `accounting_unavailable`, `guest_not_configured`, or `trial_capacity_reached`.
  Existing disabled global subsidy pools and exhausted paid pools report trial
  capacity unavailable; active zero-cost free routes retain their existing
  zero-cost admission semantics. No grants or accounts are created by discovery.
  These are server capabilities; they grant no workspace membership, balance,
  trial allowance, quota, or reserved capacity. Admission checks remain final.

Inference discovery requires approved unexpired defaults, workflow **and**
activity pollers for `vibe-evals`, and shared Redis heartbeat evidence from
successful outbox and accounting maintenance passes. Heartbeats expire after
two minutes and are keyed by the matching API/worker configuration (including
provider credential hash and stable funding identity), not process memory.
A newly started worker is unavailable until its real loops succeed; the outbox
normally ticks every two seconds and accounting maintenance every minute.
Stopping a worker can take up to two minutes to age out its last evidence.
No provider request or paid smoke probe is performed by readiness/config.
Discovery and dependency probes have a two-second context budget; configured
Redis clients honor that deadline. PDF extraction remains independently gated
by its existing real worker lease, blob store and configured material capability.

Public config also supplies `server_time` (UTC RFC3339), `material_build`, and the
same guest/workspace `max_message_bytes`/request/file/string limits enforced by
the API. Limits count UTF-8 **bytes**, not JavaScript characters. Successful build
and run quotes supply `server_time` alongside their expiry; their original
30-minute and 10-minute lifetimes and every existing spend ceiling are unchanged.
Model discovery excludes unsupported, unapproved or expired profiles without
performing a local verification network call. Contact email is advertised only
when the real recipient and sender are configured. Missing sender means contact
is unavailable; it is not a successful enquiry delivery claim.

An operator must supply or verify all of the following before enabling inference:

1. A persisted, random `VIBE_COOKIE_SECRET` of at least 32 bytes, stored only in the
   deployment secret manager or ignored restricted env file. Missing/short secrets
   disable guest creation; they must not stop signed-in history or reconciliation.
   Preserve the secret and previous accounting ledger across restarts; do not
   rotate an identity or change databases to refill the trial.
2. An authorized `VIBE_OPENROUTER_KEY`, exact `VIBE_DEFAULT_MODEL`, and
   `VIBE_MODELS_JSON` profiles with operator-approved paid prices (or the exact
   supported free routes), context/framing bounds, route flags and a verified
   conformance expiry. Public catalog listings alone do not establish approval.
   There are no shipped provider credentials, fresh conformance dates or guessed
   priced models in the example. Expired/missing models make inference unavailable
   while API history and worker maintenance still start.
3. The existing stable `VIBE_CAMPAIGN`, restored ledger and authorized
   `VIBE_ANON_DAILY_USD`/`VIBE_ANON_CAMPAIGN_USD` funding limits for guest execution.
   Missing funding disables the guest capability; the $1 lifetime trial and
   smaller exploration/build allocations remain unchanged. Signed-in work keeps
   the existing organization credit policy and permission checks.
4. Migrated PostgreSQL and matching API/worker `REDIS_URL`,
   `TEMPORAL_HOST_PORT`/`TEMPORAL_NAMESPACE`, model settings and execution-policy
   settings. Keep at least one Vibe worker process active; do not scale all
   Vibe workers to zero based solely on the canonical execution queue. The
   canonical Helm worker class scaler does not independently scale `vibe-evals`.
   Apply all additive migrations before starting the additive API/worker.
5. Production `APP_ENV`, WorkOS identity/auth configuration, CORS/frontend origins,
   TLS and existing artifact/storage/signing and workspace secret encryption
   settings. WorkOS development placeholders do not verify sign-in or saving.
   For contact, supply a real `VIBE_ENQUIRY_EMAIL`, `RESEND_API_KEY`,
   `RESEND_FROM_EMAIL`, and `VIBE_ENQUIRIES_ENABLED=true`. Enable PDF only with the
   isolated worker runtime prerequisites already documented above.

`VIBE_ENABLED=false` stops new model execution. Keep the worker and provider
receipt credential running for expiry, settled-cost reconciliation, input/project
cleanup and already retained evidence. Do not globally gate history, saves,
exports, deletions or reconciliation on inference availability. Disabling
`VIBE_MATERIAL_BUILD` blocks new material builds; keep compatible workers running
for already admitted versioned work. Legacy `VIBE_TWO_DOOR` and
`VIBE_INTERPRETED_AUTHORING` environment switches no longer control the current
contract and have been removed from the sample. Invalid boolean syntax and
unstable campaign identifiers are explicit configuration errors; missing runtime
inference prerequisites remain truthful unavailable states.

Roll out additive backend first, then a private preview, the existing server
workspace flag and finally public UI. Rollback must preserve journals, quotes,
accounting tables and compatible reconciliation workers; never reset balances or
replay an unknown provider attempt. Production values, funded provider smoke,
real identity/save verification and deployment require an authorized operator.
None is implied by fixture tests or this runbook.
