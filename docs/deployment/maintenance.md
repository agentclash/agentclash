# API and worker maintenance

This is the reusable Step 6 runbook. Actual accounts, service identifiers, roles,
endpoints, credentials, workflow inventories and backup locations belong in the
private operator checkpoint. Completing this preparation does not authorize
production maintenance, AWS provisioning or source retirement.

## Shutdown behavior and budgets

| Setting | Default | Meaning |
| --- | --- | --- |
| `WORKER_STOP_TIMEOUT` | `30s` | SDK grace for each internal worker before activity cancellation. |
| `WORKER_SHUTDOWN_TIMEOUT` | `90s` | Application deadline for all queue workers, activity cleanup and reapers. Must exceed twice SDK grace. |
| `WORKER_CLEANUP_TIMEOUT` | `30s` | Additional deadline for idle warm-pool cleanup after worker shutdown. |
| `API_SHUTDOWN_TIMEOUT` | `30s` | Deadline for ordinary in-flight HTTP requests; remaining connections are force-closed. |
| `SSE_HEARTBEAT_INTERVAL` | `15s` | Flushed SSE comments while a run is quiet; must remain below the proxy idle timeout. |

Update any existing `WORKER_SHUTDOWN_TIMEOUT=10s` override before deploying this
release; it is incompatible with the new default SDK grace and is rejected at
startup.

The execution, scoring and background queue workers receive Stop concurrently.
Within each queue, the SDK stops workflow and activity workers sequentially, so
allow two SDK grace periods plus cancellation cleanup. The activity interceptor
keeps application dependencies available until activity functions and their
defers return. It rejects late dispatch after the SDK workers have stopped.
Reapers stop on the process context. A shutdown deadline or sandbox cleanup
failure produces a failed exit; it is not evidence of a drained fleet.

Warm-pool shutdown cancels background fills, waits for late-created sessions to
be destroyed and destroys idle sessions. Cleanup uses a fresh bounded context.
A provider that ignores cancellation can outlive the deadline; reconcile its
sandbox and capacity lease before reopening. Never delete every sandbox in a
shared E2B account. Check only the privately inventoried resources for this app.

The deployment definitions in Step 8 must give the worker at least `150s` before
SIGKILL with these defaults (`90s + 30s` plus finalizer margin), and the API at
least `45s`. Match systemd and SSM command timeouts to the full deployment, not
just one container. These are required settings to implement and rehearse later;
Step 6 does not change the current Railway process-manager deadlines. A forced
kill remains a failure requiring inspection. Database/client finalizers are not
proof that arbitrary blocked code can always exit within the application budget.

## Drain before replacement

Shutdown grace is a last opportunity for short work to finish. It does not make
in-memory multi-turn or human-wait activities resumable. A stored human prompt,
conversation row or exported Temporal history is not a runnable checkpoint of
the activity's sandbox and process memory. The existing activity retry policies
remain unchanged; a forced interruption can fail work or repeat provider calls.
Reconciled database spend and webhook idempotency do not guarantee exactly-once
external charges.

For normal deployments, close admission, drain existing work, stop the old worker
and terminal instance, then start replacements. For migration between Temporal
clusters, there must be no unresolved open execution to silently abandon. An
owner must choose to finish it, explicitly cancel and reconcile it, or approve a
separate migration design. Do not copy old workflow IDs into a new namespace and
claim those executions can resume.

## Cutover procedure

Execute these stages only during the later approved rehearsal/cutover. Record
each gate, timestamp and exact tested platform command privately. This document
does not contain production credentials or service IDs.

### 1. Establish ownership and stop automatic changes

Record the source release, destination account guard, immutable candidate image
digests, rollback release, operator and database/Temporal schema versions.
Preserve all existing application encryption/signing secrets in the destination
secret manager and verify a private decryption canary. Keep separate owners and
credentials for application and Temporal databases, including visibility.

Pause Railway auto-deploys and any scheduled or GitHub-triggered deployment that
could restart source services. Pause external job producers, Temporal schedules,
cron jobs and automated CLI runs. Inventory the namespace's actual schedules and
all open workflow types; searching this repository cannot prove an external
schedule does not exist. Keep this freeze through cutover and rollback decisions.
Retain the source account/services/data until retirement is separately approved.

