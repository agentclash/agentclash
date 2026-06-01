---
name: agentclash-challenge-pack-yaml-author
description: Use when writing or editing AgentClash challenge pack YAML, including pack/version metadata, execution mode, challenges, cases, input sets, scoring blocks, tools, sandbox settings, assets, and validation handoff.
metadata:
  agentclash.role: challenge-pack-authoring
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack YAML Author

## Purpose
Write challenge-pack YAML that matches the current AgentClash parser and validators, without requiring access to the AgentClash source repo.

Use this skill after planning. The output should be a concrete YAML file plus the exact validation commands a coding agent should run before publish.

## Use When
- A challenge-pack plan needs to become valid YAML.
- An existing pack YAML needs source-compatible edits.
- A coding agent needs the exact YAML object shape for `pack`, `version`, `challenges`, `input_sets`, scoring, tools, sandbox, assets, cases, inputs, and expectations.
- The agent will later hand the file to `agentclash-challenge-pack-validation-publish`.

## Do Not Use When
- The user only has a vague eval idea; use `agentclash-challenge-pack-planner` first.
- The YAML is finished and only needs validation or publish; use `agentclash-challenge-pack-validation-publish`.
- The task is to start runs or choose deployments; use `agentclash-eval-runner` or deployment skills.

## Environment
Use hosted production unless the user intentionally points at another backend.

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

## CLI Commands
Start from the CLI template when possible, then edit the YAML.

```bash
agentclash challenge-pack init support-eval.yaml --template prompt_eval --name "Support Eval" --slug support-eval
agentclash challenge-pack init native-files.yaml --template native --name "Native Files" --slug native-files
agentclash challenge-pack validate support-eval.yaml
agentclash challenge-pack validate support-eval.yaml --json
```

Human validation output prints `Challenge pack is valid` or `Challenge pack has errors`. Use `--json` when a coding agent needs structured fields such as `valid` and `errors`.

## YAML Skeleton
These are the top-level fields accepted by the bundle parser:

```yaml
pack:
  slug: support-eval
  name: Support Eval
  family: support
  description: Evaluates concise customer support answers.

version:
  number: 1
  execution_mode: prompt_eval
  evaluation_spec:
    name: Support Eval Scoring
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: mentions_refund_policy
        type: contains
        target: final_output
        expected_from: literal:refund policy
    scorecard:
      strategy: weighted
      dimensions:
        - key: correctness
          source: validators
          weight: 1

challenges:
  - key: refund-question
    title: Refund Policy Question
    category: support
    difficulty: easy
    instructions: Answer the customer in a concise, helpful tone.

input_sets:
  - key: smoke
    name: Smoke
    description: Fast validation cases.
    cases:
      - challenge_key: refund-question
        case_key: basic-refund
        payload:
          customer_message: Can I get a refund after 14 days?
        inputs:
          - key: prompt
            kind: text
            value: Can I get a refund after 14 days?
        expectations:
          - key: expected_policy
            kind: text
            source: input:prompt
```

## Required Fields
- `pack.slug`, `pack.name`, and `pack.family` are required.
- `version.number` must be greater than zero.
- `version.execution_mode` should be explicit: use `prompt_eval` or `native`.
- `version.evaluation_spec.validators` must contain at least one validator.
- Every challenge needs `key`, `title`, `category`, and `difficulty`.
- `difficulty` must be `easy`, `medium`, `hard`, or `expert`.
- Every input set needs `key`, `name`, and at least one `cases` entry.
- Every case needs `challenge_key` referencing a declared challenge and a stable `case_key`.
- Use `cases`, not legacy `items`, in new YAML.

## Execution Modes
Use `prompt_eval` for prompt-style tasks where the agent only needs the prompt and final output.

`prompt_eval` cannot use:
- top-level `tools`
- `version.tool_policy`
- `version.sandbox`

Use `native` when the challenge needs files, tools, network policy, package installation, file validators, directory checks, code execution, or sandbox behavior.

## Native Tools And Sandbox
Only include these blocks for `native` packs.

```yaml
tools:
  custom:
    - name: lookup_order
      description: Looks up an order by ID.
      parameters:
        type: object
        properties:
          order_id:
            type: string
        required:
          - order_id
      implementation:
        primitive: http_request
        args:
          method: GET
          url: "https://example.test/orders/${order_id}"
          headers:
            Authorization: "Bearer ${secrets.ORDER_API_KEY}"

version:
  number: 1
  execution_mode: native
  tool_policy:
    allowed_tool_kinds:
      - browser
      - file
      - network
  sandbox:
    network_access: true
    network_allowlist:
      - 203.0.113.0/24
    env_vars:
      DATASET_MODE: fixture
    additional_packages:
      - jq
```

Supported `version.tool_policy.allowed_tool_kinds` values are exactly `browser`, `build`, `data`, `file`, and `network`. Do not use `shell` as an allowed tool kind.

For `tools.custom`, each tool needs `name`, `parameters`, and `implementation`. Non-`mock` implementations need `implementation.primitive` and `implementation.args`. Template placeholders in `args` use `${parameter_name}` for declared parameters and may reference `${secrets.SECRET_KEY}` when the runtime provides that secret; never paste raw secret values into YAML.

Sandbox rules:
- `network_allowlist` entries must be valid CIDR ranges.
- `env_vars` keys must look like shell env names, for example `DATASET_MODE`.
- `additional_packages` entries must be valid apt-style package names.
- Never put raw secrets in YAML. Name required secret keys in notes and configure them through the workspace/runtime/provider flow.

## Assets, Inputs, Expectations, And Artifacts
Assets may appear on `version`, challenges, or cases. Each asset list must use unique `key` values, and every asset needs `path`.

