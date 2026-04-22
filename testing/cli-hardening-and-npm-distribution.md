# cli-hardening-and-npm-distribution — Test Contract

## Functional Behavior
- `GET /v1/runs/{runID}/events/stream` should emit persisted run events that already exist when a client first connects, not only future live pub/sub messages.
- The stream should continue to emit newly persisted run events while the run is active, even if live Redis pub/sub messages are missing or delayed.
- SSE event IDs must be unique across a whole run, including multi-agent runs.
- The stream should end promptly after a terminal run has no more unseen persisted events to send.
- Existing auth and access checks for the stream must remain unchanged.

## Unit Tests
- `TestRunEventsStreamEmitsPersistedEventsBeforeLiveTail` — persisted run events are sent immediately on connect.
- `TestRunEventsStreamUsesUniqueCompoundEventIDs` — SSE IDs include enough information to distinguish events from different run agents.
- `TestRunEventsStreamPollsPersistedEventsWhenNoLiveMessagesArrive` — a live run still streams events through the persisted-event fallback.

## Integration / Functional Tests
- `go test ./backend/internal/api -run RunEventsStream`
- `go test ./backend/internal/repository -run RunEvents`

## Smoke Tests
- Start a run and attach with `agentclash run events <id>` after the run has already begun; existing events should print immediately.
- Start a run and attach with `agentclash run events <id>` while it is active; later events should continue to appear until completion.

## E2E Tests
- N/A — not applicable for this backend streaming fix.

## Manual / cURL Tests
```bash
TOKEN="$(jq -r .token ~/.config/agentclash/credentials.json)"
curl -N \
  -H "Accept: text/event-stream" \
  -H "Authorization: Bearer $TOKEN" \
  "https://api.agentclash.dev/v1/runs/<run-id>/events/stream"
```
- Expected: previously persisted events appear first, followed by newly persisted events while the run is still active.
