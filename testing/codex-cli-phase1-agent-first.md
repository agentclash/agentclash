# codex-cli-phase1-agent-first — Test Contract

## Functional Behavior
- Add a workflow-first Phase 1 CLI path without removing the existing resource-oriented commands.
- `agentclash link` should use the authenticated session to choose a workspace and save it as the default workspace in user config.
- `agentclash link` must not create or require a backend repo/project link.
- `agentclash challenge-pack init <path>` should scaffold a minimal valid challenge-pack YAML file based on the current documented parser shape.
- `agentclash eval start` should wrap run creation, support human-friendly challenge pack and deployment resolution, and map regression selection flags to the existing run-create API fields.
- `agentclash run create` should gain parity flags for `official_pack_mode`, `regression_suite_ids`, and `regression_case_ids`.
- `agentclash baseline set|show|clear` should manage a workspace-scoped local baseline bookmark stored in user config.
- `agentclash eval scorecard` should accept a run-oriented workflow, auto-resolve the run agent when safe, and show scorecard plus compare/release-gate output when a baseline bookmark exists.
- `agentclash doctor` should check auth, default workspace, workspace readiness, and baseline presence and print actionable output.
- Root help, npm README, repo README, quickstart docs, challenge-pack guide, and testing docs should all reflect the new workflow-first path.
- The implementation must add a final handoff document at `docs/cli-phase1-handoff.md` that explains shipped scope, remaining work, and exact verification results.

## Unit Tests
- Config tests for baseline bookmark persistence, overwrite behavior, clearing behavior, and workspace scoping.
- Command tests for `link`, `challenge-pack init`, `baseline`, `eval start`, `eval scorecard`, and `doctor`.
- Command tests for `run create` regression flag mapping.
- Output/help assertions for new commands and updated root help.

## Integration / Functional Tests
- CLI fake-API tests should verify new commands hit the expected endpoints with the expected payload shape.
- `web/src/lib/docs.ts` generated CLI reference should continue to discover and render the new Cobra commands.

## Smoke Tests
- `cd cli && go build ./...`
- `cd cli && go vet ./...`
- `cd cli && go test -short -race -count=1 ./...`
- `bash -n testing/cli-e2e-suite.sh`
- `cd web && pnpm build`

## E2E Tests
- N/A — no new browser E2E flow is required for this Phase 1 CLI/docs change.

## Manual / cURL Tests
```bash
cd cli
go run . --help
go run . link --help
go run . eval --help
go run . baseline --help
go run . doctor --help
go run . challenge-pack init --help
```

```bash
cd cli
go run . challenge-pack init /tmp/agentclash-pack.yaml
go run . challenge-pack validate /tmp/agentclash-pack.yaml
```

- In a TTY with valid credentials, run `agentclash link` and confirm it saves a usable default workspace.
- In a workspace with a recent run, run `agentclash baseline set <run-id>` followed by `agentclash baseline show`.
- Run `agentclash doctor` and confirm the output calls out missing or healthy prerequisites accurately.