```yaml
version:
  assets:
    - key: policy_pdf
      kind: file
      path: assets/policy.pdf
      media_type: application/pdf

challenges:
  - key: summarize-policy
    title: Summarize Policy
    category: documents
    difficulty: medium
    artifact_refs:
      - key: policy_pdf

input_sets:
  - key: full
    name: Full
    cases:
      - challenge_key: summarize-policy
        case_key: policy-summary
        inputs:
          - key: source_doc
            kind: file
            artifact_key: policy_pdf
            path: assets/policy.pdf
        expectations:
          - key: summary_requirements
            kind: text
            value: Mention refund window and exclusions.
```

Case input fields are `key`, `kind`, optional `value`, optional `artifact_key`, and optional `path`.

Case expectation fields are `key`, `kind`, optional `value`, optional `artifact_key`, and optional `source`. Supported `source` values are empty, `input:<case-input-key>`, or `artifact:<version-asset-key>`, for example `source: input:prompt`.

## Evaluation Spec
`evaluation_spec` controls scoring. Keep deterministic checks deterministic, and use LLM judges only when subjective quality is genuinely needed.

```yaml
evaluation_spec:
  name: Support Eval Scoring
  version_number: 1
  judge_mode: hybrid
  validators:
    - key: has_json
      type: json_schema
      target: final_output
      expected_from: 'literal:{"type":"object","required":["answer"]}'
  llm_judges:
    - key: helpfulness
      mode: rubric
      model: gpt-4.1
      rubric: Judge whether the answer is helpful, grounded, and concise.
      context_from:
        - challenge_input
        - final_output
  scorecard:
    strategy: hybrid
    dimensions:
      - key: schema_gate
        source: validators
        validators:
          - has_json
        weight: 1
        gate: true
        pass_threshold: 1
      - key: helpfulness
        source: llm_judge
        judge_key: helpfulness
        weight: 1
```

Supported `judge_mode` values are `deterministic`, `llm_judge`, and `hybrid`.

Supported validator types include `exact_match`, `contains`, `regex_match`, `json_schema`, `json_path_match`, `boolean_assert`, `fuzzy_match`, `numeric_match`, `normalized_match`, `token_f1`, `math_equivalence`, `bleu_score`, `rouge_score`, `chrf_score`, `file_content_match`, `file_exists`, `file_json_schema`, `directory_structure`, `code_execution`, `tool_call_assertion`, and `postcondition`.

Evidence references accepted by validators and judges include `final_output`, `run.final_output`, `challenge_input`, `case.payload`, `case.payload.<path>`, `case.inputs.<path>`, `case.expectations.<path>`, `artifact.<path>`, `file:<post_execution_check_key>`, and `literal:<value>`.

File validators require a `file:` target. `code_execution` validators must target a `post_execution_checks` entry of type `file_capture`. `tool_call_assertion` validators must target `tool_calls` and omit `expected_from`. `postcondition` validators target `file:<post_execution_check_key>`, omit `expected_from`, and use `config.condition` for declarative post-run checks.

For `judge_mode: deterministic`, omit `llm_judges`. For `judge_mode: llm_judge`, include at least one judge. For `judge_mode: hybrid`, include validators and at least one judge; hybrid scorecards need a gated dimension.

For scorecard dimensions with `source: validators`, omit the `validators` list only when the dimension should score every validator. Add `validators: [<validator_key>]` when the dimension should cover a specific subset.

Do not put `${secrets.*}` references in LLM judge `rubric`, `assertion`, or `prompt` text. Secrets are allowed in native tool implementation args when the runtime provides them, but judge prompt text rejects secret references.

## Authoring Procedure
1. Start with `agentclash challenge-pack init ... --template prompt_eval` or `--template native`.
2. Fill `pack` metadata with stable slug/name/family.
3. Set `version.number: 1` for a new pack and choose `execution_mode`.
4. Write challenges before cases so every `case.challenge_key` can reference a real challenge.
5. Add input sets by run purpose: `smoke`, `ci`, `regression`, `full`, or similar.
6. Add deterministic validators first; add LLM judges only when deterministic evidence cannot capture quality.
7. Add native-only tools, sandbox, files, assets, and artifact refs only when the execution mode is `native`.
8. Run validation with and without `--json`.
9. Hand off to publication only after validation passes.

## Common Validation Failures
- Missing `pack.family`, `challenge.category`, or `case_key`.
- Case `challenge_key` does not match any challenge `key`.
- `difficulty` is not one of `easy`, `medium`, `hard`, or `expert`.
- A `prompt_eval` pack includes `tools`, `tool_policy`, or `sandbox`.
- `allowed_tool_kinds` contains unsupported values such as `shell`.
- Asset or artifact reference keys are missing, duplicated, or point at undeclared version assets.
- Case expectation `source` is not empty, `input:<case-input-key>`, or `artifact:<version-asset-key>`.
- File validators do not use a `file:` evidence target.
- `judge_mode` conflicts with the presence or absence of `llm_judges`.

## Report Back Format
```text
YAML file:
Execution mode:
Challenges:
Input sets:
Scoring mode:
Native tools/sandbox/assets:
Validation command:
Validation result:
Ready for publish: <yes/no>
Next skill: agentclash-challenge-pack-validation-publish
Open issues:
```

## Related Skills
- `agentclash-challenge-pack-planner`
- `agentclash-challenge-pack-input-sets`
- `agentclash-challenge-pack-scoring-validators`
- `agentclash-challenge-pack-llm-judges`
- `agentclash-challenge-pack-tools-sandbox`
- `agentclash-challenge-pack-artifacts`
- `agentclash-challenge-pack-validation-publish`
