# feat/local-m1-docker-sandbox — Test Contract

Parent: #1153 (Local M1 [1/5]: Docker sandbox provider for sandbox.Provider)
Epic: #1147

## Functional Behavior

- Add a Docker implementation of `runtime/sandbox.Provider` under `runtime/sandbox/docker` with **no** Postgres, Temporal, SQLC, or `backend/internal` dependencies.
- Map container lifecycle to the shared sandbox contracts:
  - `Create` → pull (if needed) / create / start a container from a configurable image
  - `Destroy` → stop + remove the container (idempotent if already gone)
- Session ops must match what executors expect:
  - `UploadFile` / `WriteFile` / `ReadFile` / `DownloadFile` / `ListFiles`
  - `Exec` with working directory, env merge (create-time `EnvVars` + per-exec override), timeout, stdout/stderr capture + optional stream callbacks
- Default working directory is `/workspace` when the create request leaves it empty; create the directory in the container on session start.
- Inject create-time `EnvVars` into the container environment.
- When `ToolPolicy.AllowNetwork` is false, create the container with network disabled (`NetworkMode: none`). When true, use the default bridge network (full allowlist enforcement is out of scope for this issue; document as follow-up).
- When `ToolPolicy.AllowShell` is false, `Exec` of shell interpreters (`sh`, `bash`, `ash`, `zsh`, `/bin/sh`, `/bin/bash`) returns `sandbox.ErrShellNotAllowed`.
- Clear, wrap-detectable errors when the Docker daemon is missing or unreachable (`ErrDockerUnavailable`).
- Hosted E2B provider under `backend/internal/sandbox/e2b` remains unchanged and continues to compile against `runtime/sandbox`.
- Do **not** add `agentclash local run`, BYO-key wiring, harness-builder, or local UI in this PR.

## Unit Tests

- Fake/docker-less client: provider create/start/destroy lifecycle, env + working-dir setup, network mode selection, shell policy gate, post-destroy rejection (`ErrSessionDestroyed`), file read/write/list/upload/download via mocked client, exec env merge + timeout context, missing-daemon error mapping.
- `rg "backend/internal|postgres|pgx|sqlc|temporal" runtime/sandbox/docker` returns no matches in non-test production code (test fixtures may mention strings only if needed; prefer none).

## Integration / Functional Tests

- `cd runtime && go test -short -race -count=1 ./...` passes (includes new package).
- `cd backend && go test -short -race -count=1 ./internal/sandbox/e2b` still passes (E2B unchanged).

## Smoke Tests

- Opt-in live Docker smoke behind `AGENTCLASH_DOCKER_SMOKE=1` (build tag `dockersmoke` or env skip): create session → write/read file → exec `echo` → list files → destroy.
- Document how to run the smoke in the package doc comment or README snippet in the contract validation notes.

## E2E Tests

N/A — full pack eval on a laptop is #1157; this issue only ships the provider.

## Manual / cURL Tests

N/A — library-only change. Optional maintainer check:

```bash
AGENTCLASH_DOCKER_SMOKE=1 go test -tags dockersmoke -count=1 ./sandbox/docker -run TestDockerSmokeLifecycle
```

(from `runtime/`)
