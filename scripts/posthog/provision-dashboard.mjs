#!/usr/bin/env node
/**
 * Provision the AgentClash "Usage" dashboard in PostHog as code.
 *
 * Creates a dashboard plus a set of insights (top CLI commands, top API
 * routes, DAU, onboarding funnels, run outcomes, web lifecycle) via the
 * PostHog API so the internal team doesn't have to click them together by
 * hand. Re-running is safe: insights are matched by name and skipped if they
 * already exist on the dashboard.
 *
 * This script cannot be exercised from CI (it needs a live PostHog project +
 * personal API key), so treat the query payloads as a starting point and
 * adjust in the PostHog UI if your instance's query schema differs. The
 * equivalent manual setup is documented in docs/analytics.md.
 *
 * Usage:
 *   POSTHOG_PROJECT_ID=12345 \
 *   POSTHOG_PERSONAL_API_KEY=phx_xxx \
 *   [POSTHOG_HOST=https://us.posthog.com] \
 *   [POSTHOG_API_SCOPE=projects|environments] \
 *   node scripts/posthog/provision-dashboard.mjs
 *
 * Requires Node 18+ (global fetch).
 */

const PROJECT_ID = process.env.POSTHOG_PROJECT_ID;
const API_KEY = process.env.POSTHOG_PERSONAL_API_KEY;
const HOST = (process.env.POSTHOG_HOST ?? "https://us.posthog.com").replace(/\/$/, "");
// Older PostHog projects expose `/api/projects/:id`; newer ones use
// `/api/environments/:id`. Default to projects; override if your instance 404s.
const SCOPE = process.env.POSTHOG_API_SCOPE ?? "projects";

const DASHBOARD_NAME = "AgentClash — Usage";

if (!PROJECT_ID || !API_KEY) {
  console.error(
    "Missing env. Required: POSTHOG_PROJECT_ID, POSTHOG_PERSONAL_API_KEY.\n" +
      "Optional: POSTHOG_HOST (default https://us.posthog.com), POSTHOG_API_SCOPE (projects|environments).",
  );
  process.exit(1);
}

const base = `${HOST}/api/${SCOPE}/${PROJECT_ID}`;

