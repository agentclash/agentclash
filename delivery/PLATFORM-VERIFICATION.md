# Step 9.2 local platform-image verification

Verified on 2026-09-12. The five consumed-image blockers from
[Step 9.1](PATCH-VERIFICATION.md) are resolved by maintained images. The complete
twelve-image graph passes secret and HIGH/CRITICAL vulnerability gates, and the
exact selected images pass disposable recovery and terminal/edge rehearsals.
Source changes are committed locally. Publication, AWS provisioning, live delivery
activation and cutover remain at later numbered approvals.

## Changes and retained evidence

Alpine, Go and Bun bases receive pinned, signature-verified APK updates. Caddy
2.11.4 is rebuilt with Go 1.26.8 and patched module locks. PostgreSQL 18.6 keeps
its database/admin behavior while its gosu 1.19 helper is rebuilt with patched Go
and its affected OS packages are updated. Temporal server/admin 1.32.0 and
Valkey 8.1.10 retain their exact reviewed public digests. Production PostgreSQL
remains RDS; no additional always-running service was introduced.

Original source images are classified as remediation inputs and never directly
consumed by application builds or runtime services. Each affected component must
remain visible to the scanner at a changed version, and the full resulting image
must pass. The final reports map 77 input HIGH/CRITICAL findings: 40 in Caddy,
31 in PostgreSQL, and two each in Go, Bun and Alpine. These are finding instances
across images/components, not 77 distinct CVEs. Raw reports, source identities,
SBOMs, component mappings, image sizes and build logs remain private and unedited.

Format 2 releases bind four application and eight platform references to build
evidence. Nine artifacts are destined for the same approved private ECR repository;
three remain exact public runtime references. Host settings separately pin the
platform-map hash. An application release cannot replace platform artifacts or
silently accept a new bootstrap. Existing IAM destination restrictions and exact
operator/source/recovery approval guards remain in force.

## Verification

| Check | Result |
| --- | --- |
| Clean committed source | Actual `build.prepare` completed from a clean detached checkout and an allowlisted Git archive; source secret scan passed |
| Complete image graph | Five maintained, three unchanged runtime/admin and four application images built or selected for Linux/amd64; all twelve passed strict secret and HIGH/CRITICAL scans |
| Scanner coverage | Exact image IDs and architecture, nonempty Alpine package inventory/SBOMs, required Go binaries and terminal dependency inventory verified; scanner errors and missing coverage fail |
| Input remediation | All 77 input finding instances mapped to changed clean components; no ignored-unfixed flag, edited report or vulnerability suppression |
| Reproducibility | A separate `--no-cache` build from the working checkout, with a different process umask, reproduced all five maintained image IDs from the clean-source build; three public references remained unchanged |
| PostgreSQL and Temporal | SQL TLS, runtime-role DDL denial, migrations, private server/client mTLS, three queues, new-IP replacement, CA rotation, three-database restore and visibility passed |
| Valkey | TLS, real Go/Linux-Bun clients, ACLs, transactions/Lua/pub-sub, AOF restart/offline restore, spend/trial expiry and corrupt-tail rejection passed |
| Edge | Rebuilt Caddy passed non-root startup, all-method maintenance fencing, reload and removal of request metadata from error logs |
| Terminal | Exact rebuilt Linux Caddy/Bun images passed HTTP identity, WebSocket reconnect/65-second idle, callbacks and drain against synthetic providers; frozen core/demo import also passed offline as the runtime user |
| Final bootstrap | Exact committed Compose configuration passed offline/non-root validation; two deterministic bundles matched, and source-secret plus executable Compose 5.5.1 dependency scans passed |
| Delivery and platform guards | 51 delivery tests and 13 platform guards passed, including platform tampering, source-input/coverage failures and mocked nine-artifact publication checks |
| Source checks | Real secret-policy regression cases, actionlint, cfn-lint, Python lint and shell syntax passed; reviewed local commits passed secret/private-identifier checks |

The full image build is bound to its recorded source revision. The final source
increment only added Bun's `--no-install` flag to Compose and its cache probe; its
bootstrap was rebuilt and rescanned separately, and the affected rehearsals used
that flag. Subsequent verification documents change no image or bootstrap input.
Exact revisions and hashes are retained privately. A future publication must
prepare the then-approved clean revision; these local artifacts are not silently
relabelled as a different source commit.

## Issues caught and fixed

The Linux terminal rehearsal exposed a packaged file dependency that could not
resolve `zod` from the shared core's directory. A common `/app/node_modules`
ancestor now exposes the frozen production install. The image build verifies core
imports and packaged demos with networking disabled and the non-root runtime user.
Image and Compose startup both disable automatic dependency installation.

An APK install log embedded wall-clock timestamps, and private build-context modes
differed from Git archive modes. Removing only that install log and normalizing
public context permissions/timestamps made the platform build cache-independent.
Installed-package databases and original build/scanner evidence are retained.

Repeated remote scans hit Docker Hub's anonymous pull limit. Pure-source,
single-platform exports now reuse the builder's pinned-digest cache and preserve
the original image configuration. Trivy checks the exported image ID and retains
the source reference/archive hash. An unavailable uncached source or scanner error
still fails. No credentials or mutable-tag fallback were introduced.

## Limits and operator handoff

This step exercised local Linux/amd64 containers on the existing workstation
engine. It did not upgrade that engine or prove the eventual AWS host. Real RDS
minor/extensions, AMI/Engine patches, IAM/SSM/secret access, GitHub protection,
host rebuild, capacity, real providers, DNS/public TLS and Vercel/browser acceptance
remain later checks. Frontend source and actual E2B/R2/provider integrations were
not changed or contacted by these rehearsals.

Follow the [maintenance procedure](PLATFORM-IMAGES.md) for future upstream/security
updates. Next, perform the [Step 10 resource and cost review](RELEASE-BLOCKERS.md)
and obtain approval for the concrete private bill of materials before provisioning.
