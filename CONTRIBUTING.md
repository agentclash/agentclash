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
5. Confirm you agree to the [CLA](CONTRIBUTOR_LICENSE_AGREEMENT.md) by checking its box in the PR template.
6. Open the PR against `main` and fill in the template.

## Reporting bugs & requesting features

Use the issue templates under **New issue**. For security issues, do **not** open
a public issue — see [SECURITY.md](SECURITY.md).

## License

The project is currently distributed under the [MIT License](LICENSE). Your
contributions are accepted under the
[Contributor License Agreement](CONTRIBUTOR_LICENSE_AGREEMENT.md), which permits
distribution under MIT today and possibly other terms in the future.
