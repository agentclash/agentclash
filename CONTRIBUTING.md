# Contributing to AgentClash

Thanks for your interest in improving AgentClash — an open-source race engine
that pits AI models and agents against each other on real tasks with live
scoring. This guide covers how to get set up and what we expect in a
contribution.

By participating you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Developer Certificate of Origin (DCO)

AgentClash uses the [Developer Certificate of Origin](DCO.md) (DCO) instead of a
CLA. Contributions stay under the project's MIT license — inbound equals
outbound, and AgentClash takes no additional or relicensing rights over your
work.

Every commit must carry a `Signed-off-by` trailer matching the commit author.
Add it with the `-s` flag:

```bash
git commit -s -m "feat: ..."        # sign off as you commit
git commit -s --amend --no-edit     # add a sign-off to your latest commit
```

The trailer looks like `Signed-off-by: Your Name <you@example.com>` and certifies
that you wrote the change, or otherwise have the right to submit it under the MIT
license, per clauses (a)–(d) of the [DCO](DCO.md). This applies to everyone,
founders included — there are no exceptions or allowlists. (An automated DCO
check may be added later; until then, please self-check your sign-offs.)

If your employer or another party may own your work, get the needed authorization
before contributing. Do not include private keys, proprietary code, customer
data, or third-party material you do not have the right to contribute under the
MIT license.

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
5. Sign off your commits with `git commit -s` (see the [DCO](DCO.md)).
6. Open the PR against `main` and fill in the template.

## Reporting bugs & requesting features

Use the issue templates under **New issue**. For security issues, do **not** open
a public issue — see [SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contributions are licensed under the
project's [MIT License](LICENSE).
