---
name: agentclash-challenge-pack-input-sets
description: Use when designing AgentClash challenge pack cases and input sets for smoke, full benchmark, regression, edge-case, or CI suite-only coverage.
metadata:
  agentclash.role: challenge-pack-inputs
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack Input Sets

## Purpose
Design case coverage that makes runs repeatable and interpretable.

## Use When
- A pack has too few, too many, or poorly grouped cases.
- The user needs smoke, regression, CI, or full benchmark subsets.
- A run should focus on a specific failure mode or capability.

## Inputs Needed
- Candidate cases.
- Expected difficulty bands.
- Run budget and desired confidence.
- Regression or CI gating requirements.

## Procedure
1. Separate smoke cases from full benchmark cases.
2. Label edge cases by behavior, not by implementation detail.
3. Keep regression inputs reproducible and minimal.
4. Avoid mixing unrelated capabilities in one input set.
5. Record why each case exists.

## Output Shape
```text
Input set:
Purpose:
Cases:
Difficulty:
Expected signal:
Use in run:
```

## Related Skills
- `agentclash-regression-flywheel`
- `agentclash-eval-runner`
