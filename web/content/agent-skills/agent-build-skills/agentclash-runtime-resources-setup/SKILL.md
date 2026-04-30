---
name: agentclash-runtime-resources-setup
description: Use when configuring AgentClash provider accounts, model aliases, runtime profiles, workspace secrets, tools, and other resources required before agent deployments or runs.
metadata:
  agentclash.role: runtime-resources
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Runtime Resources Setup

## Purpose
Prepare workspace resources that agent builds, deployments, and challenge packs rely on.

## Use When
- A deployment cannot run because provider accounts, model aliases, secrets, or runtime profiles are missing.
- A challenge pack references tools or secrets that are not configured.
- The user needs a checklist before creating runs.

## Inputs Needed
- Workspace ID.
- Provider account requirements.
- Model alias names.
- Secret names, never secret values in chat.
- Runtime profile and tool requirements.

## Environment
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

## Procedure
1. Verify workspace context.
2. Configure provider accounts and model aliases.
3. Create runtime profiles required by the agent or pack.
4. Add workspace secrets by name.
5. Run a small smoke eval before broader runs.

## Related Skills
- `agentclash-agent-deployment-setup`
- `agentclash-challenge-pack-tools-sandbox`
