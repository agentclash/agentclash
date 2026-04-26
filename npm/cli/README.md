# agentclash

Command-line interface for the [AgentClash](https://www.agentclash.dev) race
engine — evaluate, compare, and deploy AI agents.

## Install

```bash
npm i -g agentclash
# or
npx agentclash --help
```

The install pulls exactly one prebuilt binary for your OS/architecture from
the matching optional dependency (`@agentclash/cli-<platform>-<arch>`). No
postinstall scripts; no downloads at install time.

Supported platforms:

- `darwin-arm64`, `darwin-x64`
- `linux-arm64`, `linux-x64`
- `win32-arm64`, `win32-x64`

## Get started

```bash
agentclash auth login
agentclash link
agentclash challenge-pack init support-eval.yaml
agentclash eval start --help
```

## Use a local CLI build against a hosted backend

If you're working on the CLI itself, you can run the local Go binary against a hosted AgentClash API. Staging is the safest default:

```bash
export AGENTCLASH_API_URL="https://staging-api.agentclash.dev"

cd cli
go run . auth login --device
go run . link
go run . run list
go run . eval start --help
# When the workspace already has challenge packs and deployments:
go run . eval start --follow
```

`--api-url` overrides `AGENTCLASH_API_URL` for one-off commands.

## Test before release

```bash
cd cli
go build ./...
go vet ./...
go test -short -race -count=1 ./...
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
cd ../web && pnpm build
```

If you changed npm packaging, rehearse it locally:

```bash
node scripts/publish-npm/assemble.mjs v0.0.0-rehearse cli/dist
for p in npm-out/platforms/*/ npm-out/cli; do
  (cd "$p" && npm pack --dry-run)
done
```

## Release flow for maintainers

Routine npm releases should not be manual.

1. Land a releasable CLI change under `cli/` on `main` with a conventional commit (`fix:`, `feat:`, or `feat!:`).
2. Merge the Release Please PR (`chore(main): release x.y.z`).
3. Let `.github/workflows/release-cli.yml` publish GitHub release assets, npm packages, and smoke installs automatically.

## Other install channels

Source, Homebrew, install scripts, and direct downloads are documented at
<https://github.com/agentclash/agentclash#cli>.

The full maintainer playbook, including the one-time npm Trusted Publishing bootstrap, lives at
<https://github.com/agentclash/agentclash/blob/main/docs/cli-distribution.md>.

## License

[FSL-1.1-MIT](https://fsl.software) — see `LICENSE`.

Short version: use and modify it for anything except running a competing
commercial eval-engine service; each version auto-converts to MIT two years
after release.
