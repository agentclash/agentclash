# codex/agent-tryouts-execution-pipeline - Test Contract

## Functional Behavior
- Creating an anonymous tryout for an anonymous-enabled template persists a tryout, dispatches one backend execution when a public execution target is configured, sets `run_id`, and returns a non-queued status once dispatch starts.
- Creating a workspace tryout uses the same dispatch path, scoped to the caller workspace, and links the tryout to the canonical harness run created by the execution pipeline.
- Dispatch is idempotent: retrying dispatch for the same tryout must not create a second execution/run and must return the existing linked tryout.
- Tryout status mirrors execution lifecycle at user-facing read time: queued/provisioning/running/scoring map to `running`, completed maps to `completed`, failed maps to `failed`, and cancelled maps to `cancelled`.
- Completion persists a safe summary with status, execution identifiers, cost/latency fields when known, and redaction status.
- Execution startup failures are persisted as `failed` with a stable machine-readable code and safe user-facing message; raw internal errors must not leak to public responses.
- Invalid templates, disabled anonymous templates, malformed UUIDs, oversized bodies, invalid JSON, duplicate JSON documents, unauthorized workspace access, already claimed tryouts, and missing tryouts return stable API error codes.

## Unit Tests
- `TestAgentTryoutManagerDispatchesAnonymousTryout` - creates a tryout, starts exactly one harness execution, sets `run_id`, and returns running tryout.
- `TestAgentTryoutManagerDispatchIsIdempotentWhenRunAlreadyLinked` - duplicate dispatch does not create another execution.
- `TestAgentTryoutManagerMarksTryoutFailedWhenDispatchFails` - startup failures produce safe summary code/message and failed status.
- `TestAgentTryoutManagerMapsHarnessExecutionStatusOnRead` - read path maps active, completed, failed, and cancelled execution states to tryout statuses.
- `TestBuildAgentTryoutHarnessPayload` - template and input produce the expected task prompt, execution config, evaluation config, and harness snapshot.

## Integration / Functional Tests
- Repository integration verifies setting `run_id` only when absent and no-op behavior when an existing run is already linked.
- Repository integration verifies status/summary/cost/latency/redaction updates round-trip.
- API handler tests verify anonymous and workspace create responses include linked execution metadata.

## Smoke Tests
- `go test -short -count=1 ./internal/api -run AgentTryout`
- `go test -short -count=1 ./internal/repository -run AgentTryout`
- `go test -short -count=1 ./...`
- Local API `/healthz` returns 200.

## E2E Tests
- N/A - full sandbox execution requires provider credentials; this change must still curl the local API with dev auth and verify dispatch/failure behavior against the local backend.

## Manual / cURL Tests
- `GET /v1/agent-tryout-templates` returns built-in templates.
- `POST /v1/agent-tryouts` with meeting-minutes input returns `201`, an id, status, and linked execution metadata when configured.
- `GET /v1/agent-tryouts/{id}` returns the latest mapped status and summary.
- `POST /v1/workspaces/{workspaceID}/agent-tryouts` with dev auth returns workspace-scoped tryout and linked execution metadata.
- `GET /v1/workspaces/{workspaceID}/agent-tryouts?limit=1&offset=0` returns the created item.
- `GET /v1/workspaces/{workspaceID}/agent-tryouts/{id}` returns the created workspace item.
- `POST /v1/agent-tryouts/{anonymousID}/claim` claims an anonymous tryout once; the second claim returns conflict.
- `POST /v1/agent-tryouts/{workspaceID}/share` and `GET /public/shares/{token}` expose the private share safely.
- Error curls cover malformed JSON, duplicate JSON, invalid tryout id, unknown template, disabled anonymous template, oversized input, unauthenticated workspace create, forbidden workspace create, missing tryout, and wrong workspace.
