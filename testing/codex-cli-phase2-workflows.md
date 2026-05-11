# codex/cli-phase2-workflows - Test Contract

## Functional Behavior
- Implement only the Phase 1 workflow wrappers: eval session inspection, quickstart readiness, latest comparison, replay triage, and the workflow handoff document.
- `agentclash eval session list` calls the existing eval-session list API for the resolved workspace and prints concise human output plus raw structured output when `--json` is requested.
- `agentclash eval session get <eval-session-id>` calls the existing eval-session read API and shows session status, child runs, aggregate metrics, warnings, and comparison/winner information when present.
- `agentclash eval session follow <eval-session-id>` polls the session read API until the session reaches a terminal state or the timeout expires, showing progress without requiring run-level polling by hand.
- `agentclash eval start --repetitions >= 2` prints the follow/get commands for the created eval session.
- `agentclash quickstart` is read-only: it checks auth/API URL/workspace resolution, challenge packs, deployments, baseline bookmark state, and prints the next useful command without creating remote resources or starting runs.
- `agentclash compare latest` resolves the workspace baseline bookmark plus the latest non-baseline run, supports agent selection, optionally evaluates the release gate with `--gate`, and preserves `--json` output for scripts.
- `agentclash replay triage [run-id]` combines run agents, ranking, scorecard, failures, replay snippets, artifact pointers, and next debugging commands into one workflow-first summary.
- Rename the detailed CLI handoff from `docs/cli-phase1-handoff.md` to `docs/cli-workflow-handoff.md`, leave a pointer at the old path, and structure the new document around current state, user flows, added work, deferred phases, known gaps, and test/release notes.
- Deferred items in the handoff must include Claude/Codex skill install flows, MCP, CI gate fidelity, harness lifecycle, and future integration commands.

## Unit Tests
- Add fake-API command tests for eval session list/get/follow endpoint calls, terminal polling, human output, and JSON output.
- Add command tests for `eval start --repetitions` output guidance.
- Add quickstart tests for unauthenticated, configured-but-empty, and ready workspace states.
- Add compare latest tests for baseline/candidate resolution, agent flag forwarding, same-run rejection, release-gate evaluation, and JSON output.
- Add replay triage tests for aggregation of ranking, scorecard, failures, replay events, artifact pointers, and missing-agent guidance.
- Add help/output tests for the new commands where existing Cobra tests make that practical.

## Integration / Functional Tests
- `go test ./cmd -run '(EvalSession|Quickstart|CompareLatest|ReplayTriage)' -count=1` passes from `cli/`.
- `go test -short ./...` passes from `cli/`.

## Smoke Tests
- `go vet ./...` passes from `cli/`.
- `go build ./...` passes from `cli/`.

## E2E Tests
- N/A - this phase is covered by fake-API command tests. Live hosted smoke remains optional because it requires credentials and seeded workspace data.

## Manual / cURL Tests
```bash
cd cli
go run . eval session --help
go run . quickstart --help
go run . compare latest --help
go run . replay triage --help
```

```bash
cd cli
go run . eval session list --workspace ws_123 --json
go run . eval session get eval_session_123 --workspace ws_123 --json
go run . quickstart --workspace ws_123
go run . compare latest --workspace ws_123 --json
go run . replay triage run_123 --workspace ws_123 --json
```

Expected: help text exposes the new workflow commands, JSON output is valid, and commands either call the documented backend endpoints or fail with actionable missing-auth/workspace messages.
