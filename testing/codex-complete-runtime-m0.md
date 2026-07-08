# codex/complete-runtime-m0 — Test Contract

## Functional Behavior

- Complete the M0 runtime foundation by moving the backend-neutral frozen execution context model into `runtime/runner`.
- Keep hosted persistence, SQL mapping, Temporal workflows, API handlers, billing, storage, and E2B implementation in `backend/internal`.
- Preserve backend behavior by making repository execution-context types aliases of the shared runtime types.
- Rewire backend engine entrypoints and helpers to accept the shared runtime execution context where feasible.
- Ensure `runtime` remains independent of `backend/internal`.
- Do not add `agentclash local run`, Docker sandboxing, SQLite persistence, local UI, sync, or harness-builder behavior in this PR.

## Unit Tests

- `runtime/runner` tests cover the shared execution context model helpers.
- Backend repository and engine tests continue to compile against the alias types.
- Existing runtime tests continue to pass.

## Integration / Functional Tests

- `go test ./...` from `runtime/` passes.
- `go test ./internal/engine ./internal/repository ./internal/worker ./internal/workflow` from `backend/` passes.
- Full backend short race tests pass when practical for final validation.

## Smoke Tests

- `go list ./...` succeeds in `runtime/`.
- `go list ./...` succeeds in `backend/`.
- A temporary external module can import `runtime/runner` and construct a shared execution context without importing backend internals.
- `rg "backend/internal" runtime` returns no matches.

## E2E Tests

N/A — this is a library-boundary completion for M0 and does not add a user-facing local run mode.

## Manual / cURL Tests

N/A — no HTTP API route behavior is intentionally changed.
