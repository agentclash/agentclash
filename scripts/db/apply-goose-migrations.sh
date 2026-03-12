#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <database-url>" >&2
  exit 1
fi

database_url="$1"
migration_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../backend/db/migrations" && pwd)"

for migration in "$migration_dir"/*.sql; do
  echo "[db] applying $(basename "$migration")"

  awk '
    /^-- \+goose Up$/ { in_up=1; next }
    /^-- \+goose Down$/ { in_up=0; exit }
    in_up { print }
  ' "$migration" | psql "$database_url" -v ON_ERROR_STOP=1 -f -
done
