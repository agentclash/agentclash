#!/bin/sh
# Secret values enter only the child process environment, not Docker's config.
set -eu
set +x
. /run/secrets/env.sh
exec "$@"
