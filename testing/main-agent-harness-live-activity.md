# main-agent-harness-live-activity - Test Contract

## Functional Behavior
- Agent Harnesses must show what the latest harness execution is doing, not only a run icon.
- The list should surface the latest execution status, current phase, recent event message, and when that event happened.
- Users should be able to expand a harness row to inspect a readable timeline of execution events.
- Event payloads should expose decision-like details when present, including phase, message, command, tool, and result/summary fields.
- Empty, loading, and no-event states should be explicit and compact.

## Unit Tests
- Agent Harness UI tests should cover rendering latest activity from execution events.
- Agent Harness UI tests should cover expanding a row to show the event timeline.

## Integration / Functional Tests
- Existing API types for `AgentHarnessExecution.events` should compile through the UI.
- Existing create/run Agent Harness behavior should continue to compile.

## Smoke Tests
- From `web/`: `npm test -- --run agent-harnesses`
- From `web/`: `npx tsc --noEmit`
- From `web/`: targeted `npx eslint` on changed Agent Harness files.

## E2E Tests
- N/A - this is a focused UI improvement over existing execution list data.

## Manual / cURL Tests
- Open Agent Harnesses in a workspace with at least one execution containing events.
- Confirm the table shows latest status plus live activity details.
- Expand the row and confirm recent events are readable without raw JSON-first UX.
