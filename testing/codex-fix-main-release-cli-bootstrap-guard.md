# codex/fix-main-release-cli-bootstrap-guard — Test Contract

## Functional Behavior
- `scripts/publish-npm/publish-one.sh` must keep treating reruns of an already-published version as success instead of failing the whole workflow.
- If `npm publish` returns a 404- or permission-style error for a package, the script must probe the registry for the target `name@version` before deciding the publish truly failed.
- If the target version is still missing and the package itself does not yet exist on npm, the script must fail with an explicit message that Trusted Publishing cannot create first-time packages and that maintainers need the one-time bootstrap in `docs/cli-distribution.md`.
- If the package exists but publish still fails with a 404- or permission-style error, the script must fail with an explicit message pointing maintainers to Trusted Publishing configuration for `.github/workflows/release-cli.yml`.

## Unit Tests
- N/A — shell script change with focused command-level verification.

## Integration / Functional Tests
- `bash -n scripts/publish-npm/publish-one.sh`
- Run the helper against a mocked `npm`/`jq` environment where publish succeeds.
- Run the helper against a mocked environment where `npm publish` returns a conflict but the version is visible afterward; expect success.
- Run the helper against a mocked environment where `npm publish` returns a 404 and the package is missing from npm; expect a targeted bootstrap error.
- Run the helper against a mocked environment where `npm publish` returns a 404, the package exists, but the target version never appears; expect a targeted Trusted Publishing configuration error.

## Smoke Tests
- Confirm the script still prints grouped publish logs and exits non-zero for unrecoverable publish failures.
- Confirm the script does not regress the existing post-failure visibility retry loop.

## E2E Tests
- N/A — no live npm publish should be triggered from local verification.

## Manual / cURL Tests
- `gh run view 24720436422 --log-failed`
- `npm view @agentclash/cli-darwin-arm64 version --silent`
- `npm view @agentclash/cli-darwin-arm64@0.3.0 version --silent`
