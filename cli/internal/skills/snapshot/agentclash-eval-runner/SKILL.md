---
name: agentclash-eval-runner
description: Use when starting, following, inspecting, or reporting AgentClash eval runs with the CLI, especially eval start, run create, deployment selection, input set selection, suite-only scopes, repetitions, events, rankings, failures, and scorecards.
metadata:
  agentclash.role: running
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Eval Runner

## Purpose
Create an AgentClash eval run or eval session against a published challenge pack, follow it when useful, inspect evidence after it runs, and report stable commands a reviewer can repeat.

## Use When
- A user asks to run one or more agent deployments against a published challenge pack.
- A user wants to choose a challenge pack, version, input set, deployment, regression suite, or run scope from the CLI.
- A user wants live run events, rankings, failures, agents, or scorecards after a run starts.
- A CI or local workflow needs exact non-interactive commands.

## Do Not Use When
- The challenge pack is not authored or published yet; use the challenge-pack skills first.
- The user needs to create deployments or runtime resources; use `agentclash-agent-deployment-setup` or `agentclash-runtime-resources-setup`.
- The task is only to interpret an already generated scorecard in depth; use `agentclash-scorecard-reader`.
- The task is a release gate or CI manifest workflow; use `agentclash-ci-release-gate`.

## Inputs Needed
- Workspace ID or configured default workspace.
- Challenge pack selector: pack ID, slug, exact name, or challenge pack version ID.
- Challenge pack version selector: version ID or version number.
- Input set selector: input set ID, key, or exact name.
- Agent deployment selectors: deployment IDs or exact names.
- Scope: `full` or `suite_only`.
- Optional regression suite IDs/names or regression case IDs.
- Whether to stream events with `--follow`.
- Whether this is a repeated eval session with `--repetitions`.

## Environment
Use hosted production by default unless the user intentionally targets local or self-hosted infrastructure:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

Before creating a run, verify auth and workspace context:

```bash
agentclash auth status
agentclash workspace use <WORKSPACE_ID>
agentclash challenge-pack list --json
agentclash deployment list --json
```

Workspace resolution follows the CLI setup rules: `--workspace`, `AGENTCLASH_WORKSPACE`, saved config, or `.agentclash.yaml`. `eval start`, `run create`, `run list`, `run failures`, and `eval scorecard` require a workspace.

## Prefer `eval start` for Humans and Agents
`agentclash eval start` wraps `agentclash run create` but resolves selectors through workspace reads. Use it when names, slugs, input-set keys, or guided selection are useful.

```bash
agentclash eval start \
  --pack <PACK_ID_OR_SLUG_OR_EXACT_NAME> \
  --pack-version <VERSION_ID_OR_VERSION_NUMBER> \
  --input-set <INPUT_SET_ID_OR_KEY_OR_EXACT_NAME> \
  --deployment <DEPLOYMENT_ID_OR_EXACT_NAME> \
  --name "Smoke eval" \
  --follow
```

Exact `eval start` flags:

- `--pack`: challenge pack ID, slug, or exact name.
- `--pack-version`: challenge pack version ID or version number. Use `--pack` when selecting by version number; a version ID can identify the pack by itself.
- `--input-set`: challenge input set ID, key, or exact name.
- `--deployment`: deployment ID or exact name. Repeat this flag for multiple deployments.
- `--name`: optional run name.
- `--follow`: stream run events after creation.
- `--scope`: `full` or `suite_only`; default is `full`.
- `--suite`: regression suite ID or exact name. Repeatable.
- `--case`: regression case IDs. Repeatable.
- `--race-context`: enable live peer-standings injection during the run.
- `--race-context-cadence`: 0 for backend default, otherwise 1 through 10.
- `--repetitions`: repeat the eval 1 through 100 times; values 2 or greater use `/v1/eval-sessions`.

Selector behavior:

