# AWS release delivery

Step 9 prepares this source locally. Publishing, paid provisioning and live
GitHub/AWS activation remain separate approvals. The workload maintenance owner
and all populated configuration/evidence are recorded privately.

## Checks and release artifacts

`aws-checks.yml` runs on every PR/main push and exposes **Release checks** as the
aggregate required status. Conditional jobs cover backend, shared runtime and CLI,
frontend Vitest/lint/types/build, terminal/core, migrations, images and platform
tooling. Shared runtime changes check both consumers; delivery/workflow changes
check all groups. The aggregate requires every selected job to succeed and rejects
missing/cancelled checks. It does not rely on a path-filtered required workflow.

`aws-release.yml` reruns checks for trusted main. Only an explicitly activated
`aws-build` environment can publish. Four Linux/amd64 release images are built
once; staging and production subsequently use those exact digests. Builds export
tracked Git source into a private temporary context, then apply the dedicated
Dockerfile allowlists. Ignored workstation files and credentials cannot enter
the context. Local image IDs are captured before scanning and used for publication
so later changes to a local tag cannot change the candidate.

Source secret scanning and all-image secret/high/critical vulnerability gates run
before AWS credential exchange or publication. The selected public runtime/admin/
base images are also scanned. Scanner failures have no bypass flag. Public platform
digests use Trivy's remote source with explicit amd64 selection; application image
IDs use the local Docker source. This avoids Docker's incomplete multi-platform
index export problem while preserving the exact selected architecture.

The private bucket receives:

| Prefix | Contents / writer |
| --- | --- |
| `releases/<digest>.json` | Account/region, four ECR digests, source commit, migration-set and platform hashes / build role |
| `releases/by-source/<commit>.json` | Immutable source-to-manifest index / build role |
| `build-evidence/` | Scanner results, CycloneDX SBOMs and build provenance / build role |
| `bundles/<digest>.tar.gz` | Allowlisted host scripts/configuration and checksum-verified Compose binary / build role or approved bootstrap publisher |
| `evidence/operator/<digest>.json` | Reviewed drain/recovery/rehearsal/smoke evidence / human SSO only |
| `approvals/<environment>/<operation>/<digest>.json` | Expiring exact-target acceptance / human SSO only |
| `deployments/<environment>/<digest>.json` | Successful promotion history that survives host replacement / host role |

The build record contains source revision, workflow/run references, tool/base
hashes, exact image identities and report hashes. It is private provenance backed
by scoped publishing permissions, not a claim of signed SLSA attestation. No private
registry names, manifests, reports, resources or runtime secrets are emitted as
GitHub outputs, job summaries or public artifacts. CI child output stays private;
failed checks can be reproduced locally with an external evidence directory.
SSM responses contain only generic status. Do not enable shell tracing.

ECR tags and S3 release/index/evidence objects are immutable. Conditional object
creation is enforced in the bucket policy. A retry may reuse identical content;
it cannot replace a source index with a different candidate. Human approvals can
be renewed under bucket versioning, and the host records the exact approval hash.

## Activation and permissions

The default is inert: `AWS_DELIVERY_ENABLED` is absent, example configurations
contain `enabled: false`, and host delivery is disabled. Runtime credentials never
go to GitHub. Store the populated configuration as **environment secret**
`AWS_DELIVERY_CONFIG`, not a repository variable. Each environment has its own
role/configuration. All actual parameters and role/subject evidence stay private.

`cloudformation/delivery.yaml` under `deploy/aws` defines separate build, staging
and production roles. Trust requires the exact observed OIDC subject and audience;
it does not assume which subject format the repository uses. Reuse an existing
provider, or explicitly enable creation after confirming its absence. Staging
trust stays absent until a temporary instance and exact subject are supplied.
No wildcard branch/PR trust or long-lived AWS keys are accepted.
Build permissions cover one repository and the build artifact prefixes. Deploy
permissions cover two fixed SSM documents and one exact, correctly tagged host.
CI cannot read runtime secrets, publish operator approvals, modify IAM or run
arbitrary shell. `GetCommandInvocation` and ECR authorization require the API's
unscoped resource form; neither grants general SSM shell access.

