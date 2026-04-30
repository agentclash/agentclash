---
name: agentclash-agent-deployment-setup
description: Use when creating, selecting, or diagnosing AgentClash agent deployments for runs, including build selection, deployment IDs, workspace context, and run compatibility.
metadata:
  agentclash.role: agent-deployments
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Agent Deployment Setup

## Purpose
Make an agent deployment available as a runnable participant in AgentClash.

## Use When
- A run needs deployment IDs.
- A build exists but is not yet runnable.
- Deployment selection or workspace context is unclear.

## Inputs Needed
- Workspace ID.
- Agent build ID or build description.
- Runtime resources required by the deployment.
- Challenge pack compatibility requirements.

## Environment
```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

## Procedure
1. Verify the workspace context.
2. List available builds and deployments.
3. Create or select the deployment.
4. Confirm provider, model alias, runtime profile, and secret dependencies.
5. Return deployment IDs for `run create` or `eval start`.

## Commands
```bash
agentclash workspace use <workspace-id>
agentclash deployment list
agentclash deployment create --help
```

## Related Skills
- `agentclash-agent-build-author`
- `agentclash-eval-runner`
