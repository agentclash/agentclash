---
name: agentclash-challenge-pack-planner
description: Use when turning a vague AgentClash evaluation idea into a source-backed challenge pack plan with task boundaries, target agents, cases, input sets, scoring strategy, tools, artifacts, runtime policy, validation criteria, and handoff steps.
metadata:
  agentclash.role: challenge-pack-planning
  agentclash.version: "1"
  agentclash.requires_cli: "false"
---

# AgentClash Challenge Pack Planner

## Purpose
Turn an eval idea into a concrete challenge-pack plan before anyone writes YAML.

Use this skill to produce the planning artifact that downstream skills can convert into an AgentClash challenge pack. Planning does not require CLI access, but the plan must match the current challenge-pack model: `pack`, `version`, optional top-level `tools`, `challenges`, and `input_sets`.

## Use When
- A user describes a benchmark, regression idea, eval suite, or "can we test this agent?" scenario.
- The workload needs boundaries before YAML authoring: what behavior is tested, what cases exist, what counts as success, and which tools/files are allowed.
- A coding agent needs enough AgentClash product context to plan a pack without reading the AgentClash source repo.
- You need a handoff to `agentclash-challenge-pack-yaml-author`, `agentclash-challenge-pack-input-sets`, `agentclash-challenge-pack-scoring-validators`, or `agentclash-challenge-pack-llm-judges`.

## Do Not Use When
- The user already has a finished plan and wants valid YAML; use `agentclash-challenge-pack-yaml-author`.
- The task is only to validate or publish a YAML file; use `agentclash-challenge-pack-validation-publish`.
- The task is to choose deployments or start runs; use `agentclash-agent-deployment-setup` or `agentclash-eval-runner`.
- The user needs CLI installation, auth, workspace linking, or hosted setup; use `agentclash-cli-setup`.

## Inputs Needed
- Evaluation goal: the behavior, capability, or failure mode the pack should expose.
- Target agent class: coding agent, support bot, research agent, workflow agent, extraction agent, etc.
- Good, bad, and borderline outputs.
- Expected evidence: final text, JSON fields, captured files/directories, artifacts, metrics, latency, cost, or judge rationale.
- Case inventory: representative, edge, adversarial, regression, and smoke examples.
- Execution needs: `prompt_eval` versus `native`, tools, network, files, packages, secrets, and time budget.
- Release intent: exploration, regression suite, CI gate, public comparison, or customer demo.

## Planning Procedure
1. State the pack boundary in one sentence: what is being tested and what is explicitly out of scope.
2. Choose execution mode:
   - `prompt_eval` for prompt-style tasks that do not need pack-defined tools, sandbox config, or native file/tool execution.
   - `native` when the agent must use files, tools, sandbox policy, network, packages, artifacts, or code/file validators.
3. Define one or more `challenges`. Each challenge should have a stable `key`, title, category, difficulty (`easy`, `medium`, `hard`, or `expert`), and instructions.
4. Design cases before scoring. For each case, define a stable `case_key`, the `challenge_key` it targets, concrete inputs, expected outputs or expectations, and why the case exists.
5. Group cases into `input_sets`: at minimum `smoke` or `default`; add `full`, `regression`, or `ci` only when their purpose and budget differ. Each input set must contain cases for a single `challenge_key`; split mixed-challenge suites into separate input sets.
6. Pick evidence sources. Decide whether success is visible in `final_output`, structured JSON, files, artifacts, tool behavior, metrics, or LLM-judge rationale.
7. Choose scoring:
   - deterministic validators for exact, regex, JSON, numeric, token overlap, math, file, directory, or code-execution checks.
   - LLM judges for subjective quality where deterministic checks cannot honestly capture the behavior.
   - hybrid scoring when hard gates and qualitative judgment both matter.
8. Decide runtime policy only if needed: allowed tool kinds, sandbox network access, package needs, file assets, and secrets. Keep the policy as narrow as the workload allows.
9. Define publish criteria: what must be true before the pack can be validated, published, and used in an eval run.
10. Produce a handoff plan naming the next skill and the missing information, if any.

## Challenge Pack Model
Use these product nouns consistently:

- `pack`: human metadata such as `slug`, `name`, `family`, and optional description.
- `version`: executable version data: `number`, `execution_mode`, `evaluation_spec`, and optional `tool_policy`, `filesystem`, `sandbox`, and `assets`.
- `tools`: optional top-level pack-defined composed tools. Do not plan these for `prompt_eval`.
- `challenges`: task definitions. Cases reference them by `challenge_key`.
- `input_sets`: named groups of runnable cases.
- `cases`: concrete workload items with `case_key`, `payload`, `inputs`, `expectations`, case `artifacts`, and case-local `assets` as needed.
- `evaluation_spec`: score contract with `judge_mode`, validators, optional metrics or LLM judges, runtime limits, pricing, and scorecard dimensions.

