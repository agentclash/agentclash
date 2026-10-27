# Vibe Evals rollout and rollback

Deploy the additive database schema and backend before exposing the workspace UI.
This roadmap retains migrations 1–92; the CI roundtrip itself adds no migration.
Check the existing readiness response, including connector and experiment schema
checks, using the deployment's configured dependencies. Liveness alone does not
establish worker, funding, provider, email or PDF readiness.

Upgrade compatible canonical workers and drain older workers before enabling
controlled requests. Older binaries can ignore additive admission fields and
callbacks. A controlled admission receipt alone is insufficient: the persisted
settings version and `dispatch_guard_version=1` plus `dispatch_validated_at` are
required before presenting guarded config/model fingerprints. Missing or unknown
guard versions remain unknown. Platform validation does not acknowledge an
upstream seed, external model revision or application source build.

Use a private preview before public availability. The existing server switch is
`VIBE_ENABLED`; `/v1/vibe/config` reports `availability.workspace` and
`availability.guest` from readiness and guest capacity. There is no independent
workspace-only environment flag in this implementation. Apply preview access at
the existing authenticated workspace boundary. Keep guest/workspace admission and material-build capability
checks authoritative on the backend. Do not implement preview by changing the
full execution-config fingerprint independently of workers: that can disable
compatible API/worker readiness. Enable the existing public entry only after the
same readiness, permission, accounting and actual application journey checks.

Operator-supplied production provider credentials, approved connector origins and
secrets, funded accounts, real authentication, delivery configuration and runtime
limits remain deployment prerequisites. The local deterministic verification
record is not an authorization to change these values or run paid probes.

Before rolling back to older canonical workers, stop controlled writers and drain
or reconcile outstanding controlled work. Preserve admission, dispatch, source,
accounting, publication and evidence receipts. Unknown execution outcomes remain
unknown; do not clear reservations or fabricate completion to make a rollback
appear healthy. Disable the workspace/public admission surface through the
existing server settings while retaining read access as supported. Roll back
application binaries only after the old worker's compatibility boundary is safe.
Do not reverse additive migrations or erase retained data as a normal rollback.

Reimport creates another version. Publication is explicit and immutable; CI
baseline changes are separate explicit profile writes. Restore a previous
reviewed published version or pinned completed baseline through those existing
owners when needed. Retain newer results and historical imported claims for
reconciliation and audit rather than overwriting them.

Local verification uses disposable child databases with exact-name/OID creation
receipts, isolated process sessions and durable JSONL ownership records. Shared
PostgreSQL parents and shared Temporal services are not reset. Temporal workflow-history
retention does not delete shared-service namespace metadata; those namespaces remain
retained. The browser fixture can instead own the existing SDK in-memory Temporal
devserver; its process and files are stopped and removed with that exact fixture.
