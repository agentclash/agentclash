# CLI Distribution

AgentClash ships the CLI through GitHub Releases first. Package managers and installer scripts consume those release assets.

## User Install Paths

macOS and Linux package manager after the tap is populated by a release:

```bash
brew install --cask agentclash/tap/agentclash
```

Linux/macOS fallback script:

```bash
curl -fsSL https://raw.githubusercontent.com/agentclash/agentclash/main/scripts/install/install.sh | sh
```

Windows fallback script:

```powershell
irm https://raw.githubusercontent.com/agentclash/agentclash/main/scripts/install/install.ps1 | iex
```

Windows package manager after Winget manifest approval:

```powershell
winget install AgentClash.AgentClash
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

Winget:

```powershell
winget uninstall AgentClash.AgentClash
```

## Release Flow

Stable releases are not cut on every merge. Release Please watches CLI-impacting paths and opens a version bump PR from conventional commits. Merging that release PR creates the `v*` tag, and the tag-triggered GoReleaser workflow publishes archives, checksums, Homebrew metadata, and Winget metadata.

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
- `agentclash/winget-pkgs`, a fork of `microsoft/winget-pkgs`.
- `WINGET_TOKEN`, a PAT or GitHub App token that can push branches to that fork and open PRs to `microsoft/winget-pkgs`.
- `RELEASE_PLEASE_TOKEN`, a PAT or GitHub App token for Release Please. Do not use the default `GITHUB_TOKEN` for this, because tags created by `GITHUB_TOKEN` do not trigger the downstream GoReleaser workflow.

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
