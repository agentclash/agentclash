---
name: agentclash-challenge-pack-scoring-validators
description: Use when defining deterministic AgentClash scoring validators, score dimensions, evidence sources, pass/fail rules, numeric metrics, and validator failure messages.
metadata:
  agentclash.role: challenge-pack-scoring
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack Scoring Validators

## Purpose
Design deterministic scoring before falling back to subjective judging.

## Use When
- Expected outputs can be checked with exact, JSON, file, math, or structural rules.
- The user needs stable pass/fail criteria for CI or regression.
- A scorecard should explain precisely what failed.

## Inputs Needed
- Final output shape.
- Required artifacts or files.
- Tolerance rules for numeric or text comparisons.
- Score dimensions and weights.

## Procedure
1. Define one scoring dimension per behavior.
2. Prefer deterministic validators for objective requirements.
3. Attach validators to the narrowest evidence source that proves the claim.
4. Write failure messages that explain the expected behavior.
5. Validate with at least one passing and one failing sample when possible.

## Output Shape
```text
Dimension:
Evidence source:
Validator:
Pass condition:
Failure message:
Score impact:
```

## Related Skills
- `agentclash-challenge-pack-llm-judges`
- `agentclash-scorecard-reader`
