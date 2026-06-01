---
name: agentclash-challenge-pack-scoring-validators
description: Use when defining deterministic AgentClash scoring validators, scorecard dimensions, evidence sources, pass/fail rules, numeric metrics, file checks, and validator result interpretation.
metadata:
  agentclash.role: challenge-pack-scoring
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack Scoring Validators

## Purpose
Design deterministic scoring that is valid, explainable, and stable enough for CI, regression, and benchmark comparisons.

Use deterministic validators when objective evidence can prove the behavior. Reach for LLM judges only when the output truly needs subjective or rubric-based assessment.

## Use When
- A pack needs `version.evaluation_spec.validators`.
- Scoring can use exact text, JSON, numeric, math, file, directory, or code-execution evidence.
- A scorecard dimension should average one or more validator results.
- A pack needs numeric run metrics such as latency, token count, tool calls, cost, or validator pass rate.
- A reviewer needs to understand why a validator passed, failed, errored, or was unavailable.

## Do Not Use When
- The challenge, cases, or artifacts are still undefined; use the planner, input-sets, and artifacts skills first.
- The evaluation needs rubric, assertion, n_wise, or reference judging; use `agentclash-challenge-pack-llm-judges`.
- The task is publishing or running an already authored pack; use validation/publish or eval-runner skills.

## Environment
Use hosted production for CLI examples unless the user intentionally targets a local or self-hosted backend.

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

`agentclash challenge-pack validate` calls the hosted API and requires auth plus a workspace. Use `agentclash link`, `--workspace`, `AGENTCLASH_WORKSPACE`, or `.agentclash.yaml` before validating.

## Validation Commands
Validate after changing validators, metrics, dimensions, strategies, file captures, or evidence references.

```bash
agentclash challenge-pack validate path/to/pack.yaml
agentclash challenge-pack validate path/to/pack.yaml --json
```

Human output prints `Challenge pack is valid` or `Challenge pack has errors`. Use `--json` for structured `valid` and `errors` fields.

## Evaluation Spec Shape
Deterministic scoring lives under `version.evaluation_spec`.

```yaml
version:
  evaluation_spec:
    name: support-answer-scoring
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: mentions_refund_window
        type: contains
        target: final_output
        expected_from: literal:30 days
    metrics:
      - key: latency_ms
        type: numeric
        collector: run_total_latency_ms
        unit: ms
    scorecard:
      strategy: weighted
      dimensions:
        - key: correctness
          source: validators
          validators:
            - mentions_refund_window
          weight: 1
```

Required fields:

- `name`: required non-empty string.
- `version_number`: required integer greater than `0`.
- `judge_mode`: `deterministic`, `llm_judge`, or `hybrid`; use `deterministic` for this skill.
- `validators`: required and must contain at least one validator.
- `scorecard.dimensions`: required and must contain at least one dimension.

Optional scoring sections used by this skill:

- `metrics`: run metric declarations.
- `post_execution_checks`: file or directory capture declarations used by file validators.
- `runtime_limits`, `pricing`, and `behavioral` exist, but use focused skills unless they are directly needed for scoring.

## Validator Fields
Every validator has this source-backed shape:

```yaml
validators:
  - key: stable_validator_key
    type: exact_match
    target: final_output
    expected_from: literal:approved
    config: {}
```

Fields:

- `key`: required, trimmed, and unique.
- `type`: required and must be one of the supported validator types below.
- `target`: required supported evidence reference.
- `expected_from`: required for most validators; omitted for `file_exists`, `file_json_schema`, `directory_structure`, `code_execution`, `tool_call_assertion`, and `postcondition`.
- `config`: optional JSON/YAML object interpreted by the validator type.

There is no validator-level `failure_message`, `pass_message`, or custom result text field. Results emit `state`, `verdict`, `normalized_score`, `reason`, `raw_output`, `target`, `expected_from`, `actual_value`, and `expected_value`; the reason text is produced by the scorer.

## Supported Validator Types
These are the exact validator type strings accepted by the scoring model:

```text
exact_match
contains
regex_match
json_schema
json_path_match
boolean_assert
fuzzy_match
numeric_match
normalized_match
token_f1
math_equivalence
bleu_score
rouge_score
chrf_score
file_content_match
file_exists
file_json_schema
directory_structure
code_execution
tool_call_assertion
postcondition
```

Do not use `has_json`, `json_equals`, `semantic_match`, `unit_test`, `shell`, or provider-specific names; the validator rejects unknown `type` values.

