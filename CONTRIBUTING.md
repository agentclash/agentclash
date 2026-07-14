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

AgentClash is a monorepo with three independently buildable parts:

- **Backend (Go)** — `backend/`: REST API server + Temporal worker.
- **CLI (Go)** — `cli/`: a **separate Go module**, distributed as the `agentclash`
  npm package. Run Go commands from inside `cli/`.
- **Frontend (Next.js)** — `web/`.

Because the backend and CLI are separate Go modules, a change that spans both
must build and test from **each** directory.

## Prerequisites

What you need depends on what you're changing (see the tiers below). The full set is:

- Go 1.25+
- Node.js 18+ and `pnpm` (`corepack enable` provides it)
- Docker (for Postgres, Redis, and Temporal in the full local stack)
- A `psql` / libpq client — needed for migrations and seeding even though
  Postgres runs in Docker (`brew install libpq` · `apt install postgresql-client`)
- Temporal CLI is **optional** — `make start` runs Temporal as a Docker
  container; the host CLI (`brew install temporal`) is only a fallback.

## Run AgentClash locally

**Most contributions are Tier 0/1 — you probably don't need the backend.** Pick
the smallest tier that covers your change:

| Tier | You're changing… | You need | Run |
| --- | --- | --- | --- |
| **0** | docs, web, marketing, content | Node 18+ & `pnpm` | `cd web && pnpm install && pnpm dev` → http://localhost:3000 |
| **1** | the CLI | Go 1.25+ | `export AGENTCLASH_API_URL=https://api.agentclash.dev` then `cd cli && go run . --help` |
| **2** | backend / full stack | Go, Docker, `psql` | `make setup && make start` |

Tier 0 renders the marketing and docs site standalone — no Go, Docker, Temporal,
or database. (Authenticated app pages need WorkOS keys; see `web/.env.local.example`.)
Tier 1 runs the CLI against the hosted API, so no local backend is required.

### Full stack (Tier 2)

```bash
make setup     # installs deps, starts Postgres + Redis, runs migrations
make start     # boots Postgres, Redis, Temporal, API server, and worker
make doctor    # confirms the stack is healthy and prints the URLs
```

`make setup` is idempotent — re-run it any time. Run `make help` to list targets.

**Ports:**

| Service | Port | Source |
| --- | --- | --- |
| API server | 8080 | `make api-server` |
| Web | 3000 | `cd web && pnpm dev` |
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
| `Temporal not reachable on :7233` | `make start` runs it in Docker; check `docker compose logs temporal`, or `brew install temporal` for the host fallback. |
| Port 8080 / 3000 already in use | Stop the other process, or change `API_SERVER_BIND_ADDRESS` / the web port. |
| `pnpm: command not found` | `corepack enable` (or `npm i -g pnpm`). |
| Wrong Go version | This repo pins **Go 1.25.5** (`.tool-versions`); install a matching toolchain. |
| Windows | Use **WSL2** — the scripts assume a POSIX shell, `make`, and Docker. |
| Apple Silicon | Fully supported; the images are multi-arch. |

### After certain changes

- Backend API route added/changed → update `docs/api-server/openapi.yaml`.
- DB queries changed → `cd backend && sqlc generate`.
- Agent Skill changed → `make cli-skills-snapshot`.

**Run `make check` before pushing** — it builds, vets/lints, type-checks, and
tests all three modules.

## Build & test

```bash
# Backend
cd backend && go build ./... && go vet ./... && go test -short -race -count=1 ./...

# CLI (from the cli/ module)
cd cli && go build ./... && go vet ./... && go test -short -race -count=1 ./...

# Frontend
cd web && pnpm install && pnpm lint && npx tsc --noEmit && pnpm test
```

Useful entry points: `make api-server`, `make worker`, `./scripts/dev/start-local-stack.sh`.
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

We use [all-contributors](https://allcontributors.org) to credit everyone who
helps — not just code. On any merged issue or PR, a maintainer (or you) can comment
`@all-contributors please add @user for code, doc` to add someone to the README.

## License

The project is currently distributed under the [MIT License](LICENSE). Your
contributions are accepted under the
[Contributor License Agreement](CONTRIBUTOR_LICENSE_AGREEMENT.md), which permits
distribution under MIT today and possibly other terms in the future.
