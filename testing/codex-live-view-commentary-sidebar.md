# codex/live-view-commentary-sidebar — Test Contract

## Functional Behavior
- The run detail live arena must consume SSE envelopes emitted by the backend's snake_case wire format and project them into per-agent lane state correctly.
- When a run is live and SSE events arrive, each lane should update its now-doing banner, activity ticker, streaming output, counters, and step indicator instead of remaining stuck in an idle state.
- A commentator sidebar can be enabled from the run detail UI with a toggle and disabled again without changing challenge pack configuration.
- When enabled, the commentator sidebar should derive short commentary entries from live run events and show them newest-last in a bounded feed.
- When commentary is disabled, the feed should stop accumulating hidden history and should reopen as a fresh feed the next time the user enables it.
- The sidebar should render the same bounded number of commentary entries that the commentary store retains rather than silently hiding half of them.
- Commentary timestamps should be displayed in UTC and labeled accordingly so backend event times are not reinterpreted in the browser's local timezone.
- The commentator feed should work for comparison and single-agent runs, handle repeated events without duplicate entries, and degrade gracefully when no commentary has been generated yet.
- Existing run detail features such as scorecards, ranking, replay links, and polling fallback should continue to work.

## Unit Tests
- `normalizeRunEvent` converts snake_case SSE payloads into the camel/Pascal-shaped event object consumed by the live arena code.
- `normalizeRunEvent` preserves already-normalized event objects so existing callers do not regress.
- Commentator helpers produce commentary for meaningful arena events and suppress noisy events like token deltas.
- Commentator state stays bounded and deduplicates by event ID.
- The commentary sidebar renders all entries up to `MAX_COMMENTARY_ENTRIES` and labels timestamps as UTC.

## Integration / Functional Tests
- The run detail live event pipeline accepts a backend-shaped SSE event and updates the rendered live arena lane.
- Toggling commentary on shows the sidebar and toggling it off hides it.
- When commentary is enabled and live events arrive, rendered commentary text reflects those events.
- Toggling commentary off clears the feed and hidden SSE events do not repopulate commentary until the user opts back in.

## Smoke Tests
- `npm test -- --runInBand` or targeted Vitest runs for the new arena/commentary tests pass.
- `npm run lint` passes for the touched web files.

## E2E Tests
- N/A — not applicable for this change. No browser automation suite is currently being added.

## Manual / cURL Tests
```bash
cd web
npm test -- src/hooks/use-run-events.test.ts src/hooks/use-agent-commentary.test.ts src/app/'(workspace)'/workspaces/'[workspaceId]'/runs/'[runId]'/run-detail-client.test.tsx
npm test -- src/components/arena/live-commentary-sidebar.test.tsx
npm run lint -- src/hooks/use-run-events.ts src/hooks/use-agent-commentary.ts src/components/arena/live-commentary-sidebar.tsx src/components/arena/live-commentary-sidebar.test.tsx src/app/'(workspace)'/workspaces/'[workspaceId]'/runs/'[runId]'/run-detail-client.tsx
```

Manual UI steps:
1. Start a live run in the web app.
2. Open the run detail page and confirm the "Live" badge appears.
3. Verify an active lane advances from "Waiting for next action…" into model/tool/scoring activity as SSE events arrive.
4. Enable the commentator toggle and confirm a sidebar appears with short play-by-play commentary.
5. Disable the toggle and confirm the sidebar disappears, the feed resets, and the live lanes continue updating.
6. Re-enable commentary and confirm the feed starts empty until new commentary-worthy events arrive.
7. Confirm commentary timestamps are labeled `UTC`.
