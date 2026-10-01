# Step 9.2 maintained platform images — Test Contract

Locked before implementation. This step authorizes local source preparation,
builds, scans, rehearsals and reviewed commits. Publication, paid resources,
AWS/GitHub activation, provider changes and production cutover need later gates.
All populated inventories, reports, logs and operator details remain private.

## Functional Behavior

- Maintain patched Alpine, Go, Bun, Caddy and PostgreSQL administrative images
  using immutable reviewed upstream inputs. Preserve provenance and notices;
  retain input findings and explicitly verify their remediation in consumed
  outputs. No hidden suppression, ignored-unfixed gate or implicit input waiver.
- Rebuild vulnerable Caddy and gosu binaries with supported patched Go and
  compatible dependencies. Preserve PostgreSQL major 18, non-root runtime,
  streaming, TLS and administrative dump/restore behavior. RDS remains the
  production database; R2, E2B and the deferred Vercel handoff remain unchanged.
- Build and scan the complete dependency graph before publication credentials
  are acquired. Require actual package/binary inventory and scanner coverage for
  every consumed base, runtime, admin and application image and bootstrap tool.
  Scanner errors or unresolved high/critical findings block publication.
- Bind additional artifacts to reviewed source, build evidence and immutable
  private repository digests. Reject missing, foreign, mutable or tampered image
  references. Keep existing IAM restrictions, exact approvals, default-disabled
  CI/CD and separately approved platform bootstrap changes.
- Document rebuild/update duties and the path back to clean upstream images.
  Record local image sizes for later cost review; add no always-running service.

## Unit Tests

- Focused build/manifest tests reject incomplete or mismatched artifacts and
  missing scanner coverage; source inputs cannot silently become runtime images.
- Host/release guards reject changed platform artifacts in an application-only
  release and accept a complete separately approved platform bootstrap contract.
- Existing delivery/platform security and source-secret-policy suites pass.

## Integration / Functional Tests

- Build all maintained images and dependent applications for Linux/amd64; verify
  exact identities, source labels, OS package inventory and executable versions.
- Scan immutable inputs and outputs without modifying reports. Preserve complete
  reports, remediation mapping, SBOMs and build provenance privately.
- Rehearse three-database TLS/role separation/migration/restore; Temporal mTLS,
  queues, recovery and certificate/replacement behavior; Valkey ACL/durability.
- Exercise the rebuilt Caddy with terminal WebSocket reconnect/idle streaming,
  maintenance drain and callback behavior, plus non-root edge fencing.

## Smoke Tests

- Validate deterministic bootstrap packaging, Compose, infrastructure templates,
  workflows, image lock/build graph and affected source checks.
- Review exact and cumulative diffs, scan each commit and verify original
  unrelated worktree files are unchanged before final handoff.

## E2E Tests

Live AWS/IAM/SSM, host rebuild, DNS, actual provider use and browser/Vercel
acceptance remain in later approved migration steps. Disposable local tests do
not establish production readiness or authorize deployment.

## Manual / Operator Tests

Review upstream source and package checksums, licenses and dependency changes.
Record dated verification, unresolved limits and maintenance instructions in
sanitized public documentation and a private operator checkpoint. Finish this
local step and present the next concrete resource/cost approval gate.
