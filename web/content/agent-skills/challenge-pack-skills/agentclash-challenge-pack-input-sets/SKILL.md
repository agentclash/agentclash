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
Design `input_sets` and cases that are valid, repeatable, and useful for runs, regression promotion, and CI gates.

Use this skill after the challenge pack structure is known and before scoring is finalized. The goal is to make every case observable: each case should have a stable key, a declared challenge, concrete inputs, expected evidence, and a clear reason to exist.

## Use When
- A challenge pack needs smoke, full, regression, edge-case, or CI-oriented case subsets.
- Cases exist but are poorly named, duplicated, too broad, or hard to score.
- A coding agent needs exact `input_sets[].cases[]` YAML shape without reading the AgentClash source repo.
- You need to decide which cases are safe for fast checks versus full benchmark runs.

## Do Not Use When
- The pack idea is still vague; use `agentclash-challenge-pack-planner`.
- The user needs the whole YAML file written; use `agentclash-challenge-pack-yaml-author`.
- The task is to configure validators, judges, tools, sandbox, artifacts, validation, publish, or run creation; use the focused downstream skills.

## Environment
Use hosted production for CLI examples unless the user intentionally targets a local or self-hosted backend.

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

## Validation Commands
Validate the pack after editing input sets.

```bash
agentclash challenge-pack validate path/to/pack.yaml
agentclash challenge-pack validate path/to/pack.yaml --json
```

Human output prints `Challenge pack is valid` or `Challenge pack has errors`. Use `--json` for structured `valid` and `errors` fields.

## Exact YAML Shape
The current bundle parser accepts top-level `input_sets`, each with `key`, `name`, optional `description`, and `cases`.

```yaml
input_sets:
  - key: refund-smoke
    name: Refund Smoke
    description: Fast refund-policy checks for CI and authoring smoke tests.
    cases:
      - challenge_key: refund-question
        case_key: refund-window-basic
        payload:
          customer_message: Can I get a refund after 14 days?
          account_tier: standard
        inputs:
          - key: prompt
            kind: text
            value: Can I get a refund after 14 days?
        expectations:
          - key: policy_reference
            kind: text
            source: input:prompt
```

Case fields:

- `challenge_key`: required, must reference a declared `challenges[].key`.
- `case_key`: required for new YAML, stable, and unique per challenge within the input set.
- `payload`: optional structured data for the case, such as text, JSON-like fields, IDs, or fixture metadata.
- `inputs`: optional list of concrete case inputs.
- `expectations`: optional list of expected evidence or references.
- `artifacts`: optional list of case artifact refs that must reference declared version assets.
- `assets`: optional case-local assets, each with unique `key` and required `path`.

Use `cases`, not legacy `items`, in new YAML. Legacy `items` are normalized by the parser, but new skills and packs should not author them.

## Hard Validator Rules
- Every input set needs `key`, `name`, and at least one case.
- `input_sets[].key` values must be unique.
- Every case needs `challenge_key` and `case_key`.
- Every `challenge_key` must reference a declared challenge.
- All cases inside the same input set must reference the same `challenge_key`.
- A `case_key` must be unique per challenge within that input set.
- Every case input needs unique `key` and required `kind`.
- Every case expectation needs unique `key` and required `kind`.
- `artifact_key` on inputs or expectations must reference a declared version asset.
- Expectation `source` must be empty, `input:<case-input-key>`, or `artifact:<version-asset-key>`, for example `source: input:prompt`.

Because an input set cannot mix challenge keys today, use separate input sets per challenge when designing pack-wide smoke or CI coverage:

```yaml
input_sets:
  - key: refund-smoke
    name: Refund Smoke
    cases:
      - challenge_key: refund-question
        case_key: refund-window-basic
  - key: billing-smoke
    name: Billing Smoke
    cases:
      - challenge_key: billing-question
        case_key: invoice-copy-basic
```

## Input And Expectation Patterns
Use `payload` for case data the evaluator or prompt builder should understand as structured context. Use `inputs` when individual evidence keys need to be referenced by expectations, validators, judges, or review output.

```yaml
cases:
  - challenge_key: summarize-policy
    case_key: policy-summary-edge-exclusions
    payload:
      audience: customer-support-agent
      risk: exclusion missed
    inputs:
      - key: prompt
        kind: text
        value: Summarize the policy and call out exclusions.
      - key: source_doc
        kind: file
        artifact_key: policy_pdf
        path: assets/policy.pdf
    expectations:
      - key: required_topics
        kind: json
        value:
          must_include:
            - refund window
            - exclusions
      - key: prompt_reference
        kind: text
        source: input:prompt
```