- Pack selectors match ID, slug, or exact name.
- Deployment selectors match ID or exact name.
- Suite selectors match ID or exact name, are filtered to the selected pack when possible, and must resolve to active suites.
- Input set selectors match ID, input key, or exact name.
- Selectors are exact or case-insensitive exact matches, not substring search.
- If no pack is specified and there is one pack, the CLI uses it; with multiple packs in non-interactive mode, pass `--pack` or `--pack-version`.
- If no version is specified, the CLI uses the highest `version_number` for the selected pack.
- If a version has no input sets, the CLI submits without `challenge_input_set_id`.
- If a version has one input set, the CLI uses it.
- If a version has multiple input sets in non-interactive mode, pass `--input-set`.
- If multiple deployments exist in non-interactive mode, pass at least one `--deployment`.

## Use `run create` for ID-First Automation
`agentclash run create` posts directly to `/v1/runs`. Use it when a script already has IDs.

```bash
agentclash run create \
  --challenge-pack-version <CHALLENGE_PACK_VERSION_ID> \
  --input-set <CHALLENGE_INPUT_SET_ID> \
  --deployments <AGENT_DEPLOYMENT_ID> \
  --name "Smoke eval" \
  --follow
```

Exact `run create` notes:

- The lower-level flag is plural `--deployments`; it expects deployment IDs.
- `--challenge-pack-version` expects a challenge pack version ID.
- `--input-set` expects a challenge input set ID.
- In non-interactive mode, `--challenge-pack-version` and `--deployments` are required.
- In a TTY, missing challenge pack version, input set, or deployments can open pickers.
- `run create` does not resolve pack slugs, input set keys, or deployment names. Use `eval start` for that.
- `--scope`, `--suite`, `--case`, `--race-context`, and `--race-context-cadence` behave like `eval start`, but suite and case flags are ID-first.

The run create request body sent by the CLI contains:

```json
{
  "workspace_id": "<WORKSPACE_ID>",
  "challenge_pack_version_id": "<CHALLENGE_PACK_VERSION_ID>",
  "challenge_input_set_id": "<CHALLENGE_INPUT_SET_ID>",
  "agent_deployment_ids": ["<AGENT_DEPLOYMENT_ID>"],
  "official_pack_mode": "full",
  "name": "Smoke eval",
  "regression_suite_ids": ["<REGRESSION_SUITE_ID>"],
  "regression_case_ids": ["<REGRESSION_CASE_ID>"],
  "race_context": true,
  "race_context_min_step_gap": 3
}
```

Optional fields are omitted when not set. The create-run API requires JSON, caps the body at 1 MiB, rejects unknown JSON fields, and returns:

```json
{
  "id": "<RUN_ID>",
  "workspace_id": "<WORKSPACE_ID>",
  "challenge_pack_version_id": "<CHALLENGE_PACK_VERSION_ID>",
  "challenge_input_set_id": "<CHALLENGE_INPUT_SET_ID>",
  "official_pack_mode": "full",
  "status": "queued",
  "execution_mode": "single_agent",
  "created_at": "<timestamp>",
  "queued_at": "<timestamp>",
  "race_context": false,
  "links": {
    "self": "/v1/runs/<RUN_ID>",
    "agents": "/v1/runs/<RUN_ID>/agents"
  }
}
```

## Suite-Only Runs
Use suite-only scope when you want to run only selected regression suites or cases.

With `eval start`, suites can be IDs or exact names:

```bash
agentclash eval start \
  --pack <PACK_ID_OR_SLUG> \
  --pack-version <VERSION_ID_OR_NUMBER> \
  --deployment <DEPLOYMENT_ID_OR_NAME> \
  --scope suite_only \
  --suite <REGRESSION_SUITE_ID_OR_EXACT_NAME> \
  --follow
```

With `run create`, use IDs:

```bash
agentclash run create \
  --challenge-pack-version <CHALLENGE_PACK_VERSION_ID> \
  --deployments <AGENT_DEPLOYMENT_ID> \
  --scope suite_only \
  --suite <REGRESSION_SUITE_ID> \
  --follow
```

`--scope suite_only` requires at least one `--suite` or `--case`.

## Repeated Eval Sessions
Use `--repetitions` on `eval start` for repeated runs of the same eval.

