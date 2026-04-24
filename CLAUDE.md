# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

AgentClash is an open-source race engine that pits AI models against each other on real tasks with live scoring. It's a monorepo with a Go backend (API server + Temporal worker), a Go CLI (`cli/`, Cobra — also distributed as the `agentclash` npm package), and a Next.js 16 frontend.

## Commands

### Backend (Go)

```bash
make api-server                    # Run API server (localhost:8080)
make worker                        # Run Temporal worker
cd backend && go build ./...       # Build all packages
cd backend && go vet ./...         # Static analysis
cd backend && go test -short -race -count=1 ./...          # Run tests (CI mode)
cd backend && go test -short -race -count=1 ./internal/engine -run TestName  # Single test
cd backend && sqlc generate        # Regenerate DB code from SQL queries
```

### CLI (Go)

`cli/` is a separate Go module — run commands from inside that directory.

```bash
cd cli && go build ./...                                  # Build
cd cli && go vet ./...                                    # Static analysis
cd cli && go test -short -race -count=1 ./...             # Run tests
cd cli && ~/go/bin/govulncheck ./...                      # Vuln scan (install once: go install golang.org/x/vuln/cmd/govulncheck@latest)
cd cli && go run . --help                                 # Run locally against $AGENTCLASH_API_URL (default http://localhost:8080)
cd cli && go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean   # Local release rehearsal into cli/dist/
```

### CLI Against Hosted Backend

If the task is CLI-only, you usually do not need a local API server or worker. Point the local CLI at staging:

```bash
export AGENTCLASH_API_URL="https://staging-api.agentclash.dev"

cd cli
go run . auth login --device
go run . workspace list
go run . workspace use <workspace-id>
go run . run list
go run . run create --help
# When the workspace already has challenge packs and deployments:
go run . run create --follow
```

Resolution order for the API base URL is:

```text
--api-url > AGENTCLASH_API_URL > saved user config > default
  (source builds: http://localhost:8080; released binaries: https://api.agentclash.dev)
```

Useful automation / CI env vars:

```bash
export AGENTCLASH_API_URL="https://staging-api.agentclash.dev"
export AGENTCLASH_TOKEN="..."
export AGENTCLASH_WORKSPACE="workspace-id"
```

### Frontend (Next.js)

```bash
cd web && pnpm install             # Install dependencies
cd web && pnpm dev                 # Dev server (localhost:3000)
cd web && pnpm build               # Production build
cd web && pnpm lint                # ESLint
cd web && npx tsc --noEmit         # Type check
```

### Database

```bash
make db-up                         # Start PostgreSQL container
make db-migrate                    # Apply migrations
make db-seed                       # Seed dev data
make db-reset                      # Destroy + recreate database
make db-psql                       # Open psql shell
```

### Full Local Stack

```bash
./scripts/dev/start-local-stack.sh  # Starts Postgres, Temporal, API, worker
```

Requires: Go 1.25+, Docker, Temporal CLI (`brew install temporal`).

## Architecture

### Two-Plane Split

- **Control Plane** (`backend/cmd/api-server`): REST API, auth, tenancy, run submission. Synchronous operations.
- **Execution Plane** (`backend/cmd/worker`): Temporal workflows, sandbox provisioning, LLM provider calls, scoring, replay generation. Durable async operations.

The API server submits workflows but never executes them. The worker runs workflows but never serves HTTP.

### Temporal Workflow Hierarchy

`RunWorkflow` (top-level) spawns `RunAgentWorkflow` (one per agent) as child workflows. All DB mutations happen inside Temporal activities, never in workflow code directly. Workflow functions are pure and deterministic.

Error types matter: activities wrap errors with `temporal.ApplicationError` using specific type strings (`repository.ErrInvalidTransition`, `provider.FailureCodeAuth`, etc.) that workflows check to decide retry vs fail vs skip.

### Run State Machine

Defined in `backend/internal/domain/run.go`. Transitions are enforced in both Go code (`CanTransitionTo`) and SQL (`WHERE status = @from_status`). If an UPDATE returns 0 rows, the repository wraps it as `ErrInvalidTransition`.

### Manager/Service Pattern

API handlers delegate to manager structs that encapsulate authorization + business logic:

```
Handler → Manager (auth check + business logic) → Repository (DB access)
```

Authorization is data-aware — it happens in the manager after loading the resource, not in middleware. Managers are constructor-injected and wired in `cmd/api-server/main.go`.

### Provider Router

`backend/internal/provider/` uses an adapter pattern. Each LLM provider (OpenAI, Anthropic, Gemini, xAI, OpenRouter, Mistral) implements the `Client` interface. The `Router` dispatches by provider name. Providers normalize tool call shapes and classify errors into provider-agnostic failure codes. Adapter registrations are wired in `backend/cmd/worker/main.go`.

### Sandbox Abstraction

`backend/internal/sandbox/sandbox.go` defines a `Provider` interface. E2B is the current implementation, but it's replaceable. When `SANDBOX_PROVIDER=unconfigured`, a noop provider is used (runs queue but don't execute).

