# Analytics (PostHog)

AgentClash uses PostHog for one pseudonymous acquisition-to-first-run funnel.
This document and the typed/code-owned event names are the event contract.

## Canonical dataset and cutoff

Event contract version: **1**.

The canonical dataset starts at the production deployment timestamp for the
API and worker implementation in this change. Record that timestamp as
`POSTHOG_CANONICAL_CUTOFF` when reconciling the dashboard. Never splice legacy
browser success events or run-agent events into a `product.*` funnel before
that cutoff.

Legacy names such as `web.auth.login.success`, `web.workspace.created`,
`web.provider_account.added`, `web.pack.uploaded`, `web.run.created`, and
unscoped `run.completed` do not mean the same thing as the contract below.
They may remain in historical data, but they are not canonical milestones.

## Identity and attribution

- The internal user UUID is the identified PostHog `distinct_id` in the web,
  API, CLI, and worker.
- Browser identification sends only pseudonymous organization/workspace IDs,
  immutable acquisition properties, and optional `is_internal`.
- Raw email and display name are not sent to PostHog identification.
- Anonymous events use the PostHog device identity. Unattributed backend events
  set `$process_person_profile=false`.
- First-touch entry pathname, referrer hostname, allowlisted UTM values, and
  the latest auth/signup CTA persist for 30 days across the WorkOS redirect.
  They are applied with `$set_once` when the internal UUID is identified.
- Logout calls `posthog.reset(true)` and clears analytics attribution/session
  storage so a shared browser starts with a new device identity.

Set `NEXT_PUBLIC_ANALYTICS_INTERNAL_USER_IDS` to a comma-separated allowlist of
internal user UUIDs if the hosted project needs the pseudonymous
`is_internal` person property. Do not restore email-based filtering.

## Event contract

| Stage | Canonical events |
| --- | --- |
| Visit | sanitized `$pageview` |
| Try | `web.tryout.*` |
| CTA | `web.marketing.cta.clicked` |
| Authenticate | `web.auth.completed`; `product.account.signup_completed` only for a newly created internal user or first activation of an invited `pending:` user |
| Configure | `web.setup.step.viewed`, `web.setup.step.clicked`, `product.organization.created`, `product.workspace.created`, `product.provider_account.created`, `product.agent_deployment.created`, `product.challenge_pack.published` |
| Run | `product.run.created`, `product.run.started`, `product.run.completed`, `product.run.failed`, `product.run.cancelled` |
| Return | `web.app.session_started`, once per identified PostHog session |

Every `product.*` event includes:

- `schema_version=1`;
- the relevant entity, workspace, and organization UUIDs;
- `surface=web|cli|api` when the originating request is known;
- a deterministic PostHog event UUID for retry deduplication.

Product milestones are recorded only after the state change succeeds. Signup
does not fire for ordinary login, existing-user lookup, account relinking, or a
create race recovered as an existing account. Run completion is the top-level
run transition after scoring and scorecard construction, not a run-agent event.

### Browser events

The browser adapter in `web/src/lib/analytics/posthog-client.ts` queues
pageview, capture, identify, callback, and reset operations in FIFO order until
PostHog initializes. Missing configuration explicitly disables and clears the
queue. One provider is mounted in the root app providers.

`web.marketing.cta.clicked` properties are code-owned and low-cardinality:
`cta_id`, `intent`, `placement`, `source_path`, `destination_kind`, and an
optional safe `destination_path`. CTA IDs use
`page-or-template.placement.intent`. Specialized tryout and promo events remain
alongside the generic CTA event.

`web.auth.completed` is driven by a short-lived marker written only after a
successful WorkOS callback. It is not inferred from opening an authenticated
tab. A later tab/session emits `web.app.session_started`, not another auth or
signup event.

The existing anonymous tryout events remain:

