# AGENTS.md

This file has two audiences:

- **Using AgentClash from your own project** (most agents) — see the next section.
- **Contributing to the AgentClash CLI itself** — see "Contributing to the CLI" below.

## Using AgentClash from your own repo

AgentClash races AI models/agents against each other on real tasks with live
scoring. To drive it from a coding agent in *your* project:

1. Install the CLI and wire its Agent Skills into your host (one time):

   ```bash
   npm i -g agentclash
   agentclash integration <agent> install   # claude | codex | cursor | openclaw | hermes | opencode
   ```

   This installs the AgentClash Agent Skills (SKILL.md files) into your agent's
   skills directory. It writes **only** SKILL.md files — never `CLAUDE.md`,
   `AGENTS.md`, `.mcp.json`, or any project config.

2. **Load the `agentclash-hub` skill first** — it is the entrypoint and carries
   the full workflow map, skill dependency order, hosted defaults, and product
   UI links.

3. Introspect the whole CLI — every command, flag, stable exit code, and
   status lifecycle — as JSON, no auth required:

   ```bash
   agentclash schema --json
   ```

   Read `.status_enums` for each status lifecycle: the states, the terminal
   states that end a follow loop, and the legal next states. Do not write
   your own status list.

4. Humans do one-time setup on the **web** (sign in, add provider API keys /
   BYOK, deployments) at https://www.agentclash.dev. The CLI is the
   iterate-on-challenge-packs loop; there are no built-in packs — you author
   your own.

Verify any time with `agentclash integration <agent> doctor` and
`agentclash doctor` (add `--json` for machine-readable output). The same
guidance ships inside the published npm package at
`node_modules/agentclash/AGENTS.md`.

## Contributing to the CLI

Quick reference for AI coding agents working on the AgentClash CLI itself.

### What matters for CLI work

- The CLI lives in `cli/` and is a separate Go module.
- It can run against local backend services or a hosted API.
- The published npm package is `agentclash`; platform binaries live in `@agentclash/cli-<os>-<arch>`.

### Run the CLI locally against a hosted backend

Use production unless you intentionally need a local or self-hosted backend:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"

cd cli
go run . auth login --device
go run . workspace list
go run . workspace use <workspace-id>
go run . run list
go run . run create --help
# When the workspace already has challenge packs and deployments:
go run . run create --follow

# Multi-turn human takeover (while a run agent awaits operator input):
go run . run turn status <runAgentId> --run <runId>
go run . run turn submit <runAgentId> --run <runId> --message "Your message here"
```

Resolution order for the API base URL is:

```text
--api-url > AGENTCLASH_API_URL > saved user config > default
  (source builds: http://localhost:8080; released binaries: https://api.agentclash.dev)
```

Useful env vars:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
export AGENTCLASH_TOKEN="..."
export AGENTCLASH_WORKSPACE="workspace-id"
```

### CLI test commands

Run these from `cli/`:

```bash
go build ./...
go vet ./...
go test -short -race -count=1 ./...
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```

If packaging changed, rehearse npm locally from the snapshot output:

```bash
node scripts/publish-npm/assemble.mjs v0.0.0-rehearse cli/dist
for p in npm-out/platforms/*/ npm-out/cli; do
  (cd "$p" && npm pack --dry-run)
done
```

Optional local install smoke test:

```bash
(cd npm-out/platforms/<triple> && npm pack --pack-destination /tmp)
(cd npm-out/cli && npm pack --pack-destination /tmp)
mkdir -p /tmp/agentclash-smoke && cd /tmp/agentclash-smoke
npm init -y
npm i /tmp/agentclash-cli-<triple>-*.tgz /tmp/agentclash-*.tgz
./node_modules/.bin/agentclash version
```

### Routine npm release flow

Do not manually publish for normal releases.

1. Make and validate a releasable CLI change under `cli/` locally.
2. Use a conventional commit: `fix:` = patch, `feat:` = minor, `feat!:` = major.
3. Merge to `main`.
4. Release Please opens `chore(main): release x.y.z` and turns on auto-merge. The PR merges when "Release checks" passes.
5. `.github/workflows/release-cli.yml` builds release assets, publishes npm, and runs smoke installs on Ubuntu, macOS, and Windows.

The one-time npm Trusted Publishing bootstrap is documented in `docs/cli-distribution.md`. That bootstrap should not be part of routine releases.

## Marketing site (web/)

When editing public marketing pages under `web/`, read `web/AGENTS.md` first. It bans Instrument Serif (`--font-display`) on new marketing headlines and documents conversion layout patterns.
