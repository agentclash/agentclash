---
name: agentclash-challenge-pack-artifacts
description: Use when specifying AgentClash challenge pack assets, artifact references, produced file captures, evidence references, artifact upload/download expectations, and review-only evidence.
metadata:
  agentclash.role: challenge-pack-artifacts
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# AgentClash Challenge Pack Artifacts

## Purpose
Make challenge pack files and evidence explicit without confusing three different surfaces:

- source assets declared in challenge-pack YAML
- case artifact references exposed to scoring evidence
- files produced by the agent and captured after execution

Use this skill when a coding agent needs exact artifact fields and evidence references without reading the AgentClash source repo.

## Use When
- A pack includes fixture files, expected reports, images, logs, datasets, or other stored assets.
- Cases need `inputs[].artifact_key`, `expectations[].artifact_key`, `artifact_refs`, or case `artifacts`.
- Scoring needs `artifact...` or `file:...` evidence references.
- Native execution should capture files from the sandbox through `post_execution_checks`.
- A reviewer needs to inspect artifacts after a run.

## Do Not Use When
- The pack only needs plain text final-output scoring; use `agentclash-challenge-pack-scoring-validators`.
- The task is selecting input sets or regression suites; use `agentclash-challenge-pack-input-sets` or `agentclash-regression-flywheel`.
- The task is configuring tools, sandbox network, or package access; use `agentclash-challenge-pack-tools-sandbox`.

## Environment
Use hosted production for CLI examples unless the user intentionally targets a local or self-hosted backend.

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
```

`agentclash challenge-pack validate` is a hosted API call: authenticate first and select a workspace with `agentclash link`, `--workspace`, `AGENTCLASH_WORKSPACE`, or local `.agentclash.yaml`. Use `agentclash-cli-setup` if the machine is not already logged in.

## Validation Commands
Validate after changing any asset, artifact reference, input artifact key, expectation artifact key, or file evidence target.

```bash
agentclash challenge-pack validate path/to/pack.yaml
agentclash challenge-pack validate path/to/pack.yaml --json
```

Human output prints `Challenge pack is valid` or `Challenge pack has errors`. Use `--json` for structured `valid` and `errors` fields.

## Asset Declaration Shape
Challenge-pack assets use the same field shape at `version.assets`, `challenges[].assets`, and `input_sets[].cases[].assets`.

```yaml
version:
  assets:
    - key: policy_pdf
      kind: document
      path: assets/policy.pdf
      media_type: application/pdf
      artifact_id: 00000000-0000-0000-0000-000000000000

challenges:
  - key: summarize-policy
    title: Summarize Policy
    category: document
    difficulty: medium
    assets:
      - key: challenge_notes
        path: assets/challenge-notes.md

input_sets:
  - key: policy-smoke
    name: Policy Smoke
    cases:
      - challenge_key: summarize-policy
        case_key: summary-basic
        assets:
          - key: case_notes
            path: assets/cases/summary-basic-notes.md
```

Source-backed fields:

- `key`: required and unique inside that one `assets` list.
- `path`: required, even when `artifact_id` is present.
- `kind`: optional string.
- `media_type`: optional string.
- `artifact_id`: optional UUID for an already stored artifact.

The current validator does not enforce enums for `kind` or `media_type`; use clear, conventional values and validate the pack.

## Version Assets Versus Local Assets
Only `version.assets` form the reference pool for artifact references.

These fields must reference a declared `version.assets[].key`:

- `challenges[].artifact_refs[].key`
- `input_sets[].cases[].artifacts[].key`
- `input_sets[].cases[].inputs[].artifact_key`
- `input_sets[].cases[].expectations[].artifact_key`
- `input_sets[].cases[].expectations[].source: artifact:<key>`

Challenge-level and case-level `assets` are validated for `key` and `path`, but they are not accepted as targets for `artifact_refs`, `artifacts`, `artifact_key`, or `source: artifact:...`. If a file needs to be referenced by scoring or case evidence, declare it in `version.assets`.

## Artifact References
Use `artifact_refs` on a challenge to say which version assets matter to that challenge. Use case `artifacts` to expose version assets as artifact evidence for a case.

```yaml
version:
  assets:
    - key: policy_pdf
      path: assets/policy.pdf
      media_type: application/pdf
    - key: expected_summary
      path: assets/expected-summary.json
      media_type: application/json

