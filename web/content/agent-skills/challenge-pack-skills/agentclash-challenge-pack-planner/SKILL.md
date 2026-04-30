---
name: agentclash-challenge-pack-planner
description: Use when turning a vague AgentClash evaluation idea into a challenge pack plan with task boundaries, target agents, input coverage, scoring strategy, tools, artifacts, and publish criteria.
metadata:
  agentclash.role: challenge-pack-planning
  agentclash.version: "1"
  agentclash.requires_cli: "false"
---

# AgentClash Challenge Pack Planner

## Purpose
Plan the shape of a challenge pack before writing YAML.

## Use When
- The user has a benchmark idea but not a concrete pack structure.
- The pack needs decomposition into tasks, input sets, scoring dimensions, tools, and artifacts.
- An agent needs enough product context to proceed without reading AgentClash source code.

## Inputs Needed
- Evaluation goal and intended agent type.
- What a good, bad, and borderline answer looks like.
- Required files, tools, network, secrets, and time limits.
- Whether the pack is for smoke, regression, CI, or public comparison.

## Procedure
1. Define the agent behavior being tested.
2. Split the workload into representative input cases.
3. Choose scoring dimensions before writing prompts.
4. Decide whether validators, LLM judges, or both are needed.
5. Identify tool, sandbox, artifact, and secret requirements.
6. Produce a pack outline that another skill can turn into YAML.

## Output Shape
```text
Pack name:
Goal:
Target agent behavior:
Input sets:
Scoring dimensions:
Validators:
LLM judges:
Tools/sandbox:
Artifacts:
Publish checks:
Next skill:
```

## Related Skills
- `agentclash-challenge-pack-yaml-author`
- `agentclash-challenge-pack-scoring-validators`
- `agentclash-challenge-pack-llm-judges`
