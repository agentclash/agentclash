---
name: agentclash-challenge-pack-author
description: Use when authoring, validating, or publishing AgentClash challenge packs, including YAML task design, input sets, tools, scoring, artifacts, and staged publication.
metadata:
  agentclash.role: authoring
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack Author

## Purpose
Turn task requirements into a valid, repeatable AgentClash challenge pack.

## Use When
- A user wants a new challenge pack or changes to an existing pack.
- A pack needs validation, publication, or review before an eval.
- The task requires clear scoring, tools, inputs, artifacts, or setup assets.

## Do Not Use When
- The user only wants to run an already published pack.
- The user is asking to interpret results from a completed run.

## Inputs Needed
- Task goal and success criteria.
- Required input cases and expected outputs.
- Tool, network, sandbox, secret, and artifact requirements.
- Target workspace and whether publication should go to staging or local.

## Environment
Use staging unless the user intentionally targets another backend:

```bash
export AGENTCLASH_API_URL="https://staging-api.agentclash.dev"
```

## Procedure
1. Draft the pack with a small representative input set first.
2. Make scoring criteria concrete enough to distinguish acceptable and unacceptable submissions.
3. Add tool and artifact declarations only when the task truly needs them.
4. Validate locally.
5. Publish only after validation passes and the user intends to create a reusable pack.
6. Report the pack ID, input set IDs, and the exact follow-up run command.

## Commands
```bash
export AGENTCLASH_API_URL="https://staging-api.agentclash.dev"
agentclash challenge-pack init <pack-name>
agentclash challenge-pack validate path/to/pack.yaml
agentclash challenge-pack publish path/to/pack.yaml
agentclash challenge-pack list
```

## Expected Output
- Validation exits 0 and reports no schema or scoring errors.
- Publication returns a challenge pack ID and available input sets.
- The pack can be selected by `run create` or `eval start`.

## Failure Modes
- Ambiguous scoring creates noisy results. Tighten validators or evidence requirements.
- Missing assets break execution. Keep assets next to the pack or upload them explicitly.
- Secret names drift between the pack and workspace. Verify names before publishing.

## Safety Notes
- Do not publish to production unless the user explicitly asks.
- Avoid embedding real credentials or customer data in pack YAML.
- Keep destructive tool permissions out of MVP packs unless the challenge requires them.

## Report Back Format
```text
Pack: <name>
Validation: <pass/fail and notes>
Published ID: <id or not published>
Input sets: <ids>
Next run command: <command>
```

## Related Docs
- `/docs-md/guides/write-a-challenge-pack`
- `/docs-md/concepts/challenge-packs-and-inputs`
- `/docs-md/concepts/tools-network-and-secrets`
