---
name: agentclash-cli-setup
description: Use when configuring the AgentClash CLI, authenticating, selecting a workspace, linking a project, or diagnosing CLI access against staging, production, or local backends.
metadata:
  agentclash.role: setup
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash CLI Setup

## Purpose
Configure a local AgentClash CLI session that can talk to the intended backend and workspace.

## Use When
- A user asks to install, authenticate, or verify the AgentClash CLI.
- A user needs to switch workspaces or connect a repo to a workspace.
- A CLI command fails because the API URL, token, or workspace context is unclear.

## Do Not Use When
- The task is only to read scorecards or replay evidence from an already configured CLI.
- The user is asking for production release automation rather than setup.

## Inputs Needed
- Target backend: staging, production, or local.
- Workspace ID or enough context to choose one from `workspace list`.
- Whether browser-based device auth is acceptable.

## Environment
Use staging unless the user explicitly asks for production:

```bash
export AGENTCLASH_API_URL="https://staging-api.agentclash.dev"
```

Resolution order for the API base URL:

```text
--api-url > AGENTCLASH_API_URL > saved user config > http://localhost:8080
```

## Procedure
1. Confirm the backend target.
2. Authenticate with device login unless a token is already provided.
3. List workspaces and select the intended workspace.
4. Link the current project when the workflow benefits from repo-local workspace context.
5. Run `doctor` and report the effective API URL, workspace, and any warnings.

## Commands
```bash
export AGENTCLASH_API_URL="https://staging-api.agentclash.dev"
agentclash auth login --device
agentclash workspace list
agentclash workspace use <workspace-id>
agentclash link --workspace <workspace-id>
agentclash doctor
```

For local backend work:

```bash
agentclash --api-url http://localhost:8080 doctor
```

## Expected Output
- Auth succeeds or prints a device verification URL.
- `workspace list` shows accessible workspaces.
- `doctor` reports the resolved API URL and workspace context.

## Failure Modes
- `401` or `403`: token is missing, expired, or does not belong to the selected workspace.
- Empty workspace list: wrong account or backend.
- Commands unexpectedly hit localhost: `AGENTCLASH_API_URL` and saved config are missing.

## Safety Notes
- Do not print tokens in chat or commit them to files.
- Prefer staging for exploratory work.
- Ask before changing a user's saved production config.

## Report Back Format
```text
Backend: <url>
Workspace: <workspace-id or none>
Auth: <ok or action needed>
Doctor: <summary>
Next command: <command>
```

## Related Docs
- `/docs-md/getting-started/quickstart`
- `/docs-md/reference/cli`
- `/docs-md/reference/config`