## Evidence References
Validator `target` and required `expected_from` values must use supported evidence references:

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
- `tool_calls` (only for `tool_call_assertion`)

## Postconditions
Use `postcondition` to score post-run captured files or directory listings without shelling out through `code_execution`. It must use `target: file:<post_execution_check_key>` and omit `expected_from`. Its strict `config.condition` supports `exists`, `not_exists`, `contains`, `not_contains`, `regex_match`, `json_path_match`, and `equals`.

## Tool Call Assertions
Use `tool_call_assertion` to score executed tool-call traces without asking the final answer to self-report behavior. It must use `target: tool_calls` and does not use `expected_from`.

```yaml
validators:
  - key: submitted_answer
    type: tool_call_assertion
    target: tool_calls
    config:
      tool_name: submit
      must_call: true
      arguments_contain:
        answer: "42"
```

Config supports `tool_name`, `must_call`, `count`, `min_count`, `max_count`, `arguments_contain`, `ordered_tools`, and `order_mode`. `order_mode` is `subsequence` by default and can be `exact`. Scorecard evidence includes counts, matched indices, and tool names, but not raw tool arguments.

Use `literal:` for inline expected values. Use `case.expectations.<key>` or `artifact.<artifact_key>.path` when the expected value should come from case evidence rather than the skill text.

## Common Text And JSON Validators
Use these when final output or case evidence is already text or JSON.

```yaml
validators:
  - key: exact_decision
    type: exact_match
    target: case.payload.expected_decision
    expected_from: literal:approve

  - key: contains_policy_term
    type: contains
    target: final_output
    expected_from: literal:refund window

  - key: matches_ticket_pattern
    type: regex_match
    target: final_output
    expected_from: literal:TICKET-[0-9]+

  - key: response_is_schema_valid
    type: json_schema
    target: final_output
    expected_from: 'literal:{"type":"object","required":["decision"],"properties":{"decision":{"type":"string"}}}'

  - key: decision_is_approved
    type: json_path_match
    target: final_output
    expected_from: 'literal:{"path":"$.decision","comparator":"equals","value":"approve"}'

  - key: escalation_flag
    type: boolean_assert
    target: case.payload.should_escalate
    expected_from: literal:true
```

`json_path_match` expected values are either a JSON object with `path`, optional `comparator`, and optional `value`, or a path string that starts with `$` for an existence check. Supported comparators are `equals`, `contains`, `greater_than`, `less_than`, and `exists`.

## Similarity, Numeric, And Math Validators
These validators accept typed `config` fields.

```yaml
validators:
  - key: answer_fuzzy
    type: fuzzy_match
    target: final_output
    expected_from: case.expectations.answer
    config:
      threshold: 0.85
      case_insensitive: true
      normalize: true

  - key: total_matches
    type: numeric_match
    target: case.payload.agent_total
    expected_from: case.expectations.expected_total
    config:
      absolute_tolerance: 0.01
      extract_number: true

  - key: normalized_phrase
    type: normalized_match
    target: final_output
    expected_from: literal:refund window is 30 days
    config:
      pipeline:
        - trim
        - lowercase
        - collapse_whitespace

  - key: token_overlap
    type: token_f1
    target: final_output
    expected_from: case.expectations.answer
    config:
      threshold: 0.75
      normalize: true
      remove_articles: true
      remove_punctuation: true

  - key: formula_equivalent
    type: math_equivalence
    target: final_output
    expected_from: literal:x^2 + 2*x + 1
    config:
      comparison_mode: symbolic
```

Source-backed config notes:

- `fuzzy_match.threshold` and `token_f1.threshold` must be between `0` and `1` when set.
- `numeric_match` accepts `absolute_tolerance`, `relative_tolerance`, `extract_number`, `significant_digits`, `tolerance_mode`, and `tolerance`; tolerances must be non-negative, and `significant_digits` must be greater than `0` when set.
- `normalized_match.pipeline` accepts `trim`, `lowercase`, `collapse_whitespace`, `strip_punctuation`, `strip_currency`, `strip_formatting`, `normalize_unicode`, `remove_articles`, `sort_words`, and `sort_lines`.
- `math_equivalence.comparison_mode` must be `symbolic` or `numeric`; `tolerance` must be non-negative.

## Generation-Style Validators
Use these for text similarity against references when exact wording is not required.

```yaml
validators:
  - key: bleu_reference_overlap
    type: bleu_score
    target: final_output
    expected_from: case.expectations.answer
    config:
      threshold: 0.4
      max_ngram: 4
      smoothing: method1

  - key: rouge_summary_overlap
    type: rouge_score
    target: final_output
    expected_from: case.expectations.answer
    config:
      threshold: 0.5
      variant: rouge-l

  - key: chrf_summary_overlap
    type: chrf_score
    target: final_output
    expected_from: case.expectations.answer
    config:
      threshold: 0.5
      char_order: 6
```

