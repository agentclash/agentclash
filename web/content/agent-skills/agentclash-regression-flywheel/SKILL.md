---
name: agentclash-regression-flywheel
description: Use when inspecting AgentClash run failure-review items, promoting useful failures into regression suites, editing regression suites or cases, and verifying suite-only reruns.
metadata:
  agentclash.role: regression
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Regression Flywheel

## Purpose
Turn understood AgentClash failures into durable regression coverage, then verify the promoted cases with suite-only runs.

## Use When
- A user wants to inspect failure-review items and decide which failures should become regression cases.
- A failure item has `promotable: true` and a useful `promotion_mode_available`.
- A regression suite needs to be created, renamed, archived, reactivated, or used for verification.
- A regression case needs title, description, status, or severity cleanup after promotion.
- A fix needs to be checked against a targeted regression suite or case.

## Do Not Use When
- The run has not produced failure evidence yet; use `agentclash-eval-runner` to run or follow it.
- The user only needs to interpret a scorecard, replay, artifact, or ranking; use `agentclash-scorecard-reader` first.
- The challenge pack itself needs authoring, validation, or publishing; use the challenge-pack skills.
- The task is to configure release gates or CI promotion policy; use `agentclash-ci-release-gate`.

## Inputs Needed
- Workspace ID or configured workspace context.
- Run ID containing failure-review items.
- Source challenge pack ID for the target regression suite.
- Target suite ID, or the suite name/details needed to create one.
- Failure selector: `challenge_identity_id` from `run failures --json`, plus `run_agent_id` when more than one agent failed the same challenge.
- Promotion mode from the failure item's `promotion_mode_available`: `full_executable` or `output_only`.
- Case title, optional failure summary, optional severity, and any validator overrides.
- Deployment and challenge pack version IDs/selectors for a suite-only verification run.

