# Worker Local Development

Purpose: give the repo one short path for starting the first Temporal worker introduced in issue `#21`.

## What exists in this step

This worker slice now covers both execution paths already implemented in Step 5:

- config bootstrapping from environment
- Postgres connection setup for workflow activities
- Temporal client connection and worker startup
- registration of `RunWorkflow`, `RunAgentWorkflow`, and the first hosted-external activity path from `backend/internal/workflow`
- `hosted_external` black-box execution via the standardized `POST /agentclash/runs` contract
- native execution through the sandbox abstraction
- E2B-backed sandbox provisioning when `SANDBOX_PROVIDER=e2b`
- polling of the `RunWorkflow` task queue used by API-started runs
- graceful shutdown for local development
- one explicit fail-closed sandbox selector: `unconfigured` or `e2b`

It does not yet provide:

- replay or scorecard generation
- durable sandbox-handle persistence
- challenge workspace bundle storage beyond the JSON execution inputs already stored in the frozen execution context

## Local startup

From the repository root:

```bash
cp backend/.env.example backend/.env
cd backend
set -a
source .env
set +a
go run ./cmd/worker
```

Or from the repository root with the make target:

```bash
make worker
```

The worker expects:

- Postgres reachable through `DATABASE_URL`
- Temporal reachable through `TEMPORAL_HOST_PORT` and `TEMPORAL_NAMESPACE`
- for native runs with a real sandbox: `SANDBOX_PROVIDER=e2b`, `E2B_API_KEY`, and `E2B_TEMPLATE_ID`

For local development, the intended setup is:

1. `make db-up`
2. `make db-migrate`
3. start a Temporal local dev server or point the env vars at a dev namespace
4. if you want native runs, publish the repo template from `backend/e2b-template/` and set `E2B_TEMPLATE_ID`
5. run `make worker`
6. separately run `make api-server`

Native sandbox staging in this issue is intentionally narrow. The worker uploads only the execution inputs that already exist in the frozen context today:

- `/workspace/agentclash/run-context.json`
- `/workspace/agentclash/challenge-pack-manifest.json`
- `/workspace/agentclash/challenges.json`
- `/workspace/agentclash/challenge-input-set.json` when present

This issue does not introduce a full challenge workspace bundle system.

## Worker env vars

- `DATABASE_URL`: Postgres connection string. Default `postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable`
- `TEMPORAL_HOST_PORT`: Temporal target. Default `localhost:7233`
- `TEMPORAL_NAMESPACE`: Temporal namespace. Default `default`
- `WORKER_IDENTITY`: Temporal worker identity string. Default `agentclash-worker@<hostname>`
- `HOSTED_RUN_CALLBACK_BASE_URL`: public API base URL used in hosted callback URLs. Default `http://localhost:8080`
- `HOSTED_RUN_CALLBACK_SECRET`: shared secret used to sign hosted callback bearer tokens. Default `agentclash-dev-hosted-callback-secret`
- `WORKER_SHUTDOWN_TIMEOUT`: graceful shutdown timeout duration. Default `10s`
- `SANDBOX_PROVIDER`: sandbox provider selector. Supported values: `unconfigured`, `e2b`. Default `unconfigured`
- `E2B_API_KEY`: E2B API key. Required when `SANDBOX_PROVIDER=e2b`
- `E2B_TEMPLATE_ID`: E2B template ID or alias for the AgentClash worker sandbox. Required when `SANDBOX_PROVIDER=e2b`
- `E2B_API_BASE_URL`: optional E2B API base URL override
- `E2B_REQUEST_TIMEOUT`: HTTP timeout for E2B API calls. Default `30s`

## Template Setup

The repo-owned E2B template lives in `backend/e2b-template/`.

It preserves the `/workspace` contract used by the engine and tests. Build and publish it with the E2B CLI, then set the resulting template ID or alias in `E2B_TEMPLATE_ID`.

The template uses E2B's TypeScript SDK, so local template builds require Node.js in addition to the E2B CLI.

## Smoke Verification

There is an env-gated smoke test for the adapter lifecycle:

```bash
cd backend
go test -tags e2bsmoke ./internal/sandbox/e2b
```

Required env vars:

- `E2B_API_KEY`
- `E2B_TEMPLATE_ID`

The smoke test verifies:

- sandbox create
- file upload
- command execution
- file download
- sandbox destroy
