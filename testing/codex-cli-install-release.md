# codex/cli-install-release — Test Contract

## Functional Behavior
- Installation docs present truthful, working paths for macOS, Linux, Windows, and direct GitHub release downloads.
- `go install` is removed from primary user install guidance while the CLI module path still points at the old GitHub owner.
- `scripts/install/install.sh` supports Linux and macOS on amd64/arm64, respects `VERSION` and `INSTALL_DIR`, uses `curl -fsSL`, verifies release assets and `checksums.txt`, falls back to a user install directory when `/usr/local/bin` is not writable and sudo is unavailable, and prints clear uninstall guidance.
- `scripts/install/install.ps1` supports Windows amd64/arm64, respects `-Version` and `-InstallDir`, downloads the matching zip, verifies `checksums.txt`, installs to `%LOCALAPPDATA%\agentclash\bin` by default, and prints PATH guidance when needed.
- GoReleaser keeps the stable release model tag-triggered, publishes GitHub release assets for linux/darwin/windows amd64/arm64, and is configured for first-class Homebrew cask metadata without making prereleases overwrite stable package-manager channels.
- Release Please creates CLI-scoped release PRs from conventional commits affecting CLI, installer, or release config paths.
- A main-branch snapshot workflow builds fresh CLI artifacts on CLI-impacting merges without marking every merge as a stable release.
- `agentclash run events` authenticates SSE with the `Authorization` header and never places the CLI bearer token in the URL.
- The backend SSE endpoint accepts `Authorization` first and keeps `?token=` only as a browser `EventSource` compatibility fallback.
- `agentclash secret set` preserves exact `--value` and piped stdin bytes, including newlines and leading/trailing whitespace, and uses hidden terminal input for interactive secrets.
- The CLI retry layer retries `5xx` only for `GET`; `POST`, `PATCH`, `PUT`, and `DELETE` are attempted once unless future idempotency support is added.

## Unit Tests
- CLI SSE tests prove `run events` sends an `Authorization` header and no `token` query parameter.
- Backend SSE handler tests prove header auth succeeds, query-token fallback succeeds, missing credentials return `missing_token`, and invalid credentials return `unauthorized`.
- Secret command tests prove multiline stdin, trailing newline, leading/trailing whitespace, exact `--value`, zero-length rejection, and hidden-input path behavior.
- API client tests prove `GET` still retries on `5xx`, while `POST`, `PATCH`, `PUT`, and `DELETE` do not retry on `5xx`.

## Integration / Functional Tests
- `cd cli && go test -short -race -count=1 ./...` passes.
- `cd backend && go test ./internal/api` passes.
- `cd cli && goreleaser check` passes with the updated GoReleaser config.
- `cd cli && goreleaser release --snapshot --clean` builds local artifacts without publishing.
- `sh -n scripts/install/install.sh` passes.
- PowerShell parses `scripts/install/install.ps1` successfully when `pwsh` is available.
- Workflow YAML parses as valid YAML.

## Smoke Tests
- The Unix installer can be run with `VERSION=<tag>` and an isolated `INSTALL_DIR` against a published release.
- The Windows installer documents the equivalent invocation and can be syntax-checked locally.
- Installed binaries should answer `agentclash version` after install.
- Manual `agentclash run events <run-id>` should succeed after backend deploy without exposing the bearer token in the request URL.
- Manual `printf 'line1\nline2\n' | agentclash secret set KEY` should store the full value without trimming.

## E2E Tests
- N/A for local automation — full Homebrew publishing still requires repository secrets and the `agentclash/homebrew-tap` repository.
- Browser run-event streaming remains compatible with the legacy query-token SSE path until a separate browser-safe design replaces `EventSource`.

## Manual / cURL Tests
- Confirm the next real release creates Linux, macOS, and Windows archives plus `checksums.txt`.
- Confirm the Homebrew tap repository `agentclash/homebrew-tap` exists and `HOMEBREW_TAP_TOKEN` can push to it before advertising Homebrew as live.
- Install matrix after release:
  - macOS Intel and Apple Silicon: Homebrew cask and `install.sh`.
  - Linux amd64 and arm64: Homebrew cask on Linux where supported and `install.sh`.
  - Windows amd64 and arm64: npm install, `install.ps1`, and direct zip.
