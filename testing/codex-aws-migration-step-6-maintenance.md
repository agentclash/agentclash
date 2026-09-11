# Step 6 — maintenance readiness test contract

## Functional Behavior

- Worker SDK stop grace is explicit and shorter than the application shutdown
  budget. Stop every configured queue concurrently; stop reapers on cancellation.
  Wait for activity cleanup before closing shared dependencies. A stuck activity
  or reaper produces a bounded shutdown error, never a false successful drain.
- Worker process cleanup also runs on startup/shutdown errors, with a fresh,
  bounded context for sandbox pool cleanup.
- API shutdown marks readiness unavailable, closes event streams and allows
  ordinary in-flight requests to finish within its deadline. Force-close after
  that deadline. Configured Redis is checked along with Postgres and Temporal;
  intentionally disabled Redis remains supported.
- Quiet SSE connections emit flushed comments at a configurable interval without
  inventing event IDs. Detect write failures. Persisted polling and reconnects
  using Last-Event-ID must not resend events preceding the cursor on that
  connection; an unknown cursor retains the existing full-replay behavior.
- Publish a sanitized drain/write-fence runbook covering all source writers,
  generated and custom ingress, authentication writes, callbacks, schedules,
  PostgreSQL/Redis, final backups, reconciliation and the rollback boundary.
- Shutdown grace does not make in-memory multi-turn activities resumable or
  guarantee exactly-once provider charges. Migration still requires drained
  executions or an explicit decision about unresolved work.

## Unit Tests

- Worker configuration rejects invalid durations and inconsistent budgets.
- All queues receive Stop before any blocked queue can hold up another; reapers
  and activity cleanup complete, or the application deadline reports failure.
- SDK activity tracking does not start business logic after shutdown completes.
- SSE comments, write failures, cursor replay, persisted polling and released
  capacity are covered; shutdown terminates long-lived stream subscriptions.
- Readiness tests cover draining, configured Redis failure and optional Redis.
- Existing workflow cancellation/retry classification, human-turn cancellation,
  spend reconciliation and sandbox cleanup tests remain passing. Repeated spend
  reconciliation must not add the same case cost twice.

## Integration / Functional Tests

- A disposable local Temporal server demonstrates activity completion within
  SDK grace and cancellation followed by cleanup after grace expiry. No Temporal
  Cloud or E2B calls are permitted by this test.
- A local HTTP stream remains alive while quiet, resumes from a saved cursor,
  and closes on shutdown without waiting for the whole API deadline. A blocked
  ordinary handler demonstrates the forced-close path.
- Disposable PostgreSQL tests exercise persisted billing webhook replay and
  retry behavior, using the application migrator. No existing database is used.

## Smoke Tests

- Build and vet the backend; run appropriate backend/runtime race tests in a
  clean environment without production connection variables.
- Local liveness/readiness checks return the documented status. Actual proxy,
  container stop budgets and AWS readiness remain deployment rehearsal checks
  for later steps, not claims made by this local preparation step.

## E2E Tests

Production E2E is deferred to the approved deployment rehearsal and cutover.
This step uses local lifecycle tests and existing workflow suites. It must not
claim that a forced process kill can resume an in-memory human-turn activity.

## Manual / cURL Tests

- Follow the maintenance runbook against a disposable environment: connect a
  quiet event stream, preserve its last real event ID, signal shutdown, reconnect
  to the replacement and confirm only later events arrive.
- Verify the final source fence rejects every application path before auth,
  including GET requests, generated service URLs and direct endpoints. Confirm
  all writers are stopped before the final snapshot. Live execution is deferred.
- Review the exact diff and artifacts and run the configured secret scanner
  before each local commit. Private operational evidence stays outside Git.
- Stop after the Step 6 handoff. Do not push, provision, cut over or start Step 7.