Config validation:

- `bleu_score.smoothing` must be `none` or `method1`; `max_ngram` must be greater than `0`.
- `rouge_score.variant` must be `rouge-1`, `rouge-2`, or `rouge-l`; `beta` must be greater than `0` when set.
- `chrf_score.char_order` and `chrf_score.beta` must be greater than `0` when set.

## File And Directory Validators
File validators must use `target: file:<post_execution_check_key>`. Declare the capture first with `version.evaluation_spec.post_execution_checks`.

```yaml
version:
  execution_mode: native
  tool_policy:
    allowed_tool_kinds:
      - file
      - build
  evaluation_spec:
    name: file-scoring
    version_number: 1
    judge_mode: deterministic
    post_execution_checks:
      - key: generated_summary
        type: file_capture
        path: /workspace/summary.json
      - key: project_listing
        type: directory_listing
        path: /workspace
        recursive: true
    validators:
      - key: summary_exists
        type: file_exists
        target: file:generated_summary
      - key: summary_matches_schema
        type: file_json_schema
        target: file:generated_summary
        config:
          schema:
            type: object
            required:
              - decision
      - key: no_secret_file
        type: directory_structure
        target: file:project_listing
        config:
          forbidden_files:
            - .env
      - key: summary_mentions_decision
        type: file_content_match
        target: file:generated_summary
        expected_from: literal:decision
        config:
          match_mode: contains
```

File validator rules:

- `file_content_match` requires `expected_from` and supports `match_mode`: `exact`, `contains`, `regex`, `not_contains`, or `json_equal`; default is `contains`.
- `file_exists` defaults to `must_exist: true`; set `config.must_exist: false` when the file must be absent.
- `file_json_schema` requires `config.schema`.
- `directory_structure` requires config and supports `required_files`, `forbidden_files`, and `required_directories`.
- If any validator target starts with `file:` and checks are declared, the key must match a `post_execution_checks[].key`.

## Code Execution Validator
`code_execution` is a file validator. Its `target` must reference a `file_capture`, not a `directory_listing`, and `config.test_command` is required.

```yaml
post_execution_checks:
  - key: generated_code
    type: file_capture
    path: /workspace/app.py
validators:
  - key: generated_code_tests
    type: code_execution
    target: file:generated_code
    config:
      test_command: python -m pytest tests/ -q
      timeout_ms: 30000
      scoring: fraction_passed
      pass_threshold: 0.8
```

Source-backed config:

- `test_command`: required non-empty string.
- `timeout_ms`: optional integer greater than `0`.
- `scoring`: `fraction_passed` or `all_or_nothing`; `pass_at_k` is defined but currently rejected.
- `pass_threshold`: optional number between `0` and `1`; default effective threshold is `1.0`.

## Metrics
Metrics have `key`, `type`, `collector`, and optional `unit`.

```yaml
metrics:
  - key: latency_ms
    type: numeric
    collector: run_total_latency_ms
    unit: ms
  - key: validator_rate
    type: numeric
    collector: validator_pass_rate
```

Metric `type` must be `numeric`, `text`, or `boolean`. The schema accepts `text`, but the current implemented collectors produce numeric or boolean values. The scorer currently implements these collectors:

```text
run_total_latency_ms
run_ttft_ms
run_input_tokens
run_output_tokens
run_total_tokens
run_tool_call_count
run_agent_tokens
run_race_context_tokens
run_model_cost_usd
run_completed_successfully
run_failure_count
behavioral_recovery_score
behavioral_exploration_efficiency_score
behavioral_error_cascade_score
behavioral_scope_adherence_score
behavioral_confidence_calibration_score
validator_pass_rate
```

Validation rejects `behavioral_confidence_calibration_score` for metrics until confidence reporting lands, even though the engine has a collector branch. Avoid it in new packs.

## Scorecard Dimensions
Use object-form dimensions for source-fidelity and explicit routing.

```yaml
scorecard:
  strategy: weighted
  pass_threshold: 0.8
  dimensions:
    - key: correctness
      source: validators
      validators:
        - mentions_refund_window
        - decision_is_approved
      weight: 0.8
      gate: true
      pass_threshold: 0.9
    - key: speed
      source: metric
      metric: latency_ms
      better_direction: lower
      normalization:
        target: 1000
        max: 60000
      weight: 0.2
```

