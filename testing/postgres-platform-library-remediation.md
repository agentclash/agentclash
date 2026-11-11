# PostgreSQL platform library remediation — Test Contract

## Functional Behavior

The maintained PostgreSQL image must upgrade `krb5-libs` from `1.22.2-r1` to
`1.22.3-r0` and `libxslt` from `1.1.43-r3` to `1.1.43-r4` before its final scan.
Add exactly two package entries to `deploy/aws/platform/inputs.lock.json` and
include them only in the PostgreSQL package build context. Other package contexts
and all existing package pins remain unchanged.

Use only these official Alpine v3.24/main/x86_64 archives:

- https://dl-cdn.alpinelinux.org/alpine/v3.24/main/x86_64/krb5-libs-1.22.3-r0.apk
- https://dl-cdn.alpinelinux.org/alpine/v3.24/main/x86_64/libxslt-1.1.43-r4.apk

| Package | Version | Archive SHA256 |
| --- | --- | --- |
| `krb5-libs` | `1.22.3-r0` | `fad5b46a953e6a37a1bb61a84538fa08ecb6a07816f439eb06f2735aebfcbfbf` |
| `libxslt` | `1.1.43-r4` | `40e6cc7d09ce1e69421c2b37e1da4e9afef486843f21c3a79339cd6d93c1c617` |

Both archives must report the matching name/version and `x86_64` in `.PKGINFO`.
Compare their metadata, byte size and compressed-control checksums with the
official branch APKINDEX before pinning. Retain the existing SHA256 verification,
APK signature validation and `apk add --no-network --no-cache --upgrade` behavior;
no new trust keys, `--allow-untrusted`, edge fallback or unpinned resolution.
`krb5-libs` depends on `krb5-conf`, musl, libcom_err, libcrypto, libkeyutils,
libssl and libverto; `libxslt` depends on musl and libxml2. Linux signed offline
installation must confirm dependency compatibility with the source image.

Keep the locked PostgreSQL upstream digest remediation-source-only, rebuild gosu
through the existing path, and retain runtime user, entrypoint and persistence
behavior. Both the changed input lock and maintained recipe already participate
in the recipe hash: old platform caches must be invalidated. Preserve the strict
consumed-output HIGH/CRITICAL gate, scanner coverage, input-finding reconciliation,
source/scanner input hashes and evidence integrity. No CVE ignore or gate changes.

## Unit Tests

- Extend the existing mocked `maintained.build` harness with a focused PostgreSQL
  regression. Inspect actual package and final binary build contexts, showing
  both exact new archives and lock entries before the PostgreSQL final gate.
- Assert the final PostgreSQL build consumes the patched package-stage image;
  retain the exact locked source and final output gate assertions.
- Assert both new archives are absent from every other build context and preserve
  those contexts' complete existing package inventories.
- Run the regression red against the old implementation, then green after the
  minimal recipe/lock edit. This checks build data flow, not just constants.
- Run maintained, build, platform-cache, image/scanner and release contract units
  appropriate to this change, plus Python syntax, available ruff, JSON validation
  and `git diff --check`.

## Integration / Functional Tests

Root-controlled Linux CI must fetch and hash-verify the exact pinned APKs, accept
their signatures with the source image's existing Alpine trust store, install
both offline with all dependencies satisfied, rebuild gosu and pass the unchanged
strict full consumed PostgreSQL image scan. Confirm the reported installed
versions and retain all original input findings and reconciliation evidence.
All other maintained images and applications must retain their existing gates.
Local mocked tests cannot establish a clean output image or an actual APK install.

## Smoke Tests

Root-controlled image checks must preserve PostgreSQL entrypoint, runtime user,
gosu, data-directory/persistence and health behavior. No database migration is
introduced. No local Docker, AWS or deployment action is authorized here.

## E2E Tests

N/A locally: this bounded package-input correction changes no application flow.
The full consumed-image Linux acceptance above remains required before release.

## Manual / cURL Tests

The official HTTPS archives and APKINDEX were downloaded to owned private scratch
storage, inspected without executing package code, and hashed. Recheck the two
lock SHA256 values against those exact archive bytes; parse `.PKGINFO` and compare
the official index identity/architecture/size/control checksum. Signature member
presence is only structural verification; real signature acceptance is required
in the unchanged Linux installation. Keep receipts and raw diagnostic evidence
outside Git. No private artifact or host metadata belongs in this contract.

## Boundaries and Remaining Unknowns

Owned implementation files are `delivery/maintained.py` (PostgreSQL package list),
`deploy/aws/platform/inputs.lock.json` (two additions) and the focused
`delivery/tests/test_maintained.py` regression. This contract is the only public
document addition. No scanner, workflow, digest, application, auth, CLI, migration,
provider, model or SDK change. No local Docker/build, GitHub/push, CI trigger, AWS,
credentials or environment-file access. Linux installation, runtime compatibility
and full consumed-output scan remain pending root review and CI.
