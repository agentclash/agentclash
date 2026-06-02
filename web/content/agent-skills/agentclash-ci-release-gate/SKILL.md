---
name: agentclash-ci-release-gate
description: Use when wiring AgentClash manifest-based CI gates, deciding whether a PR should run AgentClash, resolving baselines, running `agentclash ci run`, interpreting gate exit codes, collecting CI artifacts, or configuring regression promotion policy in GitHub Actions.
metadata:
  agentclash.role: ci
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash CI Release Gate

## Purpose
Wire an AgentClash candidate build/deployment into a repo-tracked CI manifest, compare it against a baseline run, and turn the release gate verdict into CI status.

## Use When
- A pull request or mainline workflow should run AgentClash only when relevant files or labels match.
- A user needs a `.agentclash/ci.yaml` manifest for a candidate agent, workload, baseline, gate, and regression promotion policy.
- CI should fail on AgentClash release gate failures, warnings, insufficient evidence, setup errors, run timeouts, or candidate run failures.
- A GitHub Actions workflow should use the repo-local `.github/actions/agentclash-ci` composite action.

## Do Not Use When
- The agent build spec does not exist yet; use `agentclash-agent-build-author`.
- Runtime profiles, provider accounts, model aliases, or workspace tools are not configured; use `agentclash-runtime-resources-setup`.
- The challenge pack or input set is not validated and published; use the challenge-pack skills first.
- The task is only to read scorecards or failure evidence from an existing run; use `agentclash-scorecard-reader` or `agentclash-regression-flywheel`.

