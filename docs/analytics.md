# Analytics (PostHog)

AgentClash tracks product usage entirely through **PostHog**. There is no
custom analytics dashboard and no Postgres analytics table — events are emitted
from the backend, the CLI (via the backend), and the web app, and everything is
viewed in PostHog's native UI (Funnels, Trends, Paths, Retention, Lifecycle,
Person view, SQL/HogQL).

This doc is the source of truth for the event taxonomy and how to view each
metric. If you change an event name or property, update this file.

## Identity model — one distinct_id everywhere

Every event uses the **user's UUID** as the PostHog `distinct_id`. That's what
lets a funnel chain a web `$pageview` to a server-side `cli.command.invoked` to
a worker `run.completed` for the same person.

- **Web** calls `posthog.identify(user_id, { email, … })` after login
  (`web/src/components/posthog-identify.tsx`).
- **Backend** middleware sets `distinct_id = caller.UserID` on every request
  (`backend/internal/api/middleware.go`).
- **Worker** run-lifecycle events resolve the run's `created_by_user_id` and use
  it as `distinct_id` (`backend/internal/worker/posthog_recorder.go`).

Anonymous/unattributed events are sent with `$process_person_profile: false` so
they don't create junk person profiles (and don't inflate MAU billing).

## Event taxonomy

### Backend HTTP middleware (`trackUsage`) — one event per request
- `cli.command.invoked` — request from the CLI. Props: `command` (e.g.
  `run.create`), `cli_version`, `os`, `arch`, `route`, `method`, `status_code`,
  `duration_ms`, `workspace_id`, `org_id`, `$request_id`.
- `api.request` — non-CLI authenticated request (no browser origin).
- `web.api.request` — request carrying a browser Origin/Referer.

`/healthz` and `/v1/model-catalog` are skipped.

### Web (`web/src/lib/analytics/events.ts`)
- `$pageview` — auto-captured on every App Router navigation
  (`web/src/components/posthog-provider.tsx`).
- `web.auth.login.success` — once per fresh tab session after login.
- `web.workspace.created`, `web.provider_account.added`, `web.run.created`,
  and (defined, wire as needed) `web.org.created`, `web.pack.uploaded`,
  `web.regression.case_promoted`.

### Worker run lifecycle (`backend/internal/worker/posthog_recorder.go`)
- `run.started`, `run.completed`, `run.failed`. Props: `run_id`,
  `run_agent_id`, `status`, `model`, `provider`, `source`, `workspace_id`,
  `org_id`. `distinct_id` = the run creator when known.

## Configuration

### Backend (api-server **and** worker)
```bash
POSTHOG_API_KEY=phc_xxxxxxxx          # project API key; unset → noop (no events)
POSTHOG_ENDPOINT=https://us.i.posthog.com   # optional; EU: https://eu.i.posthog.com
```
Both binaries flush on shutdown (`defer client.Close()`), so events aren't lost
on SIGTERM.

### Web (`web/`)
```bash
NEXT_PUBLIC_POSTHOG_KEY=phc_xxxxxxxx
# Default "/ingest" (first-party reverse proxy — see below). Only override to
# bypass the proxy.
NEXT_PUBLIC_POSTHOG_HOST=/ingest
# Reverse-proxy upstreams (next.config.ts). EU: https://eu.i.posthog.com /
# https://eu-assets.i.posthog.com
POSTHOG_CLOUD_HOST=https://us.i.posthog.com
POSTHOG_ASSETS_HOST=https://us-assets.i.posthog.com
```

**Reverse proxy:** `web/next.config.ts` rewrites `/ingest/*` to PostHog so
posthog-js loads and sends from a first-party path. Without it, ad-blockers and
browser tracking-protection silently drop a meaningful share of client events.

### Dashboard provisioning (one-off, local)
```bash
POSTHOG_PROJECT_ID=12345
POSTHOG_PERSONAL_API_KEY=phx_xxxxxxxx   # personal key, insight:write + dashboard:write
node scripts/posthog/provision-dashboard.mjs
```
Creates the "AgentClash — Usage" dashboard with the insights below. Idempotent
(skips insights that already exist by name). If you get a 404 on
`/api/projects/...`, re-run with `POSTHOG_API_SCOPE=environments`.

## Viewing each metric natively

| You want | PostHog feature |
| --- | --- |
| Top CLI commands | Trends on `cli.command.invoked`, breakdown by `command` (or the provisioned HogQL insight) |
| Top pages / routes | Trends on `$pageview` breakdown by `$pathname`; API routes via `api.request` breakdown by `route` |
| Onboarding drop-off | **Funnels** — the two provisioned funnels (`Onboarding funnel — web` / `— CLI`) |
| User journeys | **Paths** |
| DAU / WAU / MAU | Trends with "Active users" math at day/week/month interval |
| Signups over time | **Lifecycle** insight ("new" series) — no dedicated signup event needed |
| Most active workspace | Trends breakdown by `workspace_id` (we use event properties, not Group analytics) |
| One user's full activity | **Person** view → Activity tab |
| Run outcomes | Trends on `run.completed` / `run.failed`, breakdown by `model` |

## Excluding internal/team traffic

No `is_platform_admin` flag is needed. In PostHog: **Project settings → Product
analytics → internal & test users** — add a filter such as
`email` contains your team addresses (the team uses gmail, so list the specific
addresses rather than a domain). All insights honor the toggle. We send `email`
on `identify`, so this works without extra code.

## CLI telemetry privacy

The CLI only sends command-level telemetry (the `cmd=…` User-Agent segment)
when the resolved API base URL is `api.agentclash.dev`. Pointed anywhere else
(localhost, self-hosted), it sends a neutral `agentclash-cli/<version>`
User-Agent and the backend records nothing command-specific. See
`cli/internal/api/useragent.go`.
