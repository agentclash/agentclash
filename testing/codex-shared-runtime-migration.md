# codex/shared-runtime-migration — Test Contract

## Functional Behavior

- Add a public shared Go module at `runtime/` for AgentClash runtime code that can be imported by `backend`, `cli`, and future local/desktop surfaces.
- Migrate reusable runtime packages out of `backend/internal` where feasible without changing behavior.
- Preserve existing package behavior, exported types, validation rules, scoring semantics, provider behavior, sandbox contracts, and tests.
- Keep hosted/backend-specific infrastructure such as Temporal, Postgres repositories, API handlers, billing, and S3 outside the shared runtime.
- Do not include unrelated workspace files in the PR.

## Unit Tests

- `runtime/...` tests pass after migration.
- `backend/...` tests that cover migrated package callers pass, or compile if a full suite is too slow.
- Existing tests for scoring, challenge packs, datasets, providers, sandbox contracts, and engine callers continue to pass under their new import paths.

## Integration / Functional Tests

- `go test ./...` from `runtime/` passes.
- `go test ./...` from `backend/` passes or any failure is documented as unrelated to the migration with evidence.
- `go test ./...` from `cli/` passes if the CLI module is touched or if workspace module wiring affects it.

## Smoke Tests

- `go list ./...` succeeds in `runtime/`.
- `go list ./...` succeeds in `backend/`.
- The backend module imports shared runtime packages without using `backend/internal` for migrated code.

## E2E Tests

N/A — this PR is a library migration and does not add a user-facing command or runtime flow.

## Manual / cURL Tests

N/A — no HTTP API surface is changed by this migration.