Before activation, a repository administrator must configure all three
environments (`aws-build`, `aws-staging`, `aws-production`), required reviewers,
reviewer availability, an explicit main-only branch policy, disabled administrator
bypass and a main ruleset requiring PRs and **Release checks**. Configure the
self-review policy intentionally for the available operators. CI checks the actual
reviewers, branch policy, required checks and the privately reviewed policy hash
before OIDC. A YAML environment name is not protection.

The REST API does not consistently expose administrator bypass state. The human
must verify it and real denial cases in Step 11; `admin_bypass_verified_disabled`
records that fact. It is not a substitute for a live denial test. The independent
SSO-only host approval remains necessary even if a GitHub administrator bypasses
an environment. If the required GitHub controls/API access are unavailable, leave
OIDC delivery disabled and use the human SSO path below.

Use `protections.py` after configuring the real environment/ruleset. It checks
the approved personal GitHub login, reads the live controls and writes their
snapshot/hash privately. Its token input is a 0600 JSON file with a `token` field,
created temporarily from the approved personal CLI login. Remove it after use.
Inspect the actual OIDC subject privately; never print the JWT or its claims.
The snapshot command does not change any GitHub settings.

```sh
python3 delivery/protections.py --config "$DELIVERY_PRIVATE/production.json" \
  --token-file "$DELIVERY_PRIVATE/github-token.json" \
  --output "$DELIVERY_PRIVATE/protection-snapshot.json"
```

Step 10 provisions the separately reviewed platform through human SSO. Step 11
configures GitHub protections before installing OIDC trust, then verifies actual
denials with the scoped roles present. Enable staging trust only when its separately
approved temporary target exists. Infrastructure apply is
not granted to app CI; a future infrastructure workflow needs its own reviewed
execution/PassRole policy. CLI Release Please/GoReleaser/npm and self-host GHCR
distribution retain their existing separate paths. Vercel uses no AWS credential
or cross-provider trigger and remains the frontend owner's final handoff.

## Host transaction and promotion

Install the reviewed bootstrap and put its `delivery/release.py` SHA256 in private
host settings. Enable the `delivery` block only after bootstrap/restore verification.
Changing it changes the runtime fingerprint and requires guarded preparation.
Pin both SSM document versions and content hashes in each private CI configuration.
The fixed documents accept only a release digest; one deploys and one promotes.
The driver is imported under the existing host lock, without nested lock acquisition.

Before deployment, close admission and complete the detailed SQL/session/E2B/
schedule/callback drain described in the maintenance runbooks. Prepare private
evidence for the exact candidate. The driver rechecks open Temporal work after
fencing, verifies clean application/platform exits and confirms that no database
clients remain before its offline backup. It does not infer a drained application
from UI inactivity or from counts alone.

The sequence is: validate manifest/platform/build evidence and operator approval;
full fence; terminal admission signal; drain check; clean app/Temporal/Valkey stop;
three-database and durable cache backup; select/pull the four immutable images;
one-off application migrator and grants; start dependencies and app; verify
API/terminal health and fresh workflow/activity pollers on all three queues.
Temporal schema upgrades are never part of an application release.

Successful deployment leaves a candidate **closed to public intake**. Review real
browser/provider/stream smoke evidence and issue a separate promotion approval.
Promotion verifies the same candidate/predecessor, requires pollers seen within
the last two minutes as well as after candidate startup, opens the
fence, checks both public HTTPS readiness endpoints and records private history.
If reopening or history recording fails, the fence recloses and a blocking marker
remains. If Caddy reload fails, the failure path stops the public listener.
An unavailable Docker/host control plane still requires operator network fencing;
do not treat an unsuccessful stop command as proof that traffic is blocked.

The worker can write to data services while intake is closed. Both deploy and
promote approvals must acknowledge destination writes. The first AWS release has
**no previous compatible AWS image rollback**. Before destination writes, the
cutover can return to the frozen source under its approved plan; afterwards use
fix-forward or an explicitly rehearsed data/platform restore and reconciliation.
Subsequent releases record a previous manifest but do not automatically declare
it schema/workflow compatible. No down migrations or cache resets are automated.

On failure, inspect `deployment-unresolved.json`, cleanup/restore markers, the
private backup and actual containers before reconciling. Do not delete a marker
just to retry. A completed candidate retry only rechecks health. Redeploying the
current image requires a fresh approval naming it as the current predecessor.
Restore the promotion record from private history when rebuilding a host; keep
`recovery-unverified` until that state and its data boundary have been verified.

