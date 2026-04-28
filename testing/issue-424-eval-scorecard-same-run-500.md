# Issue 424 Contract: Same-Run Agent Comparison

## Scope

Fix issue #424 so `eval scorecard` and `compare runs` do not return HTTP 500 when comparing two different agents from the same multi-agent run.

## Functional Expectations

- `GET /v1/compare` should allow equal `baseline_run_id` and `candidate_run_id` when both explicit run-agent IDs are provided and the run-agent IDs differ.
- Same-run same-agent comparison should be rejected as a client error, not as an internal server error.
- Same-run comparison without enough explicit agent identity should still be rejected clearly because run-level comparison is ambiguous.
- The API handler should map validation failures from compare input to HTTP 400 with a machine-readable error code.
- The CLI should keep calling the compare endpoint for `eval scorecard` and `compare runs`; it should benefit from the backend behavior without a workflow-specific workaround.
- Cross-run comparison behavior should remain unchanged.

## Tests To Add Or Run

- Add backend API tests for the compare read manager:
  - same run plus two different run-agent IDs reaches `BuildRunComparison`.
  - same run plus the same run-agent ID returns a validation/client error before repository comparison.
  - same run without explicit agent IDs returns a validation/client error before repository comparison.
- Add or update handler tests so compare validation errors return HTTP 400 instead of HTTP 500.
- Run targeted backend tests:
  - `cd backend && go test ./internal/api -run 'TestCompare|TestGetRunComparison'`
- Run targeted CLI tests:
  - `cd cli && go test ./cmd -run 'TestEvalScorecard|TestCompareRuns'`
- Run broader touched-package validation:
  - `cd backend && go test ./internal/api ./internal/repository`
  - `cd cli && go test ./cmd`

## Manual Verification

- For a real multi-agent run, this should no longer return HTTP 500:
  - `agentclash compare runs --baseline <run> --baseline-agent <agent-a> --candidate <same-run> --candidate-agent <agent-b> --json`
- `agentclash eval scorecard <run> --agent <non-baseline-label> --json` should either return a valid comparison or a non-internal, actionable client error.

## Out Of Scope

- Rebuilding scorecard comparison semantics.
- Changing the CLI output schema beyond avoiding internal errors.
- Supporting same-run run-level comparison without explicit agent IDs.
