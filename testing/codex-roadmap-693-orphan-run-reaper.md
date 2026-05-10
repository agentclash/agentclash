# codex/roadmap-693-orphan-run-reaper — Test Contract

## Files touched

- `backend/db/queries/runs.sql` — add an atomic query for stale orphan run cleanup.
- `backend/internal/repository/repository.go` — expose a cleanup method that maps cleaned run rows and writes status history.
- `backend/internal/repository/repository_integration_test.go` or focused repository tests — cover cleanup predicates.
- `backend/internal/worker/config.go` — add conservative reaper interval/threshold configuration.
- `backend/internal/worker/orphan_run_reaper.go` — periodic worker loop.
- `backend/internal/worker/orphan_run_reaper_test.go` — unit-test loop behavior without sleeping.
- `backend/cmd/worker/main.go` — wire the reaper into worker startup.

## External APIs used

- Go `time.Ticker` and context cancellation for the periodic worker loop — verified-as-of: 2026-05-09 — source URL: https://pkg.go.dev/time#Ticker
- No Temporal API calls are used in this PR. Runs are selected only when `temporal_workflow_id IS NULL`.

## Rollback strategy

Revert this PR. The change is additive and does not introduce a schema migration. If the worker loop causes unexpected cleanup, set the interval to zero or revert worker wiring; already-marked rows remain failed as an auditable terminal state.

## Functional Behavior

- The cleanup predicate is exactly:
  - `status IN ('queued', 'provisioning')`
  - `temporal_workflow_id IS NULL`
  - `temporal_run_id IS NULL`
  - `created_at < cutoff`
- Matching runs transition to `failed`.
- `finished_at` and `failed_at` are set if they were unset.
- A `run_status_history` row is written with a clear reaper reason.
- Runs in `running` or `scoring` are not reaped in this first PR.
- Runs with any Temporal workflow/run ID are not reaped.
- Fresh rows newer than the cutoff are not reaped.
- Monthly quota counters are not changed.
- The worker loop is conservative and configurable:
  - default threshold is at least 15 minutes.
  - default interval is nonzero in worker runtime.
  - interval <= 0 disables the loop.

## Unit Tests

- Repository cleanup test marks only old queued/provisioning NULL-Temporal rows failed.
- Repository cleanup test skips fresh NULL-Temporal rows.
- Repository cleanup test skips rows with Temporal workflow or run IDs.
- Repository cleanup test skips running/scoring rows even if old and NULL-Temporal.
- Repository cleanup test writes one status-history row per cleaned run.
- Worker reaper test invokes cleanup once per tick and stops cleanly on context cancellation.
- Worker reaper test does nothing when interval is disabled.

## Integration / Functional Tests

```bash
cd backend
go test ./internal/repository ./internal/worker
```

## Smoke Tests

```bash
cd backend
go test ./...
```

## E2E Tests

No hosted mutation smoke for this PR. The reaper is a backend worker safety mechanism and should be verified through repository/worker tests before deployment. After deploy, operators can observe logs for cleaned run counts.

## Manual / cURL Tests

N/A — this PR intentionally adds no public API or CLI command.
