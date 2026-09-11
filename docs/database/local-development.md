# Database Local Development

Purpose: give the team one short path for bringing up Postgres and applying the schema on this repository.

## Commands

From the repository root:

```bash
make db-up
make db-migrate
make db-psql
```

`make db-migrate` uses the Go application migrator and is incremental. It records
complete filenames (without `.sql`) in `schema_migrations`, preserving files
that share numeric prefixes. A session lock serializes concurrent invocations,
and each migration commits with its ledger entry. Go is required; psql is only
needed for commands such as `make db-psql` and `make db-seed`.

Supply custom connections through the `DATABASE_URL` environment variable; the
script no longer accepts a connection string as an argument. See the
[application migration guide](../deployment/application-migrations.md) for
timeouts, failure recovery, the one-off image and isolated PostgreSQL tests.

To reset the local database volume and start clean:

```bash
make db-reset
make db-migrate
```

## Local defaults

The local database uses:

- database: `agentclash`
- user: `agentclash`
- password: `agentclash`
- port: `5432`

Default connection string:

```text
postgres://agentclash:agentclash@localhost:5432/agentclash?sslmode=disable
```

## File layout

- migrations: [`backend/db/migrations`](../../backend/db/migrations)
- local migration runner: [`scripts/db/apply-goose-migrations.sh`](../../scripts/db/apply-goose-migrations.sh)
- local env example: [`backend/.env.example`](../../backend/.env.example)
- docker compose: [`docker-compose.yml`](../../docker-compose.yml)

## Schema shape

The migration set is intentionally organized by domain:

1. extensions and shared helpers
2. identity and tenancy
3. challenge catalog
4. provider infrastructure
5. agent registry
6. run orchestration
7. replay and scoring
8. publication and arena

That order matches the current implementation build path in [`docs/build-order/build-order.md`](../build-order/build-order.md) and the domain ownership in [`docs/domains/domains.md`](../domains/domains.md).
