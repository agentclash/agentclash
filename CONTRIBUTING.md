# Contributing to AgentClash

Thanks for your interest in improving AgentClash — an open-source race engine
that pits AI models and agents against each other on real tasks with live
scoring. This guide covers how to get set up and what we expect in a
contribution.

By participating you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Contributor License Agreement

AgentClash uses a [Contributor License Agreement](CONTRIBUTOR_LICENSE_AGREEMENT.md)
(CLA). By contributing, you agree to it.

**What it grants — please read.** The CLA gives the project rights **broader than
the MIT license**: a sublicensable, relicensable copyright license, a patent
license, and the right to distribute your contributions under the current MIT
license **or under different terms in the future, including commercial or
non-open-source terms.** AgentClash is MIT today; the CLA preserves the option to
change that later. Don't contribute unless you're comfortable with this effect.

Until AgentClash forms a legal entity, the grant is held jointly by the founders
`Atharva-Kanherkar`, `AyushRajSinghParihar`, and `Shubham2582`, and transfers to
that entity once it exists.

Confirm your agreement by checking the CLA box in the pull-request template. If
your employer or another party may own your work, get authorization first. Do not
include private keys, proprietary code, customer data, or third-party material you
do not have the right to contribute. The CLA's exact wording is pending review by
counsel; the terms above apply in the meantime.

## Repository layout

AgentClash is a monorepo with four independently buildable parts:

- **Backend (Go)** — `backend/`: REST API server + Temporal worker.
- **CLI (Go)** — `cli/`: a **separate Go module**, distributed as the `agentclash`
  npm package. Run Go commands from inside `cli/`.
- **Runtime (Go)** — `runtime/`: a **third Go module** holding the shared engine
  library (challenge packs, providers, sandbox, scoring, run events). Both
  `backend/` and `cli/` consume it through `replace` directives, so a change here
  can break either of them.
- **Frontend (Next.js)** — `web/`.

Because `backend/`, `cli/`, and `runtime/` are separate Go modules, a change that
spans them must build and test from **each** directory. `make check` does all
four for you.

## Prerequisites

What you need depends on what you're changing (see the tiers below). The full set is:

- Go 1.26.8+
- Node.js 22 and `pnpm` (`corepack enable` provides it) — pinned in `.nvmrc` and
  `.tool-versions`, and matched by CI
- Docker (for Postgres, Redis, and Temporal in the full local stack)
- A `psql` / libpq client — needed for migrations and seeding even though
  Postgres runs in Docker (`brew install libpq` · `apt install postgresql-client`)
- `curl` for API readiness and local smoke checks
- Temporal CLI is **optional** — `make start` runs Temporal as a Docker
  container; the host CLI (`brew install temporal`) is only a fallback.

## Run AgentClash locally

### Environment files: backend and web are intentionally different

The backend lifecycle loads `backend/.env`; `make setup` copies it from
`backend/.env.example` when it is missing and never overwrites an existing
file. The Next.js app follows its ecosystem convention and reads
`web/.env.local`; copy that file yourself from `web/.env.local.example` before
running `pnpm dev`.

The local-stack lifecycle manages Postgres, Redis, Temporal, the API server,
and the worker. It does **not** start or stop the web development server.

**Most contributions are Tier 0/1 — you probably don't need the backend.** Pick
the smallest tier that covers your change:

| Tier | You're changing… | You need | Run |
| --- | --- | --- | --- |
| **0** | docs, web, marketing, content | Node 22 & `pnpm` | `cp web/.env.local.example web/.env.local` then `cd web && pnpm install && pnpm dev` → http://localhost:3000 |
| **1** | the CLI | Go 1.26.8+ | `export AGENTCLASH_API_URL=https://api.agentclash.dev` then `cd cli && go run . --help` |
| **2** | backend / full stack | Go, Docker, `psql` | `make setup && make start` |

Tier 1 runs the CLI against the hosted API, so no local backend is required.

#### Tier 0: the `web/.env.local` step is not optional

