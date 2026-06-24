---
name: agentclash-challenge-pack-validation-publish
description: Use when validating AgentClash challenge pack YAML, fixing schema/scoring/tool/asset errors, publishing runnable pack versions, recording returned IDs, and preparing next eval commands.
metadata:
  agentclash.role: challenge-pack-publication
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack Validation and Publish

## Purpose
Validate a complete AgentClash challenge pack against the real workspace-backed parser, fix any source-reported errors, publish a runnable version only when intended, and report the IDs needed to start an eval.

## Use When
- A challenge pack YAML file is ready for validation after planning, YAML authoring, inputs, tools, artifacts, scoring, and judges are in place.
- The user asks whether a pack is publishable.
- The user asks to publish a pack and needs the returned pack/version/evaluation/input-set IDs.
- A reviewer needs exact next commands for `agentclash eval start` or `agentclash run create`.

## Do Not Use When
- The pack idea is still being designed; use `agentclash-challenge-pack-planner`.
- The YAML structure is being written from scratch; use `agentclash-challenge-pack-yaml-author`.
- The task is only to run an already published eval; use `agentclash-eval-runner`.
- The task is to upload standalone workspace artifacts; use `agentclash-challenge-pack-artifacts`.

## Inputs Needed
- Pack YAML path.
- Target workspace ID or a configured workspace from `agentclash link`, `agentclash workspace use`, `--workspace`, `AGENTCLASH_WORKSPACE`, or `.agentclash.yaml`.
- Whether publish is explicitly allowed now.
- Deployment ID/name for the next run command, if the user wants a ready-to-run command.

## Environment
Use hosted production by default unless the user intentionally targets local or self-hosted infrastructure:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

`agentclash challenge-pack validate` is not an offline-only local parser in the current CLI. It reads the YAML file, requires auth and a workspace, then posts the raw bundle to:

```text
POST /v1/workspaces/<workspace-id>/challenge-packs/validate
```

`agentclash challenge-pack publish` posts the same raw bundle to:

```text
POST /v1/workspaces/<workspace-id>/challenge-packs
```

Use `--json` for coding-agent workflows because human output intentionally omits several IDs.

## CLI Surface
Use the full command names in docs and automation. `challenge-pack` also has the `cp` alias, but the full form is clearer for agents.

```bash
agentclash challenge-pack validate path/to/pack.yaml --json
agentclash challenge-pack publish path/to/pack.yaml --json
agentclash challenge-pack list --json
```

Exact command notes from the CLI:

- `validate` and `publish` each take exactly one file path.
- `validate` and `publish` have no command-specific flags today; `--json`, `--workspace`, `--api-url`, and `--quiet` are global flags.
- `publish` does not implement a challenge-pack-specific dry run, confirmation flag, or update flag.
- `list --json` returns visible packs for the current workspace, with each pack's runnable `versions`.
- The API request body is capped at 1 MiB.

