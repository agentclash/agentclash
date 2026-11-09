#!/usr/bin/env node
/**
 * Reconcile the canonical AgentClash funnel dashboard with event contract v1.
 * Existing insights are PATCHed by exact name so query fixes roll forward.
 *
 * Required:
 *   POSTHOG_PROJECT_ID
 *   POSTHOG_PERSONAL_API_KEY
 *   POSTHOG_CANONICAL_CUTOFF=2026-08-20T12:34:56Z
 * Optional:
 *   POSTHOG_HOST=https://us.posthog.com
 *   POSTHOG_API_SCOPE=projects|environments
 */

import { readFileSync } from "node:fs";
import { cohortInsights, validateCohortConfig } from "./cohorts.mjs";

const DRY_RUN = process.argv.includes("--dry-run");
const configIndex = process.argv.indexOf("--config");
const COHORT_CONFIG = configIndex >= 0
  ? validateCohortConfig(JSON.parse(readFileSync(process.argv[configIndex + 1], "utf8"))) : null;
if (DRY_RUN && !COHORT_CONFIG) throw new Error("--dry-run requires --config with explicit synthetic or reviewed boundaries");

const PROJECT_ID = DRY_RUN ? null : process.env.POSTHOG_PROJECT_ID;
const API_KEY = DRY_RUN ? null : process.env.POSTHOG_PERSONAL_API_KEY;
const HOST = ((DRY_RUN ? null : process.env.POSTHOG_HOST) ?? "https://us.posthog.com").replace(/\/$/, "");
const SCOPE = DRY_RUN ? "projects" : (process.env.POSTHOG_API_SCOPE ?? "projects");
const RAW_CUTOFF = COHORT_CONFIG?.cutoff ?? process.env.POSTHOG_CANONICAL_CUTOFF;
const EVENT_CONTRACT_VERSION = 1;
const DASHBOARD_NAME = "AgentClash — Acquisition to first run";

if (!DRY_RUN && (!PROJECT_ID || !API_KEY || !RAW_CUTOFF)) {
  console.error(
    "Missing env. Required: POSTHOG_PROJECT_ID, POSTHOG_PERSONAL_API_KEY, " +
      "POSTHOG_CANONICAL_CUTOFF (the API/worker milestone deployment timestamp).",
  );
  process.exit(1);
}

const parsedCutoff = new Date(RAW_CUTOFF);
if (Number.isNaN(parsedCutoff.getTime())) {
  console.error("POSTHOG_CANONICAL_CUTOFF must be an ISO-8601 timestamp.");
  process.exit(1);
}
const CUTOFF = parsedCutoff.toISOString();
const base = `${HOST}/api/${SCOPE}/${PROJECT_ID}`;

