#!/usr/bin/env bash
# Compatibility entrypoint for existing deploy hooks and local development.
# The API process never invokes this automatically.
set -euo pipefail

if [[ -x /migrator ]]; then
  exec /migrator "$@"
fi

backend_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export MIGRATION_DIR="${MIGRATION_DIR:-${backend_dir}/db/migrations}"
cd "$backend_dir"
exec go run ./cmd/db-migrate "$@"
