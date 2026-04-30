---
name: agentclash-eval-runner
description: Use when starting, following, or reporting AgentClash runs and evals with the CLI, especially run create, eval start, live events, rankings, and suite-only scopes.
metadata:
  agentclash.role: running
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Eval Runner

## Purpose
Start an AgentClash run or eval, follow it to a useful stopping point, and report the evidence a reviewer needs.

## Use When
- A user asks to run agents against a challenge pack, input set, or regression suite.
- A user wants live run progress, rankings, or final status.
- A CI or local workflow needs a command that can be repeated.

## Do Not Use When
- The task is to design a new challenge pack from scratch.
- The task is to make a release decision from an already completed comparison.

## Inputs Needed
- Workspace ID.
- Challenge pack ID or regression suite ID.
- Deployment IDs or agent targets.
- Input set, scope, and whether the run should be followed live.

## Environment
Use production by default; only override for local or self-hosted work:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

## Procedure
1. Verify CLI auth and workspace context.
2. Resolve challenge pack, deployments, and input set before creating the run.
3. Prefer `--follow` for interactive work so failures are visible immediately.
4. Capture run ID, status, rankings, and follow-up inspection commands.
5. If the run fails, collect `run events` or `run failures` before proposing fixes.

## Commands
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash workspace use <workspace-id>
agentclash run create --follow
agentclash run get <run-id>
agentclash run events <run-id>
agentclash run ranking <run-id>
```

For regression-only verification:

```bash
agentclash run create --scope suite_only --follow
```

## Expected Output
- A run ID is created.
- Live events show agent progress or terminal failure.
- Rankings show participant order once scoring completes.

## Failure Modes
- Missing deployment IDs: list deployments and rerun with explicit selections.
- Pack has no compatible input set: inspect pack details before retrying.
- Follow stream disconnects: use `run get`, `run events`, and `run ranking` with the run ID.

## Safety Notes
- Confirm before creating expensive or production-scale runs.
- Prefer small input sets for smoke checks.
- Do not paste secrets from event logs into chat.

## Report Back Format
```text
Run: <run-id>
Status: <status>
Deployments: <ids>
Ranking: <summary or unavailable>
Evidence commands: <commands>
Next action: <recommendation>
```

## Related Docs
- `/docs-md/getting-started/first-eval`
- `/docs-md/concepts/runs-and-evals`
- `/docs-md/reference/cli`
