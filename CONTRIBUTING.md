# Contributing to AgentClash

Thanks for your interest in improving AgentClash — an open-source race engine
that pits AI models and agents against each other on real tasks with live
scoring. This guide covers how to get set up and what we expect in a
contribution.

By participating you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Repository layout

AgentClash is a monorepo with three independently buildable parts:

- **Backend (Go)** — `backend/`: REST API server + Temporal worker.
- **CLI (Go)** — `cli/`: a **separate Go module**, distributed as the `agentclash`
  npm package. Run Go commands from inside `cli/`.
- **Frontend (Next.js)** — `web/`.

Because the backend and CLI are separate Go modules, a change that spans both
must build and test from **each** directory.

## Prerequisites

- Go 1.25+
- Node.js 18+ and `pnpm`
- Docker (for Postgres/Temporal in the full local stack)
- Temporal CLI (`brew install temporal`) for the full stack

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
5. Open the PR against `main` and fill in the template.

## Reporting bugs & requesting features

Use the issue templates under **New issue**. For security issues, do **not** open
a public issue — see [SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contributions are licensed under the
project's [MIT License](LICENSE).
