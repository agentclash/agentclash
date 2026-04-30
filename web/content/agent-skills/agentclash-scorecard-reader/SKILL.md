---
name: agentclash-scorecard-reader
description: Use when interpreting AgentClash rankings, scorecards, replay timelines, artifacts, and evidence into engineering findings and next actions.
metadata:
  agentclash.role: reviewing
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Scorecard Reader

## Purpose
Convert AgentClash result evidence into a concise engineering readout.

## Use When
- A user asks why an agent won, failed, regressed, or drifted.
- A run has rankings, scorecards, replay events, or artifacts that need interpretation.
- A reviewer needs concrete next actions instead of raw event dumps.

## Do Not Use When
- The user needs to create a new run first.
- The user is asking to promote failures into a regression suite.

## Inputs Needed
- Run ID.
- Optional run agent ID for a specific scorecard or replay.
- Comparison baseline or expected winner when relevant.

## Environment
Use production by default; only override when another backend is explicit:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

## Procedure
1. Fetch ranking first to identify participants and outcome.
2. Inspect scorecards for top failures, validator evidence, and judge rationale.
3. Use replay events to confirm whether the scorecard matches observable behavior.
4. Check artifacts when output files are part of the task.
5. Report findings evidence-first: claim, evidence, impact, next action.

## Commands
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
agentclash run ranking <run-id>
agentclash run agents <run-id>
agentclash run scorecard <run-id> <run-agent-id>
agentclash replay get <run-id> <run-agent-id>
agentclash artifact list --run <run-id>
```

## Expected Output
- Ranking identifies winners and score spread.
- Scorecards identify passed and failed dimensions.
- Replay and artifacts provide enough evidence for root-cause notes.

## Failure Modes
- Scorecard is missing: scoring may still be running, or the agent failed before scoring.
- Replay is too noisy: filter to task, tool, artifact, and final answer events.
- Artifact download fails: verify workspace, run ID, and signed URL expiry.

## Safety Notes
- Do not overstate judge rationale as ground truth. Tie conclusions to replay evidence.
- Avoid exposing private artifact contents unless the user has asked for them.
- Keep recommendations specific and testable.

## Report Back Format
```text
Outcome: <winner/status>
Evidence:
- <claim> - <scorecard/replay/artifact pointer>
Root cause: <short explanation>
Next actions:
- <action>
```

## Related Docs
- `/docs-md/guides/interpret-results`
- `/docs-md/concepts/replay-and-scorecards`
- `/docs-md/concepts/artifacts`
