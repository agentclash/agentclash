# Step 9.1 local patch verification

Verified on 2026-09-12. Application remediation passes; release readiness remains
blocked by the [upstream image findings](RELEASE-BLOCKERS.md). This records local
source work, not publication or deployment. Complete reports, image identities,
synthetic credentials, review records and operational checkpoints stay outside Git.

## Changes

Go 1.26.8 is aligned across all three modules, `.tool-versions`, build files and
contributor instructions. CI reads the module version. The runtime now uses Moby
client 0.6.0/API 1.56.0 instead of the deprecated Docker module. pgx, gRPC and
affected Go dependencies were updated with their required transitive versions.
The Docker adapter retains explicit host selection, API negotiation, file IO,
exec output/exit/timeout handling, resource limits and idempotent cleanup. No
Docker socket is mounted into the AWS app containers.

Bun 1.4.2 matches terminal CI, image and type pins. Exact compatible security
overrides update brace-expansion, tar and undici. E2B and R2 integrations remain
in place. The application images install patched OpenSSL packages on top of
immutable upstream bases; the upstream bases are independently scanned.

Temporal server/admin move together to 1.32.0, Valkey stays on the 8.1 series at
8.1.10, and the local PostgreSQL admin/test image advances to 18.6. The RDS minor
still needs a live availability/compatibility check. Caddy remains at 2.11.4.

## Evidence

| Check | Result |
| --- | --- |
| Backend, runtime and CLI | Build, vet and short race suites passed with Go 1.26.8 |
| Moby adapter | Existing Docker unit/race tests and a real disposable lifecycle passed, including upload/download, stdout/stderr, nonzero exit, timeout, limits and cleanup |
| Application migrations | Full migration runner/database integration passed, including repeat, concurrent/failing migration and rollback/wrapper/application compatibility checks |
| Terminal | Frozen install, type checks, 22 unit tests and all five explicitly enabled isolated integration tests passed |
| Four Linux/amd64 application images | Built and scanned by exact local image ID; zero secret and high/critical vulnerability findings |
| Selected platform/base images | All eight passed secret scans; Temporal server/admin and Valkey passed high/critical scans; the other five remain blockers |
| PostgreSQL and Temporal | Verified SQL TLS, runtime-role DDL denial, server/client mTLS, three queues, replacement at a new IP, CA rotation, three-database restore and visibility passed |
| Valkey | Real Go/Linux-Bun clients, TLS/ACLs, transactions/Lua/pub-sub, AOF restart/offline restore, spend/trial expiry and corrupt-tail rejection passed |
| Caddy image | Non-root startup, all-method maintenance fence, reload and request-metadata removal from error logs passed |
| Bootstrap | Verified Compose 5.5.1 bytes, deterministic archive and allowlisted inventory; secret and Go-binary high/critical scans passed |
| Delivery/platform checks | 35 delivery tests and 13 platform guards passed; actionlint, cfn-lint, Python lint and shell syntax checks passed |

The migration test initially exposed a stale Go 1.25 Dockerfile pin, which was
corrected in all existing backend build files. The platform rehearsal exposed a
PostgreSQL initialization race: its temporary Unix-socket server can accept a
probe before restarting. The rehearsal now waits for the final TCP listener;
the complete recovery rehearsal then passed.

The bootstrap vulnerability gate now scans the installed executable form with
Trivy `rootfs` and requires a `gobinary` result for `tools/docker-compose`.
An empty result set is refused. Scanning the archive's non-executable copy had
returned no binary coverage; that was not accepted as a vulnerability pass.
Regression tests cover omitted/wrong binary coverage and scanner failure.
[Trivy Go-binary coverage](https://trivy.dev/docs/latest/guide/coverage/language/golang/).

## Compatibility and remaining gates

Temporal 1.32 enables stricter visibility query conversion and activity eager
execution by default, and is the last release retaining deprecated worker
versioning. Local SQL visibility and workflow recovery were exercised with these
defaults. Future upgrades still require release-note and routing-state review;
do not infer that a 1.33 upgrade or migration of live Cloud histories is verified.
[Temporal 1.32 release notes](https://github.com/temporalio/temporal/releases/tag/v1.32.0).

The regular terminal suite skips five environment-dependent integration tests.
All five passed when explicitly enabled with native Bun 1.4.2, the pinned
Linux/amd64 Valkey image and local Caddy 2.11.4. Checks included real process
signals/cleanup, production startup refusal, HTTP proxy identity, WebSocket
reconnect and 65-second idle survival, gateway callbacks during drain, TLS
rejection and persisted spending. Sandboxes/providers were synthetic; no E2B,
OpenAI or Vercel request was made. The Linux terminal image separately passed the
platform client's TLS/cache checks.

Frontend checks were not repeated because this step changed no frontend source;
the historical Step 9 result remains in [VERIFICATION.md](VERIFICATION.md).
Live IAM/SSM, protected GitHub execution, root host rebuild, RDS engine/extension
checks, workload capacity, real provider/browser/DNS/TLS and Vercel acceptance
remain later gates. No source cutover or paid provisioning occurred. Neither
passing local checks nor operator ownership waives the remaining image gate.
