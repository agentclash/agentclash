# codex/issue-341-batch-regression-counts - Test Contract

## Functional Behavior
- Listing regression suites for a workspace returns the same `case_count` values as before.
- The repository computes listed suite case counts with one grouped query rather than one count query per suite.
- Existing suite detail and regression case listing behavior remains unchanged.
- The already-batched latest-promotion lookup from PR #547 remains in place for case list metadata.

## Unit Tests
- `TestRepositoryRegressionSuiteListBatchesCaseCounts` verifies listed suites receive correct counts for zero, one, and multiple cases without per-suite count fan-out.
- Existing regression repository tests continue to pass.

## Integration / Functional Tests
- Backend regression API tests continue to return the same suite response shape.
- Backend repository integration tests continue to validate regression suite/case round trips.

## Smoke Tests
- `cd backend && go test ./internal/repository -run 'TestRepositoryRegression|TestRepositoryRegressionSuiteListBatchesCaseCounts'`
- `cd backend && go test ./internal/api -run 'TestRegressionSuite'`
- `cd backend && go test ./...`
- `git diff --check`

## E2E Tests
N/A - this is a backend repository performance cleanup with unchanged API/UI behavior.

## Manual / cURL Tests
N/A - behavior is covered by repository/API tests and response shapes are unchanged.