```bash
agentclash eval start \
  --pack <PACK_ID_OR_SLUG> \
  --pack-version <VERSION_ID_OR_NUMBER> \
  --input-set <INPUT_SET_ID_OR_KEY> \
  --deployment <DEPLOYMENT_ID_OR_NAME> \
  --repetitions 3 \
  --json
```

Exact repetition behavior:

- `--repetitions` must be between 1 and 100.
- `--repetitions 1` creates a normal run through `/v1/runs`.
- `--repetitions >= 2` posts to `/v1/eval-sessions`.
- `--follow` is not supported with `--repetitions >= 2`; tail individual child runs with `agentclash run events <RUN_ID>`.
- `--scope suite_only`, `--suite`, `--case`, and race-context flags are not supported with `--repetitions >= 2`.
- The eval-session response is `{ "eval_session": {...}, "run_ids": [...] }`.

In human output, the CLI prints eval session ID, status, repetitions, and child run IDs. In structured output, it prints the raw response envelope.

## Eval Session Commands
When an eval session already exists (from `--repetitions >= 2` or API), inspect and follow aggregation with:

```bash
agentclash eval session list
agentclash eval session list --limit 20 --offset 0
agentclash eval session get <EVAL_SESSION_ID>
agentclash eval session follow <EVAL_SESSION_ID>
agentclash eval session follow <EVAL_SESSION_ID> --poll-interval 5s --timeout 30m
```

Behavior:

- `eval session list` — paginated workspace eval sessions (`--limit` 1–100, `--offset` ≥ 0).
- `eval session get` — session detail plus aggregate metrics when available.
- `eval session follow` — polls until aggregation finishes; `--timeout 0` disables timeout.
- Requires workspace context like other eval commands.

Use `eval session follow` after creating a multi-repetition eval when you need aggregated results before reading scorecards or comparisons.

## Run Series Commands
`run series` crosses deployment lineups with seeds for race-style series evals:

```bash
agentclash run series create \
  --challenge-pack-version <CHALLENGE_PACK_VERSION_ID> \
  --deployment-lineups <LINEUP_ID> \
  --seeds 3 \
  --name "Series smoke"

agentclash run series report <EVAL_SESSION_ID>
```

`run series create` flags:

- `--challenge-pack-version` — required pack version ID.
- `--input-set` — optional challenge input set ID.
- `--deployment-lineups` — repeatable lineup IDs crossed with `--seeds`.
- `--seeds` — integer 1–100 per lineup.
- `--name` — optional series name.
- `--max-iter` — optional per-child-run iteration override (1–1000).

`run series create` posts to `/v1/eval-sessions` and returns an eval session plus child run IDs (same family as `eval start --repetitions`).

`run series report <eval-session-id>` reads `/v1/eval-sessions/<id>` and prints aggregate score, correctness, and cost for the series.

## Follow and Events
Use `--follow` for interactive runs when you want immediate event visibility.

```bash
agentclash eval start ... --follow
agentclash run create ... --follow
agentclash run events <RUN_ID>
```

`run events` streams `/v1/runs/<runID>/events/stream` via SSE.

- In structured output mode (`--json` or `--output yaml`), `eval start --follow` and `run create --follow` print the created run and do not stream events. Use `agentclash run events <RUN_ID> --json` or `--output yaml` for structured event streams.
- Human output prints timestamped event summaries.
- `--json` prints one NDJSON event payload per line.
- `--output yaml` prints a YAML multi-document stream.
- Press Ctrl+C to stop an event stream.

## Inspect After Creation
Use these read commands after a run is created:

```bash
agentclash run list --json
agentclash run get <RUN_ID> --json
agentclash run agents <RUN_ID> --json
agentclash run ranking <RUN_ID> --json
agentclash run ranking <RUN_ID> --sort-by composite
agentclash run failures <RUN_ID> --json
agentclash eval scorecard <RUN_ID> --agent <RUN_AGENT_ID_OR_LABEL> --json
agentclash run scorecard <RUN_AGENT_ID> --json
```

Read command notes:

- `run list` lists runs in the workspace.
- `run get` reads `/v1/runs/<id>`.
- `run agents` lists run agents and labels.
- `run ranking --sort-by` accepts `composite`, `correctness`, `reliability`, `latency`, or `cost`.
- `run failures` accepts `--agent`, `--severity`, `--class`, `--evidence-tier`, `--cluster`, `--cursor`, and `--limit`.
- `run scorecard` expects a run agent ID.
- `eval scorecard [run]` is run-first. If run is omitted, it selects the latest workspace run; with multiple run agents, pass `--agent` in non-interactive mode.
- `eval scorecard --json` returns an envelope with `candidate`, `baseline`, `scorecard`, `comparison`, and `release_gate`.
- If scorecard generation is pending, stateful scorecard reads can return a pending payload instead of a final scorecard.

## Common Failure Modes
- No workspace: run `agentclash link`, `agentclash workspace use <id>`, pass `--workspace`, or set `AGENTCLASH_WORKSPACE`.
- No challenge packs: publish a pack first with `agentclash-challenge-pack-validation-publish`.
- Multiple packs in non-interactive `eval start`: pass `--pack` or a version ID through `--pack-version`.
- Version number without pack: pass `--pack` as well, because a bare version number cannot identify a pack.
- Multiple input sets: pass `--input-set`; `eval start` can use ID/key/exact name, while `run create` expects ID.
- Multiple deployments: pass one or more `--deployment` flags for `eval start`, or `--deployments` IDs for `run create`.
- `missing_challenge_input_set_id`: the selected pack version has multiple input sets and no input set ID was submitted.
- `invalid_agent_deployment_ids`: deployment IDs must be active deployments with snapshots in the selected workspace, with no duplicates.
- `invalid_challenge_pack_version_id`: the version must be runnable and visible to the selected workspace.
- `invalid_challenge_input_set_id`: the input set must belong to the selected challenge pack version.
- `invalid_race_context`: race context requires at least two agents.
- `--race-context-cadence must be 0 (backend default) or between 1 and 10`: fix the cadence value.
- `--follow is not supported with --repetitions >= 2`: create the eval session, then use `eval session follow` or stream individual child runs with `run events`.
- `--scope suite_only requires at least one --suite or --case`: add a suite or case selection.
- Scorecard pending or errored: report the state, then collect `run events`, `run agents`, and `run failures`.

## Safety Notes
- Creating runs can spend provider budget and may execute tools or network access allowed by the deployment/runtime profile.
- Confirm before running production-scale, multi-deployment, high-repetition, or network-enabled evals.
- Prefer small input sets and `--scope suite_only` for smoke checks.
- Do not paste secrets from run events, scorecards, failures, artifacts, or logs into chat.
- Use `--json` for automation and save run IDs before starting follow streams.

## Report Back Format
```text
Workspace: <workspace-id>
Command used:
Run ID: <id or none>
Eval session ID: <id or none>
Child run IDs: <ids if repetitions >= 2>
Challenge pack version: <id>
Input set: <id/key/name or none>
Deployments:
- <id/name>
Scope: <full|suite_only>
Followed: <yes/no>
Status: <queued/running/completed/failed/etc>
Agents: <count and labels>
Ranking: <summary or unavailable>
Failures: <count/filter summary or unavailable>
Scorecard: <state/link/summary or unavailable>
Evidence commands:
- agentclash eval session get <EVAL_SESSION_ID> --json
- agentclash run series report <EVAL_SESSION_ID>
- agentclash run get <RUN_ID> --json
- agentclash run agents <RUN_ID> --json
- agentclash run ranking <RUN_ID> --json
- agentclash run failures <RUN_ID> --json
Next action: <recommendation>
```

## Related Skills
- `agentclash-hub`
- `agentclash-cli-setup`
- `agentclash-quickstart`
- `agentclash-agent-deployment-setup`
- `agentclash-challenge-pack-validation-publish`
- `agentclash-scorecard-reader`
- `agentclash-compare-and-triage`
- `agentclash-multi-turn-operator`
- `agentclash-regression-flywheel`
- `agentclash-ci-release-gate`

## Related Docs
- `/docs-md/getting-started/first-eval`
- `/docs-md/concepts/runs-and-evals`
- `/docs-md/concepts/replay-and-scorecards`
- `/docs-md/reference/cli`
