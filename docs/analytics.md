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