## Validation Procedure
1. Set hosted API URL unless the workflow is explicitly local or self-hosted.
2. Verify the workspace context before making workspace-backed calls.
3. Run validation with `--json`.
4. If the command fails, inspect both stdout and stderr. Current validation failures use HTTP 400, so the CLI can report the response body through its generic API-error path instead of returning a clean JSON object on stdout.
5. Fix every `field` and `message` pair reported by the API.
6. Re-run validation until it succeeds.
7. Only then publish, and only when the user has already approved publishing.

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash auth status
agentclash workspace use <WORKSPACE_ID>
agentclash challenge-pack validate path/to/pack.yaml --json
```

On success, `validate --json` returns:

```json
{
  "valid": true,
  "errors": null
}
```

On validation failure, the API response shape is:

```json
{
  "valid": false,
  "errors": [
    {
      "field": "version.evaluation_spec.validators[0].type",
      "message": "must be one of ..."
    }
  ]
}
```

Because invalid validation is returned with HTTP 400 today, do not assume a failing CLI invocation will print that object as successful structured stdout. Still use its `field` and `message` details when they are present.

## What Validation Checks
Validation parses the YAML bundle, normalizes legacy case data, runs bundle validation, validates scoring spec fields, and checks stored artifact IDs against the selected workspace.

Pack and version checks:

- `pack.slug`, `pack.name`, and `pack.family` are required.
- `version.number` must be greater than 0.
- `version.execution_mode` can be omitted, `native`, or `prompt_eval`.
- `prompt_eval` packs must not include top-level `tools`, `version.sandbox`, or `version.tool_policy`.
- At least one challenge is required.

Challenge checks:

- Every challenge needs `key`, `title`, `category`, and `difficulty`.
- `difficulty` must be exactly `easy`, `medium`, `hard`, or `expert`.
- Challenge keys must be unique.
- `artifact_refs[].key` must reference a declared `version.assets[].key`.

Input set checks:

- Every input set needs `key`, `name`, and at least one `cases` entry.
- Input set keys must be unique.
- All cases in the same input set must reference the same `challenge_key`.
- `challenge_key` must reference a declared challenge.
- `case_key` is the current field; legacy `item_key` is normalized but should not be newly authored.
- Case keys must be unique per challenge inside the input set.
- Case `inputs[].key`, `expectations[].key`, and local asset keys must be unique where they appear.
- `inputs[].artifact_key`, `expectations[].artifact_key`, and `source: artifact:<key>` must reference `version.assets`.
- `expectations[].source` may use `input:<input_key>` or `artifact:<asset_key>`; other prefixes are rejected.

Tools, sandbox, and artifact checks:

- `version.tool_policy.allowed_tool_kinds` must be an array containing only `browser`, `build`, `data`, `file`, and `network`.
- Custom composed tools must have a valid `implementation.primitive`, `implementation.args`, and parameter schema.
- Tool delegation cycles and delegation chains deeper than 8 are rejected.
- `version.sandbox.network_allowlist[]` entries must be valid CIDR strings.
- `version.sandbox.env_vars` keys must match `[A-Za-z_][A-Za-z0-9_]*`.
- `version.sandbox.additional_packages[]` values must look like apt package names.
- Each asset needs `key` and `path`; duplicate asset keys in the same list are rejected.
- If an asset includes `artifact_id`, validation checks that the artifact exists and belongs to the selected workspace.

Scoring checks:

- Errors from `version.evaluation_spec` are prefixed with `version.` in challenge-pack validation output.
- Strict evaluation-spec decoding catches unknown fields before scoring validation runs.
- Keep deterministic, LLM judge, and hybrid scoring mode rules aligned with `agentclash-challenge-pack-scoring-validators` and `agentclash-challenge-pack-llm-judges`.

## Fix Patterns
- `pack.family is required`: set a stable family string, usually the pack slug or product/workload family.
- `version.execution_mode must be one of "native", "prompt_eval"`: use exactly `native` or `prompt_eval`, or omit the field.
- `tools must be empty when version.execution_mode is prompt_eval`: switch to `native` or remove top-level `tools`.
- `version.tool_policy.allowed_tool_kinds[...] must be one of browser, build, data, file, network`: remove unsupported kinds such as `shell`.
- `input_sets[...].cases[...].challenge_key must reference the same challenge as the other cases in this input set`: split cases by challenge into separate input sets.
- `case_key is required`: add `case_key`; do not author new `item_key` entries.
- `expectations[...].source must start with input: or artifact:`: use `input:<input_key>` or `artifact:<version_asset_key>`.
- `artifact_id must reference an existing artifact`: upload or find the artifact first with the artifact CLI, then use its UUID.
- `artifact_id must belong to the workspace`: switch workspace or use an artifact from the selected workspace.
- `decode evaluation spec from yaml`: look for a misspelled or unsupported scoring field; this may surface as a CLI/API error rather than a normal `errors[]` item.

## Publish Procedure
Publish is a workspace mutation that creates a runnable challenge-pack version. Do it only after validation passes and the user has approved publishing.

```bash
agentclash challenge-pack validate path/to/pack.yaml --json
agentclash challenge-pack publish path/to/pack.yaml --json
```

On success, `publish --json` returns:

```json
{
  "challenge_pack_id": "<CHALLENGE_PACK_ID>",
  "challenge_pack_version_id": "<CHALLENGE_PACK_VERSION_ID>",
  "evaluation_spec_id": "<EVALUATION_SPEC_ID>",
  "input_set_ids": ["<CHALLENGE_INPUT_SET_ID>"],
  "bundle_artifact_id": "<BUNDLE_ARTIFACT_ID>"
}
```

`bundle_artifact_id` is optional. It is present when the backend stores the authored YAML bundle as a workspace artifact.

Without `--json`, the current CLI only prints the pack ID and version ID. Always use `--json` when you need `evaluation_spec_id`, `input_set_ids`, or `bundle_artifact_id`.

Publish stores:

- a workspace-scoped challenge pack, keyed by `pack.slug`;
- a runnable challenge pack version using `version.number`;
- one evaluation spec from `version.evaluation_spec`;
- one challenge input set row for each authored `input_sets[]` entry;
- one challenge input item row for each case;
- optionally, a `challenge_pack_bundle` artifact for the source YAML.

## Publish Failure Modes
- `challenge_pack_version_exists`: the same pack already has that `version.number`; increment `version.number` or intentionally target a different pack slug.
- `challenge_pack_metadata_conflict`: an existing pack with the same workspace and slug has a different `pack.name` or `pack.family`; keep metadata stable or choose a new slug.
- Billing or entitlement errors: the workspace cannot publish private challenge packs under its current plan.
- Authorization errors: the current caller cannot publish to the selected workspace.
- Validation errors: publish re-runs the same bundle and stored-artifact checks as validate.
- Oversized bundle errors: keep the YAML body at or below the current 1 MiB request limit.

## Next Eval Commands
Record the IDs from `publish --json` before starting a run.

Workflow-first command, with selectors accepted by the CLI:

```bash
agentclash eval start \
  --pack <PACK_ID_OR_SLUG_OR_EXACT_NAME> \
  --pack-version <VERSION_ID_OR_VERSION_NUMBER> \
  --input-set <INPUT_SET_ID_OR_KEY_OR_EXACT_NAME> \
  --deployment <DEPLOYMENT_ID_OR_EXACT_NAME> \
  --follow