Use `value` when the expected content is inline. Use `source: input:<key>` when the expectation should refer to an input in the same case. Use `source: artifact:<key>` only for version-level assets.

## Input Set Types
Use names that describe run intent and challenge scope.

| Input set | Purpose | Typical size | Guidance |
| --- | --- | --- | --- |
| `<challenge>-smoke` | Fast sanity check | 1-3 cases | Covers the most ordinary success path and one cheap edge case. |
| `<challenge>-ci` | CI gate input set | 1-5 cases | Deterministic, stable, low-cost, no flaky external dependency. |
| `<challenge>-full` | Benchmark coverage | 5+ cases | Representative distribution across easy, medium, hard, and expert cases. |
| `<challenge>-regression` | Known failure replay | As needed | Minimal reproductions of real failures with stable expectations. |
| `<challenge>-edge` | Boundary behavior | Focused | Valid unusual inputs, ambiguity, malformed-but-recoverable payloads, or safety guardrails. |

Do not put unrelated capabilities into one input set. If a run needs multiple challenges, model that through multiple challenge-specific input sets or downstream run/eval selection, not a mixed `challenge_key` input set.

## Coverage Review
For each challenge, check:

- Happy path: ordinary user request or fixture.
- Edge path: unusual but valid input.
- Negative path: refusal, abstention, rejection, or safe fallback when appropriate.
- Ambiguous path: should ask for clarification or make a defensible assumption.
- Regression path: known previous failure with the smallest reproducible case.
- Budget path: confirms the case can run within intended time, tool, and cost limits.

Each case should have a reason. If two cases would fail for the same reason and exercise the same evidence, keep the clearer one unless you need variance.

## Stable Key Rules
- Use lowercase kebab-case keys: `refund-window-basic`, `invoice-missing-id`, `policy-summary-edge-exclusions`.
- Do not include dates, random IDs, or run IDs unless they are part of the scenario being tested.
- Keep `case_key` stable after publish; downstream results, regressions, and reports become easier to compare.
- Prefer descriptive input keys such as `prompt`, `source_doc`, `expected_schema`, or `customer_record`.
- Keep fixture IDs inside `payload`, not in the `case_key`, unless the fixture identity is the scenario.

## Regression And CI Guidance
Regression input sets should be small and forensic: they preserve the evidence needed to reproduce a known failure. Use them to seed regression suites later, but do not confuse pack `input_sets` with regression suites.

CI-oriented input sets should be deterministic. Avoid:
- live third-party data that changes without fixture control
- broad network dependency
- subjective-only expectations with no stable evidence
- large file sets when a smaller fixture proves the behavior

The eval runner can select a published input set with:

```bash
agentclash eval start --input-set <INPUT_SET_ID_OR_KEY_OR_EXACT_NAME>
```

`--scope suite_only` is for regression suite/case selection, not a replacement for `input_sets`.

## Common Validation Failures
- One input set mixes `challenge_key` values.
- The case has neither `case_key` nor legacy `item_key`.
- Duplicate `input_sets[].key`, duplicate case keys, duplicate input keys, or duplicate expectation keys.
- `challenge_key` points at a title instead of the declared challenge `key`.
- `source: input:...` references an input key that does not exist in the same case.
- `source: artifact:...` or `artifact_key` references an undeclared version asset.
- Case-local assets omit `path`.
- The case has expectations that are impossible to observe in final output, files, artifacts, or judge context.

## Authoring Procedure
1. List challenges and confirm their exact `key` values.
2. Draft cases per challenge, not across challenges.
3. Split by run intent: smoke, CI, full, regression, and edge.
4. Give every case a stable `case_key`, concrete `payload`, and clear reason.
5. Add `inputs` when expectations or scoring need named evidence.
6. Add `expectations` that reference inline `value`, `input:<key>`, or `artifact:<key>` only when those references exist.
7. Review duplicates and remove cases with no unique signal.
8. Validate the pack with `agentclash challenge-pack validate ... --json`.
9. Hand off to scoring, validation/publish, or eval runner skills.

## Report Back Format
```text
Challenge:
Input sets:
- key:
  purpose:
  case count:
  intended use: <smoke | ci | full | regression | edge>
  cases:
    - case_key:
      payload summary:
      inputs:
      expectations:
      reason:
Coverage gaps:
Validation command:
Ready for scoring: <yes/no>
Next skill:
```

## Related Skills
- `agentclash-challenge-pack-planner`
- `agentclash-challenge-pack-yaml-author`
- `agentclash-challenge-pack-artifacts`
- `agentclash-challenge-pack-scoring-validators`
- `agentclash-challenge-pack-validation-publish`
- `agentclash-eval-runner`
- `agentclash-regression-flywheel`
