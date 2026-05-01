# Fix E2B Process Stream Timeout — Test Contract

## Functional Behavior

- Long-running E2B process streams, including `codex exec`, must not be cut off by the provider HTTP client's default 30 second total request timeout.
- E2B control-plane calls should continue using the configured request timeout.
- Command execution should still be bounded by the `ExecRequest.Timeout` context passed to `session.Exec`.

## Unit Tests

- Add coverage proving the E2B API client keeps the configured timeout for control-plane requests.
- Add coverage proving the E2B process stream client has no total `http.Client.Timeout`.

## Integration / Functional Tests

- `go test ./internal/sandbox/e2b`
- `go test ./internal/sandbox/...`

## Smoke Tests

- `go test ./internal/workflow -run AgentHarness`
- `go test ./internal/api -run AgentHarness`

## E2E Tests

- N/A — live harness testing already reproduced the stream timeout; this patch covers the client wiring that caused it.

## Manual / cURL Tests

- After deploy, rerun Agent Harness execution `agentclash/agentclash Codex` with an explicit no-clarification prompt and confirm it can stream beyond 30 seconds without `deadline_exceeded`.
