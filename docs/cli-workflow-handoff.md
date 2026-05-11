# AgentClash CLI Workflow Handoff

Last updated: May 12, 2026

This document is the current handoff for the AgentClash CLI user workflow work.
The original Phase 1 handoff has been removed; this file is the canonical CLI
workflow handoff. It supersedes that older snapshot because the CLI now has
substantial additional surface area: CI, harnesses, infra resources, artifacts,
regression suites, release gates, prompt evals, quota reporting, run series,
run comparison/replay tools, run transcripts, and repeated eval sessions.

## Current State

### Workflow-first CLI path

The happy path is now:

```bash
agentclash auth login
agentclash link
agentclash quickstart
agentclash challenge-pack init agentclash-pack.yaml
agentclash challenge-pack validate agentclash-pack.yaml
agentclash challenge-pack publish agentclash-pack.yaml
agentclash eval start --follow
agentclash baseline set
agentclash eval scorecard
```

Existing low-level commands remain available for users who already know the IDs
they need: `run`, `challenge-pack`, `deployment`, `build`, `infra`, `artifact`,
`regression-suite`, `release-gate`, `secret`, `agent-harness`, `ci`,
`prompt-eval`, `playground`, and `quota`.

### Surfaces that already exist

- Auth and workspace setup: `auth login`, `link`, `init`, `config`, and `doctor`.
- Evaluation workflow: `eval start`, `eval scorecard`, `baseline set/show/clear`,
  `run create`, `run agents`, `run ranking`, `run scorecard`, `run failures`,
  `run compare`, `run replay`, `run transcript`, and `run promote-failure`.
- Repeated eval creation: `eval start --repetitions N` creates eval sessions.
- Durable race series: `run series create` and `run series report` expose
  lineup/seed aggregate reporting.
- Prompt evals and quota: `prompt-eval` manages prompt-eval configs and
  playground experiments; `quota` shows workspace usage.
- CI/CD: `ci init`, `ci validate`, `ci should-run`, `ci baseline`, `ci run`, and
  the GitHub Action under `.github/actions/agentclash-ci`.
- Agent harnesses: `agent-harness` supports `codex_e2b` and `claude_e2b`
  payloads, including provider API-key secrets.
- Runtime setup: `infra`, `build`, `deployment`, `secret`, and related agent
  build skills cover the raw resource lifecycle.
- Portable Agent Skills exist under `web/content/agent-skills` and are exposed in
  docs for Codex, Claude Code, Cursor, and generic agents.

## User Flows

### First eval

1. User logs in and chooses a workspace.
2. User runs `agentclash quickstart`.
3. The CLI checks auth, API URL, workspace access, challenge packs, deployments,
   and baseline status.
4. If prerequisites are missing, quickstart prints the next setup command.
5. Once ready, quickstart points to `agentclash eval start --follow`.
6. After the first run, user runs `agentclash baseline set`.
7. Future runs can use `agentclash eval scorecard` or `agentclash compare latest`.

### Repeated eval

1. User runs `agentclash eval start --repetitions 5`.
2. The CLI creates an eval session and prints:

   ```bash
   agentclash eval session follow <eval-session-id>
   agentclash eval session get <eval-session-id>
   ```

3. `eval session follow` polls session status until aggregation completes.
4. `eval session get` shows child runs, run counts, aggregate metrics, pass@K,
   pass^K, winner/leader information, and evidence warnings when available.
5. For race-series sessions, `agentclash run series report <eval-session-id>`
   remains the focused aggregate report for lineup, seed, correctness, cost, and
   token totals.

### Regression debugging

1. User runs `agentclash compare latest --gate`.
2. The command uses the saved baseline bookmark and the latest non-baseline run.
3. If the gate returns a non-pass verdict, the user runs:

   ```bash
   agentclash replay triage
   ```

4. Triage summarizes ranking, scorecard, failure review items, replay snippets,
   artifact pointers, and the next low-level commands to inspect or promote
   failures.

### Returning user

1. User runs `agentclash quickstart`.
2. If config and resources are healthy, the output stays short and points to
   `agentclash eval start --follow`.
3. If a baseline exists, it also suggests `agentclash compare latest --gate`.

### CI flywheel

1. User creates or validates `.agentclash/ci.yaml`.
2. GitHub Actions invokes `agentclash ci run`.
3. The CLI creates a candidate build/deployment, starts a run, evaluates the
   release gate, writes reports/artifacts, and can promote failures into
   regression suites.
4. PR comments and summaries are handled by the existing GitHub Action.

## What This Phase Added

- `agentclash quickstart`
  - Read-only readiness and next-command guidance.
  - Does not create remote resources, write config, or start runs.
