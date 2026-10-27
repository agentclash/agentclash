#!/bin/sh
set -eu
export SQL_DATABASE="${1:?database required}"
case "$SQL_DATABASE" in
  temporal) export SQL_USER=temporal_owner SQL_PASSWORD="$TEMPORAL_OWNER_PASSWORD"; path=/etc/temporal/schema/postgresql/v12/temporal/versioned ;;
  temporal_visibility) export SQL_USER=visibility_owner SQL_PASSWORD="$VISIBILITY_OWNER_PASSWORD"; path=/etc/temporal/schema/postgresql/v12/visibility/versioned ;;
  *) exit 1 ;;
esac
case "${2:-}" in
  init) temporal-sql-tool setup-schema -v 0.0 ;;
  upgrade) ;;
  *) exit 1 ;;
esac
exec temporal-sql-tool update-schema -d "$path"
