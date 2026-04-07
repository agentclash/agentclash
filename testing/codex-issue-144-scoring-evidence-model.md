# codex/issue-144-scoring-evidence-model — Test Contract

## Functional Behavior
This branch broadens deterministic scoring evidence resolution from the legacy `challenge_input` / `final_output` / `literal:` model toward case-oriented references that match the generalized issue-144 contract.

- Deterministic validators can still use existing legacy sources like `final_output`, `challenge_input`, and `literal:...`.
- Deterministic validators can also reference generalized case data using path-based references such as `run.final_output`, `case.payload`, `case.inputs.<key>`, `case.expectations.<key>`, and `artifact.<key>`.
- Path-based evidence works when there is exactly one runnable case in scope, preserving compatibility with the current single-input-set execution path.
- Case input references can resolve either inline values or artifact-backed values.
- Case expectation references can resolve inline values, input-backed references, or artifact-backed references.
- Artifact references resolve from the published pack manifest's version-scoped assets and case-declared artifact refs.
- Validation rejects malformed path-based evidence references when they cannot be parsed or refer to unsupported roots.
- Existing payload-only packs continue to score correctly through normalization.
- Existing evaluation specs using `expected_from: challenge_input` continue to behave the same.

## Unit Tests
- `TestEvaluateRunAgent_ResolvesRunAndCaseEvidencePaths` — validators can compare `run.final_output` against `case.expectations.answer` and `case.inputs.prompt`.
- `TestEvaluateRunAgent_ResolvesArtifactBackedEvidencePaths` — validators can read artifact-backed case input/expectation values from the execution context manifest.
- `TestEvaluateRunAgent_KeepsLegacyChallengeInputEvidence` — legacy `challenge_input` evidence still resolves unchanged.
- `TestEvaluateRunAgent_RejectsUnsupportedEvidenceReference` — unsupported path roots fail deterministically with a clear error reason.
- `TestMapChallengeInputs_PreservesCanonicalCaseContext` — workflow-to-scoring mapping includes case keys, inputs, expectations, and artifact visibility needed by the scoring engine.

## Integration / Functional Tests
- Workflow scoring can evaluate a run using a generalized case-oriented bundle without custom test harness patches.
- Repository execution-context loading plus workflow scoring can resolve artifact-backed evidence from the stored manifest/case bundle.
- Legacy published packs remain scorable without modifying their manifests.

## Smoke Tests
- `go test ./internal/scoring ./internal/workflow`
- If repository mapping changes: `go test ./internal/repository`

## E2E Tests
N/A — not applicable for this branch. This slice is backend scoring behavior and evidence resolution.

## Manual / cURL Tests
N/A — not applicable unless an HTTP evaluation-spec or challenge-pack endpoint changes as part of implementation.