## Environment
Use hosted production by default unless the user intentionally targets local or self-hosted infrastructure:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash auth status
agentclash workspace use <WORKSPACE_ID>
```

All commands in this skill require workspace context. Workspace resolution follows the CLI setup rules: `--workspace`, `AGENTCLASH_WORKSPACE`, saved config, or `.agentclash.yaml`.

## Procedure
1. Read failure-review items for the run and group them by `failure_cluster_key`, `severity`, `failure_class`, and `promotable`.
2. Use `agentclash-scorecard-reader` evidence first: confirm the failed dimensions, judge/validator refs, replay refs, and artifact refs before promotion.
3. Choose an existing active suite whose `source_challenge_pack_id` matches the run source pack, or create one for that source pack.
4. Check for duplicates in the target suite by source failure cluster, failure fingerprint, challenge key, case key, and existing active/proposed cases.
5. Promote the failure with `run promote-failure <RUN_ID> <CHALLENGE_IDENTITY_ID>`.
6. Review the generated case JSON and update title, description, status, or severity if needed.
7. Run a suite-only verification against the updated deployment and report pass/fail coverage.

## Inspect Failures
Start with failure-review items:

```bash
agentclash run failures <RUN_ID> --json
agentclash run failures <RUN_ID> --agent <RUN_AGENT_ID> --json
agentclash run failures <RUN_ID> --severity blocking --json
agentclash run failures <RUN_ID> --class policy_violation --json
agentclash run failures <RUN_ID> --cluster <FAILURE_CLUSTER_KEY> --limit 50 --json
```

Supported filters are:

- `--agent <RUN_AGENT_ID>`
- `--severity info|warning|blocking`
- `--class <failure_class>`
- `--evidence-tier none|native_structured|hosted_structured|hosted_black_box|derived_summary`
- `--cluster <FAILURE_CLUSTER_KEY>`
- `--cursor <NEXT_CURSOR>`
- `--limit <COUNT>`

Failure classes currently accepted by the API are `incorrect_final_output`, `tool_selection_error`, `tool_argument_error`, `retrieval_grounding_failure`, `policy_violation`, `timeout_or_budget_exhaustion`, `sandbox_failure`, `dependency_resolution_failure`, `malformed_output`, `flaky_non_deterministic`, `insufficient_evidence`, and `other`.

The fields that matter for promotion are:

```json
{
  "items": [
    {
      "run_id": "<RUN_ID>",
      "run_agent_id": "<RUN_AGENT_ID>",
      "challenge_identity_id": "<CHALLENGE_IDENTITY_ID>",
      "challenge_key": "<challenge_key>",
      "case_key": "<case_key>",
      "item_key": "<item_key>",
      "failure_fingerprint": "frf_...",
      "failure_cluster_key": "frc_...",
      "failure_state": "failed",
      "failed_dimensions": ["correctness"],
      "failed_checks": ["<validator_or_judge_key>"],
      "failure_class": "policy_violation",
      "headline": "<headline>",
      "detail": "<detail>",
      "recommended_action": "<recommended action>",
      "promotable": true,
      "promotion_mode_available": ["full_executable", "output_only"],
      "replay_step_refs": [],
      "artifact_refs": [],
      "judge_refs": [],
      "metric_refs": [],
      "evidence_tier": "hosted_structured",
      "severity": "blocking"
    }
  ],
  "clusters": [
    {
      "failure_cluster_key": "frc_...",
      "representative_failure_fingerprint": "frf_...",
      "count": 2,
      "promotable_count": 1,
      "severity": "blocking",
      "failure_state": "failed",
      "failure_class": "policy_violation",
      "evidence_tier": "hosted_structured",
      "challenge_keys": ["<challenge_key>"],
      "case_keys": ["<case_key>"],
      "run_agent_ids": ["<RUN_AGENT_ID>"],
      "headline": "<headline>",
      "recommended_action": "<recommended action>"
    }
  ],
  "next_cursor": "<cursor>"
}
```

Promote only when `promotable` is true and the chosen `promotion_mode` appears in `promotion_mode_available`.

## Manage Suites
List and inspect suites:

```bash
agentclash regression-suite list --json
agentclash regression-suite get <SUITE_ID> --json
agentclash regression-suite cases <SUITE_ID> --json
```

`regression-suite` also has the alias `regression-suites`.

Create a suite:

```bash
agentclash regression-suite create \
  --source-challenge-pack-id <CHALLENGE_PACK_ID> \
  --name "Checkout regressions" \
  --description "Failures promoted from checkout evals" \
  --default-gate-severity warning \
  --json
```

Equivalent `--from-file` payload:

```json
{
  "source_challenge_pack_id": "<CHALLENGE_PACK_ID>",
  "name": "Checkout regressions",
  "description": "Failures promoted from checkout evals",
  "default_gate_severity": "warning"
}
```

Exact suite create rules:

- `source_challenge_pack_id` is required and must identify a challenge pack visible to the workspace.
- `name` is required.
- `default_gate_severity` is optional and defaults to `warning`.
- Allowed severities are `info`, `warning`, and `blocking`.
- New suites are created with `status: "active"` and `source_mode: "derived_only"`.

Update a suite:

```bash
agentclash regression-suite update <SUITE_ID> \
  --name "Checkout regressions" \
  --description "Current production blockers" \
  --status active \
  --default-gate-severity blocking \
  --json
```

Equivalent `--from-file` payload:

```json
{
  "name": "Checkout regressions",
  "description": "Current production blockers",
  "status": "active",
  "default_gate_severity": "blocking"
}
```

Exact suite update rules:

- At least one field must be provided.
- `status` must be `active` or `archived`.
- `default_gate_severity` must be `info`, `warning`, or `blocking`.
- Archived suites cannot accept new promotions.

Suite JSON includes:

```json
{
  "id": "<SUITE_ID>",
  "workspace_id": "<WORKSPACE_ID>",
  "source_challenge_pack_id": "<CHALLENGE_PACK_ID>",
  "name": "Checkout regressions",
  "description": "Current production blockers",
  "status": "active",
  "source_mode": "derived_only",
  "default_gate_severity": "blocking",
  "case_count": 3,
  "created_by_user_id": "<USER_ID>",
  "created_at": "<timestamp>",
  "updated_at": "<timestamp>"
}
```

`regression-suite list --json` prints `{ "items": [...] }` from the CLI. It does not expose the API's `total`, `limit`, or `offset` fields today.

## Promote Failures
The promotion command shape is:

```bash
agentclash run promote-failure <RUN_ID> <CHALLENGE_IDENTITY_ID> \
  --run-agent <RUN_AGENT_ID> \
  --suite <SUITE_ID> \
  --promotion-mode full_executable \
  --title "Policy answer must refuse credential disclosure" \
  --failure-summary "Agent disclosed a credential-like value instead of refusing." \
  --severity blocking \
  --json
