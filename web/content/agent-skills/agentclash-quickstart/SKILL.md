---
name: agentclash-quickstart
description: Use when checking whether a workspace is ready to run AgentClash evals, interpreting quickstart readiness checks, or choosing the next CLI command after auth and workspace selection.
metadata:
  agentclash.role: onboarding
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Quickstart

## Purpose
Run `agentclash quickstart` to verify auth, workspace, challenge packs, deployments, and baseline bookmark state, then report the CLI's suggested next command for eval or comparison workflows.

## Use When
- A user asks "am I ready to run an eval?" or "what should I run next?"
- Auth and workspace are configured but pack/deployment readiness is unknown.
- An agent needs a single read-only command before proposing `eval start` or `compare latest`.

## Do Not Use When
- CLI is not installed or auth failed — use `agentclash-cli-setup` first.
- The user needs deep doctor diagnostics — use `agentclash doctor`.
- The task is to create packs, deployments, or runs — use the eval-runner or challenge-pack skills after quickstart passes.

## Inputs Needed
- API URL (default hosted production).
- Workspace ID or saved workspace from config / `AGENTCLASH_WORKSPACE`.
- Whether the user wants structured JSON output for automation.

## Environment
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash auth login --device
agentclash workspace use <WORKSPACE_ID>
```

## Procedure
1. Confirm auth with `agentclash auth status`.
2. Run `agentclash quickstart` (human) or `agentclash quickstart --json` (automation).
3. Read each check: `auth`, `workspace`, `challenge_packs`, `deployments`, `baseline`.
4. Execute `next_command` from the result when checks are blocking; treat `info` baseline as advisory.
5. When ready and baseline exists, quickstart suggests `agentclash eval start --follow` or comparison flows.

## Commands
```bash
agentclash quickstart
agentclash quickstart --json
```

Check names and statuses in structured output:

| Check | ok | todo | info |
| --- | --- | --- | --- |
| `auth` | Token valid | Login required | — |
| `workspace` | Workspace resolved | Set workspace | — |
| `challenge_packs` | ≥1 pack visible | Init/publish pack | — |
| `deployments` | ≥1 deployment | Create deployment | — |
| `baseline` | Bookmark set | — | No baseline yet |

Structured envelope fields:

- `ready` — `true` when no blocking `todo` checks remain (`ok` and `info` are non-blocking).
- `checks` — array of `{ name, status, detail, next_step?, metadata? }`.
- `next_command` — first blocking `next_step`, or eval/compare suggestion when ready.
- `next_steps` — ordered list including advisory baseline steps.

When baseline is set and workspace is ready, quickstart may suggest:

```bash
agentclash eval start --follow
agentclash compare latest --gate
```

## Expected Output
Human output prints a bold **AgentClash Quickstart** header, per-check lines, and **Next Command** when applicable.

JSON output prints the full envelope for scripting.

## Failure Modes
- `401` / auth errors → `agentclash auth login --device`.
- No workspace → `agentclash link` or `agentclash workspace use <id>`.
- Zero challenge packs → `agentclash challenge-pack init` then publish skills.
- Zero deployments → `agentclash deployment create --from-file deployment.json`.
- API unreachable → verify `AGENTCLASH_API_URL` and network.

## Safety Notes
- Quickstart is read-only; it does not create runs or spend provider budget.
- Do not paste tokens from `--json` output into chat.

## Report Back Format
```text
Quickstart ready: <yes/no>
Checks:
- auth: <status> — <detail>
- workspace: <status> — <detail>
- challenge_packs: <status> — <detail>
- deployments: <status> — <detail>
- baseline: <status> — <detail>
Next command: <command or none>
Next skill: <agentclash-eval-runner | agentclash-cli-setup | ...>
```

## Related Skills
- `agentclash-hub`
- `agentclash-cli-setup`
- `agentclash-eval-runner`
- `agentclash-compare-and-triage`

## Related Docs
- `/docs-md/getting-started/quickstart`
- `/docs-md/getting-started/first-eval`
- `/docs-md/reference/cli`
