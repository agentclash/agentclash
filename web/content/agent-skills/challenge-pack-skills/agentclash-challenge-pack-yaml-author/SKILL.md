---
name: agentclash-challenge-pack-yaml-author
description: Use when writing or editing AgentClash challenge pack YAML, including tasks, cases, input sets, scoring blocks, tools, sandbox settings, assets, and metadata.
metadata:
  agentclash.role: challenge-pack-authoring
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack YAML Author

## Purpose
Create valid challenge pack YAML from a planned evaluation.

## Use When
- A pack outline needs to become a concrete YAML file.
- An existing pack needs structural edits.
- The agent needs a checklist for fields to fill without source-code access.

## Inputs Needed
- Pack plan.
- Case payloads and expected outputs.
- Selected scoring and judge strategy.
- Tool, artifact, sandbox, and secret requirements.

## Environment
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

## Procedure
1. Fill metadata and pack identity first.
2. Add tasks and cases with stable names.
3. Group cases into input sets that match the intended run modes.
4. Add scoring dimensions and connect them to evidence sources.
5. Add tools, sandbox policy, assets, and artifacts only when required.
6. Hand off to validation before publication.

## Commands
```bash
agentclash challenge-pack init <pack-name>
agentclash challenge-pack validate path/to/pack.yaml
```

## Fill-In Checklist
- Pack identity and description.
- Task instructions.
- Case payload schema.
- Input set names and membership.
- Scoring dimensions.
- Validator or judge references.
- Tool definitions.
- Sandbox/network policy.
- Artifact expectations.

## Related Skills
- `agentclash-challenge-pack-input-sets`
- `agentclash-challenge-pack-validation-publish`
