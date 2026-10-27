# AWS runtime tmpfs mount — test contract

## Functional behavior

Every production Compose service inherits one temporary filesystem at `/tmp`.
Its options remain read/write, noexec, nosuid, 64 MiB and mode 1777. YAML must
preserve the entire mount declaration as one string. Docker must accept the
resolved mount when creating a service with the production Compose configuration.
The correction does not alter cloud resource sizes, service access, runtime
secrets, image dependencies, or public routing.

## Unit tests

Parse the real production Compose file and check every inherited tmpfs mount:
one absolute path, the expected size/permissions, and no standalone option
entries that Docker would mistake for paths. Demonstrate failure before the fix.
Existing platform and delivery guard tests must continue to pass.

## Integration and smoke tests

Resolve the production Compose model with synthetic environment and secret files.
Create its database-admin container from the scanned PostgreSQL image and inspect
Docker's actual tmpfs configuration. Run a synthetic command through the service
wrapper and verify the `/tmp` mount is present and writable. Use no cloud or
provider credentials, no public ports and no database connection for this test.
Remove the disposable container and networks after verification.

Build and scan the corrected source, bootstrap and images before private
publication. On the destination host, install the matching reviewed bundle and
resume explicit database/platform initialization after checking partial state.

## E2E tests

Application/browser E2E tests are outside this mount correction. Existing exact
image recovery, edge and terminal checks remain applicable; public cutover stays
behind its previously agreed gate.

## Manual checks

Confirm `docker compose config --format json` resolves tmpfs as one entry per
service. Confirm Docker container inspection contains exactly the intended `/tmp`
mount. Keep all operational output, image references and host details private.
