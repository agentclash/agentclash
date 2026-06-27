# Curated "good first issue" candidates

A ready-to-file worklist for maintainers (PR F4). These are **proposals** — skim
each, confirm the pointers still hold, then file the ones you like. Each is small,
well-scoped, and has explicit acceptance criteria so a newcomer can finish it
without a back-and-forth.

File one with the GitHub CLI (labels created in PR C1):

```bash
gh issue create \
  --title "Add a /healthz/ready readiness probe" \
  --label "good first issue" --label "area:backend" \
  --body-file - <<'EOF'
...acceptance criteria from below...
EOF
```

> Aim to keep **8–12 open** at any time. When the pool runs low, refresh it — see
> `docs/maintainers/growth-checklist.md`.

---

## Backend

### 1. Add a `/healthz/ready` readiness probe
**Labels:** `good first issue`, `area:backend`
**Context:** Only `/healthz` (liveness) is registered (`backend/internal/api/server.go`,
`backend/internal/api/health.go`). `/healthz/ready` already appears in the
auth-skip test (`backend/internal/api/middleware_test.go:57`) but no route serves it.
**Acceptance:**
- `GET /healthz/ready` returns `200` with a small JSON body when Postgres and
  Temporal are reachable, `503` otherwise.
- Route registered next to `/healthz` in `server.go`.
- Unit test covering ready/not-ready.
- `docs/api-server/openapi.yaml` updated.

### 2. Add `make stop` to tear down the local stack
**Labels:** `good first issue`, `area:backend`
**Context:** `scripts/dev/start-local-stack.sh` records PIDs under
`/tmp/agentclash-local-stack` but nothing stops them; there's no teardown target.
**Acceptance:**
- New `scripts/dev/stop-local-stack.sh` kills the recorded API/worker/temporal PIDs
  and optionally runs `docker compose down`.
- `make stop` target wired in the root `Makefile` (add to `.PHONY` + `## ` help line).
- Idempotent and safe when nothing is running.

### 3. Add `make logs` to tail local-stack logs
**Labels:** `good first issue`, `area:backend`
**Context:** Stack logs live in `/tmp/agentclash-local-stack/{api-server,worker,temporal}.log`.
**Acceptance:** `make logs` tails all three (e.g. `tail -f`); documented in CONTRIBUTING's "Run AgentClash locally".

## CLI

### 4. Document shell completion install
**Labels:** `good first issue`, `area:cli`, `area:docs`
**Context:** The CLI is Cobra-based and ships a `completion` command, but install
steps aren't documented.
**Acceptance:** A short section (README or `docs/`) covering bash/zsh/fish completion install; verified for at least one shell.

### 5. Add an "Examples" block to `agentclash --help`
**Labels:** `good first issue`, `area:cli`
**Context:** Top-level help lists commands but no end-to-end example.
**Acceptance:** Root command shows 2–3 copy-pasteable examples (auth → eval start → scorecard); existing CLI tests still pass.

## CI / tooling

### 6. Lint the example challenge packs in CI
**Labels:** `good first issue`, `area:ci`
**Context:** `examples/challenge-packs/*.yaml` (12 packs) aren't validated, so they
can silently drift from the schema.
**Acceptance:** A CI job (or step) runs `agentclash challenge-pack validate` (or schema
validation) over every example; fails on an invalid pack.

### 7. Add an `.editorconfig`
**Labels:** `good first issue`, `area:other`
**Context:** No `.editorconfig`, so indentation/charset varies by editor.
**Acceptance:** Root `.editorconfig` (tabs for Go and Makefiles, 2-space YAML/JSON, final newline, UTF-8); matches existing files so it produces no diff churn.

## Docs

### 8. Cross-link the zero-key dev profile from the docs site
**Labels:** `good first issue`, `area:docs`
**Context:** CONTRIBUTING now documents the "runs with zero API keys" profile; the
docs site self-host page doesn't mention it.
**Acceptance:** Self-host / getting-started docs link the zero-key profile and the tiered setup.

### 9. Document the two env-file conventions
**Labels:** `good first issue`, `area:docs`
**Context:** The backend uses `backend/.env.example` while the web app uses
`web/.env.local.example` (Next.js convention). The split can confuse first-time
contributors setting up locally.
**Acceptance:** A short note in CONTRIBUTING's "Run AgentClash locally" (and/or README) explains which env file each module uses and when to copy it; no broken references introduced.

### 10. Add a "deeper smoke test" option to `make doctor`
**Labels:** `good first issue`, `area:backend`, `area:docs`
**Context:** `make doctor` checks ports + `/healthz`. `scripts/dev/curl-create-run.sh`
can exercise a real create-run flow.
**Acceptance:** An opt-in flag/target (e.g. `make doctor DEEP=1`) runs the curl smoke test and reports pass/fail; documented.