async function api(path, { method = "GET", body } = {}) {
  const response = await fetch(`${base}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${API_KEY}`,
      "Content-Type": "application/json",
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await response.text();
  let payload;
  try {
    payload = text ? JSON.parse(text) : {};
  } catch {
    payload = { raw: text };
  }
  if (!response.ok) {
    throw new Error(
      `${method} ${path} -> ${response.status}: ${JSON.stringify(payload)}`,
    );
  }
  return payload;
}

function hogqlInsight(name, description, sql) {
  return {
    name,
    description: `${description} Event contract v${EVENT_CONTRACT_VERSION}; data starts ${CUTOFF}.`,
    query: {
      kind: "DataTableNode",
      source: { kind: "HogQLQuery", query: sql.trim() },
    },
  };
}

function funnelInsight(name, description, steps) {
  return {
    name,
    description: `${description} Unique users, first matching milestone, contract v${EVENT_CONTRACT_VERSION}.`,
    query: {
      kind: "FunnelsQuery",
      dateRange: { date_from: CUTOFF },
      funnelOrderType: "ordered",
      funnelWindowInterval: 30,
      funnelWindowIntervalUnit: "day",
      series: steps.map((step) => ({
        kind: "EventsNode",
        event: step.event,
        name: step.label ?? step.event,
        math: "unique_users",
        ...(step.properties ? { properties: step.properties } : {}),
      })),
    },
  };
}

const cutoffSQL = `toDateTime('${CUTOFF}')`;

const INSIGHTS = [
  funnelInsight(
    "Acquisition funnel — canonical",
    "Entry page → conversion CTA → genuine signup → first completed run.",
    [
      { event: "$pageview", label: "Visited" },
      { event: "web.marketing.cta.clicked", label: "Clicked CTA" },
      { event: "product.account.signup_completed", label: "Signed up" },
      { event: "product.run.completed", label: "Completed first run" },
    ],
  ),
  funnelInsight(
    "Activation funnel — canonical",
    "Genuine signup through the authoritative setup and run milestones.",
    [
      { event: "product.account.signup_completed", label: "Signed up" },
      { event: "product.workspace.created", label: "Created workspace" },
      { event: "product.provider_account.created", label: "Connected provider" },
      { event: "product.agent_deployment.created", label: "Created deployment" },
      { event: "product.challenge_pack.published", label: "Published pack" },
      { event: "product.run.created", label: "Created run" },
      { event: "product.run.completed", label: "Completed run" },
    ],
  ),
  hogqlInsight(
    "Setup milestones and errors — canonical",
    "Setup views/clicks, authoritative successes, and failed API requests by surface.",
    `
    SELECT event,
           properties.surface AS surface,
           properties.step AS step,
           count() AS events,
           count(DISTINCT person_id) AS users
    FROM events
    WHERE timestamp >= ${cutoffSQL}
      AND (
        event IN ('web.setup.step.viewed', 'web.setup.step.clicked',
                  'product.organization.created', 'product.workspace.created',
                  'product.provider_account.created', 'product.agent_deployment.created',
                  'product.challenge_pack.published')
        OR (event IN ('web.api.request', 'cli.command.invoked', 'api.request')
            AND toInt(properties.status_code) >= 400)
      )
    GROUP BY event, surface, step
    ORDER BY users DESC, events DESC
    LIMIT 100
  `,
  ),
  hogqlInsight(
    "Entry page and CTA to first completed run",
    "Completed users grouped by immutable first-touch entry page and acquisition CTA.",
    `
    SELECT person.properties.acquisition_entry_path AS entry_path,
           person.properties.acquisition_cta_id AS cta_id,
           count(DISTINCT person_id) AS users_with_completed_run,
           min(timestamp) AS first_completion
    FROM events
    WHERE event = 'product.run.completed'
      AND timestamp >= ${cutoffSQL}
    GROUP BY entry_path, cta_id
    ORDER BY users_with_completed_run DESC
    LIMIT 100
  `,
  ),
  funnelInsight(
    "Tryouts funnel — canonical cutoff",
    "Public tryout visit → launch → message → signup CTA → genuine signup.",
    [
      {
        event: "$pageview",
        label: "Visited /tryouts",
        properties: [
          {
            key: "$current_url",
            value: "/tryouts",
            operator: "icontains",
            type: "event",
          },
        ],
      },
      { event: "web.tryout.session_started", label: "Started tryout" },
      { event: "web.tryout.message_sent", label: "Sent message" },
      { event: "web.tryout.signup_cta_clicked", label: "Clicked signup" },
      { event: "product.account.signup_completed", label: "Signed up" },
    ],
  ),
  hogqlInsight(
    "Canonical run outcomes",
    "One top-level outcome per run, excluding run-agent diagnostic events.",
    `
    SELECT event AS outcome,
           count(DISTINCT properties.run_id) AS runs,
           count(DISTINCT person_id) AS users
    FROM events
    WHERE event IN ('product.run.completed', 'product.run.failed', 'product.run.cancelled')
      AND timestamp >= ${cutoffSQL}
      AND toInt(properties.schema_version) = ${EVENT_CONTRACT_VERSION}
    GROUP BY outcome
    ORDER BY runs DESC
  `,
  ),
  hogqlInsight(
    "Retention — first completed run to app return",
    "Day offset from each user's first completed run to later identified app sessions.",
    `
    WITH first_runs AS (
      SELECT person_id, min(timestamp) AS first_completed_at
      FROM events
      WHERE event = 'product.run.completed'
        AND timestamp >= ${cutoffSQL}
      GROUP BY person_id
    )
    SELECT dateDiff('day', first_runs.first_completed_at, sessions.timestamp) AS day_offset,
           count(DISTINCT sessions.person_id) AS retained_users
    FROM events AS sessions
    INNER JOIN first_runs ON sessions.person_id = first_runs.person_id
    WHERE sessions.event = 'web.app.session_started'
      AND sessions.timestamp > first_runs.first_completed_at
    GROUP BY day_offset
    ORDER BY day_offset
  `,
  ),
  hogqlInsight(
    "Top API and CLI errors",
    "Validation and server friction by route, surface, and status.",
    `
    SELECT event,
           properties.route AS route,
           properties.surface AS surface,
           properties.status_code AS status_code,
           count() AS errors,
           count(DISTINCT person_id) AS affected_users
    FROM events
    WHERE event IN ('api.request', 'web.api.request', 'cli.command.invoked')
      AND timestamp >= ${cutoffSQL}
      AND toInt(properties.status_code) >= 400
    GROUP BY event, route, surface, status_code
    ORDER BY errors DESC
    LIMIT 100
  `,
  ),
];

async function findDashboardByName(name) {
  const response = await api(`/dashboards/?search=${encodeURIComponent(name)}`);
  return (response.results ?? []).find((dashboard) => dashboard.name === name) ?? null;
}

async function findInsightByName(name) {
  const response = await api(`/insights/?search=${encodeURIComponent(name)}`);
  return (response.results ?? []).find((insight) => insight.name === name) ?? null;
}

if (COHORT_CONFIG) INSIGHTS.push(...cohortInsights(COHORT_CONFIG));

async function main() {
  if (DRY_RUN) {
    console.log(JSON.stringify({ dry_run: true, cohort_config: COHORT_CONFIG, insights: INSIGHTS }, null, 2));
    return;
  }
  if (COHORT_CONFIG?.observationScope === "synthetic") throw new Error("Synthetic cohort configuration cannot provision production insights");
  console.log(`PostHog reconciliation against ${base}`);
  const dashboardDescription =
    `Canonical contract v${EVENT_CONTRACT_VERSION}. API/worker cutoff: ${CUTOFF}. ` +
    "Do not combine these product.* milestones with legacy browser or run-agent events.";

  let dashboard = await findDashboardByName(DASHBOARD_NAME);
  if (dashboard) {
    dashboard = await api(`/dashboards/${dashboard.id}/`, {
      method: "PATCH",
      body: { name: DASHBOARD_NAME, description: dashboardDescription },
    });
    console.log(`~ reconciled dashboard "${DASHBOARD_NAME}" (id ${dashboard.id})`);
  } else {
    dashboard = await api("/dashboards/", {
      method: "POST",
      body: { name: DASHBOARD_NAME, description: dashboardDescription },
    });
    console.log(`+ created dashboard "${DASHBOARD_NAME}" (id ${dashboard.id})`);
  }

  for (const spec of INSIGHTS) {
    const existing = await findInsightByName(spec.name);
    if (existing) {
      await api(`/insights/${existing.id}/`, {
        method: "PATCH",
        body: { ...spec, dashboards: [dashboard.id] },
      });
      console.log(`~ updated insight "${spec.name}"`);
    } else {
      await api("/insights/", {
        method: "POST",
        body: { ...spec, dashboards: [dashboard.id] },
      });
      console.log(`+ created insight "${spec.name}"`);
    }
  }

  console.log(`Done: ${HOST}/project/${PROJECT_ID}/dashboard/${dashboard.id}`);
}

main().catch((error) => {
  console.error("Provisioning failed:", error.message);
  console.error(
    "If /api/projects/... returns 404, retry with POSTHOG_API_SCOPE=environments.",
  );
  process.exit(1);
});
