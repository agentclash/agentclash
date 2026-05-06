# Test Contract: Issue #612 Harness Data Model

## Intent

Decide the canonical execution data model for Agent Harness scoring, replay, and rankings without duplicating the existing challenge-run scoring stack.

## Expectations

- Document how Agent Harness executions reuse existing `runs`, `run_agents`, `run_events`, artifacts, replays, evaluation specs, judge results, metric results, scorecards, eval sessions, run comparisons, release gates, rankings, and failure review primitives.
- Add a backwards-compatible bridge from `agent_harness_executions` to the canonical run-agent model so existing harness executions remain readable.
- Keep workspace and organization scoping explicit on new foreign keys.
- Expose bridge identifiers in repository/API types so later subissues can wire replay/scoring without schema churn.
- Do not implement parallel validator, LLM judge, pass@k, replay, ranking, or failure-review storage for harnesses.

## Verification

- `go test ./internal/repository ./internal/api`
- `go test ./internal/workflow`
- `npm test -- --run 'src/app/(workspace)/workspaces/[workspaceId]/agent-harnesses/agent-harnesses-client.test.tsx'`
