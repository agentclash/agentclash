/** Offline ACQ-10 HogQL artifact generation. No credentials or network. */
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
export function validateCohortConfig(input) {
  const config = { ...input };
  for (const key of ['cutoff', 'asOf']) {
    const value = config[key];
    if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/.test(value) || !Number.isFinite(Date.parse(value)) || new Date(value).toISOString() !== value.replace('Z', '.000Z')) {
      throw new Error(`${key} must be an explicit valid UTC timestamp (YYYY-MM-DDTHH:mm:ssZ)`);
    }
  }
  if (config.asOf <= config.cutoff) throw new Error('asOf must follow cutoff');
  if (config.exclusionsReviewed !== true) throw new Error('Review sample/internal exclusions before generating report queries');
  if (!['synthetic', 'production'].includes(config.observationScope)) throw new Error('observationScope must be synthetic or production');
  for (const key of ['excludedUserIds', 'excludedWorkspaceIds', 'excludedRunIds']) {
    if (!Array.isArray(config[key]) || config[key].some(id => typeof id !== 'string' || !UUID.test(id))) throw new Error(`${key} must be an explicit UUID array`);
    config[key] = [...new Set(config[key])].sort();
  }
  return config;
}

function list(ids) { return `(${(ids.length ? ids : ['00000000-0000-0000-0000-000000000000']).map(id => `'${id}'`).join(', ')})`; }
function insight(name, description, query) {
  return { name, description, query: { kind: 'DataTableNode', source: { kind: 'HogQLQuery', query } } };
}

