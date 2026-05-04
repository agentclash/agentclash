---
name: agentclash-challenge-pack-llm-judges
description: Use when configuring AgentClash LLM-as-judge scoring, judge prompts, rubrics, assertion/reference/n-wise modes, evidence inputs, scorecard dimensions, abstention behavior, and judge result interpretation.
metadata:
  agentclash.role: challenge-pack-judging
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack LLM Judges

## Purpose
Add LLM-as-judge scoring only where deterministic validators cannot capture the whole evaluation. Judges should complement objective checks, not replace them.

Use this skill after deterministic validators and evidence sources are known. A good judge is narrow, evidence-bound, budget-aware, and wired to one scorecard dimension through `source: llm_judge`.

## Use When
- Quality depends on reasoning, helpfulness, style, relevance, faithfulness, or nuanced task completion.
- A deterministic validator would be brittle or incomplete.
- A pack needs rubric, assertion, reference, or n-wise cross-agent ranking.
- The scorecard needs judge rationale, confidence, variance, sample count, and model count in replay/scorecard evidence.

## Do Not Use When
- The behavior can be scored with deterministic validators.
- Evidence sources are not stable yet; use input-sets, artifacts, and scoring validators first.
- The run cannot afford extra model calls.
- The judge would need secrets in prompt text or private data that should not leave the workspace.

## Environment
Use hosted production for CLI examples unless the user intentionally targets a local or self-hosted backend.

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

`agentclash challenge-pack validate` calls the hosted API and requires auth plus a workspace. Use `agentclash link`, `--workspace`, `AGENTCLASH_WORKSPACE`, or `.agentclash.yaml` before validating.

## Validation Commands
Validate after changing `judge_mode`, `llm_judges`, judge evidence, scorecard judge dimensions, consensus, or judge limits.

```bash
agentclash challenge-pack validate path/to/pack.yaml
agentclash challenge-pack validate path/to/pack.yaml --json
```

Human output prints `Challenge pack is valid` or `Challenge pack has errors`. Use `--json` for structured `valid` and `errors` fields.

## Minimal Hybrid Shape
Current evaluation specs still require at least one deterministic validator. Use `judge_mode: hybrid` when judges are paired with validators.

```yaml
version:
  evaluation_spec:
    name: support-quality
    version_number: 1
    judge_mode: hybrid
    validators:
      - key: mentions_policy
        type: contains
        target: final_output
        expected_from: literal:refund policy
    llm_judges:
      - mode: rubric
        key: helpfulness
        model: gpt-4o
        samples: 3
        context_from:
          - challenge_input
          - final_output
        rubric: |
          Score 1-5 for whether the answer is correct, complete, and easy for a support agent to use.
    scorecard:
      strategy: weighted
      dimensions:
        - key: correctness
          source: validators
          validators:
            - mentions_policy
          weight: 0.5
        - key: helpfulness
          source: llm_judge
          judge_key: helpfulness
          weight: 0.5
```

Mode coherence rules:

- `judge_mode: deterministic`: `llm_judges` must be empty.
- `judge_mode: llm_judge`: `llm_judges` must contain at least one judge.
- `judge_mode: hybrid`: `llm_judges` must contain at least one judge, and validators are still required by the current scoring validator.

## Judge Fields
Every `llm_judges[]` entry uses this source-backed shape:

```yaml
llm_judges:
  - mode: rubric
    key: stable_judge_key
    model: gpt-4o
    context_from:
      - final_output
    rubric: Rate the output 1-5.
```

Fields:

- `mode`: required; one of `rubric`, `assertion`, `reference`, or `n_wise`.
- `key`: required, unique, and must not collide with validator or metric keys.
- `model`: one model identifier.
- `models`: list of model identifiers. Set exactly one of `model` or `models`.
- `samples`: optional integer from `0` to `10`; `0` means default samples, currently `3`.
- `context_from`: optional list of supported evidence references.
- `output_schema`: optional JSON Schema draft-07 or 2020-12. Validation checks the schema; current judge message builders still instruct the built-in JSON response shape.
- `score_scale`: optional `{min, max}` for `rubric` and `reference`; `min` must be strictly less than `max`, default is `1..5`.
- `rubric`: required for `rubric` and `reference`.
- `assertion`: required for `assertion`.
- `expect`: optional boolean for `assertion`; when false, it flips the pass polarity.
- `prompt`: required for `n_wise`.
- `position_debiasing`: optional boolean for `n_wise`; rotates candidate order across samples.
- `reference_from`: required for `reference`; must be a supported evidence reference.
- `consensus`: required when `models` has more than one entry; invalid otherwise.
- `anti_gaming_clauses`: optional extra prompt clauses appended after the built-in judge instructions.
- `timeout_ms`: optional integer greater than `0`; default judge timeout is 60 seconds.

## Supported Evidence
Judge `context_from` and `reference_from` entries must use supported evidence references:

