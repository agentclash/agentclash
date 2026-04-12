#!/usr/bin/env bash
# Run database migrations against $DATABASE_URL.
# Designed to be called as a Railway deploy command.
set -euo pipefail

DATABASE_URL="${DATABASE_URL:?DATABASE_URL must be set}"
MIGRATION_DIR="${MIGRATION_DIR:-/migrations}"

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
    version text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);
SQL

for migration in "$MIGRATION_DIR"/*.sql; do
  version="$(basename "$migration" .sql)"
  if [[ "$(psql "$DATABASE_URL" -Atqc "SELECT 1 FROM schema_migrations WHERE version = '$version'")" == "1" ]]; then
    echo "[migrate] skipping $version (already applied)"
    continue
  fi

  echo "[migrate] applying $version"

  {
    printf 'BEGIN;\n'
    awk '
      /^-- \+goose Up$/ { in_up=1; next }
      /^-- \+goose Down$/ { in_up=0; exit }
      in_up { print }
    ' "$migration"
    printf "\nINSERT INTO schema_migrations (version) VALUES ('%s');\n" "$version"
    printf 'COMMIT;\n'
  } | psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f -
done

echo "[migrate] all migrations applied"
