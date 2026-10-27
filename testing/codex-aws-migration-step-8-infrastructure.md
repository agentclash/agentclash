# Step 8 infrastructure — Test Contract

Locked before implementation. This step prepares reusable infrastructure and
operator tooling locally; it does not provision, publish, cut over or retire.
Real parameters, credentials, identities and evidence remain outside Git.

## Functional Behavior

- CloudFormation describes one x86 8 GiB EC2 host in Standard credit mode,
  an encrypted retained cache volume, private Single-AZ PostgreSQL RDS, public
  HTTPS ingress and SSM administration. No SSH, NAT, ALB or public data listeners.
- RDS has deletion protection, retained state and 14-day automated backups.
  Application, Temporal persistence and visibility have separate logical
  databases, schema owners and least-privilege runtime credentials.
- Production Compose/systemd runs pinned images with bounded memory, private
  networks, durable Valkey AOF, verified database/cache/Temporal TLS and mTLS for
  Temporal frontend and internode traffic. Membership uses the current private IP.
- Runtime secret values come from Secrets Manager to protected tmpfs files;
  each container receives only its own files. No secret-bearing environment in
  Compose, Docker inspect, user data, command arguments, logs or image layers.
- API, worker and terminal preserve Step 6/7 stop budgets (45/150/60 seconds
  minimum), admission drain and non-overlapping terminal replacement. Failed
  shutdown blocks replacement. Caddy retains routes during admission drain.
- A constrained SSM document accepts an immutable release digest, invokes an
  installed operator entrypoint and serializes changes on the host. Application
  and Temporal schema jobs are separate; no schema mutation at normal startup.
- Guarded scripts cover account verification, runtime secret delivery, database
  initialization/schema jobs, backup/restore, certificate issuance/rotation and
  host rebuild. Restore never silently resets durable trial/spending limits.
- Host/operator responsibilities, restore/certificate drills, cost assumptions
  and the next approval are documented without private inventory.

## Unit / Static Tests

- CloudFormation schema/lint validation and production Compose validation pass.
- Secret-loader tests reject traversal, invalid formats, unintended services and
  unsafe permissions, preserve secrets exactly and redact failure output.
- Manifest/account guards reject wrong accounts, mutable images and malformed
  release references. SSM cannot receive arbitrary commands or script locations.
- Test infrastructure invariants, script syntax and generated configuration.

## Integration / Functional Tests

- Locally run the selected PostgreSQL, Temporal server/admin, Valkey and Caddy
  image versions. Record immutable digests and architecture in a public lockfile.
- Initialize all three databases with distinct owners/runtime roles, apply the
  selected Temporal SQL schemas and verify runtime roles cannot perform DDL.
- Real Temporal with Go SDK: mTLS, namespace, workflow completion/visibility and
  recovery across container replacement using the same persistence. Reject
  untrusted clients and incorrect server trust/name. Inspect connection limits.
- Real Valkey: TLS/auth, Lua, pub/sub and durable key/TTL preservation through
  restart and AOF rewrite; demonstrate corrupt/truncated AOF is rejected.
- Rehearse isolated backup/restore and certificate rotation. Generated private
  material/evidence must stay outside the checkout and disposable resources
  must be cleaned up. No live E2B, R2, WorkOS or LLM calls in local tests.

## Smoke Tests

Check rendered Caddy/Temporal configuration, private listener topology, service
health commands and stop budgets locally. Validate scripts fail closed before
cloud writes when account/manifest/operator prerequisites are absent.

## E2E Tests

Live AWS host rebuild, RDS minor/extension validation, SSM/IAM enforcement,
Cloudflare/public certificates, real provider callbacks and browser journeys
are explicitly deferred to the approved provisioning/rehearsal/cutover steps.
Local container evidence does not establish those live behaviors.

## Manual / Operator Tests

Provide concrete operator commands for maintenance, schema jobs, backup/restore,
certificate expiry/rotation and retained-volume host replacement. Record the
private operator assignment and exact bill of materials before provisioning.
Complete the local review and secret scan, then stop for Step 9 approval.
