# Issue 462 Agent Harness Executions — Test Contract

## Functional Behavior

- Agent harness executions are persisted separately from challenge-pack runs.
- Starting an execution snapshots harness configuration so later harness edits do not rewrite execution history.
- Execution status supports `queued`, `provisioning`, `running`, `scoring`, `completed`, `failed`, and `cancelled`.
- A workspace user can start, list, and get executions only within workspaces they can access.
- Cross-workspace fetches return not found instead of leaking existence.
- The initial API/CLI surface creates queued executions; worker/E2B execution is outside this PR unless existing worker hooks are already ready to attach safely.

## Unit Tests

- `TestAgentHarnessExecutionManagerStartSnapshotsHarness` — creates a queued execution from an existing harness and stores execution/evaluation config snapshots.
- `TestAgentHarnessExecutionManagerStartChecksWorkspaceBeforeHarnessFetch` — unauthorized workspace callers cannot probe harness IDs.
- `TestAgentHarnessExecutionManagerGetReturnsNotFoundForWorkspaceMismatch` — workspace mismatch maps to not found.
- CLI command tests verify:
  - `agent-harness run <harness-id>` posts to the execution endpoint.
  - `agent-harness executions <harness-id>` lists executions scoped to a harness.
  - `agent-harness execution get <execution-id>` fetches a single execution.

## Integration / Functional Tests

- API route tests cover:
  - `POST /v1/workspaces/{workspaceID}/agent-harnesses/{harnessID}/executions`
  - `GET /v1/workspaces/{workspaceID}/agent-harness-executions`
  - `GET /v1/workspaces/{workspaceID}/agent-harness-executions/{executionID}`
- Repository tests cover create/list/get behavior for the new execution table if the project has a suitable database test harness.

## Smoke Tests

- `cd backend && go test ./internal/api ./internal/repository`
- `cd cli && go test ./cmd`

## E2E Tests

N/A — this PR creates the execution control plane. Full hosted Codex/E2B execution, replay, artifacts, scoring, and web run UX remain follow-up slices from issue #462.

## Manual / cURL Tests

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
cd cli
go run . agent-harness run <harness-id> --workspace <workspace-id>
go run . agent-harness executions <harness-id> --workspace <workspace-id>
go run . agent-harness execution get <execution-id> --workspace <workspace-id>
```

Expected:

- `run` prints the new execution ID and `queued` status.
- `executions` includes the new execution.
- `execution get` shows the harness ID, status, created timestamp, and snapshot-backed fields.
