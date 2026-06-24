# AGENTS.md — using AgentClash

You are reading this because the `agentclash` CLI is installed in this project
(`node_modules/agentclash/`). This file tells an AI coding agent how to **use**
AgentClash. It is not the AgentClash source repo.

## What AgentClash is

AgentClash is a race engine: it pits AI models/agents against each other on real
tasks with live scoring. You define a **eval pack** (the eval workload), run
**agents** (model + runtime + tools) against it, and read a **scorecard**. There
are **no built-in packs** — every workspace authors its own.

## Division of labor (important)

- **Humans do one-time setup on the web** at https://agentclash.dev: sign in,
  add provider API keys (BYOK), and create deployments. Don't try to do these
  from the CLI.
- **The CLI is the iterate loop**: author packs, run evals, read scorecards,
  gate CI. That's your job as the agent.

## First moves

```bash
# 1. Make sure you're pointed at the hosted backend (released binaries already are).
export AGENTCLASH_API_URL="https://api.agentclash.dev"

# 2. Authenticate. Interactive shells: device login. CI: use a token env var.
agentclash auth login --device          # or: export AGENTCLASH_TOKEN=...
agentclash link                          # pick/save a workspace

# 3. Verify everything before doing work.
agentclash doctor                        # add --json for machine-readable output

# 4. Introspect the entire CLI — every command, flag, and stable exit code — as JSON.
agentclash schema --json
```

Resolution order for the API base URL:

```text
--api-url > AGENTCLASH_API_URL > saved user config > default
```

CI / non-interactive env vars:

```bash
export AGENTCLASH_TOKEN="..."
export AGENTCLASH_WORKSPACE="workspace-id"
```

## Install the deeper skills

This CLI ships a bundle of **Agent Skills** (SKILL.md files) that teach an agent
the full AgentClash workflow. Install them into your host's skills directory:

```bash
agentclash integration <agent> install   # claude | codex | cursor | openclaw | hermes | opencode
agentclash integration <agent> doctor     # report installed / missing / drifted skills
```

`install` is idempotent and writes **only** SKILL.md files — it never touches
`CLAUDE.md`, `AGENTS.md`, `.mcp.json`, or any project config.

Once installed, **load `agentclash-hub` first** — it is the entrypoint and
carries the full workflow map, skill dependency order, hosted defaults, and
product UI links. Other notable skills: `agentclash-cli-setup`,
`agentclash-eval-pack-yaml-author`, `agentclash-eval-runner`,
`agentclash-scorecard-reader`, `agentclash-compare-and-triage`,
`agentclash-regression-flywheel`, `agentclash-ci-release-gate`.

## End-to-end workflow (after web setup exists)

```text
1. agentclash-cli-setup              → auth, workspace, doctor
2. agentclash-quickstart             → readiness checks + next command
3. runtime-resources-setup           → provider, model alias, runtime profile, secrets
4. agent-build-author                → build spec + ready build version
5. agent-deployment-setup            → deployment ID for runs
6. eval-pack skills             → plan, write YAML, validate, publish a pack
7. agentclash-eval-runner            → eval start / run create / --follow / sessions
8. agentclash-scorecard-reader       → rankings, scorecards, replay, artifacts
9. agentclash-compare-and-triage     → baseline, compare latest/gate, replay triage
10. agentclash-regression-flywheel   → promote failures to suite-only reruns
11. agentclash-ci-release-gate       → CI manifest + gate (optional)
```

## Agent-friendly conventions

- Add `--json` (or `-o json`) to any command for machine-readable stdout; human
  progress goes to stderr. Prefer `doctor --json` (returns a `ready` field, exits
  non-zero on warnings) and `schema --json` for introspection.
- Exit codes are stable and documented; read them from `agentclash schema --json`
  (`.exit_codes`). Branch on the code, don't re-parse stderr.
- Errors print a structured envelope in `--json` mode (`code`, `message`,
  `details`, `next_step`).

## Common mistakes agents make

- Trying to add provider keys or create deployments from the CLI — those are
  one-time **web** actions at https://agentclash.dev.
- Running evals before a pack is published (`eval-pack ... validate` then
  `publish`) or before a deployment exists.
- Leaving `AGENTCLASH_API_URL` unset on a source build (it defaults to
  `http://localhost:8080`) and hitting an empty/wrong backend.
- Skipping `agentclash-cli-setup` / `agentclash doctor`, then failing on
  workspace/auth errors mid-workflow.
- Pasting tokens or provider secrets into chat, logs, or committed files.

## Where to send the human

Base URL **https://agentclash.dev** — docs at `/docs`, quickstart at
`/docs/getting-started/quickstart`, the agent-skills catalog at
`/docs/agent-skills`, run results on the workspace Runs list after sign-in.
