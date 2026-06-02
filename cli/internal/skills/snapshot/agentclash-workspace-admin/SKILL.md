---
name: agentclash-workspace-admin
description: Use when creating or administering AgentClash organizations and workspaces, inviting members, updating roles, or binding default workspace context beyond basic CLI login.
metadata:
  agentclash.role: admin
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Workspace Admin

## Purpose
Administer tenancy: organizations, workspaces, memberships, and default workspace binding for teams operating multiple eval environments.

## Use When
- Creating a new workspace for a team or environment (staging vs prod eval).
- Listing orgs/workspaces after login to pick the right target.
- Inviting teammates or changing workspace/org membership roles.
- Enabling `public_packs` on a workspace or archiving inactive workspaces.
- Setting default workspace context after auth (when `agentclash link` is not enough).

## Do Not Use When
- First-time CLI install and device login — use `agentclash-cli-setup` and `agentclash link`.
- Running evals in an already-selected workspace — use `agentclash-eval-runner`.
- Managing provider secrets, deployments, or runtime resources — use `agentclash-runtime-resources-setup`.

## Inputs Needed
- Authenticated CLI session (`agentclash auth login --device` or `AGENTCLASH_TOKEN`).
- Organization ID for workspace create/list (from `agentclash org list` or saved config).
- Workspace ID for member admin commands (from `agentclash workspace list` or `agentclash link`).
- Email addresses and roles for invites.

## Environment
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash auth login --device
agentclash org list
agentclash workspace list --org <organization-id>
export AGENTCLASH_WORKSPACE="<workspace-id>"
```

Role values:
- Org: `org_admin`, `org_member`
- Workspace: `workspace_admin`, `workspace_member`, `workspace_viewer`

## Procedure
1. Verify auth with `agentclash auth status`.
2. List organizations; create one if needed.
3. List or create workspaces under the org.
4. Set default workspace with `workspace use` or guided `link`.
5. Optionally bind project config with `agentclash init --workspace-id ... --org-id ...`.
6. Invite members and adjust roles/status as the team grows.
7. Run `agentclash doctor` in the target workspace to confirm eval readiness.

## Commands

### Organizations
```bash
agentclash org list
agentclash org get <org-id>
agentclash org create --name "Acme Eval" --slug acme-eval
agentclash org update <org-id> --name "Acme Eval Platform"
agentclash org members list --org <org-id>
agentclash org members invite --org <org-id> --email user@example.com --role org_member
agentclash org members update <membership-id> --role org_admin
```

Alias: `agentclash organization` = `agentclash org`.

### Workspaces
```bash
agentclash workspace list --org <org-id>
agentclash workspace get <workspace-id>
agentclash workspace create --org <org-id> --name "Staging" --slug staging
agentclash workspace update <workspace-id> --name "Staging Eval" --public-packs
agentclash workspace update <workspace-id> --status archived
agentclash workspace use <workspace-id>
agentclash link
```

Alias: `agentclash ws` = `agentclash workspace`.

`workspace use` validates access and saves `default_workspace` (and `default_org` when present) to user config.

Project-local binding:
```bash
agentclash init --workspace-id <workspace-id> --org-id <organization-id>
```

### Workspace members
```bash
agentclash workspace members list
agentclash workspace members invite --email user@example.com --role workspace_member
agentclash workspace members update <membership-id> --role workspace_admin
agentclash workspace members update <membership-id> --status suspended
```

Member commands require a resolved workspace (`--workspace`, `AGENTCLASH_WORKSPACE`, saved config, or project `.agentclash.yaml`).

## Expected Output
- `workspace create` prints workspace ID and name.
- `workspace use` / `link` saves defaults and confirms the selected workspace.
- `members invite` confirms email and role.
- `--json` on any command returns structured API payloads.

## Failure Modes
- `organization ID required` on `workspace list` → pass `--org` or set default org via `link` / config.
- `no workspace specified` on member commands → run `workspace use` or export `AGENTCLASH_WORKSPACE`.
- `Workspace <id> is not accessible` → token lacks membership; re-auth or pick another workspace.
- `no fields to update` on update commands → pass at least one changed flag.

## Safety Notes
- Confirm workspace ID before invites or archival — operations affect the whole team.
- Prefer `workspace_viewer` for read-only stakeholders; escalate to `workspace_admin` deliberately.
- Do not share `AGENTCLASH_TOKEN` in tickets or chat; use per-user device login when possible.

## Report Back Format
```text
Org: <id> (<name>)
Workspace: <id> (<name>, <status>)
Default workspace: <saved id or none>
Members: <count or n/a>
Public packs: <enabled/disabled>
Doctor ready: <yes/no>
Next: agentclash quickstart OR agentclash eval start
```

## Related Skills
- `agentclash-hub`
- `agentclash-cli-setup`
- `agentclash-quickstart`
- `agentclash-runtime-resources-setup`
- `agentclash-eval-runner`

## Related Docs
- `/docs-md/getting-started/quickstart`
- `/docs-md/reference/cli`
- `/docs-md/reference/config`
