# CLI Distribution

AgentClash ships the CLI through GitHub Releases first. npm packages, Homebrew, and installer scripts consume those release assets.

## User Install Paths

npm (any OS with Node 18+):

```bash
npm i -g agentclash
# or
npx agentclash --help
```

The root `agentclash` package carries only a small JS shim; the correct prebuilt binary is installed from one of the six per-platform optional dependencies (`@agentclash/cli-<platform>-<arch>`). No postinstall scripts run.

macOS and Linux package manager after the tap is populated by a release:

```bash
brew install --cask agentclash/tap/agentclash
```

Linux/macOS fallback script:

```bash
curl -fsSL https://raw.githubusercontent.com/agentclash/agentclash/main/scripts/install/install.sh | sh
```

Windows fallback script (use this when npm is not a fit for the machine):

```powershell
irm https://raw.githubusercontent.com/agentclash/agentclash/main/scripts/install/install.ps1 | iex
```

Direct downloads:

```text
https://github.com/agentclash/agentclash/releases
```

## Script Installer Options

Install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/agentclash/agentclash/main/scripts/install/install.sh | VERSION=v0.1.2 sh
```

Install to a specific directory:

```bash
curl -fsSL https://raw.githubusercontent.com/agentclash/agentclash/main/scripts/install/install.sh | INSTALL_DIR="$HOME/bin" sh
```

Windows equivalents:

```powershell
$env:VERSION = "v0.1.2"
$env:INSTALL_DIR = "$HOME\bin"
irm https://raw.githubusercontent.com/agentclash/agentclash/main/scripts/install/install.ps1 | iex
```

Both scripts verify the downloaded archive against the release `checksums.txt` file before installing.

## Uninstall

Script install on Linux/macOS:

```bash
rm -f /usr/local/bin/agentclash ~/.local/bin/agentclash
```

Script install on Windows:

```powershell
Remove-Item "$env:LOCALAPPDATA\agentclash\bin\agentclash.exe"
```

Homebrew:

```bash
brew uninstall --cask agentclash/tap/agentclash
```

npm install:

```bash
npm uninstall -g agentclash
```

## Release Flow

Stable releases are not cut on every merge. Release Please watches CLI-impacting paths and opens a version bump PR from conventional commits. Merging that release PR creates the `v*` tag, and the tag-triggered GoReleaser workflow publishes archives, checksums, Homebrew metadata, and npm packages.

Use these commit prefixes for the CLI release stream:

```text
fix: patch release
feat: minor release
feat!: major release
```

Main-branch CLI merges also run the snapshot workflow. Snapshot artifacts are uploaded to the workflow run and are intentionally not a stable release channel.

## Maintainer Setup

Before advertising package-manager installs as live, make sure these external pieces exist:

- `agentclash/homebrew-tap` with a writable default branch.
- `HOMEBREW_TAP_TOKEN`, a PAT or GitHub App token that can push to `agentclash/homebrew-tap`.
- `RELEASE_PLEASE_TOKEN`, a PAT or GitHub App token for Release Please. Do not use the default `GITHUB_TOKEN` for this, because tags created by `GITHUB_TOKEN` do not trigger the downstream GoReleaser workflow.

### npm setup (one-time)

The `publish-npm` job in `.github/workflows/release-cli.yml` uses npm Trusted Publishing — no long-lived `NPM_TOKEN` secret is needed. Trusted Publishing requires the package to already exist on npm, so the first publish has to happen manually. The bootstrap flow:

1. Reserve the package names on npm:
   - unscoped root: `agentclash`
   - scope: `@agentclash` (create the org)
2. Create a granular, publish-only npm token scoped to `agentclash` + `@agentclash/*` with a 7-day expiry.
3. Locally, from a clean checkout of a `v*` tag:
   ```bash
   cd cli && go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
   node scripts/publish-npm/assemble.mjs v0.0.0-bootstrap dist
   for p in npm-out/platforms/*/; do NODE_AUTH_TOKEN=$TOKEN npm publish "$p" --access=public; done
   NODE_AUTH_TOKEN=$TOKEN npm publish npm-out/cli --access=public
   ```
4. In the npm web UI, configure Trusted Publishing on each of the seven packages (root + six platform packages), pointing at repo `agentclash/agentclash` and workflow `.github/workflows/release-cli.yml`.
5. On the `agentclash` package and the `@agentclash` scope, enable "Require 2FA / disallow tokens" so future publishes must come through the trusted workflow.
6. Revoke the bootstrap token.

From then on, every tag-triggered release publishes through the workflow with provenance attestations emitted automatically.

## Local Verification

```bash
cd cli
go test -short -race -count=1 ./...
goreleaser check
goreleaser release --snapshot --clean
```

From the repository root:

```bash
sh -n scripts/install/install.sh
```

npm packaging rehearsal (against the snapshot GoReleaser just produced):

```bash
node scripts/publish-npm/assemble.mjs v0.0.0-rehearse cli/dist
for p in npm-out/platforms/*/ npm-out/cli; do (cd "$p" && npm pack --dry-run); done
# End-to-end install check (on macOS arm64 host, adjust triple to match):
(cd npm-out/platforms/darwin-arm64 && npm pack --pack-destination /tmp)
(cd npm-out/cli && npm pack --pack-destination /tmp)
mkdir /tmp/ac-scratch && cd /tmp/ac-scratch && npm init -y \
  && npm i /tmp/agentclash-cli-darwin-arm64-*.tgz /tmp/agentclash-*.tgz \
  && ./node_modules/.bin/agentclash version
```