- `web.tryout.session_started`
- `web.tryout.launch_failed`
- `web.tryout.message_sent`
- `web.tryout.session_ended`
- `web.tryout.signup_cta_clicked`
- `web.tryout.roi_cta_clicked`

Vibe Build uses low-detail browser milestones: `web.vibe.build.entry_selected`,
`message_admitted`, `clarification_viewed`, `demo_selected`, `result_viewed`,
`example_viewed`, `action_clicked`, `trial_reply_viewed`, `recovery_clicked`,
and `export_requested` (all with the `web.vibe.build.` prefix). They include
pseudonymous session/run/artifact IDs, an allowlisted action or outcome, counts,
case ordinal,
sample status, and viewport class. They never include instructions, question
text, input, output, email, title, or free-text errors. Viewed events are
deduplicated per browser analytics session and saved entity; durable operations
remain the source of truth for completion and spending.

Lead events may retain a derived `email_domain`, but never raw email or company
name.

### Request diagnostics

The authenticated HTTP middleware emits one request diagnostic:

- `cli.command.invoked` for the hosted CLI User-Agent;
- `web.api.request` for a browser Origin/Referer;
- `api.request` otherwise.

Properties include route, method, status, duration, surface, request UUID, and
unambiguous organization/workspace UUIDs. These events explain validation and
server friction; they are not success milestones.

### Run-agent diagnostics

Worker event-recorder diagnostics keep the legacy `run.started`,
`run.completed`, and `run.failed` names with `scope=run_agent`. They include a
`run_agent_id` and may include provider/model details. Only `product.run.*`
belongs in top-level run funnels and outcome reports.

## Privacy boundary

The browser `before_send` sanitizer:

- retains pathnames and only `utm_source`, `utm_medium`, `utm_campaign`,
  `utm_content`, and `utm_term` query parameters;
- replaces invite/share path tokens with `{token}`;
- drops hashes and all other query parameters;
- reduces referrers to hostnames;
- removes email, display name, names, credentials, secrets, passwords, tokens,
  and form contents.

Do not add account names, run names, submitted prompts, credentials, or form
values to analytics. Backend canonical recording also rejects common PII and
secret property keys.

## Configuration

Backend API and worker:

```bash
POSTHOG_API_KEY=phc_xxxxxxxx
POSTHOG_ENDPOINT=https://us.i.posthog.com # optional
ANALYTICS_REQUIRED=true                  # hosted fail-fast mode
```

Web:

```bash
NEXT_PUBLIC_POSTHOG_KEY=phc_xxxxxxxx
NEXT_PUBLIC_POSTHOG_HOST=/ingest
NEXT_PUBLIC_ANALYTICS_INTERNAL_USER_IDS=uuid-1,uuid-2 # optional
POSTHOG_CLOUD_HOST=https://us.i.posthog.com           # optional
POSTHOG_ASSETS_HOST=https://us-assets.i.posthog.com   # optional
ANALYTICS_REQUIRED=true                               # hosted build fail-fast
```

When `ANALYTICS_REQUIRED` is false/unset, missing keys select the no-op behavior
for local and self-hosted use. When true, the web build and both backend
processes fail before serving traffic if their key is missing.

## Dashboard reconciliation

```bash
POSTHOG_PROJECT_ID=12345 \
POSTHOG_PERSONAL_API_KEY=phx_xxxxxxxx \
POSTHOG_CANONICAL_CUTOFF=2026-08-20T12:34:56Z \
node scripts/posthog/provision-dashboard.mjs
```

The script creates or updates insights by exact name. It provisions canonical
acquisition and activation funnels, setup/error reporting, entry/CTA to first
completion, tryouts, run outcomes, and first-completion-to-return retention.
Funnels use unique users; HogQL reports count distinct people/runs rather than
raw event volume.

## Historical person-property cleanup

Removing historical `email` and `display_name` person properties is an
irreversible production privacy operation. It must be performed by an operator
with PostHog project access after:

1. exporting or recording the affected-person count;
2. confirming all internal filters use `is_internal` or user UUIDs;
3. receiving explicit operator approval for the production project;
4. deleting the two properties through PostHog person-property management;
5. sampling person timelines to confirm the fields are gone and are not being
   repopulated.

This repository change intentionally does not execute that external cleanup or
assume a production key was previously missing.

## Production acceptance path

Use a fresh browser after deploying API/worker first, then web, then dashboard:

`UTM landing → tracked CTA → new WorkOS account → web/CLI setup → completed run → later return`

Accept only when there is one merged person timeline, exactly one canonical
signup, one canonical completion per run, no signup on the returning login,
sanitized properties, and confirmed web/API/worker configuration.

Build material/contact events: `web.vibe.build.input_status` carries input ID,
kind and terminal status; `enquiry_opened` carries project/version IDs;
`enquiry_received` adds the durable enquiry ID; `email_draft_opened` records
opening a mail draft, never a send. These respect the existing consent gate.
No filenames, contact fields, document text, outputs or summaries are included.

## ACQ-10: observed host use and mature signup cohorts (source contract)

These additions are **unreleased source changes**. Production coverage, actual
cutoffs, first supporting CLI release, PostHog query compatibility and the first
observed baseline are **pending verification**. The historical canonical cutoff
above does not establish coverage of the new paths. Use an effective `cutoff`
no earlier than the verified API claim/import/host recording, web acquisition
recording and relevant worker deployments. Record the released CLI version with
host-marker support and its availability time separately; old/untagged clients
stay `unknown`. Do not substitute the dates in the synthetic example.

### Acquisition and observed use are different fields

| Field | Values and authority |
| --- | --- |
| `acquisition_channel` (person `$set_once`) | `claude`, `codex`, `unknown`; derived strictly from original `utm_source=claude|codex`, retained with the first touch across auth/onboarding and later visits within its 30-day lifetime. Referrers/crawlers do not establish host use. |
| `observed_host` (canonical events and request diagnostics) | `claude`, `codex`, `unknown`; explicit self-reported CLI User-Agent marker only. Missing/invalid/legacy markers are unknown, including worker events without request context. |
| `workflow_kind` (canonical events) | `hosted_run` for canonical run events; `platform_demo` for tryout claim; `imported_local_evidence` for SDK import; otherwise `unknown`. Assigned by event type, never arbitrary client strings. |

The browser validates persisted state as untrusted input: bounded tag strings,
allowlisted UTM keys, safe paths, referrer hostnames, finite 30-day expiry and CTA
shape. It does not let a later landing overwrite the existing first touch.
`$set_once` preserves already recorded person properties; do not backfill old
accounts' channels from a later login. Primary cohorts begin after verified new
recording coverage, so preexisting account backfills cannot become new signups.

An operator/agent can explicitly opt in for a single existing CLI invocation:

```bash
AGENTCLASH_ANALYTICS_HOST=claude agentclash run list
AGENTCLASH_ANALYTICS_HOST=codex agentclash workspace list
```

These are examples, not commands executed during validation. With source that
supports this contract, hosted requests use `host=claude|codex|unknown` in the
existing User-Agent. Non-hosted URLs retain the neutral version-only form. No
SDK beacon, extra API request, credential inspection, automatic host detection
or profile modification is added. The marker is a self-report; it proves neither
an installed plugin nor a verified host. No stable connection or installation
identifier exists, so the metric is **observed active users/workspaces by
self-reported host**, never connections or installs.

### Additional durable event dictionary

| Event | Successful boundary | Identity/dedup | Meaning |
| --- | --- | --- | --- |
| `product.tryout.claimed` | Claim repository returns a committed claim after authorization/proof validation | Internal caller UUID; `tryout_id`, `org_id`, `workspace_id`; deterministic event UUID from event + tryout UUID | Saved platform demo. A failed/cancelled demo may be saved; this is not successful execution. |
| `product.sdk_report.imported` | SDK report-v1 receipt transaction commits, including its existing project event | Internal authenticated actor UUID; internal `sdk_report_id`, `workspace_id`; deterministic event UUID | Imported supplied local evidence, not a newly executed hosted run. Zero-case/diagnostic receipts do not establish application execution. |

