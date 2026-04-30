---
name: agentclash-challenge-pack-llm-judges
description: Use when configuring AgentClash LLM-as-judge scoring, judge prompts, rubrics, dimensions, evidence inputs, abstention behavior, and judge result interpretation.
metadata:
  agentclash.role: challenge-pack-judging
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack LLM Judges

## Purpose
Add LLM judges when deterministic validators cannot capture the whole evaluation.

## Use When
- Quality depends on reasoning, style, relevance, or nuanced task completion.
- A deterministic validator would be brittle or incomplete.
- The scorecard needs judge rationale tied to replay evidence.

## Inputs Needed
- Dimension being judged.
- Rubric with pass, partial, and fail examples.
- Evidence fields available to the judge.
- Desired numeric, boolean, or categorical output mode.

## Procedure
1. Use LLM judges for subjective dimensions only.
2. Keep judge prompts narrow and evidence-bound.
3. Specify the expected output schema.
4. Define abstention behavior when evidence is insufficient.
5. Pair judges with deterministic validators for hard constraints.

## Output Shape
```text
Judge name:
Dimension:
Evidence:
Rubric:
Output mode:
Abstention rule:
Failure examples:
```

## Related Skills
- `agentclash-challenge-pack-scoring-validators`
- `agentclash-scorecard-reader`
