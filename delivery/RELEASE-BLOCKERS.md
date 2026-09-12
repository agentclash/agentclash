# Image remediation before AWS publication

The Step 9 vulnerability gate rejects the currently pinned application and
platform images. These are scanner findings, not evidence of an exploited
installation. Full scan reports, package inventories and image identities stay
in approved private evidence. No image has been published by Step 9 and no
vulnerability exception has been enabled.

## Proposed next checkpoint: Step 9.1

Prepare and verify the following source changes before approving paid Step 10:

1. Refresh `deploy/aws/images.lock.json` and the matching backend/terminal
   Dockerfile pins. Select supported, patched Go, Alpine, Bun, Temporal/admin,
   Valkey, Caddy and PostgreSQL test/admin images. Resolve exact platform digests,
   review upstream compatibility and scan each candidate. PostgreSQL's local
   image and the selected RDS engine remain separate decisions; preserve the
   source-compatible database major and verify the available RDS minor later.
2. Update the affected direct/transitive Go dependencies in `backend`, `runtime`
   and their CLI consumer as needed. Findings include pgx, gRPC, x/crypto, x/net,
   x/text, spdystream and the Go standard library. Rebuild the four release images
   and use the new reports to verify the fixes; do not edit a report to make the
   old bytes pass.
3. Resolve the legacy Docker module findings with package/function evidence.
   The upstream advisories identify Docker daemon archive handling; the inspected
   API/worker package graph includes the client and no daemon package. That is
   preliminary reachability evidence, not an approved scanner exemption. Prefer
   migrating the client dependency to maintained modules if compatible. If a
   finding remains in metadata but its affected code is absent, propose a narrow,
   reviewed artifact-specific attestation with expiry and revalidation. Do not
   add a blanket `ignore-unfixed` or severity bypass.
4. Verify a patched host Docker Engine separately from the app's Go client.
   The two relevant upstream advisories list Engine 29.5.1 as their patched floor;
   select a supported version addressing current findings when preparing the AMI.
   The app containers do not need a host Docker socket for the E2B deployment.
5. Repeat the meaningful compatibility checks affected by the pins: Go consumer
   tests, terminal tests, migrator concurrency/failure behavior, Temporal mTLS and
   queue polling, three-database restore, Valkey persistence and Caddy readiness/
   streaming. Rebuild and scan the exact candidate and verified bootstrap bytes.
   Record results privately, review the cumulative diff and run secret scans
   before local commits.

The next checkpoint is still source preparation. Publishing, infrastructure
application, source cutover, GitHub activation and provider/Vercel changes retain
their separate numbered approvals. If supported upstream images cannot satisfy
the gate, present the specific remaining findings and compatible options before
changing the accepted deployment design.

## Docker advisory sources

The archive decompression issue concerns code execution with daemon privileges
when a compressed archive is copied into a malicious container.
[Moby advisory for CVE-2026-41567](https://github.com/moby/moby/security/advisories/GHSA-x86f-5xw2-fm2r).

The mount race concerns copying into a running container with a volume mount;
the advisory describes host file overwrite and denial of service conditions.
[Moby advisory for CVE-2026-42306](https://github.com/moby/moby/security/advisories/GHSA-rg2x-37c3-w2rh).

Both list no patch in the old `github.com/docker/docker/daemon` module line.
Updating the app's client and patching the host Engine are distinct checks.
[Moby 29.5.1 release notes](https://github.com/moby/moby/releases/tag/v29.5.1).
