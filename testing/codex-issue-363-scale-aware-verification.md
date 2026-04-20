# codex/issue-363-scale-aware-verification — Test Contract

## Functional Behavior
- Add authenticated eval-session read surfaces that let a developer inspect repeated-eval state after creation without querying raw tables.
- Expose a stable `GET /v1/eval-sessions/{evalSessionID}` read path that returns:
  - the eval-session metadata and locked config snapshots
  - child runs in deterministic creation order
  - a session summary block that reports child-run counts by status
  - an `aggregate_result` field that is `null` until aggregation work lands, while remaining shape-stable for future issues
  - `evidence_warnings` that explain missing aggregate/session evidence instead of failing silently
- Expose a stable `GET /v1/eval-sessions` list path that returns recent sessions with enough metadata to inspect scale-tier coverage and status at a glance.
- Keep the read-model response compatible with future aggregation work from `#361` and metric-routing/comparison work from `#362` by avoiding issue-specific ad hoc fields.
- Add a scriptable smoke flow that exercises create -> inspect for eval sessions against a local stack.
- Add a written repeated-eval verification matrix under `testing/` for parent issue `#149` that covers:
  - `repetitions=1`
  - `repetitions=3-5`
  - `repetitions=10-30`
  - `repetitions=50-100`
- Split low-cost deterministic CI checks from opt-in/manual higher-scale checks.
- Make single-run compatibility explicit in the verification matrix rather than implied.

## Unit Tests
- API handler tests for:
  - listing eval sessions
  - fetching one eval session with child runs
  - 404 on missing eval session
  - auth failure behavior matching existing protected endpoints
- Read-model/manager tests for:
  - deterministic run ordering
  - summary counts by status
  - `aggregate_result` remaining `null` when no aggregated document exists yet
  - evidence warnings when aggregation/read evidence is not yet available
  - list pagination defaults or limit handling, if introduced

## Integration / Functional Tests
- Repository integration test that creates an eval session with queued runs, reads it back through the new read path, and verifies session metadata plus child-run ordering/status counts.
- API integration test proving a create -> get flow returns the same locked eval-session config snapshots that were persisted.
- Functional verification that the smoke script can create a session, fetch the session detail, and confirm the expected queued-run counts.

## Smoke Tests
- `cd /Users/atharva/agentclash-issue-363/backend && go test -short -race -count=1 ./internal/api ./internal/repository`
- `cd /Users/atharva/agentclash-issue-363/backend && go test -short -race -count=1 ./...`
- With the local stack running:
  - `cd /Users/atharva/agentclash-issue-363 && ./scripts/smoke/eval-session-create.sh`
  - `cd /Users/atharva/agentclash-issue-363 && ./scripts/smoke/eval-session-read.sh`

## E2E Tests
- N/A — not applicable for this slice because the user-facing work is backend/API verification plus scriptable smoke coverage, not a browser workflow.

## Manual / cURL Tests
```bash
# Start the local stack only if needed for API verification
cd /Users/atharva/agentclash-issue-363

# Create a session
curl -i -X POST http://localhost:8080/v1/eval-sessions \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: <user-id>" \
  -H "X-Agentclash-Workspace-Memberships: <workspace-id>:workspace_admin" \
  -d '{
    "workspace_id": "<workspace-id>",
    "challenge_pack_version_id": "<pack-version-id>",
    "challenge_input_set_id": "<input-set-id>",
    "participants": [
      {
        "agent_build_version_id": "<agent-build-version-id>",
        "label": "Primary"
      }
    ],
    "execution_mode": "single_agent",
    "name": "Verification matrix sanity check",
    "eval_session": {
      "repetitions": 3,
      "aggregation": {
        "method": "mean",
        "report_variance": true,
        "confidence_interval": 0.95
      },
      "routing_task_snapshot": {
        "routing": { "mode": "single_agent" },
        "task": { "pack_version": "v1", "input_set": "default" }
      },
      "schema_version": 1
    }
  }'

# Inspect the detail read model
curl -i http://localhost:8080/v1/eval-sessions/<eval-session-id> \
  -H "X-Agentclash-User-Id: <user-id>" \
  -H "X-Agentclash-Workspace-Memberships: <workspace-id>:workspace_admin"

# Inspect the list read model
curl -i "http://localhost:8080/v1/eval-sessions?limit=10" \
  -H "X-Agentclash-User-Id: <user-id>" \
  -H "X-Agentclash-Workspace-Memberships: <workspace-id>:workspace_admin"

# Expected:
# - detail response returns eval_session metadata, ordered child runs, summary counts, aggregate_result=null, and non-empty evidence_warnings
# - list response includes the created session
# - the summary block reports queued child runs for a freshly created session
```