```

Lower-level non-interactive command, using IDs:

```bash
agentclash run create \
  --challenge-pack-version <CHALLENGE_PACK_VERSION_ID> \
  --input-set <CHALLENGE_INPUT_SET_ID> \
  --deployments <AGENT_DEPLOYMENT_ID> \
  --follow
```

When the published version has multiple input sets, pass `--input-set` in non-interactive workflows. `agentclash eval start` can resolve an input set by ID, key, or exact name; `agentclash run create` expects the input set ID.

## Safety Notes
- Do not publish unless the user has already approved the workspace mutation.
- Do not include raw secret values in pack YAML, assets, examples, comments, or reports.
- `publish` does not upload local files referenced by `path`; use the artifact CLI first when a stored `artifact_id` is required.
- Prefer small smoke input sets before publishing large benchmark suites.
- Keep `pack.slug`, `pack.family`, challenge keys, input set keys, and case keys stable after publish so scorecards, regressions, and CI gates remain comparable.
- Avoid publishing customer-sensitive fixtures unless retention and workspace access are approved.

## Report Back Format
```text
Validation: <pass/fail>
Workspace: <workspace-id or configured default>
Pack file: <path>
Errors fixed:
- <field>: <message/fix>
Published: <yes/no>
Challenge pack ID: <id>
Challenge pack version ID: <id>
Evaluation spec ID: <id>
Input set IDs:
- <id> (<key/name if known>)
Bundle artifact ID: <id or not returned>
Next eval command:
Next run command:
Notes: <conflicts, entitlement/auth caveats, skipped publish reason>
```

## Related Skills
- `agentclash-cli-setup`
- `agentclash-challenge-pack-planner`
- `agentclash-challenge-pack-yaml-author`
- `agentclash-challenge-pack-input-sets`
- `agentclash-challenge-pack-tools-sandbox`
- `agentclash-challenge-pack-artifacts`
- `agentclash-challenge-pack-scoring-validators`
- `agentclash-challenge-pack-llm-judges`
- `agentclash-eval-runner`
- `agentclash-security-evaluation` — when the pack includes a `security` policy block

## Related Docs
- `/docs-md/agent-skills/challenge-pack-skills/agentclash-challenge-pack-yaml-author`
- `/docs-md/agent-skills/challenge-pack-skills/agentclash-challenge-pack-artifacts`
- `/docs-md/agent-skills/agentclash-eval-runner`
