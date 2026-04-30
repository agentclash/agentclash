# AGENTS.md

Quick reference for AI coding agents working on the AgentClash CLI.

## What matters for CLI work

- The CLI lives in `cli/` and is a separate Go module.
- It can run against local backend services or a hosted API.
- The published npm package is `agentclash`; platform binaries live in `@agentclash/cli-<os>-<arch>`.

## Run the CLI locally against a hosted backend

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
```

Resolution order for the API base URL is:

```text
--api-url > AGENTCLASH_API_URL > saved user config > http://localhost:8080
```

Useful env vars:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
export AGENTCLASH_TOKEN="..."
export AGENTCLASH_WORKSPACE="workspace-id"
```

## CLI test commands

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

## Routine npm release flow

Do not manually publish for normal releases.

1. Make and validate a releasable CLI change under `cli/` locally.
2. Use a conventional commit: `fix:` = patch, `feat:` = minor, `feat!:` = major.
3. Merge to `main`.
4. Wait for Release Please to open `chore(main): release x.y.z`.
5. Merge the release PR.
6. `.github/workflows/release-cli.yml` builds release assets, publishes npm, and runs smoke installs on Ubuntu, macOS, and Windows.

The one-time npm Trusted Publishing bootstrap is documented in `docs/cli-distribution.md`. That bootstrap should not be part of routine releases.
