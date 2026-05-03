---
name: agentclash-runtime-resources-setup
description: Use when configuring AgentClash workspace secrets, provider accounts, model catalog entries, model aliases, runtime profiles, workspace tools, and readiness checks required before agent builds, deployments, evals, or runs.
metadata:
  agentclash.role: runtime-resources
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Runtime Resources Setup

## Purpose
Prepare the workspace infrastructure chain that lets AgentClash turn a ready agent build version into a runnable deployment: secrets, provider accounts, model aliases, runtime profiles, optional workspace tools, and readiness checks.

## Use When
- A deployment cannot run because provider accounts, model aliases, workspace secrets, runtime profiles, or workspace tools are missing.
- A user has a ready agent build/version but does not yet have the runtime resources needed to deploy it.
- A challenge pack or deployment references a secret, tool, model alias, provider account, or runtime profile that does not exist in the selected workspace.
- A coding agent needs a checklist before moving from CLI setup to agent build/deployment setup.

## Do Not Use When
- The CLI is not authenticated or no workspace is selected; use `agentclash-cli-setup` first.
- The user is authoring challenge pack YAML fields; use challenge-pack skills after workspace resources are clear.
- The user is creating the agent build itself; use `agentclash-agent-build-author`.
- The user already has resource IDs and only needs to create/select a deployment; use `agentclash-agent-deployment-setup`.

## Inputs Needed
- Workspace ID and confirmation that `agentclash doctor` can reach it.
- Provider key, such as `openai`, and the credential secret key name.
- Desired model catalog entry ID or enough criteria to pick one from the model catalog.
- Runtime profile requirements: execution target, trace mode, iteration/tool limits, timeouts, and sandbox/network policy.
- Model alias key/display name and, when needed, the provider account it should bind to.
- Optional workspace tool names, tool kinds, and JSON specs.
- Whether the user wants to create resources now or only audit readiness.

## Environment
Use hosted production by default:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

Commands that create or list workspace resources also need a resolved workspace:

```bash
agentclash doctor
agentclash secret list
```

If workspace resolution fails, run the CLI setup skill first and do not create resources yet.

## Resource Order
1. Verify CLI auth and workspace context.
2. Store provider credentials as workspace secrets.
3. Inspect the global model catalog and choose a model catalog entry.
4. Create a provider account that references a workspace secret.
5. Create a runtime profile with execution and sandbox limits.
6. Create a model alias that points at a catalog entry and optionally binds to a provider account.
7. Create optional workspace tools if deployments or packs expect reusable workspace tools.
8. List resources and record IDs for agent build/deployment setup.

## Procedure
1. Run `agentclash doctor` and stop on auth or workspace warnings.
2. Run `agentclash secret list` to see which secret keys already exist. If a secret value is not already available in the user's shell, ask the user to set it themselves with `agentclash secret set <KEY>`; do not request or receive the value in chat.
3. Run `agentclash infra model-catalog list` and `get` the candidate model entry before creating aliases.
4. Create or select the provider account. Prefer `credential_reference: "workspace-secret://KEY"` over putting raw keys in JSON files.
5. Create or select a runtime profile. Keep limits explicit: iterations, tool calls, step timeout, run timeout, and sandbox/network policy.
6. Create or select a model alias that points to the model catalog entry and, when needed, the provider account.
7. Create workspace tools only when the deployment or product workflow needs reusable workspace tool resources. Keep these separate from pack-defined composed tools.
8. Re-list all resources and report the IDs downstream skills need.