### 2. Close admission and finish existing work

Use a tested ingress rule that denies application traffic by default and allows
only the exact paths needed to drain existing work. Do not merely disable the
frontend's Run button. Verify both custom domains and Railway-generated URLs.

| Surface | Drain policy |
| --- | --- |
| Runs, eval sessions/sets, dataset generation/evals/gates, harness execution and agent tryout/rerun/compare/promote | Block new starts, expansions and work-producing actions. Route families are in `backend/internal/api/routes.go`, `eval_sets.go` and `agent_tryouts.go`. |
| Existing hosted-run callbacks | Temporarily allow `POST /v1/integrations/hosted-runs/{runID}/events` for inventoried draining runs, with its existing signature checks. |
| Human turns, cancellation and operator status | Temporarily allow the exact existing run/agent IDs needed to settle work. Human submission uses `POST /v1/workspaces/{workspaceID}/runs/{runID}/run-agents/{runAgentID}/turns`; anonymous tryouts have their own turn route. |
| Terminal/Try CLI and E2B sessions | Refuse new sessions and preserve only existing sessions during drain. Apply the Step 7 terminal contract before live migration. |
| Dodo/GitHub webhooks and other integrations | Record the outage boundary and retry/redelivery plan. GitHub webhooks may create work; do not blanket-allow all webhooks during drain. |
| Other application routes, including metadata changes and token issuance | Deny by default; an allowlist is safer to review than an incomplete list of start routes. |

Keep required API callbacks and all queue workers alive until approved work has
settled. Review PostgreSQL statuses alongside Temporal executions and pending
activities, human waits, API requests, terminal sessions, E2B sandboxes and Redis
leases. Reapers can still write during this stage. A zero queue backlog or zero
website users is insufficient. An unknown status or unaccounted execution blocks
the final freeze; stop for the owner's concrete decision.

### 3. Fence all source writes

After drain, deny **every application path before authentication**, including
GET, HEAD, OPTIONS, public routes, callbacks and webhooks. CLI authentication
updates `cli_tokens.last_used_at` even for reads. UI maintenance, DNS changes and
an HTTP-method filter do not enforce this fence.

Stop every source API, worker, reaper and terminal/Try CLI instance. Stop operator
scripts, backup jobs that mutate application data, delayed webhook processing,
scheduled consumers and any second region/replica. Prevent auto-restart and
auto-deploy. Restrict source DB/Redis access to the named backup/audit operator
using a rehearsed network or database-role control. Audit any pooled proxy or
shared role first; do not disable unrelated organization workloads. Where role
controls are used, account for existing sessions as well as future logins.

Test the public custom URL, generated URL, direct origin/port and internal routes.
They must reject traffic before application/auth handlers or be unreachable.
Verify no app DB sessions or transactions remain, including idle connections and
prepared transactions. Verify Redis has no application clients and no producer
can reconnect. Export the final Temporal history delta after executions settle.
Do not publish histories, logs or SQL output as GitHub artifacts.

`scripts/deployment/drain-audit.sql` is a read-only status/session count aid. With
a private PostgreSQL service file and a private output directory already set up:

```sh
umask 077
PGSERVICE=source-app-audit psql -X --set=ON_ERROR_STOP=1 \
  --file=scripts/deployment/drain-audit.sql > "$PRIVATE_EVIDENCE/source-drain.txt"
```

`source-app-audit` is a placeholder profile. The SQL requires the current app
schema and does not establish the fence itself. Classify every remaining DB
connection privately; background database engine processes are not app writers.
Record Redis client/key-family/TTL evidence privately without dumping tokens or
lease values to the terminal. Time-based TTL expiry is expected even with writers
stopped, so preserve absolute expirations in the Redis transfer.

### 4. Take the final copy and verify it

Only after the fence is proven, take the final encrypted PostgreSQL dump and the
approved Redis snapshot/expiry-aware transfer. Preserve encryption keys, the full
filename migration ledger, extensions, sequences, ownership mapping, relationships,
business counts and encrypted-field readability. Keep encrypted backup checksums
and restore evidence privately. A stale pre-freeze dump is not the final copy.

