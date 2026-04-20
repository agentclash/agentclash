# codex/issue-361-eval-session-workflow — Test Contract

## Functional Behavior
- Creating an eval session starts exactly one durable parent workflow for the new session after the session and child runs are created transactionally.
- The parent workflow loads the eval session, verifies it is still `queued`, and transitions the session `queued -> running`.
- The parent workflow launches one `RunWorkflow` child per attached child run using deterministic child workflow IDs derived from each run ID.
- The parent workflow waits for every child workflow future to resolve before transitioning the session to `aggregating`.
- If some child workflows fail but at least one child completes, the parent workflow still advances the session to `aggregating` and does not cancel successful children.
- If every child workflow fails, the parent workflow marks the eval session `failed`.
- Cancelling the parent workflow marks the eval session `cancelled` and requests cancellation for still-running children through Temporal parent-close semantics.
- `repetitions = 1` behaves like a single-child fan-out: one child run is started and the session still reaches `aggregating` after that child settles.

## Unit Tests
- `TestRunCreationManagerCreateEvalSessionStartsEvalSessionWorkflow` — create path starts the parent workflow with the created session ID.
- `TestTemporalEvalSessionWorkflowStarterStartEvalSessionWorkflow` — starter uses `EvalSessionWorkflow` name, task queue, and workflow ID format `EvalSessionWorkflow/<sessionID>`.
- `TestEvalSessionWorkflowHappyPath` — queued session with two child runs transitions `queued -> running -> aggregating`.
- `TestEvalSessionWorkflowAllChildrenFailMarksSessionFailed` — all child workflow failures mark the session failed and skip `aggregating`.
- `TestEvalSessionWorkflowPartialChildFailureStillAggregates` — mixed child outcomes still reach `aggregating` after all children resolve.
- `TestEvalSessionWorkflowCancellationMarksSessionCancelled` — cancellation marks the session cancelled.
- `TestEvalSessionWorkflowRequiresQueuedSession` — non-queued sessions fail before transitions.
- `TestEvalSessionWorkflowNoChildRunsFails` — empty child run set fails explicitly.

## Integration / Functional Tests
- Existing `CreateEvalSession` service tests verify the created session and child run IDs still come back unchanged when workflow start is added.
- Workflow tests verify the parent workflow invokes existing `RunWorkflow` children rather than duplicating run-creation logic.

## Smoke Tests
- `cd backend && go test ./internal/api ./internal/workflow`
- `cd backend && go test -short ./...`

## E2E Tests
- N/A — not applicable for this workflow-orchestration slice.

## Manual / cURL Tests
```bash
curl -X POST http://localhost:8080/v1/eval-sessions \
  -H "Content-Type: application/json" \
  -H "X-User-Id: <user-id>" \
  -H "X-Workspace-Id: <workspace-id>" \
  -d @/tmp/eval-session-request.json
# Expected: 201 Created with a queued eval session and attached child run ids.

curl http://localhost:8080/v1/eval-sessions/<session-id> \
  -H "X-User-Id: <user-id>" \
  -H "X-Workspace-Id: <workspace-id>"
# Expected after workers process the session: status advances to aggregating once all child runs are terminal.
```
