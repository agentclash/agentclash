# codex/fix-goreleaser-publish-tokens — Test Contract

## Functional Behavior
- GoReleaser repository tokens for Homebrew and Winget must use direct `.Env` template references.
- Future `Release CLI` runs must not fail with `expected {{ .Env.VAR_NAME }} only`.
- Existing `v0.2.0` GitHub release assets must remain untouched; no tag deletion, retagging, or asset replacement.
- Homebrew/Winget recovery for `v0.2.0` must use the published release assets and checksums.

## Unit Tests
- N/A — release configuration and package metadata recovery only.

## Integration / Functional Tests
- `go run github.com/goreleaser/goreleaser/v2@v2.15.3 check` passes from `cli/`.
- GitHub release `v0.2.0` remains published with all six platform archives plus `checksums.txt`.

## Smoke Tests
- Homebrew tap contains a cask for `agentclash` version `0.2.0`.
- Winget fork contains manifests for `AgentClash.AgentClash` version `0.2.0`, or a PR exists to publish them upstream.

## E2E Tests
- Run the install script for `v0.2.0` into a temporary directory and verify `agentclash version`.
- Homebrew install verification may be run after tap metadata lands.

## Manual / cURL Tests
- `gh release view v0.2.0 --repo agentclash/agentclash --json assets`
- `curl -fsSL https://github.com/agentclash/agentclash/releases/download/v0.2.0/checksums.txt`
- `gh api repos/agentclash/homebrew-tap/contents/Casks/agentclash.rb`
- `gh api repos/agentclash/winget-pkgs/contents/manifests/a/AgentClash/AgentClash/0.2.0`