```

Important exact details:

- The second positional argument is `challenge_identity_id` from `run failures --json`, not `failure_fingerprint` or `failure_cluster_key`.
- Pass `--run-agent` when the same challenge identity failed for multiple run agents; otherwise the backend returns `failure_review_item_ambiguous`.
- `--suite`, `--promotion-mode`, and `--title` map to required JSON fields.
- `--promotion-mode` should be `full_executable` or `output_only`, and it must be present in the failure item's `promotion_mode_available`.
- `--severity` is optional. If omitted, `policy_violation` and `sandbox_failure` default to `blocking`; other failure classes default to `warning`.
- The CLI has no `--status`, `--validator-overrides`, or `--metadata` flags for promotion. Use `--from-file` for those fields.

Full `--from-file` payload:

```json
{
  "run_agent_id": "<RUN_AGENT_ID>",
  "suite_id": "<SUITE_ID>",
  "promotion_mode": "full_executable",
  "title": "Policy answer must refuse credential disclosure",
  "failure_summary": "Agent disclosed a credential-like value instead of refusing.",
  "status": "proposed",
  "severity": "blocking",
  "validator_overrides": {
    "judge_threshold_overrides": {
      "policy_refusal": 0.9
    },
    "assertion_toggles": {
      "must_refuse": true
    }
  },
  "metadata": {
    "source": "triage",
    "source_challenge_key": "<challenge_key>",
    "source_failure_fingerprint": "frf_...",
    "source_failure_cluster_key": "frc_..."
  }
}
```

Exact promotion rules:

- `suite_id` is required.
- `title` is required.
- `status`, when provided, must be `active` or `proposed`.
- `severity`, when provided, must be `info`, `warning`, or `blocking`.
- `validator_overrides` may contain only `judge_threshold_overrides` and `assertion_toggles`.
- `metadata` must be a JSON object or null.
- If you want `source_challenge_key`, `source_failure_fingerprint`, or `source_failure_cluster_key` on the case response for duplicate checks, include those exact keys in `metadata`.
- The target suite must be active and must have the same `source_challenge_pack_id` as the run source pack.
- The failure item must be promotable. Items without a challenge input set or with insufficient reproduction context may have no available promotion modes.

`run promote-failure --json` prints the regression case object directly. The HTTP status is 201 when a case is created and 200 when the same suite, run agent, and challenge identity already map to an existing case; the CLI JSON output is the case in both paths.

## Review and Edit Cases
List cases in a suite:

```bash
agentclash regression-suite cases <SUITE_ID> --json
```

Update a case:

```bash
agentclash regression-suite case update <CASE_ID> \
  --title "Policy answer must refuse credential disclosure" \
  --description "Covers credential disclosure requests in support chat." \
  --status active \
  --severity blocking \
  --json
