# Maintained AWS platform images

The AWS build produces five maintained images: Alpine, Go and Bun build/runtime
bases, Caddy, and PostgreSQL administrative tools. The database service remains
managed RDS. Temporal server/admin and Valkey retain their exact reviewed public
digests. No extra always-running service is introduced.

## Inputs and gates

`deploy/aws/images.lock.json` distinguishes immutable runtime images from five
**remediation-source-only** images. Those source images must not be used directly
for application builds or Compose services. Their original findings remain in
private reports; they are not treated as clean artifacts.

`deploy/aws/platform/inputs.lock.json` pins every downloaded APK and the gosu 1.19
source archive by SHA256. APK installation runs without network access and retains
Alpine signature verification. The package set supplies patched OpenSSL, curl,
c-ares and libuuid. Caddy 2.11.4 uses its standard modules with reviewed dependency
updates locked in `platform/caddy/go.mod` and `go.sum`. Go's checksum database and
`go mod verify` remain enabled. Both Caddy and gosu use the patched Go compiler.
Upstream layers and notices are retained; rebuilt binaries include module and Go
license files and their dependency manifests.
[Caddy custom-build entrypoint](https://github.com/caddyserver/caddy/blob/v2.11.4/cmd/caddy/main.go),
[gosu release/source](https://github.com/tianon/gosu/releases/tag/1.19).

The driver verifies input hashes before constructing private, minimal build
contexts and normalizes public file modes across private and Git checkouts.
It scans each patched base before building dependent binaries or apps.
All consumed images must pass secret and HIGH/CRITICAL vulnerability scans, have
nonempty OS inventories and SBOMs, and include the expected Go or terminal
dependency coverage. Each original affected component must remain visible at a
changed version and the entire resulting image must pass. Missing coverage,
unfixed findings, unchanged vulnerable versions and scanner failures stop the
build. Reports are preserved without edits; no vulnerability ignore is added.
Original image layers remain part of the derived-image provenance; only the
patched merged filesystem is used at runtime.

Upstream scans use a single-platform archive exported from the pinned digest
through Docker's build cache. A source-only `FROM` preserves the original image
configuration. Private evidence records the upstream reference, archive hash and
image ID; Trivy must report that exact ID and Linux/amd64. This avoids redundant
registry pulls and partially downloaded multiarch-index exports. An unavailable
uncached source still fails the build. It does not fall back to a mutable tag or
skip scans. [Trivy image/archive scanning](https://trivy.dev/docs/latest/target/container_image/).

## Release and host binding

Format 2 manifests contain four application references and a separate eight-entry
`platform_images` map. The five maintained artifacts are published into the same
approved private ECR repository after every build/scan gate passes. The three
unchanged public runtime references must exactly match the source lock. Build
evidence binds both maps and the platform recipe hash to the reviewed source.
IAM permissions retain the existing repository and artifact-bucket boundaries.

The host's private `platform_images_sha256` is the SHA256 of the canonical
`platform_images` map (sorted keys, compact JSON, trailing newline). A separately
approved platform preparation sets this value along with the matching bootstrap.
An application release cannot change it. Private host settings are fingerprinted;
changing the map requires preparing a new runtime generation with services stopped.
Preparation authenticates to ECR and pulls exact runtime/admin/app digests before
loading secrets. Build-only Alpine/Go/Bun images stay off the production host.

Platform builds label the recipe hash rather than the current application commit.
The lock fixes `SOURCE_DATE_EPOCH`; exports rewrite layer timestamps before an
explicit local import. APK's install log contains wall-clock timestamps, so it
is removed from the image; installed-package databases and unmodified private
build/scan reports remain. This avoids tying an unchanged platform to every app
commit. If a rebuild produces different platform digests, delivery refuses them
until separately reviewed; never update the host's hash merely to bypass a refusal.
[Docker reproducible builds](https://docs.docker.com/build/ci/github-actions/reproducible-builds/),
[timestamp rewriting](https://docs.docker.com/build/exporters/image-registry/).

## Verification and maintenance

Run the complete image build, scans and disposable recovery/edge/terminal tests:

```sh
python3 delivery/checks.py images --evidence-dir <new-private-directory>
python3 delivery/checks.py platform --evidence-dir <new-private-check-directory>
```

The image check writes `image-selection.json` privately. To repeat a failed
rehearsal against those exact bytes, supply `--images <private-image-selection>`
to `deploy/aws/tests/rehearse.py`, `rehearse-edge.py`, or `rehearse-terminal.py`.
The terminal image rehearsal exercises the rebuilt Linux Caddy and application
Bun with TLS Valkey, identity headers, WebSocket reconnect/idle, draining and
callbacks against synthetic providers. It publishes no ports and mounts no Docker
socket. These checks do not replace live AWS, provider or frontend acceptance.

The terminal image exposes the frozen production dependencies at the common
`/app/node_modules` ancestor so the shared file dependency can resolve `zod` and
`yaml`. Its build imports the core and loads packaged demos without networking as
the runtime user. Both the image entrypoint and Compose disable Bun auto-install;
a missing dependency must fail rather than appear after the image was scanned.
The three Go application mains share one compiler invocation to avoid repeating
their shared compilation work.

The maintenance owner reviews upstream/security releases, updates only reviewed
digests/package hashes/module locks, rebuilds affected images and all dependent
apps, and reruns the strict scans and compatibility tests before release approval.
An advisory with no available fix blocks publication; omission or an empty scanner
result is not a remediation. Retain private SBOMs, raw input/output reports,
component mappings, hashes, image sizes and build evidence for each candidate.

To return to upstream images, first verify a new exact upstream digest satisfies
the same scans and compatibility tests. Review the corresponding source/runtime
classification, build/manifest validation and host bootstrap change together.
Do not simply point Compose at a tag. Reprice retained ECR layers, build use and
host disk requirements at the next provisioning gate; derived builds add storage
and maintenance work, without a new monthly compute instance.

Publication, AWS provisioning, IAM/GitHub activation, cutover and the friend's
Vercel handoff remain separate numbered approvals.