- `final_output`
- `run.final_output`
- `challenge_input`
- `case.payload`
- `case.payload.<field>`
- `case.inputs.<input_key>`
- `case.expectations.<expectation_key>`
- `artifact.<artifact_key>[.<field>]`
- `file:<post_execution_check_key>`
- `literal:<value>`

Each `context_from` entry is injected into the judge prompt as `<reference>:\n<resolved value>`. Reference judges also inject `reference_answer` from `reference_from`.

If any required judge context or reference evidence is unavailable, the judge result becomes unavailable with a `reason`; it does not produce a `normalized_score`.

## Rubric Mode
Use `mode: rubric` for subjective numeric scoring against a written rubric.

```yaml
llm_judges:
  - mode: rubric
    key: persuasiveness
    model: claude-sonnet-4-6
    samples: 3
    context_from:
      - challenge_input
      - final_output
    score_scale:
      min: 1
      max: 5
    rubric: |
      Score 1-5.
      1: Incorrect, incomplete, or unsupported.
      3: Mostly correct but misses important nuance.
      5: Correct, concise, evidence-bound, and directly useful.
scorecard:
  dimensions:
    - key: persuasiveness
      source: llm_judge
      judge_key: persuasiveness
```

The judge is instructed to return JSON shaped like:

```json
{"score": 4, "confidence": "low|medium|high", "reasoning": "brief rationale"}
```

Rubric and reference scores are clamped to the configured `score_scale` and normalized to `0..1` for the scorecard dimension.

## Assertion Mode
Use `mode: assertion` for yes/no claims such as safety, groundedness, or policy compliance.

```yaml
llm_judges:
  - mode: assertion
    key: no_hallucination
    model: claude-haiku-4-5-20251001
    context_from:
      - case.payload.source_excerpt
      - final_output
    assertion: The response contains only information supported by the source excerpt.
    expect: true
scorecard:
  strategy: hybrid
  dimensions:
    - key: no_hallucination
      source: llm_judge
      judge_key: no_hallucination
      gate: true
      pass_threshold: 1.0
```

The judge is instructed to return JSON shaped like:

```json
{"pass": true, "confidence": "low|medium|high", "reasoning": "brief rationale"}
```

The parser also accepts `verdict` values such as `pass`, `true`, `yes`, `fail`, `false`, or `no`. Assertion samples aggregate by majority and normalize to `1` or `0`.

## Reference Mode
Use `mode: reference` when a gold answer or expected artifact exists.

```yaml
llm_judges:
  - mode: reference
    key: summary_quality
    model: gpt-4o
    context_from:
      - final_output
    reference_from: case.expectations.reference_summary
    rubric: |
      Compare the response to the reference summary for coverage, faithfulness, and concision.
      Penalize unsupported additions.
scorecard:
  dimensions:
    - key: summary_quality
      source: llm_judge
      judge_key: summary_quality
```

`reference_from` must resolve to available evidence. If it does not, the judge is unavailable.

## N-Wise Mode
Use `mode: n_wise` to rank all run agents in the same run against one another.

```yaml
llm_judges:
  - mode: n_wise
    key: overall_quality
    model: claude-sonnet-4-6
    samples: 3
    position_debiasing: true
    context_from:
      - final_output
    prompt: Rank the candidate outputs from best to worst on correctness, completeness, and clarity.
scorecard:
  dimensions:
    - key: overall
      source: llm_judge
      judge_key: overall_quality
```

The judge is instructed to return JSON shaped like:

```json
{"ranking": ["<run_agent_id>", "..."], "confidence": "low|medium|high", "reasoning": "brief rationale"}
```

The parser also accepts `ranked_ids`. Every candidate must appear exactly once. `n_wise` requires at least two run agents in the run. The current run agent receives a normalized Borda-style score from its rank.

## Multi-Model Consensus
Use `models` only when you need cross-model agreement. Multiple models require `consensus`.

```yaml
llm_judges:
  - mode: rubric
    key: quality
    models:
      - claude-sonnet-4-6
      - gpt-4o
    consensus:
      aggregation: median
      min_agreement_threshold: 0.6
      flag_on_disagreement: true
    rubric: Rate overall quality 1-5.
```

Consensus rules:

- `aggregation`: `median`, `mean`, `majority_vote`, or `unanimous`.
- `median` and `mean` are valid only for numeric modes: `rubric`, `reference`, and `n_wise`.
- `majority_vote` is valid only for `assertion`.
- `unanimous` is valid for numeric and assertion modes.
- `min_agreement_threshold` must be between `0` and `1`.
- `flag_on_disagreement` is optional boolean.

Single-model judges must not include `consensus`.

## Judge Limits
Judge limits live under `scorecard.judge_limits`.

```yaml
scorecard:
  judge_limits:
    max_samples_per_judge: 3
    max_calls_usd: 2.50
    max_tokens: 50000
```

Validation rules:

- `max_samples_per_judge`: `0..10`; `0` means use each judge's own `samples` defaulting behavior.
- `max_calls_usd`: greater than or equal to `0`.
- `max_tokens`: greater than or equal to `0`.

