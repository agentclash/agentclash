# codex/runtime-runner-boundary — Test Contract

## Functional Behavior

- Add shared runtime runner boundary types that future CLI/local/desktop code can depend on without importing `backend/internal`.
- Move backend-neutral execution contracts out of `backend/internal/engine` when they do not depend on repositories, Temporal, Postgres, API handlers, or hosted-only services.
- Keep hosted execution behavior unchanged.
- Keep backend-specific adapters and orchestration in `backend/internal`.
- Do not add `agentclash local run`, Docker sandboxing, SQLite storage, or harness-builder behavior in this PR.

## Unit Tests

- New or moved runtime runner package tests pass.
- Backend engine, worker, workflow, and API tests pass.
- Runtime tests continue to pass.

## Integration / Functional Tests

- `go test ./...` from `runtime/` passes.
- `go test ./...` from `backend/` passes.
- `go test ./...` from `cli/` passes if CLI module wiring is touched.

## Smoke Tests

- `go list ./...` succeeds in `runtime/`.
- `go list ./...` succeeds in `backend/`.
- A temporary external module can import the new runner boundary package without importing backend internals.

## E2E Tests

N/A — this PR is a library boundary extraction and does not add a new user-facing run mode.

## Manual / cURL Tests

N/A — no HTTP API route behavior is intentionally changed.
