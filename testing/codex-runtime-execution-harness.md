# codex/runtime-execution-harness — Test Contract

## Functional Behavior

- Add a shared executable runner harness in `runtime/runner` for terminal observer handling and runtime deadline context creation.
- Rewire backend executors to use the shared harness without changing hosted execution behavior.
- Preserve existing native, prompt-eval, and responses executor result/error semantics.
- Keep repository-specific execution context, Temporal, storage, sandbox implementations, and provider orchestration in backend.
- Do not add `agentclash local run`, Docker sandboxing, SQLite storage, or harness-builder behavior in this PR.

## Unit Tests

- `runtime/runner` tests cover terminal success/failure observer behavior and timeout context helpers.
- Backend engine tests continue to pass.

## Integration / Functional Tests

- `go test ./...` from `runtime/` passes.
- `go test ./...` from `backend/` passes.
- `go test ./...` from `cli/` passes if CLI module wiring is touched.

## Smoke Tests

- `go list ./...` succeeds in `runtime/`.
- `go list ./...` succeeds in `backend/`.
- A temporary external module can import and use the new `runtime/runner` execution harness.

## E2E Tests

N/A — this PR is a runner harness extraction and does not add a user-facing run mode.

## Manual / cURL Tests

N/A — no HTTP API route behavior is intentionally changed.