Dimension fields:

- `key`: required and unique.
- `source`: `validators`, `metric`, `reliability`, `latency`, `cost`, `behavioral`, or `llm_judge`.
- `validators`: optional list of validator keys when `source: validators`; omitted means average all validators.
- `metric`: required when `source: metric` and must reference `metrics[].key`.
- `better_direction`: required for `metric`, `latency`, and `cost`; must be `higher` or `lower`.
- `normalization.target` and `normalization.max`: required for `metric`, `latency`, and `cost`.
- `weight`: optional and must be greater than or equal to `0`.
- `gate`: optional boolean.
- `pass_threshold`: required when `gate: true` or when `strategy: binary`; must be between `0` and `1`.
- `judge_key`: only valid when `source: llm_judge`; use the LLM judges skill for that path.

Strategy rules:

- Missing `strategy` defaults to `weighted`.
- `weighted`: optional scorecard-level `pass_threshold`; explicit gates are allowed.
- `binary`: every dimension is implicitly gated, every dimension needs `pass_threshold`, and scorecard-level `pass_threshold` must not be set.
- `hybrid`: requires at least one `gate: true`; gates must pass and the non-gate weighted average must clear any scorecard-level threshold.

## Result Interpretation
Validator results can be:

- `verdict: pass`: evidence was available and the validator condition passed.
- `verdict: fail`: evidence was available and the condition failed.
- `verdict: error`: evidence existed but parsing, config, regex, schema, JSONPath, or execution-result interpretation errored.
- unavailable state with no verdict: target or expected evidence could not be resolved.

Each available validator contributes `normalized_score` on a `0..1` scale. `source: validators` dimensions average the scoped validator scores; if scoped validators are unavailable, the dimension is unavailable.

## Common Validation And Scoring Failures
- `validators` is empty.
- Duplicate validator `key`.
- Unknown validator type such as `has_json`.
- Missing `target`.
- Missing `expected_from` for a validator that requires it.
- `target` or `expected_from` is not a supported evidence reference.
- A file validator targets `final_output` instead of `file:<post_execution_check_key>`.
- A `file:` target references a missing `post_execution_checks` key.
- `code_execution` targets a `directory_listing`.
- `file_json_schema` omits `config.schema`; this becomes a scoring error if it slips past pack validation.
- `directory_structure` omits `config`; this becomes a scoring error if it slips past pack validation.
- `code_execution` omits `config.test_command`; validation catches this when config is present, and scoring cannot produce a useful result without it.
- `metric` dimensions omit `normalization`.
- `binary` strategy sets scorecard-level `pass_threshold`.
- `hybrid` strategy has no gated dimension.
- A non-`llm_judge` dimension includes `judge_key`.

## Authoring Procedure
1. Identify the evidence source for each behavior: final output, case payload, case input, expectation, artifact metadata, captured file, or literal.
2. Pick the simplest supported validator type that proves the claim.
3. Add `expected_from` unless the validator type explicitly does not require it.
4. Keep file checks under `post_execution_checks` and target them with `file:<key>`.
5. Group validators into scorecard dimensions with `source: validators`.
6. Add numeric metrics only when the scorecard needs latency, cost, token, tool, completion, failure, or pass-rate signals.
7. Add gates and pass thresholds only for hard requirements.
8. Run `agentclash challenge-pack validate path/to/pack.yaml --json` and fix every returned field error.
9. Report which validators are scored, which are gates, and which evidence refs each one reads.

## Safety
- Do not put secrets in `literal:` expected values, captured files, artifact metadata, or validator config.
- Keep `file_capture` paths narrow; captured content becomes scoring evidence.
- Prefer deterministic fixture expectations over live mutable data.
- Use regex and JSONPath carefully so failures explain behavior instead of implementation trivia.
- Avoid scoring on private customer data unless retention and access are approved.

## Report Back Format
```text
Evaluation spec:
Validator summary:
- key:
  type:
  target:
  expected_from:
  config:
  score dimension:
  gate: <yes/no>
Metrics:
Scorecard:
- strategy:
- dimensions:
File captures:
Evidence references:
Validation command:
Validation result:
Expected result fields:
Open issues:
```

## Related Skills
- `agentclash-challenge-pack-input-sets`
- `agentclash-challenge-pack-artifacts`
- `agentclash-challenge-pack-tools-sandbox`
- `agentclash-challenge-pack-llm-judges`
- `agentclash-challenge-pack-validation-publish`
- `agentclash-eval-runner`
- `agentclash-scorecard-reader`
