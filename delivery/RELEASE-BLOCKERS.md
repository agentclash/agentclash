# Remaining AWS release gates

Step 9.2 resolves the five consumed-image blockers from Step 9.1. All twelve
selected application, platform, administrative and build images pass the local
secret and HIGH/CRITICAL vulnerability gates. The original upstream findings
remain in private reports with explicit component remediation mappings. See the
[dated verification](PLATFORM-VERIFICATION.md) and
[maintenance procedure](PLATFORM-IMAGES.md).

This completes local source preparation. No image, source branch or bootstrap
was published, and no paid platform or live delivery configuration was activated.
A later publication must build the approved clean source and repeat current
scans; this local result is not a permanent security exemption.

## Next checkpoint: Step 10 resource and cost review

Before creating resources, inspect the intended member account and region using
the approved personal SSO identity. Keep actual identifiers, account details and
resource selections private. Present the exact proposed bill of materials and
monthly estimate for the owner's approval, including:

- EC2 capacity and architecture, patched AMI/Engine packages, retained EBS and
  public IPv4; verify that self-hosted Temporal and the application fit the host.
- The available PostgreSQL 18 RDS minor, extensions, CA, storage and backup plan;
  the local PostgreSQL 18.6 image does not prove regional RDS availability.
- Nine private ECR artifacts, layer sharing and retention, build time, artifact
  storage, backups, Secrets Manager, monitoring/log retention and network traffic.
- The approved VPC/subnets, account/region boundaries, budget alerts, SSM access
  and named maintenance/restore ownership.

Local image sizes have been retained privately for this review. Uncompressed
image sizes include shared layers and do not establish billable compressed ECR
storage. Maintained images add storage and rebuild duties without introducing
another always-running instance. Price the actual regional configuration and
include temporary rehearsal resources before seeking paid provisioning approval.

## Host Engine remains a separate check

Replacing the application's deprecated Docker module with maintained Moby
client/API modules does not patch the Docker host. Engine 29.8.0 was the upstream
candidate reviewed in Step 9.1. Step 10 must verify the actual AMI package and
vendor backports against then-current advisories. The local workstation Engine
was not upgraded during Steps 9.1 or 9.2.
[Moby Engine release](https://github.com/moby/moby/releases/tag/docker-v29.8.0).

The original daemon archive and mount-race advisories remain relevant to host
selection, even though the legacy module is absent from the application graph.
[Archive advisory](https://github.com/moby/moby/security/advisories/GHSA-x86f-5xw2-fm2r),
[mount-race advisory](https://github.com/moby/moby/security/advisories/GHSA-rg2x-37c3-w2rh).

## Later numbered approvals

Step 10 provisions only the privately reviewed and approved resources, with
application intake closed. Step 11 configures and verifies protected GitHub/OIDC/
SSM delivery; the source remains disabled until then. Step 12 verifies the real
AWS host, restoration, IAM/secret delivery, workload capacity and provider paths.
The friend's Vercel/DNS/public TLS handoff and browser acceptance remain at the
final cutover steps. R2 and E2B remain in use.

The local rehearsals use synthetic providers and disposable data services. They
do not prove production IAM, actual RDS compatibility, live Temporal Cloud history
migration, external callbacks or frontend acceptance. These are separate evidence
and human-intervention gates, not unresolved local image remediations.
