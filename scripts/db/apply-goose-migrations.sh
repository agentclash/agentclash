#!/usr/bin/env bash
# Retain the historical command path; DATABASE_URL is environment-only.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
exec "${repo_root}/backend/scripts/migrate.sh" "$@"