## Commands
Verify setup and workspace:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash doctor
```

Store provider credentials as workspace secrets:

```bash
printf '%s' "$OPENAI_API_KEY" | agentclash secret set OPENAI_API_KEY
agentclash secret list
```

Inspect the global model catalog:

```bash
agentclash infra model-catalog list
agentclash infra model-catalog get <MODEL_CATALOG_ENTRY_ID>
```

Create a provider account from a JSON file:

```json
{
  "provider_key": "openai",
  "name": "OpenAI Workspace Account",
  "credential_reference": "workspace-secret://OPENAI_API_KEY",
  "limits_config": {
    "rpm": 60
  }
}
```

```bash
agentclash infra provider-account create --from-file provider-account.json
agentclash infra provider-account list
agentclash infra provider-account get <PROVIDER_ACCOUNT_ID>
```

Create a runtime profile:

```json
{
  "name": "default-native",
  "execution_target": "native",
  "trace_mode": "full",
  "max_iterations": 24,
  "max_tool_calls": 32,
  "step_timeout_seconds": 120,
  "run_timeout_seconds": 1800,
  "profile_config": {
    "sandbox": {
      "allow_shell": true,
      "allow_network": false
    }
  }
}
```

```bash
agentclash infra runtime-profile create --from-file runtime-profile.json
agentclash infra runtime-profile list
agentclash infra runtime-profile get <RUNTIME_PROFILE_ID>
```

Create a model alias:

```json
{
  "alias_key": "primary-chat",
  "display_name": "Primary Chat Model",
  "model_catalog_entry_id": "<MODEL_CATALOG_ENTRY_ID>",
  "provider_account_id": "<PROVIDER_ACCOUNT_ID>"
}
```

```bash
agentclash infra model-alias create --from-file model-alias.json
agentclash infra model-alias list
agentclash infra model-alias get <MODEL_ALIAS_ID>
```

Optional workspace tools:

`tool.json`:

```json
{
  "name": "inventory-api",
  "tool_kind": "http",
  "capability_key": "inventory.lookup",
  "definition": {}
}
```

```bash
agentclash infra tool list
agentclash infra tool create --from-file tool.json
agentclash infra tool get <TOOL_ID>
```

Archive or delete only with explicit user confirmation:

```bash
agentclash infra runtime-profile archive <RUNTIME_PROFILE_ID>
agentclash infra model-alias delete <MODEL_ALIAS_ID>
agentclash infra provider-account delete <PROVIDER_ACCOUNT_ID>
agentclash secret delete <SECRET_KEY>
```

## Expected Output
- `secret list` shows secret keys and timestamps, never secret values.
- `model-catalog list` returns global model entries with provider, model, family, and lifecycle status.
- `provider-account list` shows provider key, account name, status, and ID.
- `runtime-profile list` shows execution target, max iterations, and ID.
- `model-alias list` shows alias key, display name, status, and ID.
- `infra tool list` shows workspace tool name, kind, lifecycle status, and ID.
- The final handoff contains the provider account ID, runtime profile ID, model alias ID, relevant secret key names, and optional tool IDs.

## Failure Modes
- `no workspace specified`: run `agentclash link`, pass `--workspace`, set `AGENTCLASH_WORKSPACE`, or add project config with `agentclash init`.
- Provider account creation fails because the secret is missing: run `agentclash secret list`, then set the expected key and use `workspace-secret://KEY`.
- A raw `api_key` was passed and cannot be read back: expected behavior; the infrastructure manager stores it as a workspace secret named `PROVIDER_<PROVIDER_KEY>_API_KEY` and keeps only `workspace-secret://PROVIDER_<PROVIDER_KEY>_API_KEY` on the provider account. The provider key is uppercased and hyphens become underscores, so `x-ai` becomes `PROVIDER_X_AI_API_KEY`.
- Model alias creation fails because the model catalog entry is wrong or unavailable: inspect with `agentclash infra model-catalog get <MODEL_CATALOG_ENTRY_ID>` and choose an active entry for the intended provider.
- Deployment setup later fails because no runtime profile exists: create or select a runtime profile and pass its ID into deployment setup.
- Runs fail because network, shell, timeout, or tool-call limits are too strict: review `profile_config`, `max_iterations`, `max_tool_calls`, `step_timeout_seconds`, and `run_timeout_seconds`.
- Workspace tools are confused with pack-defined tools: workspace tools are `agentclash infra tool ...` resources; pack-defined tools live inside challenge pack YAML.

## Safety Notes
- Never print, paste, request, receive, or commit raw provider keys. Prefer stdin for `secret set`; if the value is not already in the user's shell, ask the user to run the command themselves.
- Prefer `credential_reference: "workspace-secret://KEY"` in provider account specs.
- Treat `delete` and `archive` commands as destructive enough to require explicit user confirmation.
- Use `list` and `get` before `create`, `delete`, or `archive` to avoid duplicating or mutating the wrong workspace resource.
- Keep local/self-hosted API URLs explicit; hosted examples should use `https://api.agentclash.dev`.

## Report Back Format
```text
Workspace: <workspace-id>
Secrets: <KEY present | KEY missing>
Provider account: <id or action needed>
Model catalog entry: <id and provider/model>
Runtime profile: <id or action needed>
Model alias: <id or action needed>
Workspace tools: <ids or none>
Readiness: <ready for deployment setup | blocked>
Next skill: agentclash-agent-build-author | agentclash-agent-deployment-setup
Notes: <credential, limit, sandbox, or tool caveats>
```

## Related Skills
- `agentclash-cli-setup`
- `agentclash-agent-build-author`
- `agentclash-agent-deployment-setup`
- `agentclash-challenge-pack-tools-sandbox`

## Related Docs
- `/docs-md/guides/configure-runtime-resources`
- `/docs-md/concepts/agents-and-deployments`
- `/docs-md/concepts/tools-network-and-secrets`
- `/docs-md/reference/cli`
- `/docs-md/reference/config`
