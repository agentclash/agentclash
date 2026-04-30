---
name: agentclash-agent-build-author
description: Use when creating or editing AgentClash agent build specifications, including agent identity, runnable configuration, prompts, model choices, runtime expectations, and version notes.
metadata:
  agentclash.role: agent-builds
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Agent Build Author

## Purpose
Describe an agent build clearly enough that it can be deployed and evaluated.

## Use When
- The user needs a new agent build or version.
- A build should capture prompt, model, tool, or runtime changes.
- A deployment needs a build with stable metadata.

## Inputs Needed
- Agent name and intended behavior.
- Model/provider choice.
- Prompt or implementation entrypoint.
- Runtime and tool requirements.
- Version notes.

## Procedure
1. Define what the build is supposed to do.
2. Capture model, prompt, and runtime configuration.
3. Record tool and secret dependencies.
4. Add version notes that explain what changed.
5. Hand off to deployment setup.

## Related Skills
- `agentclash-agent-deployment-setup`
- `agentclash-runtime-resources-setup`