### SQLC Code Generation

All DB queries live in `backend/db/queries/*.sql`. Run `cd backend && sqlc generate` to regenerate `backend/internal/repository/sqlc/`. Config is in `backend/sqlc.yaml`. UUIDs use `github.com/google/uuid`. Never write raw SQL in Go code outside the repository layer.

### Event Model

Events are immutable, schema-versioned envelopes (`backend/internal/runevents/`). Metadata stored in `run_events` table; large payloads go to S3. Sequence numbers assigned by the DB on INSERT. Events are persisted in real-time (not batched), enabling live tailing via Redis pub/sub → WebSocket.

### Frontend API Client

`web/src/lib/api/client.ts` is environment-aware: uses `API_URL` server-side (internal network) and `NEXT_PUBLIC_API_URL` client-side. Path alias: `@/*` maps to `web/src/*`.

### Auth

Production uses WorkOS AuthKit. Local dev uses `AUTH_MODE=dev` which reads `X-Dev-User-ID` header — no setup needed. Optional services use noop implementations when unconfigured.

## Key Conventions

- Go module paths: `github.com/agentclash/agentclash/backend` and `github.com/agentclash/agentclash/cli`. Two separate modules — migrations and refactors must touch both and run builds/tests from each directory.
- License: **FSL-1.1-MIT** (Functional Source License, MIT Future). Not MIT, not OSI-approved. Any metadata that names the license must say `FSL-1.1-MIT`; npm `package.json` uses `"SEE LICENSE IN LICENSE"` because the SPDX list doesn't cover FSL yet. Do not "fix" that string to `"MIT"` or an SPDX identifier — it will misrepresent the license.
- Migrations: goose format (`-- +goose Up` / `-- +goose Down`) in `backend/db/migrations/`
- All dependencies are constructor-injected; no global state
- Optional services default to noop implementations (see `newRouter()` in routes.go)
- Tool visibility per run is determined by the challenge pack's tool policy, not the LLM provider
- Hosted external agents integrate via Temporal workflow signals (API callback → signal → workflow resumes)

## OpenAPI Specification

The backend API spec lives at `docs/api-server/openapi.yaml` (OpenAPI 3.1).

When you add, modify, or remove a backend API route in `backend/internal/api/`:

1. Update the corresponding path in `docs/api-server/openapi.yaml`
2. If the request or response shape changed, update the matching schema under `components/schemas/`
3. If a new status enum value was added in `backend/internal/domain/`, update the corresponding enum in the spec
4. Validate: `npx @redocly/cli lint docs/api-server/openapi.yaml`

### Route-to-spec mapping

- Routes are registered in `backend/internal/api/routes.go`
- Go struct `createRunResponse` maps to OpenAPI schema `CreateRunResponse`
- Go struct `getRunResponse` maps to OpenAPI schema `RunDetail`
- Naming convention: Go camelCase struct name -> OpenAPI PascalCase schema name

### What is NOT in the spec

- Next.js waitlist routes (`web/src/app/api/waitlist/`) — separate frontend service
- Internal Temporal workflow contracts — not HTTP APIs

## CLI Distribution

The `agentclash` CLI ships through four channels from a single GoReleaser build: GitHub Releases, Homebrew cask, POSIX/PowerShell install scripts, and npm.

- Release trigger: any `v*` tag runs `.github/workflows/release-cli.yml`. GoReleaser builds and uploads release assets first; the `publish-npm` job then assembles and publishes platform + root npm packages with Trusted Publishing + `--provenance`. A `smoke-npm` matrix installs on ubuntu/macOS/Windows and asserts `agentclash version`.
- npm layout: a root `agentclash` wrapper (`npm/cli/`) plus six `@agentclash/cli-<os>-<arch>` platform packages wired via `optionalDependencies` (no postinstall). Binaries come from `scripts/publish-npm/assemble.mjs` reading GoReleaser's `dist/`; idempotent republish on rerun via `scripts/publish-npm/publish-one.sh`.
- Full maintainer recipe (first-publish bootstrap for Trusted Publishing, local rehearsal, uninstall) lives in `docs/cli-distribution.md`.

### CLI Test and Release Commands

Before a CLI release, run:

```bash
cd cli
go build ./...
go vet ./...
go test -short -race -count=1 ./...
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```

If packaging changed, rehearse npm locally:

```bash
node scripts/publish-npm/assemble.mjs v0.0.0-rehearse cli/dist
for p in npm-out/platforms/*/ npm-out/cli; do
  (cd "$p" && npm pack --dry-run)
done
```

Routine npm release path:

1. Land a releasable CLI change under `cli/` on `main` with a conventional commit (`fix:`, `feat:`, `feat!:`).
2. Wait for Release Please to open `chore(main): release x.y.z`.
3. Merge the release PR.
4. Let `.github/workflows/release-cli.yml` publish npm automatically.

Do not manually `npm publish` or create ad hoc release tags for routine releases.
