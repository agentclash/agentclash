# feat/eval-sdk-1105 — Test Contract

## Functional Behavior

- An ADR at `docs/adr/predeploy-eval-sdk-repo-strategy.md` documents repository/package strategy for the pre-deploy eval SDK.
- The ADR explicitly answers: new SDK repo now vs monorepo now vs split later.
- The ADR defines package paths, shared schemas location, CLI glue ownership, and examples layout.
- The ADR lists v0 local SDK scope vs hosted AgentClash scope.
- The ADR lists non-negotiables: no auth, no telemetry, no hidden network, JSON/JUnit output, stable exit codes.
- The ADR includes a contract-sync plan between SDK packages and this repo.
- No implementation code or package publishing in this issue.

## Unit Tests

- N/A — design-only issue.

## Integration / Functional Tests

- N/A — design-only issue.

## Smoke Tests

- ADR file exists and is readable markdown.
- ADR references expected paths that align with repo conventions.

## E2E Tests

- N/A — design-only issue.

## Manual / cURL Tests

- Reviewer opens `docs/adr/predeploy-eval-sdk-repo-strategy.md` and confirms all acceptance criteria from #1105 are addressed.