## Environment
Use hosted production by default:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
export AGENTCLASH_TOKEN="<token>"
export AGENTCLASH_WORKSPACE="<workspace-id>"
```

In GitHub Actions, pass tokens through secrets or the composite action inputs. Do not print tokens. `ci run` exits with code `10` and a JSON error envelope when workspace context is missing, so set `AGENTCLASH_WORKSPACE`, pass `--workspace`, or use saved CLI workspace config.

## Manifest Shape
Create a sample with:

```bash
agentclash ci init .agentclash/ci.yaml
agentclash ci init .agentclash/ci.yaml --force
```

The CLI sample manifest is:

```yaml
version: 1
trigger:
  paths:
    - .agentclash/agent.json
    - prompts/**
    - tools/**
  labels:
    - agentclash/eval
candidate:
  build:
    agent_build_id: 00000000-0000-0000-0000-000000000001
    spec_file: .agentclash/agent.json
  deployment:
    name: pr-candidate
    runtime_profile_id: 00000000-0000-0000-0000-000000000002
    provider_account_id: 00000000-0000-0000-0000-000000000003
    model_alias_id: 00000000-0000-0000-0000-000000000004
evaluation:
  challenge_pack_version_id: 00000000-0000-0000-0000-000000000005
  input_set_id: 00000000-0000-0000-0000-000000000006
  regression_suites:
    - 00000000-0000-0000-0000-000000000007
baseline:
  run_id: 00000000-0000-0000-0000-000000000008
  refresh: manual
  max_age_days: 30
gate:
  fail_on: regression
regressions:
  promote_failures: proposed
```

Exact fields the current CLI parses:

- `version`: must be `1`.
- `trigger.paths`: required, nonblank doublestar globs; `trigger.labels`: optional labels that can force a run.
- `candidate.build.agent_build_id`: required existing agent build ID.
- `candidate.build.spec_file`: required relative path inside the repository; absolute paths and `..` escapes are rejected by `ci run`.
- `candidate.deployment.name`: optional deployment name; if blank, `ci run` generates `agentclash-ci-<unix>`.
- `candidate.deployment.runtime_profile_id`: required.
- `candidate.deployment.provider_account_id` and `candidate.deployment.model_alias_id`: optional; remote validation checks both exist and rejects a model alias whose `provider_account_id` conflicts with the provider account field.
- `evaluation.challenge_pack_version_id`: required.
- `evaluation.input_set_id`, `evaluation.regression_suites`, and `evaluation.regression_cases`: optional; blank regression entries are invalid.
- `baseline.run_id`: locked baseline run, preferred for PR gates.
- `baseline.run_agent_id`: optional but only valid with `baseline.run_id`.
- `baseline.deployment_id`: moving baseline selector; mutually exclusive with `baseline.run_id`.
- `baseline.refresh`: optional `manual`, `propose`, or `auto_on_main`; default behavior is `manual`.
- `baseline.max_age_days`: optional non-negative freshness limit; `0` means no age check.
- `gate.fail_on`: required, one of `regression`, `warning`, or `insufficient_evidence`.
- `gate.policy_file`: optional schema field, but the current `agentclash ci run` implementation does not read or post it.
- `regressions.promote_failures`: required, one of `disabled`, `proposed`, or `auto_on_main`.

Important gate fidelity note: today `ci run` posts `baseline_run_id`, `candidate_run_id`, and optional run-agent IDs to `/v1/release-gates/evaluate`. It does not pass `gate.fail_on`, does not load `gate.policy_file`, and has no `--fail-on` or `--policy-file` flag. The backend normalizes an empty policy to the default release gate policy. Do not claim manifest gate fields customize evaluation until the CLI source wires that behavior.

## Validation Commands
Validate locally first:

```bash
agentclash ci validate .agentclash/ci.yaml
agentclash ci validate .agentclash/ci.yaml --json
```

Use remote validation when workspace credentials are available:

```bash
agentclash ci validate .agentclash/ci.yaml --remote --json
```

Structured `validate` output includes:

```json
{
  "path": ".agentclash/ci.yaml",
  "valid": true,
  "manifest": {},
  "remote": {
    "workspace_id": "<WORKSPACE_ID>",
    "valid": true,
    "checks": [
      {
        "field": "candidate.build.agent_build_id",
        "resource": "agent_build",
        "id": "<AGENT_BUILD_ID>",
        "valid": true,
        "code": "ok",
        "message": "agent build is accessible in the selected workspace"
      }
    ]
  }
}
```

Remote validation checks the agent build, runtime profile, provider account, model alias, challenge pack version, input set, regression suites, regression cases, and baseline compatibility. API failures that cannot be reduced to a field problem appear as `remote API error: ...`.

## Should Run
Use this before spending hosted evaluation budget:

```bash
agentclash ci should-run --manifest .agentclash/ci.yaml --base origin/main --head HEAD --json
agentclash ci should-run --manifest .agentclash/ci.yaml --changed-file prompts/refund.md --labels agentclash/eval --json
```

`--changed-file` may be repeated. `--labels` accepts comma-separated or repeated values. If changed files are omitted and refs are present, the CLI derives files with `git diff --name-only --diff-filter=ACDMRTUXB <base>...<head>`. Ref defaults can come from `AGENTCLASH_CI_BASE`, `GITHUB_BASE_REF` as `origin/<base>`, `AGENTCLASH_CI_HEAD`, `GITHUB_SHA`, and `HEAD` when a base is set.

JSON output shape:

```json
{
  "path": ".agentclash/ci.yaml",
  "should_run": true,
  "reason": "changed files matched trigger.paths",
  "changed_files": ["prompts/refund.md"],
  "labels": ["agentclash/eval"],
  "checked_path_globs": ["prompts/**"],
  "checked_labels": ["agentclash/eval"],
  "matched_paths": [{"pattern": "prompts/**", "file": "prompts/refund.md"}],
  "matched_labels": ["agentclash/eval"]
}
```

The decision is an OR: matched paths or matched labels make `should_run: true`. Reasons are exactly:

- `changed files matched trigger.paths and labels matched trigger.labels`
- `changed files matched trigger.paths`
- `labels matched trigger.labels`
- `no changed files or labels were provided`
- `no changed files or labels matched manifest triggers`

## Baseline Resolution
Resolve and print the exact baseline before running the gate:

```bash
agentclash ci baseline --manifest .agentclash/ci.yaml --json
```

For `baseline.run_id`, strategy is `locked_run` and source is `baseline.run_id`. The run must be in the selected workspace, completed, compatible with `evaluation.challenge_pack_version_id` and optional `evaluation.input_set_id`, and within `baseline.max_age_days` when set. `baseline.run_agent_id` is resolved against that run when present.

For `baseline.deployment_id`, strategy is `deployment_latest_completed` and source is `baseline.deployment_id`. The CLI selects the newest completed compatible run whose participant used that deployment and warns that deployment baselines move over time. Prefer `baseline.run_id` for PRs.

Refresh next actions are:

- `manual`: after a successful mainline run, update `baseline.run_id` intentionally in a reviewed change.
- `propose`: after a successful mainline run, open a reviewed change that updates `baseline.run_id`.
- `auto_on_main`: after a successful protected mainline run, automation may update `baseline.run_id` with an auditable commit.

## Run The Gate
Run the manifest workflow:

```bash
agentclash ci run --manifest .agentclash/ci.yaml --json --artifact-dir agentclash-artifacts
agentclash ci run --manifest .agentclash/ci.yaml --json --summary-file agentclash-summary.md
agentclash ci run --manifest .agentclash/ci.yaml --follow --timeout 30m --poll-interval 5s
```

Flags:

- `--manifest`: defaults to `.agentclash/ci.yaml`.
- `--follow`: streams run events only for non-JSON output.
- `--timeout`: duration, default `30m`; `0` disables timeout; negative values exit `10`.
- `--poll-interval`: duration, default `5s`; must be greater than zero.
- `--summary-file`: writes a Markdown gate summary.
- `--github-step-summary`: defaults true and appends when `GITHUB_STEP_SUMMARY` is set.
- `--artifact-dir`: writes stable JSON artifacts.
- CI metadata overrides: `--ci-provider`, `--ci-repository`, `--ci-pull-request`, `--ci-branch`, `--ci-ref`, `--ci-commit`, `--ci-workflow`, `--ci-workflow-run-id`, `--ci-workflow-run-attempt`, `--ci-workflow-run-url`, `--ci-event`, `--ci-default-branch`.

`ci run` does this in order: validate local manifest, remote-validate resource IDs, create a build version from `candidate.build.spec_file`, mark it ready, create a deployment, resolve the baseline, create a run with `official_pack_mode: "full"` plus optional regression suites/cases, wait for completion, resolve the candidate run agent, optionally fetch scorecard and comparison when reports are enabled, evaluate the release gate, optionally promote regression failures, then write reports.

Structured output includes:

```json
{
  "manifest_path": ".agentclash/ci.yaml",
  "workspace_id": "<WORKSPACE_ID>",
  "remote_validation": {},
  "candidate": {
    "agent_build_id": "<AGENT_BUILD_ID>",
    "build_version_id": "<BUILD_VERSION_ID>",
    "deployment_id": "<DEPLOYMENT_ID>",
    "run_id": "<RUN_ID>",
    "run_agent_id": "<RUN_AGENT_ID>",
    "run_status": "completed",
    "run_url": "<URL>",
    "deployment_name": "pr-candidate",
    "ci_metadata": {}
  },
  "baseline_resolution": {},
  "baseline": {
    "run_id": "<BASELINE_RUN_ID>",
    "run_agent_id": "<BASELINE_RUN_AGENT_ID>",
    "status": "completed"
  },
  "release_gate": {},
  "gate_verdict": "pass",
  "failure_reason": "",
  "reports": {},
  "regression_promotions": {},
  "exit_code": 0
}
```

Exit codes are exact:

- `0`: pass.
- `1`: release gate failed.
- `2`: release gate warning.
- `3`: insufficient gate evidence.
- `10`: invalid manifest, missing workspace for `ci run`, invalid duration flag, invalid CI metadata flag, or local candidate spec error.
- `20`: API/auth failure or report-writing failure after a successful gate.
- `30`: candidate run timed out.
- `31`: candidate run failed before gate evaluation.

Successful terminal run statuses are `completed`, `succeeded`, and `success`. Failed terminal statuses include `failed`, `error`, `errored`, `canceled`, `cancelled`, `aborted`, `timed_out`, `timeout`, and `expired`.

## Reports And Artifacts
Reports are enabled only when `--summary-file`, `GITHUB_STEP_SUMMARY` with `--github-step-summary`, or `--artifact-dir` is set. `--artifact-dir agentclash-artifacts` writes:

- `agentclash-artifacts/run.json` with kind `agentclash.ci.run`.
- `agentclash-artifacts/scorecard.json` with kind `agentclash.ci.scorecard`.
- `agentclash-artifacts/comparison.json` with kind `agentclash.ci.comparison`.
- `agentclash-artifacts/gate.json` with kind `agentclash.ci.gate`.
- `agentclash-artifacts/result.json` with kind `agentclash.ci.result`.

Each artifact is wrapped in an envelope with `schema_version: "2026-05-04"`, `kind`, `generated_at`, `manifest_path`, `workspace_id`, `challenge_pack_version_id`, `candidate`, `baseline`, optional gate policy identity fields, and `payload`.

## Regression Promotion
`regressions.promote_failures` runs only when the gate verdict is `fail`. Modes:

- `disabled`: returns a skipped summary with reason `policy_disabled`.
- `proposed`: creates proposed regression candidates.
- `auto_on_main`: creates active cases only on the default branch outside pull request events; otherwise it blocks with `pull_request_event`, `missing_default_branch`, or `non_default_branch`.

Promotion also blocks when `evaluation.regression_suites` is empty, using reason `no_regression_suites`. It lists candidate run failures with `limit=200`, prefers `full_executable` over `output_only`, skips non-promotable or unsupported failures, and avoids existing cases by `source_challenge_identity_id` or metadata `source_failure_cluster_key` unless the existing case status is `archived` or `rejected`.

`regression_promotions` contains `policy`, optional `case_status`, `created`, `existing`, `skipped`, `blocked`, and `errors`. Created/existing items include `suite_id`, `case_id`, `challenge_identity_id`, `challenge_key`, `failure_cluster_key`, `status`, and `created`.

## GitHub Action
The repo-local composite action is `.github/actions/agentclash-ci`. Example:

```yaml
name: AgentClash gate
on:
  pull_request:

jobs:
  agentclash:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - id: agentclash
        uses: agentclash/agentclash/.github/actions/agentclash-ci@main
        with:
          token: ${{ secrets.AGENTCLASH_TOKEN }}
          workspace: ${{ secrets.AGENTCLASH_WORKSPACE }}
          api-url: https://api.agentclash.dev
          manifest: .agentclash/ci.yaml
          artifact-dir: agentclash-artifacts
      - uses: actions/upload-artifact@v4
        if: always() && steps.agentclash.outputs['should-run'] == 'true'
        with:
          name: agentclash-ci
          path: |
            ${{ steps.agentclash.outputs['result-file'] }}
            ${{ steps.agentclash.outputs['artifact-dir'] }}/*.json
```

Action inputs are exactly `manifest`, `token`, `workspace`, `api-url`, `install-cli`, `cli-version`, `remote-validate`, `skip-if-unmatched`, `base`, `head`, `changed-files`, `labels`, `artifact-dir`, `result-file`, `timeout`, `poll-interval`, `follow`, and `default-branch`.

Action outputs are exactly `should-run`, `skip-reason`, `run-id`, `gate-verdict`, `exit-code`, `result-file`, and `artifact-dir`.

The action exports `AGENTCLASH_TOKEN`, `AGENTCLASH_WORKSPACE`, and `AGENTCLASH_API_URL`, installs `agentclash@<cli-version>` when `install-cli` is true, runs `ci validate` with `--remote` by default, runs `ci should-run --json`, exits `0` when unmatched and `skip-if-unmatched` is true, then runs `ci run --json --artifact-dir`.

## Failure Modes
- Missing token or API access: confirm `AGENTCLASH_TOKEN` and hosted API URL.
- Missing workspace: set `AGENTCLASH_WORKSPACE`, pass `--workspace`, or configure workspace locally.
- Local validation fails: fix YAML fields before remote calls; unknown YAML fields fail because the decoder uses known fields.
- Remote validation fails: check workspace visibility for build, runtime profile, provider account, model alias, challenge pack version, input set, regression suites/cases, and baseline.
- `should-run` skips unexpectedly: inspect `trigger.paths`, `trigger.labels`, checkout depth, base/head refs, and explicitly passed `changed-files`.
- Candidate spec fails: `candidate.build.spec_file` must be readable JSON at a relative path inside the repo.
- Candidate run is slow: tune `--timeout` and `--poll-interval`.
- `auto_on_main` is blocked: pass default-branch metadata and run from the default branch, not a PR event.

## Report Back Format
```text
Manifest: <path>
Should run: <true/false + reason>
Baseline: <run-id or deployment selector + strategy>
Candidate run: <run-id>
Gate verdict: <pass|warn|fail|insufficient_evidence>
Exit code: <code + meaning>
Artifacts: <result-file and artifact-dir>
Regression candidates: <created/existing/skipped/blocked/errors>
Next command: <exact agentclash command or GitHub Actions fix>
```

## Related Skills
- `agentclash-hub`: workflow map and dependency order.
- `agentclash-cli-setup`: authenticate, select workspace, and configure hosted API.
- `agentclash-runtime-resources-setup`: create runtime profiles, provider accounts, model aliases, secrets, and tools.
- `agentclash-agent-build-author`: create the candidate build spec and ready build version.
- `agentclash-agent-deployment-setup`: understand deployment resources used by the CI manifest.
- `agentclash-challenge-pack-validation-publish`: publish the challenge pack version and optional input sets.
- `agentclash-eval-runner`: run ad hoc evals before formal CI gates.
- `agentclash-scorecard-reader`: inspect scorecard, comparison, replay, and failure evidence.
- `agentclash-compare-and-triage`: baseline bookmarks, `compare latest --gate`, and replay triage.
- `agentclash-regression-flywheel`: promote failures and manage regression suites/cases.
- `agentclash-dataset-workflows`: dataset eval gates with `--format junit` for CI pipelines.
- `agentclash-security-evaluation`: client-side security stress harnesses before full pipeline runs.

## Related Docs
- `/docs-md/guides/ci-cd-agent-gates`
- `/docs-md/guides/ci-cd-workload-recipes`
- `/docs-md/challenge-packs/eval-workflows-and-gates`
