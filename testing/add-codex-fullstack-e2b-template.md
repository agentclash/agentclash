# Add Codex Fullstack E2B Template — Test Contract

## Functional Behavior

- The repo-owned E2B template should install the Codex CLI for Agent Harness coding tasks.
- The template should keep `/workspace` empty so Agent Harness repository cloning into `/workspace` succeeds.
- The template should retain Go, Node, Python, ripgrep, git, and build tooling for common validators.
- Template build scripts should be runnable through npm.

## Unit Tests

- N/A — template behavior is validated by E2B build and sandbox smoke commands.

## Integration / Functional Tests

- Build `agentclash-codex-fullstack-dev` through the E2B TypeScript SDK.
- Create a sandbox from `agentclash-codex-fullstack-dev`.
- Run `codex --version`, `go version`, `node --version`, `python3 --version`, and `rg --version` inside the sandbox.

## Smoke Tests

- Start an Agent Harness using `agentclash-codex-fullstack-dev`.
- Confirm repository clone into `/workspace` succeeds.
- Confirm Codex starts and streams events.

## E2E Tests

- Full Agent Harness completion is blocked until PR #472 deploys, because hosted workers still have the 30 second E2B process stream timeout.

## Manual / cURL Tests

- N/A.