Ordinary claim/import retries do not emit another milestone. Concurrent first
claims retain the same deterministic UUID. Queries additionally deduplicate by
entity before joins. Rejected access, conflicts, rollback and failed commits do
not emit success. Optional/noop analytics leave persistence unchanged. Capture
is best effort, not an outbox: a process crash after commit but before enqueue can
lose a milestone; retries are not a backfill mechanism. Do not assume 100% event
coverage from a successful database transition.

`product.run.completed` retains its existing authoritative terminal transition.
Its host can be unknown because workers do not retain request context. Queries
recover the observed origin only by matching canonical `run.created` and
`run.completed` on **run UUID + creator internal UUID + workspace UUID**, with
completion no earlier than creation. Unmatched completions stay visible in an
audit and cannot enter the primary numerator. Generic hosted runs are not proof
of customer application execution. Prompt/model checks, supplied-report review,
and customer application execution are **unmeasured as distinct buckets** here;
no provider choice, compilation, tool call, import or demo claim fills those gaps.

### Exact cohort definitions

All queries use UTC, Monday weeks and the half-open observation range
`[cutoff, asOf)`. Both times are explicit and frozen in a reviewed JSON config.
A weekly row can be partial at either boundary; retain the actual bounds when
comparing periods. The query generator does not run PostHog queries.

- **Weekly use:** successful (HTTP 200–299) `cli.command.invoked` requests, or canonical claim/import milestones with
  `surface=cli`, with an internal UUID distinct ID. Vibe has its own authentication
  and does not emit the general CLI diagnostic, so its durable import counts as
  observed use only when the bounded request context says CLI. Count distinct internal users and nonempty workspace
  UUIDs per week and observed host, including a separate `unknown` row. Retries
  cannot increase these distinct counts. Users can appear under multiple hosts;
  rows are not additive. Failed requests, anonymous identities and unobserved
  local SDK/MCP activity never enter this denominator.
- **Primary new accounts:** one earliest canonical
  `product.account.signup_completed` per internal user UUID, with its matching
  `properties.user_id`, `signup_kind=new_user`, inside the effective range, and
  signup at least seven full days before `asOf`. `web.auth.completed` is not a
  signup. `invited_activation`, unknown/missing signup kinds and immature accounts
  are separately reported by the coverage audit, not silently mixed in.
- **Acquisition join:** account identity is the internal `distinct_id`, not an
  email, WorkOS ID, anonymous ID or shared workspace. PostHog's event-to-person
  mapping supplies immutable `pdi.person.properties.acquisition_channel` after
  identification/merging; absent or nonallowlisted traits become `unknown`.
  A missing merge stays unknown. Origin host is never substituted for referral.
- **Seven-day numerator:** distinct denominator users with a matched canonical
  hosted completion in `[signup_at, signup_at + 7 days)`. A separate numerator
  reports imported evidence receipts in that same window. Users with both appear
  in both separate columns; do not add those rates. Demo claims are audit-only.
  Imports are receipts, including diagnostics, not evidence of a new execution.
- **Exclusions:** filter explicit event/person `is_internal=true`, event
  `is_sample=true`, and reviewed UUID lists for internal/test users, demo/sample
  workspaces and runs. Existing source has no universal sample flag. Therefore
  absence of a flag does not establish eligibility: `exclusionsReviewed=true`
  requires an operator's review of lists and the actual coverage. This setting is
  an attestation, not automatic verification. The synthetic example attests only
  to its synthetic dataset. Keep unknown source visible; never infer installs.
