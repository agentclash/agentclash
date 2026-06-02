---
name: agentclash-compare-and-triage
description: Use when comparing baseline vs candidate AgentClash runs, evaluating release gates, managing workspace baseline bookmarks, or building a replay triage envelope after an eval completes.
metadata:
  agentclash.role: comparison
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Compare And Triage

## Purpose
Manage workspace baselines, compare runs for regressions, evaluate release gates for CI, and assemble replay triage evidence (ranking, failures, scorecard, replay steps) in one workflow.

## Use When
- A user asks whether a new run regressed vs a baseline.
- CI needs `compare gate` exit codes for pass/fail verdicts.
- A user wants the fastest path: `compare latest` against the saved baseline bookmark.
- After a run completes, the user needs structured triage with suggested follow-up commands.

## Do Not Use When
- No completed runs exist yet — use `agentclash-eval-runner` first.
- The task is only deep scorecard interpretation without comparison — use `agentclash-scorecard-reader`.
- The task is authoring CI manifest files from scratch — use `agentclash-ci-release-gate` (this skill covers CLI compare/gate/triage commands).

## Inputs Needed
- Workspace with at least one completed candidate run.
- Baseline bookmark (`baseline set`) for `compare latest`, or explicit run IDs for `compare runs` / `compare gate`.
- Optional run agent ID or label when runs have multiple agents.
- For triage: run ID or selector; optional `--agent`, `--cursor`, `--limit`.

## Environment
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash workspace use <WORKSPACE_ID>
agentclash baseline show
```

## Procedure
1. After a good eval, bookmark it: `agentclash baseline set [run] --agent <label>`.
2. Run new evals with `agentclash eval start` (see eval-runner skill).
3. Compare:
   - Ad hoc: `agentclash compare runs --baseline <ID> --candidate <ID>`
   - Fast path: `agentclash compare latest` (uses saved baseline vs latest non-baseline run)
   - CI gate: `agentclash compare gate --baseline <ID> --candidate <ID>`
   - Latest + gate: `agentclash compare latest --gate`
4. Triage evidence: `agentclash replay triage <run> [--agent <label>]`.
5. Follow `next_commands` from triage JSON for deeper replay or scorecard reads.

## Commands

### Baseline bookmark (workspace-scoped)
```bash
agentclash baseline set [run]
agentclash baseline set [run] --agent <RUN_AGENT_ID_OR_LABEL>
agentclash baseline show
agentclash baseline clear
```

- `baseline set` with no run opens an interactive picker in a TTY.
- Bookmark stores run ID, run agent ID, names, and timestamp in CLI config.
- `compare latest` reads this bookmark as the baseline side.

### Compare runs
```bash
agentclash compare runs \
  --baseline <BASELINE_RUN_ID> \
  --candidate <CANDIDATE_RUN_ID> \
  --baseline-agent <RUN_AGENT_ID_OR_LABEL> \
  --candidate-agent <RUN_AGENT_ID_OR_LABEL>

agentclash run compare \
  --baseline <BASELINE_RUN_ID> \
  --candidate <CANDIDATE_RUN_ID>
```

Shared comparison flags (both `compare runs` and `run compare`):

- `--baseline` (required)
- `--candidate` (required)
- `--baseline-agent` — optional; defaults to first agent or saved baseline agent when applicable
- `--candidate-agent` — optional

### Compare latest (baseline bookmark vs newest run)
```bash
agentclash compare latest
agentclash compare latest --gate
agentclash compare latest --agent <RUN_AGENT_ID_OR_LABEL>
agentclash compare latest --baseline-agent <ID_OR_LABEL> --candidate-agent <ID_OR_LABEL>
agentclash compare latest --json
```

- Requires a saved baseline bookmark unless baseline run is inferable from flags.
- `--gate` evaluates release gate rules and returns nonzero exit for non-pass verdicts (same as `compare gate`).
- Structured output includes comparison envelope and optional `release_gate` object.

### Compare gate (explicit IDs, CI-friendly exit code)
```bash
agentclash compare gate \
  --baseline <BASELINE_RUN_ID> \
  --candidate <CANDIDATE_RUN_ID> \
  --baseline-agent <RUN_AGENT_ID_OR_LABEL> \
  --candidate-agent <RUN_AGENT_ID_OR_LABEL>
```

- `--baseline` and `--candidate` are required.
- Non-pass gate verdicts exit nonzero for shell/CI scripts.

### Replay triage envelope
```bash
agentclash replay triage <RUN_ID_OR_SELECTOR>
agentclash replay triage <RUN_ID> --agent <RUN_AGENT_ID_OR_LABEL>
agentclash replay triage <RUN_ID> --cursor 0 --limit 5
agentclash replay triage <RUN_ID> --json
```

Flags:

- `--agent` — run agent ID or label; required in non-interactive mode when multiple agents exist.
- `--cursor` — replay step offset (default 0).
- `--limit` — steps to include, 1–50 (default 5).

Triage envelope includes:

- `run`, `agents`, `selected_agent`, `ranking`, `failures`, `artifacts`
- `scorecard` and `replay` when an agent is selected
- `next_commands` — suggested follow-ups (e.g. deeper replay, scorecard, compare)

## Expected Output
- **Compare** — human tables or JSON with candidate/baseline metrics, deltas, and optional `release_gate.verdict`.
- **compare latest --gate** — prints comparison then exits 1 on gate failure.
- **replay triage** — consolidated evidence bundle; use `--json` for automation.

## Failure Modes
- No baseline bookmark for `compare latest` → run `agentclash baseline set` on a known-good run.
- No candidate run newer than baseline → create a new eval first.
- Multiple run agents without `--agent` on triage → pass `--agent` or use interactive TTY.
- Gate pending scorecard → wait for run completion; check `agentclash run get <id>`.
- Invalid agent selector → list agents with `agentclash run agents <run_id> --json`.

## Safety Notes
- Comparisons are read-only but may surface sensitive failure excerpts — do not paste into public channels.
- Gate failures should block release; confirm with the user before overriding CI exit codes.

## Report Back Format
```text
Baseline: <run_id> / agent <id or label>
Candidate: <run_id> / agent <id or label>
Compare command: <command used>
Gate verdict: <pass|fail|pending|n/a>
Key deltas: <summary>
Triage agent: <selected agent>
Failures: <count / top class>
Next commands:
- <from triage envelope>
Recommendation: <ship|investigate|rerun>
```

## Related Skills
- `agentclash-hub`
- `agentclash-eval-runner`
- `agentclash-scorecard-reader`
- `agentclash-ci-release-gate`
- `agentclash-regression-flywheel`

## Related Docs
- `/docs-md/guides/interpret-results`
- `/docs-md/guides/ci-cd-agent-gates`
- `/docs-md/concepts/replay-and-scorecards`
- `/docs-md/reference/cli`
