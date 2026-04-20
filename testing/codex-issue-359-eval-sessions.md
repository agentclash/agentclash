# codex/issue-359-eval-sessions — Test Contract

## Functional Behavior
- Add a new authenticated `POST /eval-sessions` endpoint without changing the request or response behavior of `POST /runs`.
- Accept a stable request shape containing:
  - `workspace_id`
  - `challenge_pack_version_id`
  - optional `challenge_input_set_id`
  - `participants` with `agent_build_version_id` and `label`
  - `execution_mode`
  - optional `name`
  - required `eval_session` block with `repetitions`, `aggregation`, optional `success_threshold`, required `routing_task_snapshot`, optional `reliability_weights`, and `schema_version`
- Reject `POST /eval-sessions` requests that omit `eval_session`.
- Validate repeated-eval fields with field-specific `422 Unprocessable Entity` errors using the locked `eval_session.*` error-code namespace.
- Support aggregation methods `median`, `mean`, and `weighted_mean`.
- Require at least one reliability-weight section when `aggregation.method` is `weighted_mean`.
- Treat `repetitions = 1` as a valid degenerate single-run eval session that still creates exactly one queued child run.
- Create the eval session and all child queued runs in one transactional flow so failures do not leave partial state behind.
- Persist the canonical snapshot-safe config across the eval-session storage columns so later reads return the same validated configuration shape without mutation.
- Return `201 Created` with the created eval session and all child run ids.
- Keep workflow launch out of scope for now: session and child runs remain `queued`, and no Temporal session workflow is started yet.

## Unit Tests
- API request decoding and validation tests for:
  - valid repeated-eval payloads
  - missing `eval_session`
  - invalid `repetitions`
  - unsupported aggregation methods
  - invalid `report_variance`
  - invalid `confidence_interval`
  - invalid `success_threshold.min_pass_rate`
  - invalid `success_threshold.require_all_dimensions`
  - invalid `routing_task_snapshot`
  - `weighted_mean` without reliability weights
  - invalid per-dimension, per-judge, and per-run reliability weights
  - invalid `schema_version`
- Service-layer tests for:
  - successful create path
  - `repetitions = 1`
  - rollback on run-creation or attachment failure
  - legacy `POST /runs` behavior remaining unchanged

## Integration / Functional Tests
- Repository or service integration test that creates an eval session, reads it back, and verifies the stored aggregation, success-threshold, and routing-task snapshots match the validated request exactly.
- Transactional failure test proving no session or child runs persist after an injected mid-transaction failure.
- API integration test proving `POST /eval-sessions` returns a queued session plus non-empty child run ids.

## Smoke Tests
- `cd /Users/atharva/agentclash-issue-359/backend && go test ./internal/api ./internal/repository`
- `cd /Users/atharva/agentclash-issue-359/backend && go test ./...`
- If local stack-backed verification is needed, bring up the local stack, call `POST /eval-sessions`, verify `201`, then shut the stack back down.

## E2E Tests
- N/A — not applicable for this change because it adds backend control-plane plumbing and API validation, not a browser or CLI end-user journey.

## Manual / cURL Tests
```bash
# Start the local stack only if needed for API verification
cd /Users/atharva/agentclash-issue-359

# Example happy-path request to verify the new endpoint after implementation
curl -i -X POST http://localhost:8080/v1/eval-sessions \
  -H "Content-Type: application/json" \
  -H "X-User-ID: <user-id>" \
  -H "X-Workspace-Memberships: <workspace-id>:workspace_member" \
  -d '{
    "workspace_id": "<workspace-id>",
    "challenge_pack_version_id": "<pack-version-id>",
    "challenge_input_set_id": "<input-set-id>",
    "participants": [
      {
        "agent_build_version_id": "<agent-build-version-id>",
        "label": "Primary agent"
      }
    ],
    "execution_mode": "single_agent",
    "name": "Repeated eval sanity check",
    "eval_session": {
      "repetitions": 3,
      "aggregation": {
        "method": "mean",
        "report_variance": true,
        "confidence_interval": 0.95
      },
      "success_threshold": {
        "min_pass_rate": 0.67,
        "require_all_dimensions": ["correctness"]
      },
      "routing_task_snapshot": {
        "routing": { "mode": "single_agent" },
        "task": { "pack_version": "v1", "input_set": "default" }
      },
      "schema_version": 1
    }
  }'

# Example invalid weighted-mean request
curl -i -X POST http://localhost:8080/v1/eval-sessions \
  -H "Content-Type: application/json" \
  -H "X-User-ID: <user-id>" \
  -H "X-Workspace-Memberships: <workspace-id>:workspace_member" \
  -d '{
    "workspace_id": "<workspace-id>",
    "challenge_pack_version_id": "<pack-version-id>",
    "participants": [
      {
        "agent_build_version_id": "<agent-build-version-id>",
        "label": "Primary agent"
      }
    ],
    "execution_mode": "single_agent",
    "eval_session": {
      "repetitions": 2,
      "aggregation": {
        "method": "weighted_mean",
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

# Expected:
# - valid request returns 201 with a queued eval session and non-empty run_ids
# - invalid weighted_mean request returns 422 with eval_session.reliability_weights.required
# - POST /runs continues to return the same status/body shape as before
```
