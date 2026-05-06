# Test Contract: Issue #582 Harness Replay And Artifacts

## Intent

Make Agent Harness executions observable through the existing canonical run replay path instead of duplicating replay/scoring storage.

## Expectations

- Harness workflow events are mirrored to linked `run_events` when `agent_harness_executions.run_id` and `run_agent_id` are present.
- Mirrored events use existing `runevents` types and preserve the original harness event type in payload metadata.
- Harness completion rebuilds the linked `run_agent_replays` summary through the existing replay builder.
- API responses expose bridge IDs so the UI can link to canonical replay/scorecard endpoints.
- Large/raw payload handling should avoid adding new harness-only replay tables.

## Verification

- `go test ./internal/workflow`
- `go test ./internal/repository ./internal/api`
