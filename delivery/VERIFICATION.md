# Step 9 local verification

This records source preparation, not a live AWS deployment. Private evidence
contains commands, logs, exact image identities, approvals and scan reports.

| Check | Outcome |
| --- | --- |
| Backend, shared runtime and CLI | Build, vet and short race suites passed; application migrator integration checks passed |
| Terminal | Frozen install, type checks and tests passed |
| Frontend | Clean install, Vitest, types, lint and production build passed locally |
| Delivery tests | 31 tests passed, including exact-target fake AWS dispatch, policy guards, failed phases, closed intake, stale pollers and expiring publication approval |
| Existing platform guard tests | 13 passed |
| Workflows and infrastructure | Pinned actionlint, cfn-lint, Python lint and shell syntax checks passed |
| Bootstrap packaging | Verified Compose bytes, deterministic archive, allowlisted inventory, source hash and tamper checks passed; the pinned Linux Compose binary validated the complete configuration |
| Local command smoke | Help, synthetic configuration validation and disabled-configuration refusal passed without AWS access |
| Publication vulnerability gate | Correctly refused the current images; remediation required before publishing |

The real disposable Docker rehearsal exercised PostgreSQL TLS, self-hosted
Temporal mTLS, worker queues, server replacement/certificate rotation, durable
cache state and backup restoration. The pinned CLI's legacy timestamp format was
verified against actual workflow and activity pollers on all three queues and
corrected in the health probe. Promotion
also rejects pollers last seen more than two minutes ago, even if they appeared
after the original deployment started.

The host transaction tests use real temporary files and immutable fixture objects
with fake external adapters. They establish sequencing, destination/approval
guards and failure handling. They do not establish live IAM enforcement, root
filesystem permissions, SSM behavior or internet-facing readiness.

Before live use, Steps 10–14 must verify the exact account/region/AMI and patched
images; real secret delivery; GitHub reviewer/branch/bypass controls and OIDC
denials; SSM target/document denials; temporary staging isolation and teardown;
host rebuild and data restore; and browser, provider, DNS/TLS and Vercel handoff.
Live infrastructure, registry publication and provider changes were not performed
as part of Step 9. The first AWS release still has no previous compatible AWS
image rollback.

## Next action

Review and authorize the [Step 9.1 patch proposal](RELEASE-BLOCKERS.md), then
reprice the private Step 10 resource plan. The prepared delivery source must remain
disabled while the image gate is unresolved. There is no blanket vulnerability
waiver and no approval to incur charges in this source-preparation step.
