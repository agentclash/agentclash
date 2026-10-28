# Caddy JOSE security migration

CVE-2026-116587 has no fixed go-jose/v3 release. This build moves Smallstep's
runtime JOSE facade to v4 and explicitly selects go-jose/v4 v4.1.6, which fixes
the advisory. The module graph may retain v3 for upstream tests; the compiled
Caddy binary and its scanned package inventory must contain no v3 module.

- Vendor advisory: https://github.com/go-jose/go-jose/security/advisories/GHSA-mgcc-cww5-fp2q
- Vendor fix: https://github.com/go-jose/go-jose/commit/a0111f82742fa92bceec9ede40845e73319e6230
- Smallstep migration: https://github.com/smallstep/crypto/pull/1157
- Pinned official commit: `00c45432f48558f92a6e7da6d885d94e90db4d7a`
- Canonical module version: `go.step.sm/crypto v0.91.2-0.20261027230031-00c45432f485`

The Smallstep migration was unmerged when pinned. No official certificates
companion commit was available. A real Caddy compile exposed compatibility
edits beyond the migration PR's initial list. `apply-jose-patch.sh` records our
entire downstream patch to certificates v0.30.2: five edits across four source
files, guarded by exact upstream and result hashes and edit counts:

1. Change the two ACME challenge and the Azure and OIDC provisioner
   `Expected.Audience` fields to `AnyAudience`. All four literals require exactly
   one audience, so matching any expected audience preserves their prior
   matching condition. This would not be safe as a general replacement for
   multiple expected audiences.
2. Change the AWS token builder's `CompactSerialize()` to `Serialize()`.
   The v4 JWT builder serializes to the compact JWS representation.

Caddy's runtime dependency closure does not include certificates' `cas/stepcas`
or cli-utils' `token` package. Their old builder calls are deliberately unchanged.
If a future Caddy build imports them, compilation must fail until the additional
compatibility edits receive their own review. Existing `CompactSerialize()`
calls on JWS/JWE objects retain their valid v4 API and are also unchanged.

The build first downloads and verifies the canonical upstream modules using
go.sum and the Go checksum database. It then applies only these three edits to
the disposable builder's module cache, preserving canonical module identities
in the binary. Post-patch source hashes verify the exact intended result;
`go mod verify` is deliberately performed before the documented source edits.
Original upstream license notices, this record and the patch script ship with
the image. There are no vulnerability ignores or alternate module identities.

The build runs `jose_migration_test.go` against the real Smallstep facade: an
oversized RSA JWK must fail during parsing, a signed JWT must retain compact
serialization, and single-audience validation must reject wrong or missing
audiences. A successful complete Caddy build and clean image scanner gates are
also required; the focused tests alone do not establish downstream compatibility.
