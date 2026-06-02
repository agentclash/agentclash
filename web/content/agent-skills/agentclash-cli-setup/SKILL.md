---
name: agentclash-cli-setup
description: Use when configuring the AgentClash CLI, authenticating with device login or tokens, selecting a workspace, saving default config with link, creating project config with init, resolving API URL precedence, or diagnosing CLI access against production, local, or self-hosted backends.
metadata:
  agentclash.role: setup
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash CLI Setup

## Purpose
Configure an AgentClash CLI session that can reach the intended backend, authenticate safely, resolve the right workspace, and pass `agentclash doctor` before a user starts authoring packs or running evals.

## Use When
- A user asks to install, authenticate, verify, or repair AgentClash CLI access.
- A coding agent needs AgentClash access but does not have the AgentClash source repo.
- A user needs to switch workspaces, save a default workspace, or create project-local `.agentclash.yaml` config.
- A CLI command fails because API URL, token, organization, workspace, or saved config precedence is unclear.
- CI needs a non-interactive setup path with `AGENTCLASH_TOKEN`.

## Do Not Use When
- The CLI is already configured and the task is to run an eval or inspect a scorecard.
- The user needs to author challenge pack YAML; use `agentclash-challenge-pack-yaml-author` after setup passes.
- The user is making a release decision or CI gate from completed runs; use `agentclash-ci-release-gate`.
- The task requires changing AgentClash CLI source code rather than using the CLI.

## Inputs Needed
- Backend target: hosted production, local development, or self-hosted.
- Whether the CLI is installed as `agentclash` or run from source with `go run .` inside `cli/`.
- Whether browser/device login is acceptable, or whether `AGENTCLASH_TOKEN` must be used.
- Organization ID if `workspace list` cannot infer one from saved config.
- Workspace ID, workspace slug/name, or permission to choose one interactively with `agentclash link`.
- Whether the command should mutate saved user config under `~/.config/agentclash/config.yaml`.
- Whether the current repository should get project-local `.agentclash.yaml` config with `agentclash init`.

## Environment
Use hosted production by default:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

Use a local backend only when the user is explicitly running the API server locally:

```bash
export AGENTCLASH_API_URL="http://localhost:8080"
```

For CI or other non-interactive shells, provide a token through the environment instead of running browser login:

```bash
export AGENTCLASH_TOKEN="<token>"
export AGENTCLASH_WORKSPACE="<workspace-id>"
```

Do not print token values in chat, logs, docs, or committed files.

## Config And Precedence
API URL resolution:

```text
--api-url > AGENTCLASH_API_URL > saved user config > default
```

The source-build default is `http://localhost:8080`. Released CLI builds stamp the production default at release time, but skills should still set `AGENTCLASH_API_URL="https://api.agentclash.dev"` explicitly for hosted workflows so copied commands behave the same in source and released builds.

Workspace resolution:

```text
--workspace / -w > AGENTCLASH_WORKSPACE > project .agentclash.yaml > saved user config
```

Organization resolution for commands that need an org:

```text
AGENTCLASH_ORG > project .agentclash.yaml > saved user config
```

Auth token resolution:

```text
AGENTCLASH_TOKEN > stored CLI credentials
```

Saved user config lives at:

```text
~/.config/agentclash/config.yaml
```

Project-local config lives in `.agentclash.yaml` and is discovered by walking up from the current directory. It can store `workspace_id` and `org_id`.

## Procedure
1. Choose the backend. For normal hosted work, export `AGENTCLASH_API_URL="https://api.agentclash.dev"` first.
2. Verify the CLI is callable with `agentclash version` or, from the repo `cli/` directory, `go run . version`.
3. Authenticate. Prefer `agentclash auth login --device` for remote shells; use `AGENTCLASH_TOKEN` for CI.
4. Check the authenticated identity with `agentclash auth status`.
5. Select a workspace with `agentclash link` for the guided flow, or `agentclash workspace use <workspace-id>` when the ID is already known. Both commands write saved user config.
6. If this repository should carry its own workspace binding, run `agentclash init --workspace-id <workspace-id> --org-id <organization-id>` from the project root.
7. Run `agentclash doctor` to verify install, auth, workspace, challenge-pack visibility, deployment visibility, and baseline readiness.
8. Report the effective backend, auth state, workspace, doctor result, and the next command the user should run.

