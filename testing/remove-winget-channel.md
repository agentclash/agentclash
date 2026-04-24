# package-manager-channel-removal — Test Contract

## Functional Behavior
- GoReleaser no longer generates or publishes the retired Windows package-manager manifests.
- The release workflow no longer reads or references the retired package-manager token secret.
- User-facing install docs remove the retired Windows package-manager path from install guidance.
- Windows install guidance recommends `npm i -g agentclash` first, with PowerShell script and direct GitHub Releases downloads as fallbacks.
- Marketing and maintainer docs stop describing the retired Windows package-manager path as a supported CLI distribution channel.
- Existing Windows archives, npm platform packages, Homebrew publishing, and installer script URLs remain unchanged.
- External cleanup artifacts are created for delisting AgentClash from the public Windows package-manager source.

## Unit Tests
- N/A — this change only touches release config, docs, and marketing copy.

## Integration / Functional Tests
- `cd cli && go test -short -race -count=1 ./...`
- `cd cli && go run github.com/goreleaser/goreleaser/v2@latest check`
- `cd web && npm run lint`

## Smoke Tests
- Repo-wide grep for the retired package-manager identifiers returns no matches in maintained repo content.
- README, CLI distribution docs, and npm README all present consistent Windows install guidance.
- Website copy no longer lists the retired package-manager channel among supported channels.

## E2E Tests
- N/A — no runtime product flow changes.

## Manual / cURL Tests
- Inspect the next release workflow configuration and confirm only GitHub Releases, Homebrew, npm, and existing Windows artifacts remain in scope.
- Verify the external removal request and deletion PR are opened and point at the retired Windows package identifier.
