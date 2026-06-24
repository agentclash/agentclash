---
name: agentclash-agent-harness-setup
description: Use when creating, running, or ranking Agent Harness coding-agent tasks via the CLI, including harness specs, E2B runner kinds, suite task banks, executions, failure review, and promote-to-task flows.
metadata:
  agentclash.role: harness
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Agent Harness Setup

## Purpose
Configure and operate Agent Harnesses — workspace-scoped autonomous coding tasks with E2B runners, evaluation config, suite rankings, and failure curation. Agent Harnesses are not challenge packs.

## Use When
- A user wants long-running coding-agent checks against a repository with validators or LLM judges.
- You need to create harnesses for Codex, Claude, Hermes, or OpenClaw runners on E2B.
- The workflow involves suite task banks, multi-harness suite runs, or promoting failed executions into private tasks.

## Do Not Use When
- The workload is a standard challenge-pack eval — use `agentclash-eval-runner`.
- The user only needs deployments or runtime resources — use agent-build skills first.
- The task is prompt A/B testing without a repo harness — use `agentclash-prompt-eval-playground`.

## Inputs Needed
- Workspace with provider secrets configured (`openai_api_key_secret_name` or `--api-key-secret`).
- Task prompt, repository URL, and harness kind.
- Optional evaluation config JSON (validators, LLM judges).
- For suite runs: suite ID, harness IDs, optional task filters.

## Environment
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash workspace use <WORKSPACE_ID>
agentclash secret list --json
```

## Procedure
1. List or inspect existing harnesses.
2. Create a harness with `--name`, `--task`, `--auth-mode`, and API key secret.
3. Run a harness execution; use `--follow` to poll to terminal status.
4. Inspect executions, failure summaries, and failure reviews.
5. Optionally create suites, run across harnesses, read rankings, promote executions to private tasks.

## Commands

### Harness CRUD and runs
```bash
agentclash agent-harness list
agentclash agent-harness get <harness-id>
agentclash agent-harness create \
  --name "Refund fix" \
  --task "Fix the refund bug in services/refund.go" \
  --harness-kind codex_e2b \
  --auth-mode api_key_secret \
  --api-key-secret OPENAI_API_KEY \
  --repository-url https://github.com/org/repo \
  --base-branch main

agentclash agent-harness run <harness-id> --follow
agentclash agent-harness executions <harness-id>
```

`--harness-kind` values: `codex_e2b` (default), `claude_e2b`, `hermes_e2b`, `openclaw_e2b`.

Create from JSON:
```bash
agentclash agent-harness create --from-file harness.json
```

Optional flags: `--codex-template`, `--codex-model`, `--execution-config`, `--evaluation-config`, `--evaluation-config-file`.

### Executions
```bash
agentclash agent-harness execution get <execution-id>
agentclash agent-harness execution cancel <execution-id>
agentclash agent-harness execution retry <execution-id> --idempotency-key cli-retry-1
```

### Suites and rankings
```bash
agentclash agent-harness suite list
agentclash agent-harness suite create --name "Private bank" --task-json '{"title":"Task 1","public_prompt":"..."}'
agentclash agent-harness suite tasks <suite-id>
agentclash agent-harness suite run <suite-id> --harness <harness-id-1> --harness <harness-id-2>
agentclash agent-harness suite rankings <suite-id> --k 3
```

### Failures and promotion
```bash
agentclash agent-harness failures summary
agentclash agent-harness execution failure-review get <execution-id>
agentclash agent-harness execution failure-review update <execution-id> --human-class timeout --human-summary "Sandbox timed out"
agentclash agent-harness execution promote-task <execution-id> --suite <suite-id> --title "Promoted failure case"
```

Alias: `agentclash harness` = `agentclash agent-harness`.

## Expected Output
- Create returns harness ID, kind, auth mode, template.
- Run returns execution ID; `--follow` polls until `completed`, `failed`, or `cancelled`.
- Suite rankings table shows success@1, pass@k, cost, latency per harness.

## Failure Modes
- Missing `--api-key-secret` on create → pass workspace secret name containing provider key.
- Missing required suite flags → `--harness` required on `suite run`; `--task-json` or `--from-file` on suite create.
- Non-terminal execution on retry → only terminal executions can retry.
- Wrong harness kind for template → defaults: `codex`, `agentclash-claude-fullstack`, `agentclash-hermes-fullstack`, `agentclash-openclaw-fullstack`.

## Safety Notes
- Harness runs execute code in E2B sandboxes with repository access — confirm repo and secrets before running.
- Promoted tasks may contain sensitive failure excerpts — sanitize `public_prompt`.
- Do not paste API keys or secret values into chat.

## Report Back Format
```text
Harness: <id> (<name>, <kind>)
Execution: <id> — <status>
Suite: <id or n/a>
Ranking summary: <top harness or n/a>
Failure review: <effective_class or n/a>
Next commands: <1-3>
```

## Related Skills
- `agentclash-hub`
- `agentclash-cli-setup`
- `agentclash-runtime-resources-setup`
- `agentclash-eval-runner`
- `agentclash-scorecard-reader`
- `agentclash-regression-flywheel`

## Related Docs
- `/docs-md/guides/ci-cd-workload-recipes`
- `/docs-md/reference/cli`
