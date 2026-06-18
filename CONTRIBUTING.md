# Contributing to AgentClash

Thanks for your interest in improving AgentClash — an open-source race engine
that pits AI models and agents against each other on real tasks with live
scoring. This guide covers how to get set up and what we expect in a
contribution.

By participating you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Contributor license policy

AgentClash is preparing to require a Contributor License Agreement (CLA), but
the policy is not active until all of these are true:

- the final legal entity name replaces the placeholder in
  [CONTRIBUTOR_LICENSE_AGREEMENT.md](CONTRIBUTOR_LICENSE_AGREEMENT.md);
- counsel or another qualified reviewer approves the text;
- CLA Assistant is configured for this repository; and
- the CLA Assistant status check is required in GitHub branch protection or
  repository rulesets.

Until then, contributions continue under the existing inbound=outbound MIT
policy described in [License](#license).

If adopted, this CLA is broader than inbound=outbound MIT. It grants the named
AgentClash legal entity a sublicensable copyright license, an express patent
license, and permission to distribute contributions under future project
licenses, including non-MIT or commercial terms. Contributors should not sign it
unless they are comfortable with that effect.

Once the policy is active, non-founder contributors will need to pass the CLA
Assistant check before merge. The founders `Atharva-Kanherkar`,
`AyushRajSinghParihar`, and `Shubham2582` are intended to be treated as
internally covered contributors through private founder or company paperwork
outside this public repository.

If your employer owns or may own your work, get the needed authorization before
submitting a contribution. Do not include private keys, proprietary code,
customer data, or third-party material unless you have the right to contribute it
under the project license and any active CLA.

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
5. If a CLA Assistant check appears on the PR, follow the signing link or ask a
   maintainer whether you are covered by a documented exception.
6. Open the PR against `main` and fill in the template.

## Reporting bugs & requesting features

Use the issue templates under **New issue**. For security issues, do **not** open
a public issue — see [SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contributions are licensed under the
project's [MIT License](LICENSE). If the proposed CLA policy is adopted later,
future contributions will also be subject to the active CLA terms.
