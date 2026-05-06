# Review Checkpoint - Issue 611 Harness Execution Controls

## Expectations

- Add cancel and retry controls to the existing Agent Harness execution API; do not introduce a parallel runner.
- Persist recoverable Temporal workflow identifiers for harness executions so operations can target the right workflow.
- Make cancellation safe and idempotent for terminal executions, with workspace authorization enforced before side effects.
- Make retry explicit and idempotent via a caller-provided idempotency key, creating a normal `agent_harness_executions` row that reuses the original snapshots.
- Enforce a workspace-level active harness execution cap before starting single, suite, or retry executions.
- Preserve existing timeout cleanup behavior and make cancellation/timeout states clear in execution status and events.
- Cover repository transitions, API authorization/routes, Temporal start/cancel wiring, retry idempotency, and concurrency-limit behavior.

## DRY Constraints

- Reuse existing `agent_harness_executions`, status transitions, canonical run bridge, replay/events, validators, LLM judges, and scorecards.
- Reuse Temporal workflow operations for control; no new sandbox scheduler or evaluator path.