async function api(path, { method = "GET", body } = {}) {
  const res = await fetch(`${base}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${API_KEY}`,
      "Content-Type": "application/json",
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let json;
  try {
    json = text ? JSON.parse(text) : {};
  } catch {
    json = { raw: text };
  }
  if (!res.ok) {
    throw new Error(`${method} ${path} -> ${res.status}: ${JSON.stringify(json)}`);
  }
  return json;
}

// HogQL table insight: robust across PostHog versions (plain SQL over events).
function hogqlInsight(name, description, sql) {
  return {
    name,
    description,
    query: {
      kind: "DataTableNode",
      source: { kind: "HogQLQuery", query: sql.trim() },
    },
  };
}

// Native funnel insight. steps: [{ event, command? }] — command adds an
// event-property filter so multiple steps can share the same event name.
function funnelInsight(name, description, steps) {
  return {
    name,
    description,
    query: {
      kind: "FunnelsQuery",
      dateRange: { date_from: "-30d" },
      series: steps.map((s) => ({
        kind: "EventsNode",
        event: s.event,
        name: s.label ?? s.event,
        ...(s.command
          ? {
              properties: [
                { key: "command", value: s.command, operator: "exact", type: "event" },
              ],
            }
          : {}),
      })),
    },
  };
}

function lifecycleInsight(name, description, event) {
  return {
    name,
    description,
    query: {
      kind: "InsightVizNode",
      source: {
        kind: "LifecycleQuery",
        dateRange: { date_from: "-30d" },
        interval: "week",
        series: [{ kind: "EventsNode", event, math: "total" }],
      },
    },
  };
}

const INSIGHTS = [
  hogqlInsight(
    "Top CLI commands (7d)",
    "Most-run CLI commands, with error counts.",
    `
    SELECT properties.command AS command,
           count() AS calls,
           countIf(toInt(properties.status_code) >= 400) AS errors
    FROM events
    WHERE event = 'cli.command.invoked'
      AND timestamp > now() - INTERVAL 7 day
      AND properties.command IS NOT NULL
    GROUP BY command
    ORDER BY calls DESC
    LIMIT 25
  `,
  ),
  hogqlInsight(
    "Top API routes (7d)",
    "Most-hit API routes by request count, with avg duration and error rate.",
    `
    SELECT properties.route AS route,
           properties.method AS method,
           count() AS calls,
           round(avg(toFloat(properties.duration_ms)), 1) AS avg_ms,
           countIf(toInt(properties.status_code) >= 400) AS errors
    FROM events
    WHERE event IN ('api.request', 'web.api.request', 'cli.command.invoked')
      AND timestamp > now() - INTERVAL 7 day
    GROUP BY route, method
    ORDER BY calls DESC
    LIMIT 25
  `,
  ),
  hogqlInsight(
    "Daily active users (30d)",
    "Distinct authenticated users per day across CLI + web.",
    `
    SELECT toDate(timestamp) AS day, count(DISTINCT person_id) AS dau
    FROM events
    WHERE timestamp > now() - INTERVAL 30 day
    GROUP BY day
    ORDER BY day
  `,
  ),
  hogqlInsight(
    "Most active workspaces (7d)",
    "Workspaces ranked by event volume.",
    `
    SELECT properties.workspace_id AS workspace_id, count() AS events
    FROM events
    WHERE timestamp > now() - INTERVAL 7 day
      AND properties.workspace_id IS NOT NULL
    GROUP BY workspace_id
    ORDER BY events DESC
    LIMIT 20
  `,
  ),
  hogqlInsight(
    "Run outcomes (30d)",
    "Run completions vs failures, broken down by model.",
    `
    SELECT event AS outcome,
           properties.model AS model,
           count() AS runs
    FROM events
    WHERE event IN ('run.completed', 'run.failed')
      AND timestamp > now() - INTERVAL 30 day
    GROUP BY outcome, model
    ORDER BY runs DESC
    LIMIT 50
  `,
  ),
  funnelInsight(
    "Onboarding funnel — web",
    "Landing → login → workspace → provider key → first run.",
    [
      { event: "$pageview", label: "Visited site" },
      { event: "web.auth.login.success", label: "Logged in" },
      { event: "web.workspace.created", label: "Created workspace" },
      { event: "web.provider_account.added", label: "Added provider key" },
      { event: "web.run.created", label: "Created run" },
    ],
  ),
  funnelInsight(
    "Onboarding funnel — CLI",
    "login → workspace → pack publish → deploy → first run (server-side CLI events).",
    [
      { event: "cli.command.invoked", command: "auth.login", label: "auth login" },
      { event: "cli.command.invoked", command: "workspace.use", label: "workspace use" },
      { event: "cli.command.invoked", command: "challenge-pack.publish", label: "pack publish" },
      { event: "cli.command.invoked", command: "deployment.create", label: "deployment create" },
      { event: "cli.command.invoked", command: "run.create", label: "run create" },
    ],
  ),
  lifecycleInsight(
    "Web lifecycle — new vs returning",
    "Proxy for signups: 'new' users are first-time visitors each week.",
    "$pageview",
  ),
];

async function findDashboardByName(name) {
  const res = await api(`/dashboards/?search=${encodeURIComponent(name)}`);
  const results = res.results ?? [];
  return results.find((d) => d.name === name) ?? null;
}

async function findInsightByName(name) {
  const res = await api(`/insights/?search=${encodeURIComponent(name)}`);
  const results = res.results ?? [];
  return results.find((i) => i.name === name) ?? null;
}

async function main() {
  console.log(`PostHog provisioning against ${base}`);

  let dashboard = await findDashboardByName(DASHBOARD_NAME);
  if (dashboard) {
    console.log(`✓ dashboard exists: "${DASHBOARD_NAME}" (id ${dashboard.id})`);
  } else {
    dashboard = await api(`/dashboards/`, {
      method: "POST",
      body: {
        name: DASHBOARD_NAME,
        description:
          "Internal usage analytics — CLI commands, page hits, onboarding funnels, run outcomes. Provisioned by scripts/posthog/provision-dashboard.mjs.",
      },
    });
    console.log(`+ created dashboard "${DASHBOARD_NAME}" (id ${dashboard.id})`);
  }

  for (const spec of INSIGHTS) {
    const existing = await findInsightByName(spec.name);
    if (existing) {
      console.log(`✓ insight exists: "${spec.name}"`);
      continue;
    }
    await api(`/insights/`, {
      method: "POST",
      body: { ...spec, dashboards: [dashboard.id] },
    });
    console.log(`+ created insight "${spec.name}"`);
  }

  console.log(
    `\nDone. Open: ${HOST}/project/${PROJECT_ID}/dashboard/${dashboard.id}`,
  );
}

main().catch((err) => {
  console.error("\nProvisioning failed:", err.message);
  console.error(
    "If this is a 404 on /api/projects/..., retry with POSTHOG_API_SCOPE=environments.",
  );
  process.exit(1);
});
