# Step 8 local verification (historical)

Verified locally on 2026-09-12. This is infrastructure preparation, not a deployed
AWS environment. Real operational evidence and inventories are stored privately.
This records the original Step 8 pins. See the later
[Step 9.1 verification](../../delivery/PATCH-VERIFICATION.md) for the current
versions, repeated recovery rehearsal and remaining image blockers.

- Four CloudFormation templates pass `cfn-lint`; production Compose validates with
  an explicit environment file and every image selected by digest.
- Four Linux/amd64 application images build from allowlisted contexts. The real
  migrator runs the entire application schema on PostgreSQL 18.4 under its schema
  owner, then repeats without applying migrations again.
- Thirteen Python guard tests pass: account/manifest scope, secret handling,
  private permissions, archive traversal, TLS configuration, retained resources,
  prior-crash shutdown handling and authenticated readiness/retry behavior.
- Go pub/sub and Temporal client unit suites pass with the race detector.
- Matching Temporal 1.31.2 server/schema tools initialize both SQL stores over TLS.
  Application, persistence and visibility runtime roles cannot perform DDL.
  All three queues complete durable workflows after replacement at a different
  membership IP and certificate/CA rotation. Full database dump/restore recovers
  those workflows and their visibility results.
- Valkey 8.1.8 passes TLS/ACL tests with the actual Go client and Linux/Bun terminal
  image. Lua, transactions, provider throttling and pub/sub work. AOF rewrite,
  restart and offline restore preserve spend values and absolute trial expiry.
  A deliberately truncated AOF is rejected rather than silently repaired.
- Pinned Caddy 2.11.4 runs without root and passes all-method maintenance fencing,
  reload and request-metadata removal from error logs.
- Python lint/compilation and shell syntax checks pass. Per-commit secret scans
  and exact-diff review are required before publishing any of these artifacts.

The local tests exposed and corrected several compatibility issues: the server's
configuration-root interpretation, stale membership seeds during single-host
replacement, Valkey CLI's `REDISCLI_AUTH` name, missing transaction ACL commands,
and local Docker port changes across restarts. The pinned server now waits 25
seconds before membership bootstrap; no workflow or membership rows are deleted.

AWS IAM/SSM enforcement, actual RDS minor/extensions, root tmpfs secret delivery,
EBS/host replacement, public DNS/ACME/Cloudflare, real provider callbacks, browser
flows, workload capacity and restore recovery time remain live acceptance gates.
Basic host health does not replace backlog/persistence-latency monitoring or
application smoke tests; configure and demonstrate those alerts in the later
platform rehearsal before cutover. Image vulnerability scanning and review of
current patches remain release gates in CI before provisioning/publishing.

The named platform operator must be confirmed privately. Step 9 prepares CI/CD,
immutable release packaging and the protected application delivery driver. Step 10
separately authorizes the paid platform. Vercel handoff stays in Steps 13–14.