```

Equivalent `--from-file` payload:

```json
{
  "title": "Policy answer must refuse credential disclosure",
  "description": "Covers credential disclosure requests in support chat.",
  "status": "active",
  "severity": "blocking"
}
```

Exact case update rules:

- At least one field must be provided.
- `status` must be `proposed`, `active`, `muted`, `archived`, or `rejected`.
- `severity` must be `info`, `warning`, or `blocking`.
- There is no CLI command today to create a regression case directly, fetch a single case directly, or patch `expected_contract`, `payload_snapshot`, `validator_overrides`, or `metadata` after promotion.

Case JSON includes:

```json
{
  "id": "<CASE_ID>",
  "suite_id": "<SUITE_ID>",
  "workspace_id": "<WORKSPACE_ID>",
  "title": "Policy answer must refuse credential disclosure",
  "description": "Covers credential disclosure requests in support chat.",
  "status": "active",
  "severity": "blocking",
  "promotion_mode": "full_executable",
  "source_run_id": "<RUN_ID>",
  "source_run_agent_id": "<RUN_AGENT_ID>",
  "source_replay_id": "<REPLAY_ID>",
  "source_challenge_pack_version_id": "<CHALLENGE_PACK_VERSION_ID>",
  "source_challenge_input_set_id": "<INPUT_SET_ID>",
  "source_challenge_identity_id": "<CHALLENGE_IDENTITY_ID>",
  "source_challenge_key": "<challenge_key>",
  "source_case_key": "<case_key>",
  "source_item_key": "<item_key>",
  "source_failure_fingerprint": "frf_...",
  "source_failure_cluster_key": "frc_...",
  "evidence_tier": "hosted_structured",
  "failure_class": "policy_violation",
  "failure_summary": "<summary>",
  "payload_snapshot": {},
  "expected_contract": {},
  "validator_overrides": {},
  "metadata": {},
  "latest_promotion": {
    "id": "<PROMOTION_ID>",
    "workspace_regression_case_id": "<CASE_ID>",
    "source_run_id": "<RUN_ID>",
    "source_run_agent_id": "<RUN_AGENT_ID>",
    "source_event_refs": [],
    "promoted_by_user_id": "<USER_ID>",
    "promotion_reason": "<summary>",
    "promotion_snapshot": {},
    "created_at": "<timestamp>"
  },
  "validation": {
    "status": "not_validated",
    "run_count": 0,
    "failure_count": 0,
    "pass_count": 0,
    "reproduction_threshold": 0.6,
    "required_runs": 5,
    "remaining_runs": 5,
    "recommended_action": "<action>"
  },
  "created_at": "<timestamp>",
  "updated_at": "<timestamp>"
}
```

Validation status values are `not_validated`, `collecting_signal`, `reproducing`, `passing`, and `flaky`.

## Duplicate and Quality Checks
Before promotion:

- Compare the target suite's existing cases by `source_case_key`, `status`, and any available `source_failure_cluster_key`, `source_failure_fingerprint`, or `source_challenge_key` fields.
- Prefer updating or reusing an existing active/proposed case when a failure is the same behavior, even if it came from a different run.
- Promote only failures with concrete replay, judge, validator, metric, or artifact evidence. Avoid promoting `insufficient_evidence` unless the goal is explicitly to track missing evidence.
- Use `full_executable` when the failure has a challenge input set and enough structured evidence to replay the case. Use `output_only` when only the final output contract can be captured.

Backend duplicate protection is intentionally narrow: the same suite, run agent, and challenge identity returns the existing case. Cross-run duplicates and cross-suite duplicates are reviewer decisions.

## Verify Suite-Only
Use `eval start` when selectors can be names, slugs, or exact suite names:

```bash
agentclash eval start \
  --pack <PACK_ID_OR_SLUG_OR_EXACT_NAME> \
  --pack-version <VERSION_ID_OR_VERSION_NUMBER> \
  --deployment <DEPLOYMENT_ID_OR_EXACT_NAME> \
  --scope suite_only \
  --suite <SUITE_ID_OR_EXACT_NAME> \
  --follow
```

Use `run create` when automation already has IDs:

```bash
agentclash run create \
  --challenge-pack-version <CHALLENGE_PACK_VERSION_ID> \
  --deployments <AGENT_DEPLOYMENT_ID> \
  --scope suite_only \
  --suite <SUITE_ID> \
  --case <CASE_ID> \
  --follow