- **Dedup:** canonical schema version 1 only; earliest signup per user, earliest
  creation/completion per run/user/workspace, earliest import per receipt. Joins
  follow dedup to avoid multiplicative rows. Deterministic UUIDs are an additional
  ingestion safeguard. No legacy browser/run-agent successes enter results.
- **No mature rows:** means insufficient eligible data, not a measured 0% rate.
  Actual zero conversion is reportable only with a verified positive mature
  denominator and complete coverage/exclusion review.

### Offline reproduction and pending baseline

```bash
node --test scripts/posthog/cohorts.test.mjs
node scripts/posthog/cohorts.mjs --config scripts/posthog/cohort-example.json > /tmp/acq10-cohort-queries.json
node scripts/posthog/provision-dashboard.mjs --dry-run --config scripts/posthog/cohort-example.json > /tmp/acq10-dashboard-dry-run.json
```

The example is explicitly synthetic. Dry-run does not load PostHog credentials
or call the network. The provisioner reuses its existing named-insight format;
synthetic configurations cannot provision live insights. Query tests execute the
generated SQL through a small in-memory SQLite adapter for JSON access and
ClickHouse functions. This validates relational join/window/dedup semantics on
synthetic fixtures, **not** target PostHog parsing or live data. No second
production analytics store is introduced.

Run local attribution/identity and state-boundary fixtures:

```bash
(cd web && pnpm exec vitest run src/lib/analytics/attribution.test.ts src/components/posthog-identity-bridge.test.ts)
# Only after creating and migrating an owned isolated PostgreSQL database:
(cd backend && VIBE_TEST_DATABASE_URL='postgresql://localhost:55449/vibe_test_acq10?sslmode=disable' go test -race ./internal/api -run 'TestTagged(Claim|SDKImport)Analytics' -v)
```

Tagged browser tests mock WorkOS/session responses and retain acquisition on the
resolved internal UUID. Backend tagged fixtures exercise real PostgreSQL claims
and imports with synthetic users and no provider execution; the terminal demo
row is seeded. The import fixture rejects a deferred commit and proves rollback
produces no event. These are controlled component/state-transition traces, not a
single live WorkOS/production onboarding journey. Re-run with a fresh test DB;
never point the test gate at an existing app database.

Copy `scripts/posthog/baseline-pending.json` for the first real observation. Leave
unknown fields pending until measured. Required closure evidence includes:
actual API/web/worker deployment identifiers and timestamps, first released CLI
support, effective cutoff, PostHog dataset access/query validation, reviewed
internal/sample exclusions, tagged production identity/event coverage, frozen
as-of, a positive mature denominator, unknown/unmatched coverage, and the first
baseline period's observed counts. No uplift, target conversion or baseline
value is implied by these source fixtures. ACQ-10 stays open until those gates
are met.

HogQL references checked for these artifacts: [SQL access](https://posthog.com/docs/sql),
[supported functions](https://posthog.com/docs/sql/clickhouse-functions) and
[aggregations including argMin](https://posthog.com/docs/sql/aggregations).
PostHog's [person-processing documentation](https://github.com/PostHog/posthog/blob/master/docs/published/handbook/engineering/person-processing.md)
distinguishes ingestion-time `person.properties` from current
`pdi.person.properties`. ACQ-10 queries use the latter for acquisition/internal
traits so a backend signup ingested before browser identification can still join
the subsequently merged person. The target deployment must validate this schema
path and query behavior before baseline observation.

`scripts/posthog/tagged-journey-fixtures.json` retains sanitized Claude/Codex
receipt identities captured from the isolated PostgreSQL acceptance run. The
browser identity tests replay those internal UUIDs through tagged landing,
onboarding/auth marker and `$set_once`, showing the exact identity-key join to
the recorded durable receipt. This is a cross-component fixture replay, not a
live browser-to-database transaction or production journey. IDs are synthetic;
no prompt, trace, email or credential is retained in the artifact.
