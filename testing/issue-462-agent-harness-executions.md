# Issue 462 Agent Harness Executions — Test Contract

## Functional Behavior

- Agent harness executions are persisted separately from challenge-pack runs.
- Starting an execution snapshots harness configuration so later harness edits do not rewrite execution history.
- Execution status supports `queued`, `provisioning`, `running`, `scoring`, `completed`, `failed`, and `cancelled`.
- A workspace user can start, list, and get executions only within workspaces they can access.
- Cross-workspace fetches return not found instead of leaking existence.
- Starting an execution schedules the Temporal worker path that provisions a platform-owned E2B sandbox, clones the configured repository, runs Codex, and records trace/artifact events.
- AgentClash owns E2B billing through worker/provider config; users only provide Codex auth secrets.
- The web and CLI can start an execution from a chat-style message that overrides the reusable harness task for that execution snapshot.
- Execution status changes are persisted with status history rows.
- Execution events are persisted with per-execution sequence numbers so future workers can append replay/log events.
- Execution detail includes recent events so CLI/API users can inspect early progress without a separate replay implementation.

## Unit Tests

- `TestAgentHarnessExecutionManagerStartSnapshotsHarness` — creates a queued execution from an existing harness and stores execution/evaluation config snapshots.
- `TestAgentHarnessExecutionManagerStartChecksWorkspaceBeforeHarnessFetch` — unauthorized workspace callers cannot probe harness IDs.
- `TestAgentHarnessExecutionManagerGetReturnsNotFoundForWorkspaceMismatch` — workspace mismatch maps to not found.
- CLI command tests verify:
  - `agent-harness run <harness-id>` posts to the execution endpoint.
  - `agent-harness run <harness-id> --message "..."` snapshots the message as that execution's prompt.
  - `agent-harness run <harness-id> --follow` polls until a terminal execution status.
  - `agent-harness executions <harness-id>` lists executions scoped to a harness.
  - `agent-harness execution get <execution-id>` fetches a single execution.
- `TestAgentHarnessExecutionStatusTransitions` — legal execution status transitions succeed and illegal transitions fail.
- `TestAgentHarnessExecutionEventsSequencePerExecution` — events append in order with sequence numbers scoped to one execution.
- Workflow activity tests verify Codex execution records sandbox, command, diff, and changed-file events without requiring user-supplied E2B secrets.

## Integration / Functional Tests

- API route tests cover:
  - `POST /v1/workspaces/{workspaceID}/agent-harnesses/{harnessID}/executions`
  - `GET /v1/workspaces/{workspaceID}/agent-harness-executions`
  - `GET /v1/workspaces/{workspaceID}/agent-harness-executions/{executionID}`
- Repository tests cover create/list/get behavior for the new execution table if the project has a suitable database test harness.
- Repository tests cover transition history and event append/read behavior if the project has a suitable database test harness.
- API Temporal starter tests verify execution starts the correct workflow name, ID, and task queue.
- Web tests verify harness creation no longer asks for an E2B secret.
- API manager tests verify chat messages override only the execution snapshot prompt, not the stored harness.

## Smoke Tests

- `cd backend && go test ./...`
- `cd cli && go test ./...`
- `cd web && npm test -- agent-harnesses`
- `cd web && npx tsc --noEmit`

## E2E Tests

N/A — live hosted E2B execution requires deployed worker credentials. The worker path is covered with fake sandbox tests in this PR.

## Manual / cURL Tests

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
cd cli
go run . agent-harness run <harness-id> --workspace <workspace-id> --message "Inspect the repo and patch the failing test" --follow
go run . agent-harness executions <harness-id> --workspace <workspace-id>
go run . agent-harness execution get <execution-id> --workspace <workspace-id>
```

Expected:

- `run` prints the new execution ID and `queued` status.
- `executions` includes the new execution.
- `execution get` shows the harness ID, status, created timestamp, and snapshot-backed fields.
- When events exist, `execution get --output json` includes an `events` array ordered by `sequence_number`.
