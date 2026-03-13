#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <database-url>" >&2
  exit 1
fi

database_url="$1"
migration_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../backend/db/migrations" && pwd)"

psql "$database_url" -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
    version text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);
SQL

for migration in "$migration_dir"/*.sql; do
  version="$(basename "$migration" .sql)"
  if [[ "$(psql "$database_url" -Atqc "SELECT 1 FROM schema_migrations WHERE version = '$version'")" == "1" ]]; then
    echo "[db] skipping $version (already applied)"
    continue
  fi

  echo "[db] applying $version"

  {
    printf 'BEGIN;\n'
    awk '
      /^-- \+goose Up$/ { in_up=1; next }
      /^-- \+goose Down$/ { in_up=0; exit }
      in_up { print }
    ' "$migration"
    printf "\nINSERT INTO schema_migrations (version) VALUES ('%s');\n" "$version"
    printf 'COMMIT;\n'
  } | psql "$database_url" -v ON_ERROR_STOP=1 -f -
done