`web/src/middleware.ts` runs WorkOS `authkitMiddleware()` on **every** route
except `/docs`, `/docs-md`, `/llms.txt`, `/llms-full.txt`, and `/share`. With no
WorkOS environment variables set, that middleware throws and every other page —
**including the homepage** — returns a 500. So `pnpm dev` alone is not enough:

```bash
cp web/.env.local.example web/.env.local
```

A verbatim copy is enough — the placeholders already in that file boot the site,
no editing required. The values that matter do **not** need to be real:

| Variable | Local value |
| --- | --- |
| `WORKOS_CLIENT_ID` | any non-empty string, e.g. `client_01dummy` |
| `WORKOS_API_KEY` | any non-empty string, e.g. `sk_test_dummy` |
| `WORKOS_COOKIE_PASSWORD` | **32+ characters** — shorter values are rejected. Generate one with `openssl rand -base64 24` |
| `NEXT_PUBLIC_WORKOS_REDIRECT_URI` | `http://localhost:3000/auth/callback` — missing this is the first thing AuthKit complains about |

**What this does and does not get you.** Public pages — the marketing site, `/docs`,
and the rest of the unauthenticated app — render correctly with no Go, Docker,
Temporal, or database. Anything behind a real login (sign-in, workspaces,
dashboards) will **not** work with dummy credentials; those need real WorkOS keys,
and most of them also need a running backend (Tier 2).

### Full stack (Tier 2)

```bash
make setup     # installs deps, starts Postgres + Redis, runs migrations
make start     # boots Postgres, Redis, Temporal, API server, and worker
make status    # shows ownership, runtime state, health, and log locations
make logs      # follows Docker + host logs in one prefixed stream
make stop      # stops everything while preserving containers, data, and logs
```

`make setup`, `make start`, and `make stop` are idempotent. `make restart`
performs a guarded stop followed by a clean start. `make doctor` remains an
alias for `make status`, and `make logs FOLLOW=0 TAIL=200` prints a finite log
snapshot instead of following.

`make stop` uses `docker compose stop`, so stopped containers, named volumes,
and Docker logs remain available. Use `make db-down` when you intentionally
want Compose containers removed (data volumes are still kept), or
`make db-reset` for the destructive database reset. Run `make help` to list all
targets.

**Ports:**

| Service | Port | Source |
| --- | --- | --- |
| API server | 8080 | host process managed by `make start` |
| Web | 3000 | separate: `cd web && pnpm dev` |
| Temporal gRPC | 7233 | docker `temporal` service (host CLI fallback) |
| Temporal UI | 8233 | http://localhost:8233 |
| Postgres | 5432 | `docker compose` service `postgres` |
| Redis | 6379 | `docker compose` service `redis` |

Run the parts separately if you prefer:

```bash
make api-server          # API on :8080
make worker              # Temporal worker
cd web && pnpm dev       # web on :3000
```

Seed dev data once Postgres is up:

```bash
make db-seed                              # base dev rows (needs a psql client)
scripts/dev/seed-local-run-fixture.sh     # a full run fixture to explore
```

### Runs with zero API keys

The dev profile in `backend/.env.example` (copied to `backend/.env` by
`make setup`) boots the API, worker, web, and CLI with **no external API keys**:
`AUTH_MODE=dev` stubs auth (`X-Dev-User-ID`, no WorkOS), `SANDBOX_PROVIDER=unconfigured`
uses a noop sandbox, and an ephemeral secrets key is generated at boot.

What's degraded without keys: agent **runs queue but don't execute** until you set
`E2B_API_KEY` (a sandbox provider) plus a model-provider key; invite emails are
logged instead of sent. Everything else works for local development.

### Common issues

