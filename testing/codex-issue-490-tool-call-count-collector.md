# codex/issue-490-tool-call-count-collector - Test Contract

## Functional Behavior
- Metrics with `collector: run_tool_call_count` return an available numeric value when run evidence includes `tool_call_count` on `system.run.completed`.
- The collector does not produce `collector "run_tool_call_count" is not supported`.
- Existing collectors such as `run_total_tokens`, `run_total_latency_ms`, and `run_completed_successfully` remain unchanged.
- If explicit terminal `tool_call_count` is absent, the collector may fall back to observed tool-call events when available.

## Unit Tests
- `TestEvaluateRunAgentCollectsToolCallCountFromRunCompleted` - evaluates a run with `tool_call_count` in terminal evidence and records the numeric metric.
- Existing scoring metric tests continue to pass.

## Integration / Functional Tests
- `cd backend && go test ./internal/scoring`.

## Smoke Tests
- `cd backend && go test ./internal/scoring -run TestEvaluateRunAgentCollectsToolCallCountFromRunCompleted -count=1`.

## E2E Tests
N/A - this is a deterministic scoring-engine collector fix covered by unit tests.

## Manual / cURL Tests
N/A - no API route changes; pack authors validate this by running a pack that declares `collector: run_tool_call_count` and observing an available numeric scorecard metric.
