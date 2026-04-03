# codex/issue-144-generalized-challenge-cases — Test Contract

## Functional Behavior
This change generalizes challenge pack authoring, persistence, and execution-context loading from narrow `input_sets[].items[].payload` semantics toward a richer case-oriented model while preserving existing packs.

- Challenge packs can define reusable pack-version `artifacts` referenced by challenges and cases.
- Challenge packs can define `cases` as the canonical execution/evaluation unit, with each case bound to exactly one challenge.
- Cases support typed `inputs` rather than relying on one untyped payload blob.
- Cases support separate `expectations` rather than overloading agent-facing input payload as the evaluation oracle.
- Existing payload-only packs continue to validate, publish, and load through normalization into the broadened internal model.
- Pack validation rejects bad artifact references, malformed case shapes, duplicate keys, and invalid typed input/expectation declarations with field-specific errors.
- Publishing preserves the manifest-centric source of truth while adding whatever minimum persistence/read-model support is needed for case semantics.
- Execution context exposes a canonical materialized case bundle broad enough for future scoring/runtime expansion without breaking current runnable-pack flows.
- Current evaluation-spec behavior remains intact unless a broadened reference path is explicitly implemented and validated.
- Out of scope for this branch:
  - custom validator code
  - live API-backed fixtures
  - managed retrieval infrastructure
  - multimodal runtime execution
  - unrelated runtime/platform redesign

## Unit Tests
- `TestParseYAML_NormalizesLegacyInputItemsIntoCases` — legacy `input_sets/items/payload` authoring is normalized into the broadened internal model.
- `TestParseYAML_PreservesExplicitCasesArtifactsAndExpectations` — explicit case-oriented authoring decodes and normalizes correctly.
- `TestValidateBundle_RejectsDuplicateArtifactKeys` — duplicate artifact keys fail with field-specific validation errors.
- `TestValidateBundle_RejectsCaseArtifactReferenceMisses` — case references to missing artifacts fail validation.
- `TestValidateBundle_RejectsExpectationReferenceMisses` — expectation references to unknown inputs/artifacts fail validation.
- `TestValidateBundle_RejectsCaseShapeViolations` — missing case key/challenge binding/typed input metadata fails validation.
- `TestManifestJSON_PreservesGeneralizedContract` — manifest generation serializes the broadened contract consistently.

## Integration / Functional Tests
- Repository publish flow persists a generalized pack bundle without manual DB edits.
- Existing payload-only bundle publish flow still succeeds unchanged.
- Run execution-context loading returns the canonical materialized case bundle for generalized packs.
- Existing run-loading paths remain compatible for legacy packs.

## Smoke Tests
- `go test ./internal/challengepack ./internal/repository`
- If scoring/evaluation-spec code is touched: `go test ./internal/scoring`
- If execution-context loading is touched: repository integration tests covering run-agent execution context pass.

## E2E Tests
N/A — not applicable for this branch. This slice is contract/persistence/loading work, not a full UI/user-journey feature.

## Manual / cURL Tests
N/A — not applicable unless HTTP authoring endpoints require changes during implementation. If they do, add concrete validation/publish requests before final review.