| Symptom | Fix |
| --- | --- |
| `make ...` fails with `/usr/bin/bash: No such file or directory` | Update to latest `main` — the Makefile now uses `/bin/bash`. |
| Migration / `db-seed` / `db-psql` fails: `psql: command not found` | Install a Postgres client: `brew install libpq` (add it to `PATH`) or `apt install postgresql-client`. |
| Migration errors after schema changes | `make db-reset` to recreate the database, then `make db-migrate`. |
| `Temporal not reachable on :7233` | Run `make status` and `make logs FOLLOW=0`. Docker is preferred; `brew install temporal` enables the host fallback. |
| Port 8080 already in use | `make status` identifies managed or foreign occupancy. Use `make stop` for the managed stack, or stop the unrelated process. |
| Port 3000 already in use | The web server is separate from `make start`; stop the other `pnpm dev` process or choose another web port. |
| `pnpm: command not found` | `corepack enable` (or `npm i -g pnpm`). |
| Homepage 500s under `pnpm dev` | Missing `web/.env.local` — the WorkOS middleware runs on nearly every route. See [Tier 0](#tier-0-the-webenvlocal-step-is-not-optional). |
| Frontend CI fails after a `web/` dependency change | `web/` carries **both** lockfiles: local dev uses `pnpm` (`pnpm-lock.yaml`) but CI (`.github/workflows/frontend.yml`) runs `npm ci` against `package-lock.json`. Any dependency change must update **both** — run `cd web && pnpm install && npm install --package-lock-only` and commit both lockfiles. |
| Wrong Go version | This repo pins **Go 1.26.8** (`.tool-versions`); install a matching toolchain. |
| Windows | Use **WSL2** — the scripts assume a POSIX shell, `make`, and Docker. |
| Apple Silicon | Fully supported; the images are multi-arch. |

### After certain changes

- Backend API route added/changed → update `docs/api-server/openapi.yaml`.
- DB queries changed → `cd backend && sqlc generate`.
- Agent Skill changed → `make cli-skills-snapshot`.

**Run `make check` before pushing** — it builds, vets/lints, type-checks, and
tests all four modules (`backend/`, `cli/`, `runtime/`, `web/`).

## Build & test

```bash
# Backend
cd backend && go build ./... && go vet ./... && go test -short -race -count=1 ./...

# CLI (from the cli/ module)
cd cli && go build ./... && go vet ./... && go test -short -race -count=1 ./...

# Runtime (from the runtime/ module)
cd runtime && go build ./... && go vet ./... && go test -short -race -count=1 ./...

# Frontend
cd web && pnpm install && pnpm lint && npx tsc --noEmit && pnpm test
```

Useful entry points: `make start`, `make status`, `make logs`, `make stop`, and
`make restart`. `make api-server` and `make worker` remain available when you
intentionally want foreground processes.
See `CLAUDE.md` and `AGENTS.md` for the fuller command reference and architecture notes.

If you change DB queries, regenerate code with `cd backend && sqlc generate`.
If you add/modify a backend API route, update `docs/api-server/openapi.yaml`.

## Pull requests

1. Branch off `main`.
2. Use [Conventional Commits](https://www.conventionalcommits.org/): `fix:` (patch),
   `feat:` (minor), `feat!:` (major). Release Please uses these to version releases.
3. Keep PRs focused; include tests for behavior changes (happy path **and** a
   failure path where it applies).
4. Make sure builds, `vet`/`lint`, type checks, and tests pass for every part you
   touched.
5. Confirm you agree to the [CLA](CONTRIBUTOR_LICENSE_AGREEMENT.md) by checking its box in the PR template.
6. Open the PR against `main` and fill in the template.

## Reporting bugs & requesting features

Use the issue templates under **New issue**. For security issues, do **not** open
a public issue — see [SECURITY.md](SECURITY.md).

## Recognition

Contributions of every kind count — code, docs, triage, design, and reports.
Contributors are thanked on their merged pull requests and credited in the
release notes for the version that ships their work.

## License

The project is currently distributed under the [MIT License](LICENSE). Your
contributions are accepted under the
[Contributor License Agreement](CONTRIBUTOR_LICENSE_AGREEMENT.md), which permits
distribution under MIT today and possibly other terms in the future.
