# Issue 361 Eval Session Aggregation Contract

## Scope

This contract covers the remaining `#361` aggregation slice on top of `main` after `PR #369` landed the `EvalSessionWorkflow` fan-out prerequisite.

## Functional Expectations

1. Completed or partially completed repeated-eval sessions can persist exactly one session-level aggregate document keyed by `eval_session_id`.
2. Aggregation reads child `run_scorecards` for the session and computes deterministic aggregate statistics for:
   - `overall`
   - built-in score dimensions present in child scorecards
   - custom dimensions that appear in child scorecards
3. Aggregate statistics include, when evidence exists:
   - sample count `n`
   - `mean`
   - `median`
   - `std_dev`
   - `min`
   - `max`
   - an interval descriptor with deterministic estimator metadata
   - a deterministic `high_variance` flag with the rule encoded in the stored document
4. Aggregation is deterministic for the same child inputs and config:
   - child runs are processed in a stable order
   - dimension keys are emitted in a stable order
   - repeated aggregation on identical fixtures produces byte-identical JSON
5. Partial evidence is explicit, never silent:
   - missing child run scorecards are captured as warning evidence
   - dimensions missing from some or all children are skipped with warning evidence
   - single-sample aggregates record insufficient-evidence warnings instead of fake intervals
6. Sessions with at least one scored child can transition from `aggregating` to `completed` after the aggregate row is persisted.
7. Sessions with zero scored children fail aggregation and transition from `aggregating` to `failed` without persisting a misleading aggregate document.
8. The eval-session read surfaces return the stored aggregate JSON when present and stop synthesizing the legacy “aggregation not persisted yet” warning for those sessions.
9. The OpenAPI schema documents the aggregate result shape so the API response is typed instead of an undocumented raw blob.

## Implementation Boundaries

1. Add a dedicated persistence table for session aggregates rather than overloading `eval_sessions`.
2. Keep the aggregate document versioned so future slices can extend it without reshaping the API.
3. Reuse the existing Temporal eval-session workflow and add a dedicated aggregation activity instead of moving aggregation logic into workflow code.
4. Do not add a new direct stats dependency; aggregation math must live in repo code added in this PR.
5. Do not change unrelated run-scoring or child-run workflow behavior beyond what is required to persist and surface the session aggregate.

## Tests To Add Or Update

1. Repository/unit tests for deterministic aggregation math and warning generation.
2. Repository/integration tests for aggregate persistence, load, and single-row upsert behavior.
3. Workflow tests proving aggregation runs after child completion and transitions the session to `completed` or `failed` appropriately.
4. API read tests proving `aggregate_result` is surfaced from storage and legacy warnings disappear when an aggregate exists.
5. OpenAPI validation if the repo’s documented tooling is available.

## Manual Verification

1. `cd backend && go test ./internal/repository ./internal/workflow ./internal/api`
2. `cd backend && go test -short ./...`
3. `cd backend && go vet ./...`
4. `npx @redocly/cli lint docs/api-server/openapi.yaml`

## Out Of Scope

1. Pass@k / pass^k and comparison semantics from `#362`
2. Frontend changes
3. New product-level interpretation metrics beyond the aggregate statistics and explicit warning evidence required for `#361`
