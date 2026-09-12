# Remaining image remediation before AWS publication

Step 9.1 patched the application dependencies and rebuilt all four application
images. Each now passes secret scanning and the high/critical vulnerability gate.
Temporal server/admin 1.32.0 and Valkey 8.1.10 pass both gates as well. The verified
Compose bootstrap binary passes its dependency scan. See the dated
[verification record](PATCH-VERIFICATION.md) for compatibility checks and limits.

Publication is still blocked by five selected upstream images. Results below are
from the exact public digests in `deploy/aws/images.lock.json` scanned on
2026-09-12. Reports and exact application image identities remain private. These
are scanner findings, not evidence of an exploited installation.

| Selected upstream artifact | Remaining high/critical findings | Needed remediation |
| --- | --- | --- |
| Caddy 2.11.4 Alpine | Go standard library, x/crypto, x/net, x/text, gRPC; OpenSSL, curl and c-ares | Rebuild the Caddy binary with patched compiler/dependencies and update runtime packages |
| PostgreSQL 18.6 Alpine admin/test image | Bundled `gosu` built with Go 1.24.6; OpenSSL and libuuid | Rebuild the helper and patch the OS packages; preserve PostgreSQL major and dump/restore behavior |
| Go 1.26.8 Alpine builder | Base-image OpenSSL libraries | Select or prepare a patched builder base |
| Bun 1.4.2 Alpine base | Base-image OpenSSL libraries | Select or prepare a patched Bun base |
| Alpine 3.24.1 base | OpenSSL libraries | Select or prepare a patched runtime base |

The application Dockerfiles already install patched OpenSSL 3.5.8-r0, so their
final images pass. This does not make the original upstream base digests pass the
separate input-image gate. Debian/Bookworm and the previous Alpine series were
also examined; the tested alternatives still had high/critical findings. The
existing gate remains unchanged: no blanket ignore, ignored-unfixed switch,
edited report or Docker-module reachability waiver was added.

## Recommended next checkpoint: Step 9.2

Prepare maintained platform/base images locally, with the following concrete scope:

1. Define public, reusable build recipes and immutable upstream source/package
   inputs for patched Alpine, Go and Bun bases, Caddy, and PostgreSQL admin tools.
   Preserve upstream provenance and notices. Keep all input findings visible;
   distinguish source inputs from the actual base/runtime artifacts consumed by
   the build. No unresolved input finding is implicitly waived by repackaging it.
2. Rebuild Caddy and `gosu` from reviewed releases with the patched compiler and
   compatible dependency updates. Patch the affected OS packages. Preserve
   non-root execution, HTTPS/streaming, PostgreSQL major 18 and administrative
   backup/restore capabilities. The production database remains managed RDS.
3. Extend the source-only build/manifest/bootstrap definitions to address the
   additional artifacts by immutable private-registry digest after later
   publication. Build and inspect them before acquiring publication credentials.
   Public files contain reusable definitions; real registry destinations and
   populated inventories remain private. Keep IAM destination restrictions and
   separately approved platform bootstrap changes.
4. Require secret and high/critical scans, full inventory/coverage evidence and
   compatibility tests for the resulting application, admin, runtime and build
   bases. Keep the source-input review separate and explicit. Rehearse Caddy
   WebSockets/draining, database restore, Temporal queues and terminal behavior.
   An unresolved finding or scanner error continues to block publication.
5. Record the maintenance procedure: review upstream/security updates, rebuild
   affected images and dependent apps, rerun scans/rehearsals, then approve a
   release. Return to upstream images when reviewed upstream digests pass. Finish
   with a reviewed local checkpoint before repricing paid Step 10.

This changes the maintenance responsibility: the operator owns these derived
images until upstream replacements satisfy the gate. It requires build/registry
storage and rebuild effort, with no additional always-running server. Include
actual retained image sizes and build use in the later cost review.

The alternative is to wait for upstream rebuilds and repeat the scans; no release
date can be promised. Given the migration urgency, preparing the maintained
images is the recommended path. Approval of Step 9.2 would authorize local source
work and verification only. It would not authorize a push, publication, paid
resources, IAM/GitHub activation, production cutover or Vercel changes.

## Host Engine remains a separate check

The deprecated `github.com/docker/docker` module was removed from the app in
favor of maintained Moby client/API modules. That does not patch a Docker host.
The Engine 29.8.0 release is the reviewed upstream candidate, including its
Go/containerd/runc updates. Step 10 must verify the actual AMI package and vendor
backports against current advisories; an app scan is not host verification.
The local workstation Engine was not upgraded during Step 9.1.
[Moby Engine 29.8.0 release](https://github.com/moby/moby/releases/tag/docker-v29.8.0).

The original daemon archive and mount-race advisories remain relevant to host
selection, even though the legacy module no longer appears in the application
dependency graph.
[Archive advisory](https://github.com/moby/moby/security/advisories/GHSA-x86f-5xw2-fm2r),
[mount-race advisory](https://github.com/moby/moby/security/advisories/GHSA-rg2x-37c3-w2rh).
