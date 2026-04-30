---
name: agentclash-challenge-pack-validation-publish
description: Use when validating AgentClash challenge packs, fixing schema errors, publishing packs, recording returned IDs, and preparing follow-up run commands.
metadata:
  agentclash.role: challenge-pack-publication
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack Validation And Publish

## Purpose
Validate and publish a challenge pack only after its structure is ready.

## Use When
- A pack YAML file is ready for validation.
- The user wants to publish a pack to a workspace.
- A reviewer needs pack IDs and next run commands.

## Inputs Needed
- Pack YAML path.
- Target workspace.
- Whether publication should happen now.

## Environment
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

## Procedure
1. Validate the pack locally.
2. Fix schema, scoring, tool, or asset errors.
3. Confirm the target workspace before publishing.
4. Publish the pack.
5. Report pack ID, input set IDs, and next commands.

## Commands
```bash
agentclash challenge-pack validate path/to/pack.yaml
agentclash challenge-pack publish path/to/pack.yaml
agentclash challenge-pack list
```

## Report Back Format
```text
Validation: <pass/fail>
Published: <yes/no>
Pack ID:
Input sets:
Next command:
```
