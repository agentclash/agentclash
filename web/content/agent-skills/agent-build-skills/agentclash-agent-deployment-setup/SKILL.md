---
name: agentclash-agent-deployment-setup
description: Use when creating, selecting, or diagnosing AgentClash agent deployments for runs, including ready build versions, runtime profiles, provider/model wiring, deployment IDs, workspace context, and run compatibility.
metadata:
  agentclash.role: agent-deployments
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Agent Deployment Setup

## Purpose
Turn a ready AgentClash build version plus runtime/provider/model resources into an active deployment ID that runs and eval sessions can select.

## Use When
- A user has a ready build version and runtime resources, but no deployment ID yet.
- A run or eval flow needs `agent_deployment_ids`.
- A deployment create request is failing because build version readiness, provider account, model alias, runtime profile, or workspace context is wrong.
- A coding agent needs to audit whether an existing deployment is runnable before starting `agentclash run create` or `agentclash eval start`.

## Do Not Use When
- The CLI is not authenticated or no workspace is selected; use `agentclash-cli-setup` first.
- Provider accounts, model aliases, runtime profiles, workspace secrets, or workspace tools are missing; use `agentclash-runtime-resources-setup` first.
- The build version is still a draft or has validation errors; use `agentclash-agent-build-author` first.
- The user is authoring challenge pack YAML or choosing input sets; use challenge-pack skills.

## Inputs Needed
- Workspace ID and confirmation that `agentclash doctor` can reach it.
- Deployment name.
- `agent_build_id` and ready `build_version_id`.
- `runtime_profile_id`.
- `provider_account_id`.
- `model_alias_id`, or a raw provider model string such as `gpt-4.1` when creating from JSON so the backend can auto-create/reuse a model alias.
- Optional `deployment_config` JSON object.
- Challenge pack or run requirements that affect runtime profile, tool, network, shell, or model compatibility.

## Environment
Use hosted production by default:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash doctor
```

Commands that list or create deployments need a resolved workspace. Use `agentclash workspace use <workspace-id>`, `--workspace`, `AGENTCLASH_WORKSPACE`, or project config from `agentclash init`.

## Procedure
1. Verify CLI auth and workspace context with `agentclash doctor`.
2. Inspect the build and build version with `agentclash build get <BUILD_ID>` and `agentclash build version get <BUILD_VERSION_ID> --json`. Stop if `version_status` is not `ready`.
3. Confirm runtime resources exist: `agentclash infra runtime-profile get <RUNTIME_PROFILE_ID>`, `agentclash infra provider-account get <PROVIDER_ACCOUNT_ID>`, and either `agentclash infra model-alias get <MODEL_ALIAS_ID>` or a raw model value for JSON-file creation.
4. List existing deployments with `agentclash deployment list --json` to avoid duplicates.
5. Create the deployment with flags when you already have a model alias ID, or with `--from-file` when you need `deployment_config` or raw `model` auto-alias behavior.
6. Re-list deployments and record the created deployment ID, status, and `current_build_version_id`.
7. Check run compatibility by confirming the deployment is active and can be passed to `agentclash run create --deployments <DEPLOYMENT_ID>`.
8. Report deployment IDs and any blockers for `agentclash-eval-runner`.

## Deployment Contract
Flag-based creation requires these CLI flags when `--from-file` is not used:

```bash
agentclash deployment create \
  --name support-bot-prod \
  --agent-build-id <AGENT_BUILD_ID> \
  --build-version-id <BUILD_VERSION_ID> \
  --runtime-profile-id <RUNTIME_PROFILE_ID> \
  --provider-account-id <PROVIDER_ACCOUNT_ID> \
  --model-alias-id <MODEL_ALIAS_ID>