export function cohortInsights(input) {
  const c = validateCohortConfig(input);
  const cutoff = `toDateTime('${c.cutoff}', 'UTC')`;
  const asOf = `toDateTime('${c.asOf}', 'UTC')`;
  const host = `if(properties.observed_host IN ('claude', 'codex'), toString(properties.observed_host), 'unknown')`;
  const channel = `if(pdi.person.properties.acquisition_channel IN ('claude', 'codex'), toString(pdi.person.properties.acquisition_channel), 'unknown')`;
  const filters = `timestamp >= ${cutoff} AND timestamp < ${asOf}
      AND match(distinct_id, '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
      AND distinct_id NOT IN ${list(c.excludedUserIds)}
      AND coalesce(toString(properties.workspace_id), '') NOT IN ${list(c.excludedWorkspaceIds)}
      AND coalesce(toString(properties.run_id), '') NOT IN ${list(c.excludedRunIds)}
      AND coalesce(toString(pdi.person.properties.is_internal), 'false') != 'true'
      AND coalesce(toString(properties.is_internal), 'false') != 'true'
      AND coalesce(toString(properties.is_sample), 'false') != 'true'`;
  // Aggregate BEFORE joining: retries cannot multiply a result or denominator.
  // An internal UUID is the account key. pdi.person is used ONLY for current merged traits.
  // person.properties alone may be an ingestion-time snapshot preceding identify.
  const canonical = `canonical AS (
    SELECT event, distinct_id AS user_id, timestamp,
      toString(properties.user_id) AS signup_user_id,
      toString(properties.run_id) AS run_id,
      toString(properties.workspace_id) AS workspace_id,
      toString(properties.sdk_report_id) AS sdk_report_id,
      toString(properties.tryout_id) AS tryout_id,
      if(properties.signup_kind IN ('new_user', 'invited_activation'), toString(properties.signup_kind), 'unknown') AS signup_kind,
      ${channel} AS acquisition_channel, ${host} AS observed_host
    FROM events WHERE ${filters} AND toInt(properties.schema_version) = 1
  ), signups AS (
    SELECT user_id, min(timestamp) AS signup_at,
      argMin(signup_kind, timestamp) AS signup_kind,
      argMin(acquisition_channel, timestamp) AS acquisition_channel
    FROM canonical WHERE event = 'product.account.signup_completed' AND signup_user_id = user_id
    GROUP BY user_id
  ), created AS (
    SELECT user_id, workspace_id, run_id, min(timestamp) AS created_at,
      argMin(observed_host, timestamp) AS observed_host
    FROM canonical WHERE event = 'product.run.created' AND notEmpty(run_id) AND notEmpty(workspace_id)
    GROUP BY user_id, workspace_id, run_id
  ), completed AS (
    SELECT user_id, workspace_id, run_id, min(timestamp) AS completed_at
    FROM canonical WHERE event = 'product.run.completed' AND notEmpty(run_id) AND notEmpty(workspace_id)
    GROUP BY user_id, workspace_id, run_id
  ), hosted_results AS (
    SELECT completed.user_id AS user_id, completed.workspace_id AS workspace_id,
      completed.run_id AS entity_id, completed.completed_at AS result_at,
      created.observed_host AS observed_host, 'hosted_run' AS result_kind
    FROM completed INNER JOIN created ON completed.run_id = created.run_id
      AND completed.user_id = created.user_id AND completed.workspace_id = created.workspace_id
    WHERE completed.completed_at >= created.created_at
  ), imported_results AS (
    SELECT user_id, workspace_id, sdk_report_id AS entity_id, min(timestamp) AS result_at,
      argMin(observed_host, timestamp) AS observed_host, 'imported_local_evidence' AS result_kind
    FROM canonical WHERE event = 'product.sdk_report.imported' AND notEmpty(sdk_report_id) AND notEmpty(workspace_id)
    GROUP BY user_id, workspace_id, sdk_report_id
  ), results AS (
    SELECT * FROM hosted_results UNION ALL SELECT * FROM imported_results
  )`;
  const note = `UTC [${c.cutoff}, ${c.asOf}); ${c.observationScope} scope. Reviewed exclusion lists required; absent flags alone do not prove non-sample/non-internal. No install or connection identity.`;
  return [
    insight('Observed active users and workspaces by self-reported host — weekly', note + ' Successful authenticated CLI request diagnostics or durable CLI claim/import milestones; unknown includes missing/legacy markers. Counts across hosts are not additive.', `
    SELECT toStartOfWeek(timestamp, 1, 'UTC') AS week_start,
      ${host} AS observed_host,
      count(DISTINCT distinct_id) AS observed_active_users,
      count(DISTINCT nullIf(toString(properties.workspace_id), '')) AS observed_active_workspaces
    FROM events WHERE ${filters}
      AND ((event = 'cli.command.invoked' AND toInt(properties.status_code) >= 200 AND toInt(properties.status_code) < 300)
        OR (event IN ('product.tryout.claimed', 'product.sdk_report.imported') AND properties.surface = 'cli' AND toInt(properties.schema_version) = 1))
      AND coalesce(toString(properties.$process_person_profile), 'true') != 'false'
    GROUP BY week_start, observed_host ORDER BY week_start, observed_host`),
    insight('Mature new-account seven-day results by acquisition — weekly', note + ' Only signup_kind=new_user; seven full days observed; original acquisition is distinct from result host. Imported evidence is not hosted execution. Claim/save is not a completed result.', `
    WITH ${canonical}, mature AS (
      SELECT * FROM signups WHERE signup_kind = 'new_user' AND signup_at <= ${asOf} - INTERVAL 7 DAY
    ), per_user AS (
      SELECT mature.user_id AS user_id, mature.signup_at AS signup_at, mature.acquisition_channel AS acquisition_channel,
        max(if(results.result_kind = 'hosted_run' AND results.result_at >= mature.signup_at AND results.result_at < mature.signup_at + INTERVAL 7 DAY, 1, 0)) AS hosted_run,
        max(if(results.result_kind = 'imported_local_evidence' AND results.result_at >= mature.signup_at AND results.result_at < mature.signup_at + INTERVAL 7 DAY, 1, 0)) AS imported_local_evidence
      FROM mature LEFT JOIN results ON mature.user_id = results.user_id
      GROUP BY mature.user_id, mature.signup_at, mature.acquisition_channel
    )
    SELECT toStartOfWeek(signup_at, 1, 'UTC') AS signup_week, acquisition_channel,
      count() AS mature_new_accounts, sum(hosted_run) AS accounts_with_hosted_run,
      sum(imported_local_evidence) AS accounts_with_imported_evidence,
      sum(hosted_run) / count() AS hosted_run_rate,
      sum(imported_local_evidence) / count() AS imported_evidence_rate
    FROM per_user GROUP BY signup_week, acquisition_channel ORDER BY signup_week, acquisition_channel`),
    insight('Signup kind and maturity coverage — audit', note + ' Invitations and unknown kinds are visible but excluded from the primary new-account denominator.', `
    WITH ${canonical}
    SELECT signup_kind, acquisition_channel,
      if(signup_at <= ${asOf} - INTERVAL 7 DAY, 'mature', 'immature') AS maturity,
      count() AS accounts
    FROM signups GROUP BY signup_kind, acquisition_channel, maturity ORDER BY signup_kind, acquisition_channel, maturity`),
    insight('Observed result origins — exact run joins and imported receipts', note + ' Worker completion may lack host context; origin comes only from exact run.created identity. Completion without creation is shown as unknown in the coverage audit.', `
    WITH ${canonical}
    SELECT toStartOfWeek(result_at, 1, 'UTC') AS week_start, result_kind, observed_host,
      count(DISTINCT entity_id) AS results, count(DISTINCT user_id) AS users,
      count(DISTINCT workspace_id) AS workspaces
    FROM results GROUP BY week_start, result_kind, observed_host ORDER BY week_start, result_kind, observed_host`),
    insight('Completion and claim coverage — audit', note + ' Unjoined completions cannot enter signup-to-hosted-result numerator; claimed demos can include failed/cancelled sessions and are never called successful execution.', `
    WITH ${canonical}
    SELECT 'hosted_run_completion' AS evidence_kind,
      if(notEmpty(created.run_id) AND completed.completed_at >= created.created_at, created.observed_host, 'unknown') AS observed_host,
      if(notEmpty(created.run_id) AND completed.completed_at >= created.created_at, 'matched_creation', 'unmatched_or_invalid_creation') AS coverage,
      count(DISTINCT completed.run_id) AS entities
    FROM completed LEFT JOIN created ON completed.run_id = created.run_id
      AND completed.user_id = created.user_id AND completed.workspace_id = created.workspace_id
    GROUP BY observed_host, coverage
    UNION ALL
    SELECT 'platform_demo_claim', observed_host, 'saved_not_execution', count(DISTINCT tryout_id)
    FROM canonical WHERE event = 'product.tryout.claimed' AND notEmpty(tryout_id)
    GROUP BY observed_host`),
  ];
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const index = process.argv.indexOf('--config');
  if (index < 0 || !process.argv[index + 1]) throw new Error('Usage: node scripts/posthog/cohorts.mjs --config <reviewed-config.json>');
  const config = validateCohortConfig(JSON.parse(readFileSync(process.argv[index + 1], 'utf8')));
  console.log(JSON.stringify({ dry_run: true, cohort_config: config, insights: cohortInsights(config) }, null, 2));
}
