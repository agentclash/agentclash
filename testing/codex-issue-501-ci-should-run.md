# Codex Issue 501 CI Should Run — Test Contract

## Functional Behavior

- The CLI exposes `agentclash ci should-run`.
- The command reads a CI manifest from `--manifest`, defaulting to `.agentclash/ci.yaml`.
- The command validates the manifest before evaluating triggers.
- Changed files matching any `trigger.paths` glob produce `should_run: true`.
- Labels matching any `trigger.labels` entry produce `should_run: true`, even when no changed file matches.
- Non-matching changed files and labels produce `should_run: false` with a clear reason.
- `--json` emits a stable object suitable for shell/GitHub Actions parsing.
- Human output explains whether CI should run and why.
- The command supports local/CI usage by accepting explicit `--changed-file` values and by deriving changed files from `git diff --name-only <base>...<head>` when `--base`/`--head` are provided.
- Malformed trigger path globs fail with a clear error instead of silently skipping matches.

## Unit Tests

- `TestCIShouldRunMatchesChangedPath` — a changed file matching `trigger.paths` returns `should_run: true`.
- `TestCIShouldRunMatchesLabel` — a configured label returns `should_run: true` without a path match.
- `TestCIShouldRunNoMatch` — unrelated files/labels return `should_run: false`.
- `TestCIShouldRunJSONOutput` — JSON output includes `should_run`, `reason`, and match details.
- `TestCIShouldRunRejectsInvalidGlob` — malformed path globs fail clearly.
- Existing `ci init` and `ci validate` tests continue to pass.

## Integration / Functional Tests

- Use a temporary git repository to verify `agentclash ci should-run --base <base> --head <head>` can derive changed files via `git diff`.
- No AgentClash API calls are required for this issue.

## Smoke Tests

```bash
cd cli
go test -short -count=1 ./cmd -run 'TestCIShouldRun'
go test -short -race -count=1 ./cmd
go build ./...
```

Expected:
- All tests pass.
- The command builds into the CLI.

## E2E Tests

- N/A — full PR-gating orchestration is tracked in #499. This issue only decides whether a gate should run.

## Manual / cURL Tests

```bash
cd cli
go run . ci init /tmp/agentclash-ci-should-run/.agentclash/ci.yaml --force
go run . ci should-run \
  --manifest /tmp/agentclash-ci-should-run/.agentclash/ci.yaml \
  --changed-file prompts/system.md
go run . ci should-run \
  --manifest /tmp/agentclash-ci-should-run/.agentclash/ci.yaml \
  --changed-file docs/readme.md \
  --labels agentclash/eval \
  --json
```

Expected:
- The first `should-run` reports `should_run: true` because the changed file matches `prompts/**`.
- The second `should-run --json` reports `should_run: true` because the label matches `agentclash/eval`.