- `agentclash eval session list|get|follow`
  - Uses existing eval-session read APIs.
  - Makes repeated eval sessions inspectable from the CLI after
    `eval start --repetitions N`.
  - Complements the existing `run series report` view for durable race series.
- `agentclash compare latest`
  - Compares the local baseline bookmark against the latest non-baseline run.
  - Supports `--agent`, `--baseline-agent`, `--candidate-agent`, `--gate`, and
    structured output.
- `agentclash replay triage [run]`
  - Aggregates ranking, failures, scorecard, replay snippets, artifact pointers,
    and next commands into one debugging view.
- Root help and eval-session creation output now point users toward the workflow
  commands.

## Latest Main Merge Notes

After merging latest `main` on May 12, 2026, this branch preserves the command
surfaces that already landed there:

- `agentclash run compare`
- `agentclash run replay`
- `agentclash run transcript`
- `agentclash run series create|report`
- `agentclash prompt-eval`
- `agentclash quota`

The workflow wrappers still unique to this phase are `quickstart`,
`eval session list|get|follow`, `compare latest`, and `replay triage`.

## Deferred Phases

### Agent integrations: Claude, Codex, MCP

Existing state:

- `agent-harness create --harness-kind claude_e2b` already exists.
- Agent Skills already exist in `web/content/agent-skills`.
- Docs already describe Claude Code and Codex install targets at a high level.

Recommended next phase:

- Embed a versioned AgentClash skills snapshot into the CLI binary.
- Add `agentclash integration claude install|doctor`.
- Add `agentclash integration codex install|doctor`.
- Install Claude skills into `.claude/skills/<skill>/SKILL.md`.
- Install Codex skills into `.agents/skills/<skill>/SKILL.md`.
- Add `agentclash mcp serve --mode read-only|full`, defaulting to read-only.
- Do not modify `CLAUDE.md`, `AGENTS.md`, `.mcp.json`, or project config unless
  the user passes an explicit write flag.

### CI release-gate fidelity

Known gap:

- CI manifests already accept gate fields, but `ci run` should be audited to make
  sure `gate.policy_file` and `gate.fail_on` have the exact intended effect.

Recommended next phase:

- Load `gate.policy_file` and pass it to `/v1/release-gates/evaluate`.
- Define and document `gate.fail_on` exit-code behavior.
- Add `compare gate --policy-file`.
- Fix stale docs that mention nonexistent `ci run --config` or `--dry-run`.

### Harness lifecycle

Known gap:

- Raw `agent-harness`, `build`, `deployment`, `infra`, and `secret` commands
  exist, but there is no cohesive "set up this coding agent" wizard.

Recommended next phase:

- Add a guided harness setup/check command only after the Claude/Codex
  integration shape is locked.
- Keep raw resource commands as the source of truth.
- Avoid a generic `agentclash update` command until the product can distinguish
  CLI binary updates, agent asset updates, harness template updates, and model
  alias updates.

### Docs and reference

Known gap:

- `web/content/docs/reference/cli.mdx` is still a stub-like CLI reference page.
- Some docs still describe older run-centric or CI flag shapes.

Recommended next phase:

- Generate or refresh CLI reference docs from Cobra help.
- Add quickstart, repeated-eval, compare-latest, and replay-triage examples to
  the public docs.
- Keep `README.md` and `npm/cli/README.md` aligned with the workflow path.

### Server-side workflow polish

Known gaps:

- Baselines are still local bookmarks.
- Artifact listing is workspace-wide; triage filters locally because the backend
  list API does not expose run/run-agent query filters yet.
- `eval session follow` polls reads; there is no session-level SSE stream.

Recommended next phase:

- Consider server-side/shareable baselines.
- Add backend artifact filters before adding CLI flags for them.
- Consider eval-session events only if polling becomes noisy or expensive.

## Known Gaps After This Phase

- `quickstart` is intentionally read-only; it gives commands instead of creating
  resources.
- `compare latest` may need `--candidate-agent` in multi-agent runs.
- `replay triage` asks for `--agent` when a run has multiple agents and no single
  safe default.
- Claude/Codex skill installation and MCP are documented here but not implemented
  in this phase.
- CI gate policy fidelity remains deferred.

## Test / Release Notes

Review checkpoint contract:

```text
testing/codex-cli-phase2-workflows.md
```

Verified from `cli/` after merging latest `main` on May 12, 2026:

```bash
go test ./cmd -run '(EvalSession|Quickstart|CompareLatest|ReplayTriage|RunReplay|RunCompare)' -count=1  # pass
go test -short ./cmd                                                                                     # pass
go test -short ./...                                                                                     # pass
go vet ./...                                                                                             # pass
go build ./...                                                                                           # pass
git diff --check                                                                                         # pass
```

Packaging does not need to be rehearsed for this phase unless release packaging
files change.