## Commands
Hosted production, interactive setup:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash version
agentclash auth login --device
agentclash auth status
agentclash link
agentclash doctor
```

Hosted production when the workspace ID is already known:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash auth login --device
agentclash workspace use <workspace-id>
agentclash doctor
```

Workspace discovery when an organization must be explicit:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash workspace list --org <organization-id>
agentclash workspace get <workspace-id>
agentclash workspace use <workspace-id>
```

Project-local config for a repository:

```bash
agentclash init --workspace-id <workspace-id> --org-id <organization-id>
```

CI or non-interactive setup:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
export AGENTCLASH_TOKEN="<token>"
export AGENTCLASH_WORKSPACE="<workspace-id>"
agentclash doctor --json
```

Local development from the CLI module:

```bash
cd cli
export AGENTCLASH_API_URL="http://localhost:8080"
go run . auth login --device
go run . link
go run . doctor
```

One-off backend override without changing saved config:

```bash
agentclash --api-url http://localhost:8080 doctor
```

## Expected Output
- `auth login --device` prints a verification URL or confirms an existing valid login.
- `auth status` shows the authenticated user and accessible organization/workspace counts.
- `link` saves the selected workspace and organization in user config and prints the next suggested command. It does not write `.agentclash.yaml`.
- `workspace use <workspace-id>` validates access and saves `default_workspace`; it also saves `default_org` when the workspace details include an organization ID.
- `init --workspace-id <workspace-id> --org-id <organization-id>` writes project-local `.agentclash.yaml` in the current directory.
- `doctor` prints the effective API URL, workspace, setup checks, and suggested next steps. In JSON mode, `ready: true` means no `warn` or `fail` checks remain.

## Failure Modes
- `AGENTCLASH_TOKEN is set but could not be validated`: the environment token is present and takes precedence, but it is wrong for this backend. Unset or replace `AGENTCLASH_TOKEN`, then rerun `agentclash auth login --device` or `agentclash auth status`.
- `Not logged in. No API token is configured.`: run `agentclash auth login --device` for interactive work, or set `AGENTCLASH_TOKEN` for CI.
- Commands unexpectedly hit `http://localhost:8080`: set `AGENTCLASH_API_URL="https://api.agentclash.dev"` or pass `--api-url`; source builds default to localhost.
- `organization ID required`: pass `--org <organization-id>` or set a default organization through `agentclash link`, `agentclash workspace use`, or config.
- `no workspace specified`: run `agentclash link`, pass `--workspace <workspace-id>`, set `AGENTCLASH_WORKSPACE`, or create project config with `agentclash init`.
- `Workspace <id> is not accessible`: the saved or env workspace does not belong to the current token/backend. Run `agentclash link` or update `AGENTCLASH_WORKSPACE`.
- `doctor` reports no challenge packs: setup is valid, but the workspace needs `agentclash challenge-pack init`, `validate`, and `publish` before evals are useful.
- `doctor` reports no deployments: setup is valid, but an agent deployment must be created before starting evals.
- `doctor` reports no baseline: this is advisory on a fresh workspace; set one after the first completed eval with `agentclash baseline set`.

## Safety Notes
- Never paste, print, or commit `AGENTCLASH_TOKEN` or provider secrets.
- Ask before changing saved user config when the user is sharing a machine or switching production workspaces.
- Prefer `--api-url` for one-off local/self-hosted checks so saved production config is not accidentally changed.
- Prefer `doctor --json` in automation because it returns a machine-readable `ready` field and exits non-zero when setup warnings remain.
- Run read-only commands (`auth status`, `workspace list`, `doctor`) before write or publish commands.

## Report Back Format
```text
Backend: <effective API URL>
Auth: <ok | action needed> (<source: AGENTCLASH_TOKEN | stored credentials | none>)
Workspace: <workspace-id or none>
Doctor: <ready | warnings> - <short check summary>
Next command: <single recommended command>
Notes: <config precedence, local override, or token caveat if relevant>
```

## Related Skills
- `agentclash-hub` — load first for full workflow map and UI links
- `agentclash-quickstart` — readiness checks after auth
- `agentclash-eval-runner`

## Related Docs
- `/docs-md/getting-started/quickstart`
- `/docs-md/guides/use-with-ai-tools`
- `/docs-md/reference/cli`
- `/docs-md/reference/config`
- `/docs-md/agent-skills`