## Private operator inputs and fallback

`config.example.json`, `approval.example.json` and `evidence.example.json` are
deliberately invalid placeholders. Populate them outside Git with mode 0600.
`DELIVERY_PRIVATE` below is an operator-selected directory outside every checkout.
Commands do nothing on their own until the private configuration, SSO session,
target resources and corresponding later-step authorization exist.

An approval names account, region, instance, environment, operation, operator,
exact manifest, exact predecessor (null only for the first release), evidence
hashes and a start window of at most one hour. Promotion additionally references
smoke evidence. Production requires isolated staging evidence for the same manifest.
The CLI stages no runtime secrets. Evidence fields are operator attestations backed
by the actual privately retained results, not a way to fabricate passing tests.

```sh
python3 delivery/ci.py validate --kind production --config "$DELIVERY_PRIVATE/production.json"
python3 delivery/ci.py inspect --kind production --config "$DELIVERY_PRIVATE/production.json" \
  --source <public-commit-sha> --output "$DELIVERY_PRIVATE/candidate.json"
python3 delivery/operator.py evidence --config "$DELIVERY_PRIVATE/production.json" \
  --source <public-commit-sha> --file "$DELIVERY_PRIVATE/drain-evidence.json"
python3 delivery/operator.py approval --config "$DELIVERY_PRIVATE/production.json" \
  --source <public-commit-sha> --file "$DELIVERY_PRIVATE/deploy-approval.json"
python3 delivery/ci.py deploy --kind production --config "$DELIVERY_PRIVATE/production.json" --source <public-commit-sha>
# After independent smoke verification and uploading the promotion approval:
python3 delivery/ci.py promote --kind production --config "$DELIVERY_PRIVATE/production.json" --source <public-commit-sha>
```

Protected GitHub delivery uses the same source SHA, manifest and driver; dispatch
`aws-deploy.yml` with the target and `deploy`/`promote` operation. Never paste a
private resource ID or populated manifest into workflow inputs.

Initial bootstrap publishing can use the same build/scanner path under human SSO
before OIDC activation. Use a clean checkout of the reviewed commit, a private
build configuration (`kind: build`, approved profile/operator) and a
`publication_approval` containing that operator, account, region, source revision,
`local_required_checks_passed: true` and an expiry within one hour. Record the
actual local required-check evidence first. Then run `ci.py build --kind build
--config <private-file> --source <approved-commit>`. This is a later-step action;
it does not waive the image/source scanner gates or authorize a Git push.
The approval must remain valid after building and scanning; an expired build
requires a newly reviewed publication window before it can publish.

Staging uses a separate host, all three databases/Temporal cluster, cache and
private runtime secrets, isolated R2/provider fixtures and bounded E2B use.
Its CI and host delivery settings require an `expires_at` within 24 hours; its
approval also requires `isolated_rehearsal_verified: true`. Set and review its
capacity/cost ceiling and teardown ownership before creating it. The expiry guard
prevents further delivery; it does not stop AWS billing or delete retained state.

## Verification and next gate

Run `python3 delivery/checks.py <group> --evidence-dir <private-directory>` for a
reproducible local check group. Platform checks need the pinned cfn-lint/PyYAML/ruff
versions in the workflow. `package.py` accepts the checksum-verified Linux Compose
binary and produces a deterministic private bundle. Unit tests exercise failed
stops/migrations/backups/readiness/promotion, immutable artifacts and policy guards.
See [verification](VERIFICATION.md) for completed local checks and limitations.

The resource shape and planning cost remain the Step 8 baseline, approximately
USD 165–190/month before workload growth. Step 9 adds scoped IAM definitions and
modest private release/history storage; it creates no paid resources or permanent
staging fleet. Reprice the exact private bill of materials before Step 10.
Publication stays blocked until current image vulnerability findings are resolved
through the [Step 9.1 patch proposal](RELEASE-BLOCKERS.md). No blanket
ignore/unfixed-vulnerability switch is included.

[GitHub OIDC](https://docs.github.com/en/actions/how-tos/secure-your-work/security-harden-deployments/oidc-in-aws),
[environment protection](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments),
[conditional S3 writes](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes-enforce.html).
