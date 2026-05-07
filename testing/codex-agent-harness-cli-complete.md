# codex/agent-harness-cli-complete — Test Contract

## Functional Behavior
- The CLI must expose every merged Agent Harness backend surface needed for testing: harness CRUD/read/run, execution get/list/follow, cancel, retry, suite create/list/tasks/run/rankings, failure review get/update, failure summary, and prior-run promotion.
- Commands must preserve structured output (`--json` / `--output yaml`) by returning raw API payloads.
- Table output must show the most important identifiers/status fields and avoid leaking private suite task prompts or hidden evaluation config.
- Commands that send JSON bodies must accept either explicit flags or JSON files/inline JSON where the backend payload can be complex.

## Unit Tests
- CLI tests verify each new command calls the expected HTTP endpoint.
- Body-building tests verify suite creation, suite run, retry idempotency, failure-review update, and prior-run promotion payloads.
- Existing harness create/run/execution tests must continue to pass.

## Integration / Functional Tests
- `go test ./cmd` under `cli/` must pass against fake HTTP servers.
- `go test ./...` under `cli/` must pass.

## Smoke Tests
- `cd cli && go test ./cmd`
- `cd cli && go test ./...`
- `git diff --check`

## E2E Tests
- N/A — CLI command wrapper work. Live hosted testing requires an authenticated workspace with Agent Harness data.

## Manual / cURL Tests
```bash
cd cli
go run . agent-harness suite list --workspace "$WORKSPACE_ID"
go run . agent-harness suite rankings "$SUITE_ID" --workspace "$WORKSPACE_ID" --k 3
go run . agent-harness execution failure-review get "$EXECUTION_ID" --workspace "$WORKSPACE_ID"
go run . agent-harness execution cancel "$EXECUTION_ID" --workspace "$WORKSPACE_ID"
```