## Planning Heuristics
- Prefer fewer, sharper challenges over a broad pack that mixes unrelated behaviors.
- Prefer small smoke sets that fail fast and full sets that measure coverage.
- Each case should teach the evaluator something unique. Duplicate cases need a reason, such as variance or regression coverage.
- Make expectations observable. "Looks good" is not enough; specify the evidence path and what a pass means.
- Use deterministic validators for hard facts, schemas, files, and code behavior. Use LLM judges for judgement calls like helpfulness, prioritization, style, tradeoff quality, or incident reasoning.
- Use `native` only when the task truly needs sandbox/files/tools. Simpler packs are easier to validate and reuse.
- Do not put raw secrets in the plan. Name the required secret keys and say they must be provided through workspace secrets or runtime/provider configuration.
- Do not invent IDs. Planning should name resources by role until validation/publish creates real IDs.

## Execution Mode Decision Table
| Need | Plan |
| --- | --- |
| Single prompt and final text answer | `prompt_eval` |
| Structured extraction from text input | `prompt_eval` unless file tooling is required |
| Agent must read/write files, run tests, or produce artifacts | `native` |
| Pack-defined custom tools | `native` |
| Network access or extra packages | `native` with explicit sandbox policy |
| Code, file, directory, or artifact-backed validators | `native` |
| Pure qualitative grading | `prompt_eval` or `native`, plus `llm_judge` depending on execution needs |

## Scoring Plan Shape
For each scoring dimension, specify:

```text
Dimension: <stable key>
Source: validators | metric | reliability | latency | cost | behavioral | llm_judge
Evidence: <final_output | file:path | artifact key | metric collector | judge key>
Pass rule: <threshold, gate, or qualitative rubric>
Failure message: <what should be reported when this fails>
```

Use `judge_mode: deterministic` when validators and metrics are sufficient. Use `judge_mode: llm_judge` when judges are the main grading surface. Use `judge_mode: hybrid` when deterministic gates and LLM-judge dimensions both matter.

## Case Coverage Checklist
- Happy path: the most ordinary success case.
- Edge case: unusual but valid input.
- Negative or guardrail case: input that should be rejected, abstained from, or handled safely.
- Ambiguity case: forces prioritization or asks for clarification when appropriate.
- Regression case: a known previous failure, with the evidence that should prevent recurrence.
- Budget case: confirms the pack can run within intended time, tool, and cost limits.

## Tool, Sandbox, And Artifact Planning
Only include these when the workload needs them.

- Allowed tool kinds in `version.tool_policy.allowed_tool_kinds` must use supported broad kinds such as `browser`, `build`, `data`, `file`, and `network`.
- `version.sandbox.network_access` should stay false unless the task needs outbound network.
- `version.sandbox.network_allowlist` should be specific when network is needed.
- `version.sandbox.additional_packages` should name only packages required by the workload or validators.
- Version, challenge, and case assets should have stable `key` and `path`; artifact-backed assets also need an `artifact_id` after upload.
- Case expectations can use `value`, `artifact_key`, or `source`. Supported `source` values are empty, `input:<case-input-key>`, or `artifact:<version-asset-key>`.

## Output Format
```text
Pack name:
Slug/family:
Goal:
Out of scope:
Target agent:
Execution mode: <prompt_eval | native>

Challenges:
- key:
  title:
  category:
  difficulty:
  instructions summary:

Input sets:
- key:
  purpose:
  cases:
    - case_key:
      challenge_key:
      inputs:
      expectations:
      reason:

Scoring:
- dimension:
  judge mode:
  validators:
  llm judges:
  gates/thresholds:
  evidence:

Runtime policy:
Tools:
Assets/artifacts:
Secrets:
Publish criteria:
Risks/blockers:
Next skill:
```

## Failure Modes
- The plan has no concrete cases: ask for examples or create explicit draft cases from the user's scenario.
- Cases are not tied to a `challenge_key`: add the missing challenge structure before YAML authoring.
- The plan says `prompt_eval` but needs tools, sandbox, files, or network: switch to `native`.
- The scoring is subjective but only uses exact validators: add an LLM judge or rewrite the expected output into deterministic evidence.
- The scoring is objective but only uses LLM judges: replace with validators where possible.
- Input sets mix smoke, regression, and full benchmark cases without purpose: split them by run intent.
- The plan depends on secrets or private data: name secret keys and artifact roles, not raw values.

## Report Back Format
```text
Planned pack: <name>
Execution mode: <prompt_eval | native>
Challenge count: <n>
Case count: <n>
Input sets: <keys>
Scoring mode: <deterministic | llm_judge | hybrid>
Needs tools/sandbox: <yes/no + why>
Needs assets/artifacts: <yes/no + what>
Needs secrets: <yes/no + names only>
Ready for YAML authoring: <yes/no>
Next skill: <agentclash-challenge-pack-yaml-author | other>
Open questions: <blocking details>
```

## Related Skills
- `agentclash-cli-setup`
- `agentclash-challenge-pack-yaml-author`
- `agentclash-challenge-pack-input-sets`
- `agentclash-challenge-pack-tools-sandbox`
- `agentclash-challenge-pack-artifacts`
- `agentclash-challenge-pack-scoring-validators`
- `agentclash-challenge-pack-llm-judges`
- `agentclash-challenge-pack-validation-publish`

## Related Docs
- `/docs-md/concepts/challenge-packs-and-inputs`
- `/docs-md/guides/write-a-challenge-pack`
- `/docs-md/concepts/tools-network-and-secrets`
- `/docs-md/concepts/artifacts`
- `/docs-md/reference/cli`