R2 stays in place; validate object references and keep lifecycle/deletion jobs
paused until both data copies agree. Preserve the agreed Redis state rather than
silently resetting rate limits, leases or sessions. A stale execution lease needs
reconciliation, not blind renewal. Follow the separately rehearsed backup/restore
commands from Steps 8 and 12; do not improvise destructive restore commands here.

### 5. Validate the destination while ingress stays closed

Apply the application schema with the one-off migrator from
[application-migrations.md](application-migrations.md). Use Temporal's separate
schema/admin tools for its databases. Verify namespace registration and pollers
for execution, scoring and background queues. Confirm real Postgres, Temporal
and configured Redis are ready, secret decryption works and R2 references resolve.

Record the first destination write. Workers and reapers can write as soon as they
start; canaries and authentication also count. Before any destination write, a
rollback may discard the candidate and reopen the unchanged fenced source.
**After any destination write, DNS reversal alone is not a rollback.** Fence both
sides and reconcile/copy the accepted destination writes or restore under a new
approved recovery plan before reopening one side. Never run both writer fleets.

### 6. Reopen and reconcile

The friend-managed Vercel environment/DNS handoff remains in Steps 13–14. Verify
browser API access, CORS, SSE reconnects and the Step 7 WebSocket path before
reopening. Keep old generated URLs fenced after DNS moves. Account for cached
clients and callbacks still targeting the source.

Review Dodo/GitHub delivery windows, explicitly redeliver missed events through
their supported mechanisms and verify webhook IDs are deduplicated. Reconcile
workflow/DB terminal states, spend, outstanding sandboxes and Redis leases.
Resume selected schedules and deployment automation only on the destination.
Monitor the approved validation window; source deletion and Temporal Cloud
retirement remain later owner-approved steps.

## SSE and readiness checks

SSE comments follow the
[HTML event-stream guidance](https://html.spec.whatwg.org/dev/server-sent-events.html#authoring-notes)
and never set an event ID. Persisted polling remains the catch-up source if Redis
is unavailable. Reconnect using the last real event's `Last-Event-ID`; the skipped
prefix stays excluded from subsequent polls. An unknown cursor replays the full
snapshot. Clients should still deduplicate persisted IDs across connections.

Each stream write has a 10-second deadline; this is not a server-wide write timeout
that would cap an otherwise healthy stream. Shutdown cancels stream subscriptions
and releases their capacity while allowing ordinary requests to finish. API
readiness becomes 503 on shutdown or a configured dependency failure. Redis omitted
from configuration is reported as disabled; production definitions must explicitly
configure the intended Redis service. `/healthz` remains cheap liveness and must
not substitute for `/healthz/ready`.

For a local/manual check, use a private curl config containing the authentication
header and local test URL. Keep tokens off command arguments and query strings:

```sh
curl --no-buffer --config "$PRIVATE_SSE_CURL_CONFIG"
curl --fail --silent --show-error http://127.0.0.1:8080/healthz/ready
```

Observe comments while quiet, save a real event ID, signal the local API, then
reconnect to its replacement with that ID in the private curl config. Test actual
Caddy buffering/idle limits and container signals again in Steps 7–8 and 12.

## Local verification

From the repository root:

```sh
python3 scripts/deployment/test-maintenance.py
(cd backend && go test -short -race ./internal/api ./internal/worker ./internal/engine ./internal/workflow)
(cd runtime && go test -short -race ./sandbox ./runner)
```

Use a clean environment without production `DATABASE_URL` or provider credentials
for ordinary Go tests. The Python harness constructs its own environment, starts
unique disposable loopback-only Temporal/PostgreSQL containers, applies app
migrations, runs SDK interruption/retry and persisted billing/idempotency tests,
validates the audit SQL and removes its containers. It does not use Temporal Cloud
or E2B. Unit tests additionally cover the quiet HTTP proxy, SSE cursor polling,
forced API close, cancellation cleanup and warm-pool deadline/failure paths.

These checks establish local preparation only. Live source fencing, final restore,
frontend cutover, real proxy deadlines and encrypted production canaries still need
the later guided steps. The local SDK test uses a synthetic retryable activity;
it does not prove resumability of a production human-turn activity.
