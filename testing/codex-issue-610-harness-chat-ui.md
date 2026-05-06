# codex/issue-610-harness-chat-ui — Test Contract

Implements GitHub issue #610.

## Files Touched

- `web/src/app/(workspace)/workspaces/[workspaceId]/agent-harnesses/agent-harnesses-client.tsx` — redesign the default Agent Harness page into a chat-first workbench, using existing harness and execution APIs.
- `web/src/app/(workspace)/workspaces/[workspaceId]/agent-harnesses/agent-harnesses-client.test.tsx` — update coverage for chat-first layout, live progress, follow-up composer, and actionable failure states.

## External APIs Used

- N/A — this PR uses existing React/Next/Vitest code and existing AgentClash API client hooks already present in the repo. No new third-party API or package is introduced.

## Rollback Strategy

Revert this PR to restore the current table/form-centered Agent Harness page. No database, API, or worker changes are introduced.

## Functional Behavior

- The Agent Harness page should present a chat-first workbench as the primary experience.
- Users should see a prominent natural-language task composer for the selected harness.
- Repository, runner, auth, and advanced setup details should remain visible but visually secondary.
- Existing harnesses should still be selectable.
- Sending a message should keep using `POST /v1/workspaces/{workspaceId}/agent-harnesses/{harnessId}/executions` with `{message}` when the prompt is non-empty.
- The latest execution should be summarized as a live progress surface with approachable phase names.
- The page should still support expanding the latest execution into safe summarized timeline details: event type, actor, timestamp, phase/status, and allowlisted short payload fields only.
- The page must not render raw JSON, full diffs, long logs, artifact links/downloads, replay routes, or new artifact storage behavior. Those belong to #582.
- GitHub issue URLs/text should be accepted as normal task composer text and posted exactly as `{message}`. This PR must not fetch GitHub issues, snapshot issue metadata, write task-bank data, or implement suite ingestion.
- Failed executions should surface an actionable failure panel instead of only showing a `failed` badge.
- Failed execution copy should prefer `execution.error_message`, then fall back to the latest failed event payload's `error` or `message`.
- Empty, loading, and error states should remain understandable.
- The UI should be responsive and should not depend on new backend fields.

## Unit Tests

- `AgentHarnessesClient` renders a chat-first composer when harnesses exist.
- `AgentHarnessesClient` posts follow-up prompts to the existing execution endpoint.
- `AgentHarnessesClient` renders live execution phase/progress from existing events.
- `AgentHarnessesClient` expands safe summarized timeline details.
- `AgentHarnessesClient` does not render full diff/log payloads from `artifact.git_diff` or other large event payloads.
- `AgentHarnessesClient` accepts a GitHub issue URL as plain composer text and posts it exactly as a run message.
- `AgentHarnessesClient` renders actionable failure copy when latest execution failed.
- `AgentHarnessesClient` uses `error_message` when present and falls back to failed event payloads.
- `AgentHarnessesClient` covers loading, error, empty, queued/provisioning/running/scoring/completed/failed, no-events waiting state, harness selection, empty prompt disabled, successful post, mutation, and prompt clearing.
- `web/src/lib/api/types.ts` exposes `error_message?: string` for `AgentHarnessExecution`.
- Existing empty/loading/error state coverage remains valid or is updated.

## Integration / Functional Tests

- Frontend component tests mock the existing API hooks and verify the user workflow:
  - Select harness.
  - Type a prompt.
  - Send.
  - Observe mutation calls and cleared prompt.
  - Inspect latest execution activity.

## Smoke Tests

- `npm test -- --run 'src/app/(workspace)/workspaces/[workspaceId]/agent-harnesses/agent-harnesses-client.test.tsx'` from `web/`.
- `npm run lint -- 'src/app/(workspace)/workspaces/[workspaceId]/agent-harnesses/agent-harnesses-client.tsx' 'src/app/(workspace)/workspaces/[workspaceId]/agent-harnesses/agent-harnesses-client.test.tsx'` from `web/` if lint supports path arguments.

## E2E Tests

- N/A for this PR. A browser smoke is enough because no backend behavior changes. Later #582 will introduce deeper execution replay navigation.

## Manual / cURL Tests

Manual browser smoke after starting the web app:

1. Open `/workspaces/{workspaceId}/agent-harnesses`.
2. Confirm the first viewport is a chat/workbench experience, not a table-first admin form.
3. Confirm the selected harness details are visible but secondary.
4. Type a task prompt and send it.
5. Confirm live activity and expandable timeline remain available.
6. Confirm failed latest execution shows next-step copy.

No cURL tests are needed because this PR does not change API behavior.