challenges:
  - key: summarize-policy
    title: Summarize Policy
    category: document
    difficulty: medium
    artifact_refs:
      - key: policy_pdf

input_sets:
  - key: policy-smoke
    name: Policy Smoke
    cases:
      - challenge_key: summarize-policy
        case_key: summary-basic
        artifacts:
          - key: policy_pdf
          - key: expected_summary
        inputs:
          - key: prompt
            kind: text
            value: Summarize the refund policy.
          - key: source_document
            kind: file
            artifact_key: policy_pdf
            path: assets/policy.pdf
        expectations:
          - key: summary_reference
            kind: json
            artifact_key: expected_summary
          - key: summary_reference_via_source
            kind: json
            source: artifact:expected_summary
```

Hard validation rules:

- `artifact_refs[].key` is required, unique in that `artifact_refs` list, and must reference `version.assets`.
- Case `artifacts[].key` is required, unique in that case `artifacts` list, and must reference `version.assets`.
- `inputs[].artifact_key` and `expectations[].artifact_key` must reference `version.assets`.
- `expectations[].source` may be empty, `input:<case-input-key>`, or `artifact:<version-asset-key>`.

## Evidence References
Scoring references case and artifact evidence through supported evidence-reference strings.

Artifact evidence:

- Supported prefix shape: `artifact.<path>`.
- Do not read `<path>` as a filesystem path here. The concrete shape is `artifact.<artifact_key>[.<field>]`, where the first segment after `artifact.` is the case artifact key, for example `artifact.expected_summary`.
- Use `artifact.expected_summary.path` for the asset path.
- Use `artifact.expected_summary.media_type`, `artifact.expected_summary.kind`, or `artifact.expected_summary.key` when the metadata is what the validator or judge needs.

Case evidence:

- `case.payload`
- `case.payload.<field>`
- `case.inputs.<input_key>`
- `case.expectations.<expectation_key>`

Produced file evidence:

- `file:<post_execution_check_key>`

Use `literal:<value>` for inline expected values when a validator requires `expected_from`.

## Produced File Capture
Agent-produced files are not declared with pack `assets`. Capture them after native execution through `version.evaluation_spec.post_execution_checks`.

```yaml
version:
  execution_mode: native
  tool_policy:
    allowed_tool_kinds:
      - file
      - build
  evaluation_spec:
    name: policy-artifact-eval
    version_number: 1
    judge_mode: deterministic
    post_execution_checks:
      - key: generated_summary
        type: file_capture
        path: /workspace/summary.json
        max_size_bytes: 1048576
      - key: project_listing
        type: directory_listing
        path: /workspace
        recursive: true
    validators:
      - key: generated_summary_exists
        type: file_exists
        target: file:generated_summary
      - key: generated_summary_matches_schema
        type: file_json_schema
        target: file:generated_summary
        config:
          schema:
            type: object
    scorecard:
      dimensions:
        - key: correctness
          source: validators
          weight: 1
