# Application database migrations

The one-off `db-migrate` command applies the application schema. Both the local
command and the existing `/migrate.sh` deployment hook use this runner. Starting
the API or worker does not run migrations.

## History and transaction contract

Keep the restored `public.schema_migrations` table. Its `version` primary key is
the complete migration filename without `.sql`, such as `00020_example_change`.
Several existing files share numeric prefixes. Do not replace this ledger with
Goose's numeric version table, rename old files, or replay already applied Up
sections. Existing `applied_at` timestamps are preserved. The runner does not
store checksums: edits to an applied migration are not detected or reapplied, so
historical files must remain immutable and changes need new filenames.

Before connecting, the runner reads all `*.sql` files in `MIGRATION_DIR` in
lexical filename order. Names must match five digits, an underscore, lowercase
letters/digits/underscores and `.sql`. Each file needs exactly one `-- +goose Up`
followed by one `-- +goose Down`; only Up executes. Empty sections, symlinks,
unsupported directives (including `NO TRANSACTION`), psql commands and explicit
transaction control fail validation. SQL syntax is checked by PostgreSQL during
execution. Migration SQL is trusted, reviewed source; the validation is not a
sandbox for arbitrary SQL. Do not manipulate the migration ledger, session
settings or advisory locks inside application migrations.

One direct PostgreSQL connection holds the application session advisory lock
through ledger inspection and every migration. Each pending Up section and its
ledger insert commit together. PostgreSQL keeps a session lock across transaction
boundaries and releases it when that session ends. All migrators for this
application must use the same lock keys. Transaction pooling, independent psql
sessions and older runners that do not take the lock cannot provide this
guarantee. Disable old automatic migration jobs before enabling this runner.
See [PostgreSQL's advisory-lock semantics](https://www.postgresql.org/docs/18/explicit-locking.html#ADVISORY-LOCKS).

## Configuration

| Variable | Required/default | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | Required | Direct application database connection, supplied through the runtime secret manager. |
| `MIGRATION_DIR` | `/migrations` | Mounted or image-bundled SQL directory. The local wrapper defaults to `backend/db/migrations`. |
| `MIGRATION_LOCK_TIMEOUT` | `60s` | Maximum wait for the migration session lock. |
| `MIGRATION_STATEMENT_TIMEOUT` | `10m` | PostgreSQL statement timeout, including DDL waits. |
| `MIGRATION_TIMEOUT` | `30m` | Overall command deadline, including connection and lock acquisition. |

Durations use Go syntax and must be at least `1ms`. The earliest applicable
timeout wins. The connection establishment timeout is ten seconds. SIGINT or
SIGTERM cancels work and closes the connection. The command waits for pgx's
background cancellation/connection cleanup before exiting, with a separate
20-second bound; allow at least a 30-second container shutdown grace period.
A network partition can still prevent confirming server cleanup. The runner
uses `public` as its schema search path and standard SQL string escaping.
See [pgx's connection cleanup contract](https://pkg.go.dev/github.com/jackc/pgx/v5@v5.8.0/pgconn#PgConn.CleanupDone).

For remote production PostgreSQL, supply `sslmode=verify-full` and the approved
CA file through the secret configuration and a read-only mount. Use a dedicated
DDL role with the necessary database/schema ownership and extension permissions.
The API/worker runtime role should not own schema changes. Provisioning grants,
validating RDS permissions and exercising recovery belong to deployment rehearsal.
Keep Temporal persistence and visibility databases under their own schema jobs.

## Build and run

From the repository root:

```sh
docker build --platform linux/amd64 -f backend/Dockerfile.migrator -t agentclash-migrator:local .
# DATABASE_URL is already injected securely into the invoking environment.
docker run --rm --read-only --cap-drop=ALL --security-opt=no-new-privileges --stop-timeout 30 \
  -e DATABASE_URL agentclash-migrator:local
```

Choose the platform that matches the host. The image runs as UID/GID `65532`,
includes CA certificates and the application SQL, and contains no credentials.
Mount any custom database CA where that user can read it. The adjacent
`Dockerfile.migrator.dockerignore` allowlists the build context; only required
Go source, module files and SQL are inputs. This follows Docker's
[Dockerfile-specific ignore rules](https://docs.docker.com/build/concepts/context/#dockerignore-files).
Publishing and digest pinning are separate delivery steps.

For development, use `make db-migrate`. It delegates through
`scripts/db/apply-goose-migrations.sh` to the same Go command. Go is required;
psql is not required for migration. Custom connections must be supplied through
the environment. Positional connection-string arguments are no longer accepted,
and neither wrapper loads a project `.env` file implicitly.

## Deployment ordering and failure

1. Use the approved database backup and release image digests. Serialize the
   complete deployment; the database lock only serializes schema jobs.
2. Run exactly one migrator job with the DDL connection. Inspect both container
   completion and exit status; for remote execution, also inspect the SSM command
   status and response code. Dispatch success alone is insufficient.
3. Advance API/worker deployment only after migration exit code `0`. Revalidate
   compatibility with the previous application revision for every schema change.

The command exits `1` on migration/connection failure and `2` on invalid command
arguments or timeout syntax. Every nonzero exit blocks rollout. An error in an
Up section or ledger insert rolls both back; earlier committed migrations stay
applied and later files do not execute. Lock contention, interruption and
connection loss also stop the run. There is no automatic retry or Down path.

If the connection is lost during commit, its outcome may be unknown. Rerun the
same reviewed image: the committed ledger determines whether that file is
already complete. If SQL itself is wrong, inspect private database diagnostics
and review a repair before retrying. A failing, never-applied migration can be
corrected; never rewrite history already applied in any deployed environment.
Logs show stages, public basenames and SQLSTATE, excluding connection strings,
server error detail, SQL text and row values. Keep detailed operational evidence
private. The actual SSM rollout and recovery procedures are separate work.

## Verification

With Docker, Go and Python 3 installed:

```sh
python3 scripts/db/test-migrator.py
```

The test harness creates its own PostgreSQL 18 container on a random loopback
port, with a generated password and temporary storage. It never uses an existing
database. It tests all application migrations, exact filename/timestamp
preservation, concurrent invocations across commits, body/ledger rollback,
timeouts, cancellation, connection loss and retry. It then builds and runs the
amd64 image, checks failure exit/rollback, SIGTERM and the overall deadline,
repeats through the local wrapper and exercises representative application
repository reads/writes. It removes its
container and generated test image on exit. Backend CI runs this command.

Use `--skip-image` for the database suite during development. To verify a previous
application revision, compile its repository tests with `go test -c -o` to a
private path before changing revisions, then pass that path using
`--prior-repository-test-binary`. The tests cover run retrieval, tool writes and
workspace visibility. They establish compatibility for the tested revisions;
future schema changes need their own compatibility assessment. Production
restores, DDL permissions and remote rollout failure handling are not established
by these local tests.
