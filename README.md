# AgentClash

Opensource race engine. Pit your models against each other on real tasks. Same tools, same constraints, scored live — not benchmarks, not vibes.

**[agentclash.dev](https://www.agentclash.dev)**

## What is this?

AgentClash puts AI models on the same real task, at the same time. Scored live on completion, speed, token efficiency, and tool strategy. Step-by-step replays show exactly why one agent won and another didn't.

- Head-to-head races
- Composite scoring
- Full replays
- Failure-to-eval flywheel

## How it works

1. Define a challenge (broken code, a build task, etc.)
2. Drop in your models (OpenAI, Anthropic, Gemini, OpenRouter, Mistral)
3. Run the race — same tools, same constraints
4. See scored results with full step-by-step replays

## Architecture

AgentClash is a monorepo with three main components:

| Component | Tech | Location |
|-----------|------|----------|
| **API Server** | Go / chi | `backend/cmd/api-server` |
| **Worker** | Go / Temporal SDK | `backend/cmd/worker` |
| **CLI** | Go / Cobra | `cli/` |
| **Web** | Next.js 16 / React 19 | `web/` |

Infrastructure dependencies:

| Service | Purpose |
|---------|---------|
| **PostgreSQL 17** | Source of truth for all state |
| **Temporal** | Durable workflow orchestration for run execution |
| **Redis** (optional) | WebSocket fanout, rate limiting |
| **E2B** (optional) | Sandboxed code execution for native agent runs |
| **S3-compatible storage** (optional) | Artifact storage (filesystem fallback for dev) |

## CLI

The `agentclash` CLI lets you manage everything from your terminal — runs, builds, deployments, comparisons, and infrastructure.

### Install

```bash
# Homebrew
brew install agentclash/tap/agentclash

# Go
go install github.com/Atharva-Kanherkar/agentclash/cli@latest

# Shell script
curl -sSL https://raw.githubusercontent.com/agentclash/agentclash/main/scripts/install/install.sh | sh

# From source
cd cli && make build
```

### Quick start

```bash
agentclash auth login                  # Authenticate
agentclash workspace use <id>          # Set default workspace
agentclash run create \                # Start an evaluation
  --challenge-pack-version <id> \
  --deployments <id1>,<id2> \
  --follow
agentclash run ranking <run-id>        # View results
agentclash compare gate \              # CI/CD quality gate
  --baseline <run1> --candidate <run2>
```

### CI/CD

All commands work non-interactively with environment variables:

```bash
export AGENTCLASH_TOKEN="your-token"
export AGENTCLASH_WORKSPACE="your-workspace-id"
agentclash run list --json
agentclash compare gate --baseline $BASE --candidate $CAND  # exit 1 = regression
```

Run `agentclash --help` for the full command reference.

## Local development

### Prerequisites

- **Go 1.25+** — [go.dev/dl](https://go.dev/dl/)
- **Docker** — for PostgreSQL
- **Temporal CLI** — `brew install temporal` or [docs.temporal.io/cli](https://docs.temporal.io/cli)
- **Node.js 20+** and **pnpm** — for the web frontend (optional)
- **psql** — PostgreSQL client for running migrations

### 1. Start everything (one command)

The quickest way to get the full stack running locally:

```bash
./scripts/dev/start-local-stack.sh
```

This starts PostgreSQL, applies migrations, launches the Temporal dev server, API server, and worker. Logs are written to `/tmp/agentclash-local-stack/`.

### 2. Start services individually

If you prefer more control, start each component separately:

#### Database

```bash
# Start PostgreSQL (Docker)
make db-up

# Apply schema migrations
make db-migrate

# (Optional) Seed development data
make db-seed
```

The default connection string is `postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable`. Override it with the `DATABASE_URL` environment variable.

#### Temporal

Start the Temporal dev server on the default port:

```bash
temporal server start-dev --namespace default
```

The API server and worker connect to `localhost:7233` by default. Override with `TEMPORAL_HOST_PORT`.

#### API Server

```bash
make api-server
```

The server starts on `:8080`. Verify with:

```bash
curl http://localhost:8080/healthz
```

#### Worker

```bash
make worker
```

The worker connects to both PostgreSQL and Temporal to execute run workflows.

### 3. Web frontend (optional)

```bash
cd web
pnpm install
pnpm dev
```

The dev server starts at `http://localhost:3000`.

### Environment variables

Copy the example and fill in any keys you need:

```bash
cp backend/.env.example backend/.env
```

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable` | PostgreSQL connection string |
| `API_SERVER_BIND_ADDRESS` | `:8080` | API server listen address |
| `TEMPORAL_HOST_PORT` | `localhost:7233` | Temporal server address |
| `TEMPORAL_NAMESPACE` | `default` | Temporal namespace |
| `HOSTED_RUN_CALLBACK_BASE_URL` | `http://localhost:8080` | Base URL for hosted agent callbacks |
| `HOSTED_RUN_CALLBACK_SECRET` | dev default | Secret for callback auth |
| `WORKER_IDENTITY` | hostname-based | Worker instance identifier |
| `SANDBOX_PROVIDER` | `unconfigured` | `unconfigured` or `e2b` |
| `E2B_API_KEY` | — | Required if `SANDBOX_PROVIDER=e2b` |
| `E2B_TEMPLATE_ID` | — | Required if `SANDBOX_PROVIDER=e2b` |
| `ARTIFACT_STORAGE_BACKEND` | `filesystem` | `filesystem` or `s3` |
| `ARTIFACT_SIGNING_SECRET` | auto-generated in dev | Required in production (min 32 bytes) |
| `APP_ENV` | `development` | `development`, `staging`, or `production` |

Provider API keys (set whichever you need):

| Variable | Provider |
|----------|----------|
| `OPENAI_API_KEY` | OpenAI |
| `ANTHROPIC_API_KEY` | Anthropic |
| `GEMINI_API_KEY` | Google Gemini |
| `OPENROUTER_API_KEY` | OpenRouter |
| `MISTRAL_API_KEY` | Mistral |

### Smoke tests

After the local stack is running:

```bash
# Seed fixture data for a test run
./scripts/dev/seed-local-run-fixture.sh

# Create a run via curl
./scripts/dev/curl-create-run.sh
```

> **Note:** Without a real sandbox provider (e.g. E2B), native runs will be queued but won't execute the model-backed path.

## Deploying to Railway

AgentClash uses a multi-service Railway project with two environments: **staging** and **production**.

### Services overview

You need to deploy these Railway services:

| Railway Service | What it runs | Build arg |
|-----------------|--------------|-----------|
| **api-server** | REST API + WebSocket | `TARGET=api-server` |
| **worker** | Temporal worker | `TARGET=worker` |
| **PostgreSQL** | Database (Railway plugin) | — |

External services (not on Railway):

| Service | Notes |
|---------|-------|
| **Temporal Cloud** | Use [cloud.temporal.io](https://cloud.temporal.io) for staging/prod. Self-hosting Temporal on Railway is not recommended for production. |
| **Vercel** | Deploy the `web/` frontend on Vercel. |
| **E2B** | Sign up at [e2b.dev](https://e2b.dev) if you need sandboxed execution. |
| **S3** | Any S3-compatible provider (AWS S3, Cloudflare R2, etc.) for artifact storage. |

### Step-by-step setup

#### 1. Create the Railway project

```bash
# Install the Railway CLI
brew install railwayapp/tap/railway

# Login
railway login

# Create a new project
railway init
```

#### 2. Create environments

In the Railway dashboard, create two environments for your project:
- **staging**
- **production**

All services and databases are duplicated per environment automatically.

#### 3. Add PostgreSQL

In the Railway dashboard, click **+ New** → **Database** → **PostgreSQL**. Railway provisions the database and exposes a `DATABASE_URL` variable automatically.

#### 4. Deploy the API server

Create a new service in Railway:

- **Source:** your GitHub repo
- **Root directory:** `backend`
- **Build:** Dockerfile
- **Build args:** `TARGET=api-server`

Set these environment variables (per environment):

```
APP_ENV=staging                          # or "production"
DATABASE_URL=${{Postgres.DATABASE_URL}}  # Railway variable reference
TEMPORAL_HOST_PORT=<your-temporal-cloud-host>:7233
TEMPORAL_NAMESPACE=<your-namespace>
ARTIFACT_SIGNING_SECRET=<random-64-char-hex>
ARTIFACT_STORAGE_BACKEND=s3              # or "filesystem" for staging
ARTIFACT_STORAGE_BUCKET=<your-bucket>
ARTIFACT_STORAGE_S3_REGION=<region>
ARTIFACT_STORAGE_S3_ACCESS_KEY_ID=<key>
ARTIFACT_STORAGE_S3_SECRET_ACCESS_KEY=<secret>
HOSTED_RUN_CALLBACK_BASE_URL=https://<your-api-domain>
HOSTED_RUN_CALLBACK_SECRET=<random-secret>
```

Set the deploy command to run migrations before starting:

```
/migrate.sh && /app
```

Or set it as the **Start command** in the Railway service settings.

#### 5. Deploy the Worker

Create another service in Railway from the same repo:

- **Root directory:** `backend`
- **Build args:** `TARGET=worker`

Set environment variables:

```
DATABASE_URL=${{Postgres.DATABASE_URL}}
TEMPORAL_HOST_PORT=<your-temporal-cloud-host>:7233
TEMPORAL_NAMESPACE=<your-namespace>
HOSTED_RUN_CALLBACK_BASE_URL=https://<your-api-domain>
HOSTED_RUN_CALLBACK_SECRET=<same-secret-as-api>
SANDBOX_PROVIDER=e2b                     # or "unconfigured"
E2B_API_KEY=<your-e2b-key>
E2B_TEMPLATE_ID=<your-template>
OPENAI_API_KEY=<key>
ANTHROPIC_API_KEY=<key>
GEMINI_API_KEY=<key>
```

#### 6. Deploy the Web frontend

The Next.js frontend (`web/`) is best deployed on **Vercel**:

```bash
cd web
vercel --prod
```

Set `NEXT_PUBLIC_API_URL` to point to your Railway API server domain.

### Staging vs Production

| Concern | Staging | Production |
|---------|---------|------------|
| `APP_ENV` | `staging` | `production` |
| Temporal | Temporal Cloud (staging namespace) | Temporal Cloud (production namespace) |
| Artifacts | `filesystem` or S3 test bucket | S3 production bucket |
| Sandbox | `unconfigured` (optional) | `e2b` |
| Domain | `staging-api.agentclash.dev` | `api.agentclash.dev` |
| Signing secret | Unique per env | Unique per env |

### Running migrations

Migrations run automatically if you set the start command to `/migrate.sh && /app` on the api-server service. To run them manually:

```bash
railway run --service api-server -- /migrate.sh
```

## Project structure

```
backend/
  cmd/
    api-server/          — HTTP API entrypoint
    worker/              — Temporal worker entrypoint
  db/
    migrations/          — SQL schema migrations
    queries/             — sqlc query definitions
  internal/
    api/                 — HTTP handlers, managers, auth
    repository/          — Database access (sqlc generated)
    provider/            — LLM provider adapters
    engine/              — Execution loop, tool orchestration
    workflow/            — Temporal workflows and activities
    sandbox/             — Sandbox abstraction (E2B)
    storage/             — Artifact storage (S3/filesystem)
    domain/              — Core domain models
    scoring/             — Scorecard generation
    runevents/           — Event normalization, replay assembly
    worker/              — Worker runtime and config
web/                     — Next.js frontend
scripts/
  db/                    — Database migration scripts
  dev/                   — Local development helpers
  smoke/                 — Smoke test scripts
```

## License

Open source.
