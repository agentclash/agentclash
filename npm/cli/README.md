# agentclash

Command-line interface for [AgentClash](https://www.agentclash.dev), an AI
agent evaluation platform for real-task races, replay, scorecards, and CI
regression gates.

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
agentclash doctor
agentclash challenge-pack init support-eval.yaml
agentclash eval start --help
```

Released binaries use the hosted API (`https://api.agentclash.dev`) by default.

## Common commands

| Task | Command |
| --- | --- |
| Check auth, workspace, and eval readiness | `agentclash doctor`, `agentclash quickstart` |
| Author and publish a challenge pack | `agentclash challenge-pack init`, `validate`, `publish` |
| Add a ready-made library pack | `agentclash challenge-pack catalog list`, `catalog use <slug>` |
| Start and follow an eval | `agentclash eval start --follow` |
| Read scorecards and regression verdicts | `agentclash eval scorecard [run]` |
| Compare with a baseline | `agentclash baseline set`, `agentclash compare latest` |
| Inspect a run | `agentclash run list`, `run get`, `run ranking`, `run replay`, `run transcript` |
| Answer a human turn in a multi-turn run | `agentclash run turn status`, `run turn submit` |
| Promote failures into regression suites | `agentclash run failures`, `run promote-failure`, `regression-suite` |
| Gate CI on a release verdict | `agentclash ci init`, `ci validate`, `ci run` |
| Work with datasets | `agentclash dataset import`, `dataset eval`, `dataset test` |
| Submit multi-pack eval sets | `agentclash evalset submit`, `evalset status`, `evalset report` |
| Install Agent Skills into a coding agent | `agentclash integration <host> install` |
| Print the full command tree as JSON | `agentclash schema --json` |

Run `agentclash <command> --help` for all flags. Add `--json` to any command for output that a script can read.

## Use a local CLI build against a hosted backend

If you work on the CLI itself, you can run the local Go binary against a hosted AgentClash API. Source builds use `http://localhost:8080` by default, so set `AGENTCLASH_API_URL` for production:

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"

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
2. Release Please opens `chore(main): release x.y.z` and merges it automatically when the required checks pass.
3. Let `.github/workflows/release-cli.yml` publish GitHub release assets, npm packages, and smoke installs automatically.

npm shows the README of the latest published version. A change to this file appears on npm only after the next CLI release.

## Other install channels

Source, Homebrew, install scripts, and direct downloads are documented at
<https://github.com/agentclash/agentclash#cli>.

The full maintainer playbook, including the one-time npm Trusted Publishing bootstrap, lives at
<https://github.com/agentclash/agentclash/blob/main/docs/cli-distribution.md>.

## License

[MIT](https://opensource.org/licenses/MIT) — see `LICENSE`.