```

Exact suite-only notes:

- `--scope suite_only` requires at least one `--suite` or `--case`.
- In `eval start`, `--suite` can resolve a suite ID or exact suite name; `--case` is a case ID.
- In `run create`, `--suite` and `--case` are ID-first.
- `--repetitions >= 2` does not support `--scope suite_only`, `--suite`, or `--case`.
- After the run, inspect `agentclash run get <RUN_ID> --json` for `regression_coverage`.

`regression_coverage` contains:

```json
{
  "regression_coverage": {
    "suites": [
      {
        "id": "<SUITE_ID>",
        "name": "Checkout regressions",
        "case_count": 3,
        "pass_count": 2,
        "fail_count": 1
      }
    ],
    "unmatched_cases": [
      {
        "id": "<CASE_ID>",
        "title": "<case title>",
        "outcome": "fail"
      }
    ]
  }
}
```

Then inspect:

```bash
agentclash run failures <VERIFICATION_RUN_ID> --json
agentclash eval scorecard <VERIFICATION_RUN_ID> --agent <RUN_AGENT_ID_OR_LABEL> --json
agentclash run ranking <VERIFICATION_RUN_ID> --json
```

## Expected Output
- A small set of promoted cases with clear source evidence, status, severity, suite, and promotion mode.
- No duplicate active/proposed cases for the same behavior in the target suite.
- A suite-only verification run ID and result.
- A concise explanation of whether the fix passes, fails, or needs more validation runs.

## Failure Modes
- Missing workspace: run `agentclash link`, `agentclash workspace use <id>`, pass `--workspace`, or set `AGENTCLASH_WORKSPACE`.
- `source_challenge_pack_id is required`: create the suite with the source challenge pack ID, not a challenge pack version ID.
- `challenge_pack_not_found`: the source challenge pack is not visible to the workspace.
- `regression_suite_name_conflict`: rename the suite or reuse the existing active suite.
- `regression_suite_archived`: reactivate the suite or pick an active one.
- `regression_suite_pack_mismatch`: choose a suite whose `source_challenge_pack_id` matches the run source pack.
- `failure_review_item_not_found`: use the `challenge_identity_id` from `run failures --json`, not the fingerprint or cluster key.
- `failure_review_item_ambiguous`: pass `--run-agent <RUN_AGENT_ID>`.
- `failure_not_promotable`: do not promote; collect better evidence or run with a challenge input set.
- `promotion_mode_unavailable`: choose a mode listed in `promotion_mode_available`.
- `invalid_promotion_overrides`: use only `judge_threshold_overrides` and `assertion_toggles` with the correct map value types.
- `--scope suite_only requires at least one --suite or --case`: add a suite or case selector.

## Safety Notes
- Promotion and suite/case updates mutate shared workspace state. Confirm intent before changing production suites.
- Do not put secrets, customer data, raw artifact contents, or long traces into case titles, summaries, descriptions, metadata, or chat.
- Prefer `status: "proposed"` when a reviewer still needs to approve the case.
- Archive or reject noisy cases instead of leaving weak regressions active.
- Keep suite-only verification focused; avoid broad full-pack reruns when a targeted suite is enough.

## Report Back Format
```text
Run: <RUN_ID>
Failure reviewed:
- challenge_identity_id=<id> run_agent_id=<id> cluster=<frc_...> class=<failure_class> severity=<severity>
Suite: <SUITE_ID> (<name>)
Duplicate check: <none found | reused CASE_ID | updated CASE_ID>
Promotion:
- case=<CASE_ID> mode=<full_executable|output_only> status=<proposed|active> severity=<severity>
Case edits: <none | title/description/status/severity changes>
Verification:
- command=<exact suite-only command>
- run=<VERIFICATION_RUN_ID>
- regression_coverage=<pass/fail counts or unavailable>
Next action: <ship/fix/rerun/needs-review>
```

## Related Skills
- `agentclash-hub`
- `agentclash-cli-setup`
- `agentclash-eval-runner`
- `agentclash-scorecard-reader`
- `agentclash-compare-and-triage`
- `agentclash-ci-release-gate`

## Related Docs
- `/docs-md/concepts/replay-and-scorecards`
- `/docs-md/concepts/runs-and-evals`
- `/docs-md/reference/cli`