Current pack validation range-checks these budget knobs. Still keep each judge's `samples` and `models` small: every judge times every sample times every model can create one LLM call.

## Scorecard Wiring
Each judge-backed dimension must point at exactly one judge key.

```yaml
scorecard:
  strategy: hybrid
  dimensions:
    - key: deterministic_correctness
      source: validators
      validators:
        - mentions_policy
      weight: 0.5
    - key: groundedness
      source: llm_judge
      judge_key: no_hallucination
      better_direction: higher
      gate: true
      pass_threshold: 1.0
    - key: helpfulness
      source: llm_judge
      judge_key: helpfulness
      weight: 0.5
```

Rules:

- `source: llm_judge` requires `judge_key`.
- `judge_key` must reference an existing `llm_judges[].key`.
- `judge_key` must be empty for non-`llm_judge` dimensions.
- `better_direction`, when present for `llm_judge`, must be `higher`.
- All current judge modes produce numeric normalized scores that can feed dimensions.
- `strategy: hybrid` requires at least one gated dimension.

## Abstention And Unavailable Results
There is no `abstention_rule`, `abstain`, or `unable_to_judge` field in the YAML authoring surface.

Judges become unavailable when:

- `context_from` evidence cannot resolve.
- `reference_from` evidence cannot resolve.
- an `n_wise` run has fewer than two run agents.
- all model calls fail or return unparsable output.
- no provider client or credential path is configured for the judge model.

An unavailable judge result has no `normalized_score`; it keeps a `reason` and payload details such as failed calls or `unable_to_judge_count`. A `source: llm_judge` dimension with no normalized score becomes unavailable.

## Result Shape
Judge-backed scorecards and replay surfaces can include `llm_judge_results` with:

- `judge_key`
- `mode`
- `normalized_score`
- `payload`
- `confidence`
- `variance`
- `sample_count`
- `model_count`
- `reason`

`payload` includes call records, model scores, aggregated score, warnings, and n-wise candidates when applicable.

## Security And Anti-Gaming
- Do not put raw secrets in `rubric`, `assertion`, `prompt`, `anti_gaming_clauses`, `context_from`, `reference_from`, or `literal:` values.
- Validation rejects `${secrets.*}` references in `rubric`, `assertion`, and `prompt`; avoid them everywhere else too.
- `anti_gaming_clauses` are additive. The evaluator still injects built-in judge instructions and default anti-gaming behavior.
- Keep evidence narrow. Judges receive resolved context text in provider requests.
- Prefer deterministic validators for hard safety, schema, file, and policy constraints; judges should score nuance.

## Common Validation Failures
- `judge_mode: deterministic` includes `llm_judges`.
- `judge_mode: llm_judge` or `hybrid` has no `llm_judges`.
- A judge `key` is empty, duplicated, or collides with a validator/metric key.
- `mode` is not `rubric`, `assertion`, `reference`, or `n_wise`.
- `rubric` mode omits `rubric`.
- `reference` mode omits `rubric` or `reference_from`.
- `assertion` mode omits `assertion`.
- `n_wise` mode omits `prompt`.
- Both `model` and `models` are set, or neither is set.
- `samples` is negative or greater than `10`.
- `models` has more than one model but no `consensus`.
- `consensus` appears on a single-model judge.
- `context_from` or `reference_from` is not a supported evidence reference.
- `score_scale.min >= score_scale.max`.
- `timeout_ms <= 0`.
- `scorecard.judge_limits` values are out of range.
- `source: llm_judge` dimension omits `judge_key` or references an unknown judge.

## Authoring Procedure
1. Keep deterministic validators for hard checks.
2. Choose the judge mode: `rubric`, `assertion`, `reference`, or `n_wise`.
3. Pick the smallest useful `context_from` evidence set.
4. Write rubric/assertion/prompt text that tells the judge how to use only that evidence.
5. Select one `model`, or use `models` plus `consensus` only when cross-model agreement matters.
6. Keep `samples` low; use `scorecard.judge_limits` for budget guardrails.
7. Wire each judge to one `source: llm_judge` scorecard dimension with `judge_key`.
8. Pair hard gates with deterministic validators or assertion judges.
9. Validate with `agentclash challenge-pack validate path/to/pack.yaml --json`.
10. Report judge keys, modes, evidence refs, model fan-out, budget settings, and any unavailable-evidence risks.

## Report Back Format
```text
Judge mode:
Judges:
- key:
  mode:
  model/models:
  samples:
  context_from:
  reference_from:
  consensus:
  score_scale:
  anti_gaming_clauses:
Scorecard dimensions:
Validator pairings:
Judge limits:
Unavailable-evidence risks:
Security review:
Validation command:
Validation result:
Open issues:
```

## Related Skills
- `agentclash-challenge-pack-scoring-validators`
- `agentclash-challenge-pack-artifacts`
- `agentclash-challenge-pack-input-sets`
- `agentclash-challenge-pack-validation-publish`
- `agentclash-eval-runner`
- `agentclash-scorecard-reader`
