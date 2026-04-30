# codex/cli-surface-gaps - Test Contract

## Functional Behavior
- `agentclash artifact list` calls `GET /v1/workspaces/{workspaceID}/artifacts` and prints a table with artifact identity, type, size, creation time, and related run/run-agent IDs when available.
- `agentclash release-gate list` calls `GET /v1/release-gates`, supports `--baseline` and `--candidate` query filters, and prints evaluated gate history with verdict, policy, reason, and generation time.
- `agentclash regression-suite` exposes CLI access to the existing regression-suite API:
  - `list` calls `GET /v1/workspaces/{workspaceID}/regression-suites`.
  - `get <suiteId>` calls `GET /v1/workspaces/{workspaceID}/regression-suites/{suiteID}`.
  - `create` accepts either `--from-file` JSON or flags and calls `POST /v1/workspaces/{workspaceID}/regression-suites`.
  - `update <suiteId>` accepts either `--from-file` JSON or changed flags and calls `PATCH /v1/workspaces/{workspaceID}/regression-suites/{suiteID}`.
  - `cases <suiteId>` calls `GET /v1/workspaces/{workspaceID}/regression-suites/{suiteID}/cases`.
  - `case update <caseId>` accepts either `--from-file` JSON or changed flags and calls `PATCH /v1/workspaces/{workspaceID}/regression-cases/{caseID}`.
- `agentclash run failures <runId>` calls `GET /v1/workspaces/{workspaceID}/runs/{runID}/failures` and forwards optional filter flags.
- `agentclash run promote-failure <runId> <challengeIdentityId>` calls `POST /v1/workspaces/{workspaceID}/runs/{runID}/failures/{challengeIdentityID}/promote` with either `--from-file` JSON or changed flags.
- All new commands preserve structured output with `--json` / `--output`, keep human tables concise, and use existing helpers/patterns.

## Unit Tests
- Add CLI command tests using `fakeAPI` for:
  - `artifact list` endpoint and workspace behavior.
  - `release-gate list` query forwarding.
  - regression suite list/get/create/update/cases/case-update endpoints and payloads.
  - run failures list filter forwarding and promote-failure payload.

## Integration / Functional Tests
- `go test ./cmd -run '(Artifact|ReleaseGate|Regression|RunFailure|PromoteFailure)' -count=1` passes.
- `go test ./cmd -count=1` passes.

## Smoke Tests
- `go build ./...` passes from `cli/`.
- `go vet ./...` passes from `cli/`.

## E2E Tests
- N/A - this CLI surface patch is covered by command-level fake API tests. Live staging smoke remains optional because it requires credentials and seeded workspace data.

## Manual / cURL Tests
```bash
cd cli
go run . artifact list --workspace ws_123 --json
go run . release-gate list --baseline run_base --candidate run_candidate --json
go run . regression-suite list --workspace ws_123 --json
go run . run failures run_123 --workspace ws_123 --json
```

Expected: each command calls the documented backend endpoint, emits valid JSON under `--json`, and returns backend errors without swallowing them.
