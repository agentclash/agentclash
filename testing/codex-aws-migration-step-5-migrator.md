# Application migrator — test contract

This step prepares code and an image for application PostgreSQL migrations. It
does not migrate a live database, provision resources, or deploy services.

## Functional behavior

- Preserve `public.schema_migrations(version text PRIMARY KEY, applied_at
  timestamptz NOT NULL DEFAULT now())`. Versions remain complete file basenames
  without `.sql`; repeated numeric prefixes are independent migrations. Existing
  ledger entries and timestamps are never rewritten.
- Read and validate all migration files before connecting. Apply files in
  deterministic filename order, executing only their Goose Up sections. Reject
  missing/empty/malformed sections, unsupported directives, unsafe filenames and
  explicit transaction control before modifying the database. Historical SQL
  files remain unchanged.
- Hold one PostgreSQL session advisory lock on the same direct connection used
  for ledger checks and every migration, including across transaction commits.
  A bounded lock wait prevents an invocation waiting forever.
- Commit each migration and its ledger entry in one transaction. Failure or
  cancellation stops the run with a nonzero exit, rolls back the failing
  migration, and never applies subsequent files. Previously committed files
  remain recorded; retry safely resumes. No automatic Down or retry is attempted.
- Accept the connection string through `DATABASE_URL`, with configurable
  positive lock, statement and overall timeouts. Output identifies the stage,
  public migration filename and SQLSTATE where available, without printing
  connection details, SQL text, database error details or private file paths.
- The dedicated image runs a one-off, unprivileged migrator. Its build context
  includes only required source/module/migration files. The existing deployment
  script and local migration command delegate to the same runner. API startup
  does not automatically execute migrations.
- Application and Temporal schema tooling stay separate. Deployment callers
  must check the migrator exit status before advancing a rollout; SSM delivery
  implementation belongs to its later step.

## Unit tests

- File loading: deterministic order, duplicate numeric prefixes, Goose sections,
  comments/quoted strings/dollar-quoted functions, malformed or empty input,
  unsupported directives and transaction-control rejection.
- CLI configuration: absent/invalid connection settings, invalid timeouts,
  unsupported arguments, help, cancellation and sanitized diagnostics.
- Load the real repository migration set and assert each file has a unique
  basename; no conversion to numeric Goose versions occurs.

## Integration / functional tests

Run against disposable PostgreSQL 18, never an existing local or live database.
Use runtime-generated synthetic credentials and test data only.

- Fresh application: apply the complete repository history, compare every ledger
  name with the input file set and verify the application tables exist.
- Repeat: second invocation is a no-op and preserves every `applied_at` value.
- Concurrency: two invocations serialize; a second connection observes the lock
  held by the migration session even after an earlier migration commits. Every
  migration is applied exactly once.
- Failure: transactional DDL and ledger changes roll back; earlier migrations
  survive; later migrations do not run; a corrected retry succeeds. Exercise
  failure when recording the ledger as well as failure in the migration body.
- Lock timeout, statement timeout, cancellation and lost migration connection
  fail safely and release the lock so another invocation can resume.
- Compatibility: exercise read/write repository operations compiled from the
  preceding application revision against the schema created by the new runner.
  This establishes this step's unchanged-schema compatibility, not blanket
  compatibility for future schema changes.

## Smoke tests

- Build the dedicated image from its allowlisted context; run it twice against
  disposable PostgreSQL and verify a failing invocation exits nonzero.
- Build the existing API image with the compatible migration hook from a
  sanitized context; confirm API startup remains separate from migration.
- Run backend build, vet and the short race suite, plus the dedicated PostgreSQL
  integration suite. Add a CI check that runs those database cases explicitly.
- Validate changed shell scripts and run the existing local-stack script tests.
- Inspect the exact staged diff and run Gitleaks before every local commit.
  Private evidence and credentials remain outside Git and public artifacts.

## E2E tests

The real local PostgreSQL/image round trip is the migration E2E test for this
step. Production restore, RDS permissions, SSM rollout failure propagation,
Temporal schema jobs and browser checks remain later migration steps.

## Manual tests

Document commands to build/run the migrator and execute the disposable database
suite. Explain timeout/error handling, direct-session requirements, filename
ledger compatibility, forward-only recovery and deployment ordering. Reviewers
need no production access or provider credentials.