```

Source-backed `post_execution_checks` fields:

- `key`: required and unique.
- `type`: required and must be `file_capture` or `directory_listing`.
- `path`: required.
- `recursive`: optional boolean, useful for `directory_listing`.
- `max_size_bytes`: optional integer; must be greater than or equal to `0`.

File validators must target `file:<post_execution_check_key>`. `code_execution` validators must reference a `file_capture`, not a `directory_listing`.

## Review-Only Evidence Patterns
Use review-only evidence when humans need debugging context but scoring should not depend on it.

Good review-only patterns:

- Add a `directory_listing` check such as `project_listing` so reviewers can see what the agent created.
- Capture small logs with `file_capture` when they explain failures.
- Upload external review material with the artifact CLI and attach metadata that explains the source.
- Report artifact keys, paths, and IDs back to the user without claiming they were scored.

Keep scored evidence and review evidence separate in the report. If a file should affect the score, reference it from a validator, judge `context_from`, or judge `reference_from`. If it is only context for humans, leave it unreferenced by scoring.

## Artifact CLI Surface
Workspace artifacts are managed by the CLI, but challenge-pack YAML does not have upload or download commands inside the bundle.

```bash
agentclash artifact list
agentclash artifact list --json
agentclash artifact upload path/to/file --type reference-data
agentclash artifact upload path/to/file --type reference-data --metadata '{"purpose":"review"}' --json
agentclash artifact download <ARTIFACT_ID> --output path/to/file
```

Exact command notes:

- `agentclash artifact list` lists artifacts in the current workspace. It does not have a `--run` filter today.
- `agentclash artifact upload <file>` requires `--type`.
- Upload also accepts optional `--run <RUN_ID>`, `--run-agent <RUN_AGENT_ID>`, and `--metadata <JSON>`.
- `agentclash artifact download <artifactId>` writes to stdout by default.
- Use `--output` or `-O` to save a downloaded artifact to a file.

Use these commands for workspace artifact inspection or preloading stored artifacts. Use `version.assets[].artifact_id` only when you already have a stored artifact UUID to reference.

## Common Validation Failures
- An asset omits `key` or `path`.
- Duplicate `key` values appear inside one `assets`, `artifact_refs`, or case `artifacts` list.
- `artifact_refs[].key` points at a challenge-local or case-local asset instead of `version.assets`.
- Case `artifacts[].key`, `inputs[].artifact_key`, or `expectations[].artifact_key` points at an undeclared version asset.
- `source: artifact:...` uses an empty key or a key not declared in `version.assets`.
- A scoring validator targets `file:generated_summary` but no matching `post_execution_checks[].key` exists.
- A file validator uses `artifact.expected_summary` instead of `file:<post_execution_check_key>`.
- A `code_execution` validator targets a `directory_listing`.
- The pack uses produced-output assets under `version.assets` instead of `post_execution_checks`.

## Authoring Procedure
1. List every static file the pack needs.
2. Put any file that must be referenced by cases or scoring in `version.assets`.
3. Add challenge-local or case-local `assets` only for local organization; do not rely on them for `artifact_key` references.
4. Add `artifact_refs` to each challenge when a version asset belongs to that challenge.
5. Add case `artifacts` when scoring or review evidence should expose a version asset on that case.
6. Use `inputs[].artifact_key`, `expectations[].artifact_key`, or `source: artifact:<key>` only for declared version assets.
7. For agent-produced files, use `version.evaluation_spec.post_execution_checks`, then target them with `file:<key>`.
8. Decide which captured files are scored and which are review-only.
9. Run `agentclash challenge-pack validate path/to/pack.yaml --json`.
10. Report exact artifact keys, file paths, scored evidence references, review-only evidence, and validation result.

## Safety
- Do not include raw secret values in assets, captured files, artifact metadata, examples, chat, or commits.
- Keep captured files small and deliberate; `file_capture` content becomes scoring evidence.
- Avoid capturing broad directories unless a listing is needed for review.
- Do not store private customer data as long-lived `version.assets` unless retention and access are approved.
- Prefer deterministic fixture assets over live mutable files.

## Report Back Format
```text
Version assets:
- key:
  path:
  media_type:
  artifact_id:
Challenge artifact refs:
Case artifacts:
Input artifact keys:
Expectation artifact keys/sources:
Produced file captures:
- key:
  type:
  path:
  scored: <yes/no>
Scoring evidence refs:
Review-only evidence:
Artifact CLI commands used:
Validation command:
Validation result:
Open issues:
```

## Related Skills
- `agentclash-challenge-pack-input-sets`
- `agentclash-challenge-pack-tools-sandbox`
- `agentclash-challenge-pack-scoring-validators`
- `agentclash-challenge-pack-llm-judges`
- `agentclash-challenge-pack-validation-publish`
- `agentclash-scorecard-reader`
