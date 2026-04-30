---
name: agentclash-challenge-pack-tools-sandbox
description: Use when defining AgentClash challenge pack tool access, sandbox runtime needs, filesystem expectations, network policy, command execution, and secret references.
metadata:
  agentclash.role: challenge-pack-tools
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack Tools And Sandbox

## Purpose
Specify the execution environment a challenge pack needs.

## Use When
- The task needs filesystem access, shell commands, HTTP calls, or custom tools.
- The pack needs network restrictions or sandbox resources.
- Secret references or environment variables must be available at run time.

## Inputs Needed
- Tool list and allowed operations.
- Sandbox image/runtime expectations.
- Network policy.
- Secret names and non-secret defaults.

## Procedure
1. Start with the smallest tool surface that can solve the task.
2. Make filesystem paths and working directories explicit.
3. Define network access deliberately.
4. Reference secrets by name; never embed secret values.
5. Add a smoke case that proves the environment is available.

## Output Shape
```text
Tool:
Purpose:
Inputs:
Sandbox requirement:
Network:
Secrets:
Smoke check:
```

## Related Skills
- `agentclash-runtime-resources-setup`
- `agentclash-challenge-pack-validation-publish`