```

The CLI requires `--name`, `--agent-build-id`, `--build-version-id`, and `--runtime-profile-id` before it sends a flag-based request. The backend then requires `provider_account_id` and either `model_alias_id` or `model`.

Use JSON when you need the complete API shape, including `deployment_config` or raw model auto-aliasing:

```json
{
  "name": "support-bot-prod",
  "agent_build_id": "<AGENT_BUILD_ID>",
  "build_version_id": "<BUILD_VERSION_ID>",
  "runtime_profile_id": "<RUNTIME_PROFILE_ID>",
  "provider_account_id": "<PROVIDER_ACCOUNT_ID>",
  "model_alias_id": "<MODEL_ALIAS_ID>",
  "deployment_config": {}
}
```

Raw model auto-alias path:

```json
{
  "name": "support-bot-prod",
  "agent_build_id": "<AGENT_BUILD_ID>",
  "build_version_id": "<BUILD_VERSION_ID>",
  "runtime_profile_id": "<RUNTIME_PROFILE_ID>",
  "provider_account_id": "<PROVIDER_ACCOUNT_ID>",
  "model": "gpt-4.1",
  "deployment_config": {}
}
```

When `provider_account_id` plus `model` is supplied and `model_alias_id` is omitted, the backend looks up the provider account, upserts a model catalog entry, and reuses/unarchives/creates a model alias.

## Commands
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash doctor

agentclash build get <BUILD_ID>
agentclash build version get <BUILD_VERSION_ID> --json

agentclash infra runtime-profile get <RUNTIME_PROFILE_ID>
agentclash infra provider-account get <PROVIDER_ACCOUNT_ID>
agentclash infra model-alias get <MODEL_ALIAS_ID>

agentclash deployment list --json
agentclash deployment create --help
agentclash deployment create --from-file deployment.json
agentclash deploy list
agentclash deploy create --from-file deployment.json

agentclash run create \
  --challenge-pack-version <CHALLENGE_PACK_VERSION_ID> \
  --deployments <DEPLOYMENT_ID>
```

## Expected Output
- `deployment list --json` returns `items` with deployment `id`, `name`, `status`, `current_build_version_id`, and timestamps.
- `deployment create` returns a created deployment with `id`, `workspace_id`, `agent_build_id`, `current_build_version_id`, `name`, `slug`, `deployment_type`, `status`, `created_at`, and `updated_at`.
- Human output for deployment creation prints `Created deployment <name> (<id>)`.
- The deployment ID can be passed to run creation as `agent_deployment_ids` through `agentclash run create --deployments <DEPLOYMENT_ID>`.

## Run Compatibility Checks
- The deployment should be active in `agentclash deployment list --json`.
- The deployment should point to the intended ready build version via `current_build_version_id`.
- Runtime profile settings should satisfy the challenge pack: shell, network, timeout, max iterations, and max tool calls.
- Provider account and model alias should match the build's `model_spec` expectations.
- Run creation requires at least one deployment. Non-interactive runs use `--deployments`; TTY runs can prompt from active deployments.
- Backend run creation requires deployment IDs to reference active deployments with snapshots in the selected workspace.

## Failure Modes
- `no workspace specified`: run `agentclash link`, pass `--workspace`, set `AGENTCLASH_WORKSPACE`, or add project config with `agentclash init`.
- `missing required flags when --from-file is not used`: pass `--name`, `--agent-build-id`, `--build-version-id`, and `--runtime-profile-id`, or use `--from-file`.
- `only ready versions can be deployed`: return to `agentclash-agent-build-author`, validate the version, and mark it ready.
- `provider_account_id is required`: create/select a provider account with `agentclash-runtime-resources-setup`.
- `either model_alias_id or model ... is required`: pass `--model-alias-id` with flags, or use `--from-file` with either `model_alias_id` or raw `model`.
- `*_id must be a valid UUID`: copy IDs from `build get`, `build version get --json`, `infra ... get`, or `deployment list --json`.
- Run creation rejects the deployment: confirm it is active, belongs to the selected workspace, and has a snapshot.

## Safety Notes
- Do not put raw provider API keys or tokens in `deployment_config`; use workspace secrets and provider accounts.
- Treat deployment creation as production-affecting when `AGENTCLASH_API_URL` points at hosted production.
- Use `list` and `get` commands before creating resources to avoid duplicate deployments.
- Do not invent IDs. If a required ID is missing, run the upstream runtime resources or build authoring skill first.
- Prefer `--json` for machine-readable checks by coding agents.

## Report Back Format
```text
Workspace: <workspace-id>
Deployment: <name> (<deployment_id>)
Status: <active | paused | archived | unknown>
Build: <agent_build_id>
Build version: <build_version_id> (<ready | blocked>)
Runtime profile: <runtime_profile_id>
Provider account: <provider_account_id>
Model: <model_alias_id | raw model auto-alias>
Run compatibility: <ready | blocked>
Next skill: agentclash-eval-runner
Notes: <runtime/model/provider/challenge-pack caveats>
```

## Related Skills
- `agentclash-cli-setup`
- `agentclash-runtime-resources-setup`
- `agentclash-agent-build-author`
- `agentclash-eval-runner`

## Related Docs
- `/docs-md/concepts/agents-and-deployments`
- `/docs-md/guides/configure-runtime-resources`
- `/docs-md/reference/cli`
- `/docs-md/reference/config`
