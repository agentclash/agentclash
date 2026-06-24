---
name: agentclash-multi-turn-operator
description: Use when a multi_turn challenge pack run agent is awaiting human input and you need to check turn status or submit an operator message with agentclash run turn.
metadata:
  agentclash.role: multi-turn
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Multi-Turn Operator

## Purpose
Operate human takeover phases in multi-turn challenge packs: detect when a run agent awaits operator input and submit the next user message so the eval can continue.

## Use When
- A multi_turn pack includes a `human` phase in its user simulator manifest.
- CLI or UI shows a run agent waiting for human input mid-run.
- An operator must reply as the end user during live eval observation.

## Do Not Use When
- The pack is single-turn or fully scripted/LLM-simulated — no human turns exist.
- The run has not started — use `agentclash-eval-runner` first.
- The task is authoring multi-turn pack YAML — see `/docs-md/challenge-packs/multi-turn` and challenge-pack skills.

## Inputs Needed
- Workspace ID and run ID.
- Run agent ID (from `agentclash run agents <runId> --json`).
- Human message text for the awaiting turn.
- Pack must use `execution_mode: multi_turn` with a human phase.

## Environment
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash workspace use <WORKSPACE_ID>
agentclash run get <RUN_ID> --json
agentclash run agents <RUN_ID> --json
```

## Procedure
1. Confirm the run is `running` and the target run agent exists.
2. Check `agentclash run turn status <runAgentId> --run <runId>`.
3. If `awaiting_human` is true, read `turn_index`, `phase_id`, and optional `prompt_hint`.
4. Submit the operator message with `agentclash run turn submit`.
5. Re-check status or follow run events until the agent completes the turn.

## Commands
```bash
agentclash run turn status <RUN_AGENT_ID> --run <RUN_ID>
agentclash run turn status <RUN_AGENT_ID> --run <RUN_ID> --json

agentclash run turn submit <RUN_AGENT_ID> --run <RUN_ID> --message "I still want a full refund, not store credit."
agentclash run turn submit <RUN_AGENT_ID> --run <RUN_ID> --message "..." --json
```

API paths (for debugging):
- Status: `GET /v1/workspaces/{ws}/runs/{run}/run-agents/{runAgent}/turns/status`
- Submit: `POST /v1/workspaces/{ws}/runs/{run}/run-agents/{runAgent}/turns` with `{"message":"..."}`

Structured status fields:
- `awaiting_human` — boolean
- `turn_index` — current turn number when awaiting
- `phase_id` — manifest phase identifier
- `prompt_hint` — optional operator guidance from the pack

## Expected Output
- Status with no wait: `Awaiting human: no`
- Status when blocked: `Awaiting human: yes` plus turn index and phase
- Submit success: `Human turn submitted` (human) or `{"status":"submitted"}` (JSON)

## Failure Modes
- Missing `--run` → both commands require `--run <RUN_ID>`.
- Missing `--message` on submit → required non-empty string.
- Agent not awaiting human → API may reject submit; check status first.
- Wrong run agent ID → list agents on the run before submitting.

## Safety Notes
- Operator messages become part of the eval transcript — avoid real customer PII.
- Human turns affect scoring and calibration; confirm message intent with the operator when ambiguous.

## Report Back Format
```text
Run: <run_id>
Run agent: <run_agent_id> (<label>)
Awaiting human: <yes/no>
Turn index: <n or n/a>
Phase: <phase_id or n/a>
Message submitted: <yes/no>
Prompt hint: <summary or n/a>
Next: agentclash run events <run_id>
```

## Related Skills
- `agentclash-hub`
- `agentclash-eval-runner`
- `agentclash-scorecard-reader`
- `agentclash-challenge-pack-planner`

## Related Docs
- `/docs-md/challenge-packs/multi-turn`
- `/docs-md/getting-started/first-eval`
- `/docs-md/reference/cli`
