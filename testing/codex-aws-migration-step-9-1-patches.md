# Step 9.1 image and dependency remediation — Test Contract

Locked before implementation. Scope is local source preparation and verification;
no push, publication, paid provisioning, provider or production changes. Private
inventories, scan reports and review evidence stay in the approved private directory.

## Functional Behavior

- Select supported patched Go/compiler dependencies and immutable Linux/amd64
  image digests from authoritative upstream releases. Keep all Dockerfile/lock/
  CI pins consistent. Preserve source-compatible PostgreSQL major, self-hosted
  Temporal, Valkey durability, E2B/R2 use and the low-usage resource shape.
- Replace the deprecated Docker Go module with supported client/API modules
  where compatible. Preserve sandbox creation, exec output/exit handling, upload,
  download, cleanup, resource/network limits and explicit daemon selection.
  Do not add daemon code or mount the host Docker socket into AWS app containers.
- Rebuild exact candidates and run source/image secret and high/critical
  vulnerability gates without blanket ignores, ignored-unfixed findings or
  edited reports. Distinguish actual patched bytes from preliminary reachability
  evidence. Any unresolved upstream issue remains a publication blocker until
  a concrete narrowly scoped remediation is separately reviewed.
- Preserve Step 9's default-disabled delivery, protected roles, immutable release
  artifacts, exact private approvals, maintenance fence and recovery safeguards.
- Verify host Engine requirements separately from application Go dependencies;
  record the supported patched requirement without changing the workstation or
  deploying a host. Keep actual AMI/RDS availability checks for paid Step 10.

## Unit Tests

- Existing runtime Docker provider/session/exec/cleanup tests pass after API changes.
- Add focused regression tests for changed transport behavior when existing tests
  cannot detect a client migration error; do not merely mirror implementation.
- Existing affected backend/runtime/CLI Go tests pass with the selected toolchain.
- Existing terminal type/tests and delivery/platform guard suites pass.

## Integration / Functional Tests

- Build all four application images and verify Linux/amd64 identities.
- Exercise the new Docker client against disposable trusted local containers,
  including file round-trip, exec success/failure and removal; no providers needed.
- Rehearse application migrator concurrency/no-op/failure behavior and platform
  TLS/Temporal queues, replacement/rotation, three-database restore, Valkey
  persistence/ACLs and Caddy readiness/streaming against changed pins.
- Scan exact final images and bootstrap components; retain immutable image IDs,
  complete reports and the distinction between fixed, unresolved and scan errors.

## Smoke Tests

- Verify selected tools and Compose configuration, deterministic bootstrap hashes
  and allowlisted inventory outside Git. Re-run affected workflow/template checks.
- Before each local commit and final handoff, inspect exact/cumulative diffs,
  run secret scans and verify original unrelated worktree files are unchanged.
- Keep publication disabled if any required scanner or compatibility check fails.

## E2E Tests

Live AWS/GitHub/IAM/SSM, root host rebuild, DNS/TLS, browser/provider and Vercel
acceptance remain in Steps 10–14. No production credentials are needed here.
Local Docker tests do not substitute for those live checks.

## Manual / Operator Tests

Record upstream version/compatibility evidence, meaningful results and remaining
blockers privately. Update the public sanitized remediation/verification guide and
private checkpoint. If gates pass, prepare the next exact resource/cost review;
otherwise present specific upstream blockers and options. Stop at this boundary.
