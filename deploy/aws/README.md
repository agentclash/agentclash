# AWS single-host platform

Reusable Step 8 infrastructure for a small AgentClash installation. This directory
contains no environment inventory. Preparing it does not authorize provisioning.
CI/CD activation is Step 9; the exact paid resource plan is approved in Step 10.
Keep populated files, secret material, manifests and operational evidence outside
this public checkout and outside build contexts.

## Shape and capacity

Apply separate network, state, host and command stacks through a reviewed,
account-guarded infrastructure workflow. Enable CloudFormation termination
protection on the state stack. The state stack uses Retain on deletion and
replacement; RDS also has deletion protection. Retain means old resources keep
billing until explicitly reviewed and removed. Never use stack deletion as a
migration rollback.

The proposed Singapore shape remains one `t3a.large` (Standard credits), a private
Single-AZ `db.t4g.medium`, 30 GiB boot + 10 GiB retained cache gp3, 20 GiB initial
RDS gp3 and one EIP. There is no ALB, NAT gateway, managed cache, paid private CA,
permanent staging fleet or new organization. R2, E2B and the Vercel frontend stay.

Planning cost remains approximately **USD 165–190/month**, using the regional
rates checked during migration planning: about $153.62/month for compute, initial
storage and IPv4, plus $10–30 for eleven secret entries, alarms, modest S3/ECR and
backup/transfer use. This is not a spending limit. Reprice the exact private
change set immediately before Step 10; RDS storage autoscaling up to 100 GiB,
RDS surplus credits, retained replacement resources, image growth and transfer
increase the bill. Tagged ECR releases are intentionally retained until an
operator verifies that neither the current nor rollback manifest needs them.
[EC2 regional prices](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/ap-southeast-1/index.csv),
[RDS regional prices](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonRDS/current/ap-southeast-1/index.csv),
[IPv4 pricing](https://aws.amazon.com/vpc/pricing/).

Container memory caps total 6,464 MiB: Temporal 2,560; worker 1,536; API and
terminal 768 each; Valkey 640 (256 MiB dataset, fork/AOF headroom); Caddy 192.
That leaves about 1.7 GiB on an 8 GiB host for Linux/Docker/SSM and short admin jobs.
This is a capacity hypothesis, not a load test. Start with worker activity and
workflow task concurrency 2 per queue, sandbox warm pool 0, and small explicit
sandbox caps. Rehearse real concurrency and persistence latency before reopening.

RDS allows 160 connections. App API/worker URLs each require `pool_max_conns=10`;
Temporal pools are at most 5 persistence + 2 visibility connections per internal
service (roughly 28 across four roles), leaving admin/reconnect headroom. Runtime
DB roles have connection limits and cannot create schema objects. The exact RDS
minor, extensions, collation and actual connection use remain live rehearsal gates.

## Build and private configuration

`images.lock.json` pins public platform/build images to repository digests and
`linux/amd64`. The Temporal server and admin tools use the matching 1.31.2 release.
PostgreSQL 18.4 is the local compatibility-test image; the CloudFormation parameter
requires the separately verified available RDS 18 minor. Never silently downgrade
the source database. Application images use dedicated Dockerfiles with allowlisted
contexts. Step 9 builds and publishes them to the private ECR repository, packages
these scripts and a verified Compose executable into a hash-addressed bootstrap
bundle, and supplies the four ECR digests in an immutable release manifest.

On the host, install the approved bundle at `/opt/agentclash`, create root-owned
`/etc/agentclash/host.json` with mode 0600, and mount the retained data volume before
preparing runtime. `host.example.json` and `release.example.json` are deliberately
invalid placeholders until populated privately. Secret references include exact
Secrets Manager VersionIds; deployments do not race a moving AWSCURRENT version.
The host uses its scoped instance role, IMDSv2/hop limit 1 and a Docker firewall
rule denying metadata access. Containers receive neither a Docker socket nor AWS
credentials. Python 3.12 is explicit because AL2023's system Python may be 3.9.
[AL2023 Python](https://docs.aws.amazon.com/linux/al2023/ug/python.html),
[instance metadata controls](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-options.html).

Each secret is JSON: `{"env": {"NAME": "value"}, "files": {"ca.pem": "PEM"}}`.
The loader validates all recipients before staging a new generation on tmpfs,
with per-service owners and 0600 files. An entrypoint exports values inside the
process, so they are absent from Compose and Docker's configured environment.
A privileged host operator can still read process memory; this design does not
claim per-container IAM isolation.

| Recipient | Required contents, supplied privately |
| --- | --- |
| `api`, `worker` | Existing required application/provider/signing secrets; `APP_ENV=production`; `DATABASE_URL` with app runtime role, verify-full, `/run/secrets/db-ca.pem`, pool max 10; `REDIS_URL=rediss://<role>:<secret>@valkey:6379`; `REDIS_TLS_CA_FILE=/run/secrets/cache-ca.pem`; Temporal host `temporal:7233`, namespace `agentclash-prod`, TLS true, server name `temporal`, CA/client certificate/key file paths under `/run/secrets`; no Temporal Cloud key |
| `terminal` | Existing E2B/provider/proxy values; `NODE_ENV=production`; TLS Valkey URL and CA file; explicit HTTPS gateway and CORS origins; exact Caddy peer `172.30.72.2/32`; shutdown 45000 ms; bounded admission/spend settings from the terminal runbook |
| `app-schema` | App owner URL with verify-full and database CA; `MIGRATION_DIR=/migrations`; existing full-basename ledger remains authoritative |
| `temporal` | `DB_HOST`, `DB_PASSWORD` for temporal_runtime, `VISIBILITY_PASSWORD`; RDS CA and separate frontend/internode PEM pairs, plus the private platform CA bundle |
| `temporal-schema` | `SQL_HOST`, port 5432, plugin postgres12, `SQL_TLS=true`, CA file and server name; `TEMPORAL_OWNER_PASSWORD`, `VISIBILITY_OWNER_PASSWORD` |
| `temporal-admin` | CLI `TEMPORAL_ADDRESS`, namespace, `TEMPORAL_TLS=true`, `TEMPORAL_TLS_SERVER_NAME=temporal`, `TEMPORAL_TLS_CA`, `TEMPORAL_TLS_CERT`, `TEMPORAL_TLS_KEY` file paths; operator client certificate |
| `database-admin` | PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE; PGSSLMODE=verify-full and PGSSLROOTCERT; six `<ROLE>_PASSWORD` fields for explicit initial database setup; database CA |
| `valkey` | Four `API_PASSWORD`, `WORKER_PASSWORD`, `TERMINAL_PASSWORD`, `ADMIN_PASSWORD` values used to generate hashed ACL entries; server PEM pair and CA |
| `cache-admin` | `REDISCLI_AUTH` for the admin role and `ca.pem`; never mount this in application containers |
| `caddy` | API_DOMAIN and TERMINAL_DOMAIN DNS names, ACME_EMAIL; no Cloudflare token needed for the initial direct HTTP/TLS challenge path |

Preserve existing application encryption/signing/proxy/provider values exactly.
New database/cache credentials and private certificates are created only during
the separately approved secret/bootstrap step. Do not store actual values in
shell history, SSM parameters, GitHub variables, CI artifacts or user data.

## Initial platform setup and routine operations

Commands below run on the approved host via a private SSM session, after the
account guard and named operator have been confirmed. They are not instructions
to apply infrastructure during Step 8. Every mutating host entrypoint obtains
caller identity before work and holds the same nonblocking deployment lock.

```sh
# Exactly once for a verified, genuinely blank new cache volume:
python3.12 /opt/agentclash/deploy/aws/scripts/volume.py initialize
# A rebuilt host instead attaches the retained volume and uses its recorded UUID:
python3.12 /opt/agentclash/deploy/aws/scripts/volume.py mount
# Populate the immutable release digest privately:
python3.12 /opt/agentclash/deploy/aws/scripts/host.py prepare <release-sha256>
python3.12 /opt/agentclash/deploy/aws/scripts/host.py db-init
python3.12 /opt/agentclash/deploy/aws/scripts/host.py temporal-schema init
python3.12 /opt/agentclash/deploy/aws/scripts/host.py db-grants temporal
python3.12 /opt/agentclash/deploy/aws/scripts/host.py db-grants temporal_visibility
systemctl start agentclash-platform.service
python3.12 /opt/agentclash/deploy/aws/scripts/host.py namespace-init
python3.12 /opt/agentclash/deploy/aws/scripts/host.py health
```

Initialization fails on an existing role/database instead of resetting credentials
or dropping state. Reconcile partial initialization privately before retrying.
For an empty destination, `app-schema` applies application migrations once. For
migration, restore the existing application database **including its ledger**,
verify it, then run the one-off migrator. `temporal-schema upgrade` is an explicit
platform operation after stopping its writers. Never run schema jobs at server
startup or auto-downgrade schemas during app rollback.

The SSM document only accepts a 64-character manifest hash. The `deploy` entrypoint
holds the host lock and verifies the installed delivery driver's approved hash.
It fails closed until Step 9 supplies that driver. Step 9 must preserve this lock
across manifest download, migration, stop verification, start and readiness, and
must not recursively acquire the same lock in child operations. SSO and CI reuse
the same driver; no arbitrary remote-shell parameter is accepted.

## Maintenance, restart and recovery

First apply the reviewed admission fence from the [maintenance runbook](../../docs/deployment/maintenance.md)
and signal `terminal-drain`; existing terminal traffic remains routable while it
drains. This platform has no generic safe callback allowlist: the release operator
must review it against the current routes. The final `stop-apps` action installs
and reloads a complete Caddy fence before stopping API/worker/terminal. It covers
GET/HEAD/OPTIONS as well as mutating verbs, and inspects each retained container's
exit code and OOM status, including containers that crashed before maintenance. A cleanup failure leaves a durable blocking marker.
API, worker and terminal stop grace periods are 45, 150 and 60 seconds respectively.

Caddy never removes an upstream because admission readiness becomes 503. It uses
normal streaming cancellation, a five-minute stream close delay on reload, and
no access logs. Runtime error logs remove the entire request object, including
query strings and forwarded authentication headers. Rehearse SSE and WebSockets
through the actual edge in Step 12/14. The default trusts direct Caddy peers only;
Cloudflare proxy mode needs an explicitly reviewed source-IP/trusted-header
configuration and origin firewall policy before cutover. Do not enable blanket
private-network or arbitrary forwarded-header trust.
[Caddy logging](https://caddyserver.com/docs/caddyfile/directives/log).

A restart/crash does not automatically start writers. Docker restart policies are
`no`; an operator reloads pinned secrets into fresh tmpfs, checks the retained
volume and E2B leases, and uses the reviewed release flow while ingress is closed.
Do not remove `cleanup-unresolved` merely to make a rollout pass. Record E2B,
Temporal, PostgreSQL and cache reconciliation first. Likewise, restore leaves
`restore-unverified`; data/TTL/workflow verification must finish before clearing it.
A newly bootstrapped host also has `recovery-unverified`; record the approved
initial installation or old-host/E2B reconciliation before clearing it. Stop services before changing private runtime settings;
`prepare` records their fingerprint and refuses a mismatched generation. The app
delivery driver must enforce these markers. Neither DNS reversal nor swapping
images alone is rollback after destination writes.

For a host rebuild: stop/drain if possible, record the last approved manifest,
retain RDS/EIP/cache, verify the old host is stopped, detach the cache without force,
create the approved replacement in the same AZ, reattach by volume ID, mount by the
recorded UUID, reinstall the same bundle, reload secret versions and restore Caddy
state if needed. Then rehearse readiness, membership IP change and lease recovery.
The pinned server waits 25 seconds before bootstrap so old host heartbeats
(20-second seed cutoff in this server release) age out before the new IP joins.
[Pinned membership implementation](https://github.com/temporalio/temporal/blob/v1.31.2/common/membership/ringpop/monitor.go).
Host termination protection and retained CloudFormation resources deliberately
require an operator-reviewed replacement plan. Never format a volume during boot.
The host replacement/EBS/SSM test remains an AWS rehearsal gate.

## Backups and certificates

RDS automated backups retain 14 days. The daily cache timer makes a consistent RDB
snapshot and uploads it to private SSE-encrypted S3 with SHA-256 verification and
90-day retention (noncurrent objects 30 days). This low-cost baseline uses S3 cache
backups; any additional EBS snapshots are separately scoped and bounded before
provisioning. Missing/stale backups alarm. RDB snapshots have a recovery gap even
with `appendfsync always` protecting local AOF. Keep affected paid/trial intake
closed after uncertain cache recovery until reconciled or its **entire configured
restriction window** has expired; never clear counters to fix availability.
[Valkey persistence](https://valkey.io/topics/persistence/).

For a consistent full rehearsal backup, finish the source write fence and stop
all app/Temporal/Valkey writers, then run `backup.py offline`. It produces three
logical DB dumps plus the exact cache AOF/RDB set and a content-addressed manifest.
`restore.py <backup-sha256>` requires a root-owned 0600 approval record with `backup_sha256`, `account_id`,
`cache_volume_id` and `keep_intake_closed: true`, matching account,
volume and backup, an empty destination and all writers stopped. It refuses to
overwrite state. It verifies checksums, maps database owners explicitly, reapplies
runtime grants and leaves intake closed. Compare counts/ledgers/sequences,
encryption canary, workflow histories and absolute cache expiries before reopening.
R2 objects are never recopied or rewritten by these scripts.

Use `certificates.py init-ca <private-directory>` once on the operator's secure
machine; `issue <ca-directory> <new-output-directory> <identity>` issues a 90-day
leaf. Supported production identities: temporal, valkey, api, worker, operator.
Never deliver the CA private key to EC2 or application containers. Temporal's
frontend and internode gRPC connections require mTLS; all issued clients belong to
one trusted security domain with full Temporal access. There is no namespace RBAC
claim here. Ringpop membership gossip uses private, unpublished TCP ports and
is not a TLS transport; application payloads use the protected gRPC interfaces.
Add authorization before admitting untrusted clients/workloads.

Rotation: issue new leaves into a new private directory, verify their key matches,
SANs/usages and expiry, stage a CA bundle containing old and new issuers when
changing CAs, and create new immutable secret versions. Drain/stop clients and
platform, reload the reviewed versions, restart/verify Temporal and Valkey, then
restart clients using the release flow. The Go and Bun clients read certificates
at startup. Remove old issuer trust only after every client and admin path has
been checked. `certificates.py check <certificate> --days 30` returns failure inside
the warning window; the host monitor checks mounted leaves. Public ACME certs are
renewed by Caddy using its persisted `/data`; the real DNS/challenge path and loss
of that data must be rehearsed before cutover. RDS uses Amazon's separately
maintained CA bundle, not this private CA.
[Temporal TLS](https://docs.temporal.io/self-hosted-guide/security).

## Local verification and human handoff

```sh
docker build --platform linux/amd64 -f deploy/aws/Dockerfile.backend --build-arg TARGET=db-migrate -t agentclash-step8-test:migrator .
docker build --platform linux/amd64 -f deploy/aws/Dockerfile.terminal -t agentclash-step8-test:terminal .
cfn-lint deploy/aws/cloudformation/*.yaml
python3 -m unittest discover -s deploy/aws/tests -v
python3 deploy/aws/tests/rehearse.py --evidence-dir <private-directory>
python3 deploy/aws/tests/rehearse-edge.py --evidence-dir <private-directory>
```

Install PyYAML for static tests; the host scripts otherwise use Python's standard
library. Run Compose validation with an explicit private environment file, never
the checkout `.env`. The rehearsal creates only uniquely named local containers,
generates private fixtures outside Git and removes their keys, dumps and data.
It does not contact AWS, Temporal Cloud, Railway, E2B, R2 or model providers.

Before Step 10, privately confirm the named platform operator, subscribed alerts,
account/AZs, source-compatible RDS minor/extensions, patched AMI/SSM/Docker versions,
image scanning, exact bill of materials and stop/restore ownership. Before cutover,
verify a real host rebuild, secret delivery/IAM, backup restoration, load headroom,
Cloudflare/public certificates and provider callbacks. After the workload and first backup are verified, start the monitor and backup
timers with `systemctl start agentclash-monitor.timer agentclash-backup.timer`.
Health metrics are scoped to the specific host so rehearsal machines cannot hide
a production outage. The friend performs Vercel
changes only at Steps 13–14. No Vercel token is required for this infrastructure.
