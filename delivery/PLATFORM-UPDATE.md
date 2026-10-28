# Bounded dependency platform maintenance

`platform_update.py` prepares an existing AWS host for the reviewed package and
JOSE security updates in Caddy and Temporal. Its exact existing-file changes are
`delivery/maintained.py`, `deploy/aws/platform/inputs.lock.json`, and
`deploy/aws/platform/caddy/go.mod` and `go.sum`, plus both `go.mod`/`go.sum`
pairs in `deploy/aws/platform/temporal` and `deploy/aws/platform/temporal/cli`.
Temporal keeps its server/CLI versions and source unchanged; the lock changes
select the fixed JOSE patch release. Its exact additions are
`deploy/aws/platform/Dockerfile.caddy` and the Caddy directory's
`apply-jose-patch.sh`, `JOSE-MIGRATION.md`, and `jose_migration_test.go`.
Only these changes and their derived source-hash metadata are permitted. The
additions must be absent on the predecessor, including dangling symlinks; their
absence is recorded in the durable transaction. Existing source bytes are saved
before fencing. All host scripts, Compose/tool bytes and notices, config,
systemd files, other build recipes and upstream image lock must remain identical.
It refuses other bootstrap upgrades.

The helper lives outside the installed bootstrap allowlist. A reviewed copy can
prepare an already published application manifest; changing this helper or
`operator_cli.py` does not require rebuilding that candidate. Never copy it into
`/opt/agentclash/deploy/aws/scripts` or unpack a new host module to bypass the
installed platform checks.

## Operator acceptance

First inspect the real host privately through human SSO: account/region/instance,
installed source and platform-map hashes, current/deployed manifest, pinned secret
references, retained cache identity, runtime fingerprint, candidate and recovery
markers. Saved inventories are insufficient. Complete actual isolated staging
rehearsal of the exact candidate, SQL/session/E2B/schedule/callback drain and
recovery verification from the maintenance runbooks. Upload the corresponding
candidate-specific evidence through `operator_cli.py evidence`.

Rehearsing this transition requires a genuine old installation on isolated
staging: bootstrap the published `76047aa` manifest with isolated secrets/data,
initialize its application schema through migration 86, perform its reviewed
initial deploy/promotion, and obtain its real deployed record and offline backup.
Use a checkout matching the old platform for those old-release operations;
current-source `validate_release` correctly refuses an old platform. The repaired
operator approval publisher can issue acceptance for that old manifest. Then run
this helper and the new candidate's ordinary deploy/promotion on the same staging
data. A fresh schema-92 install or a fabricated predecessor record does not
rehearse the production transition. Staging host approval/configuration still
needs its bounded lifetime, isolation checks and cost/teardown owner.

Create a root-owned, mode **0600** private approval outside the bootstrap. The
`operation` is `platform-update`; all hashes below are exact lowercase SHA256
values, not source commit SHAs. Example structure (deliberately invalid):

```json
{
  "format": 1,
  "operation": "platform-update",
  "account_id": "<account>",
  "region": "<region>",
  "instance_id": "<instance>",
  "operator": "<configured-maintenance-owner>",
  "environment": "production",
  "previous_release_sha256": "<currently-deployed-manifest>",
  "release_sha256": "<exact-candidate-manifest>",
  "old_platform_source_sha256": "<verified-installed-source-hash>",
  "platform_source_sha256": "<candidate-source-hash>",
  "old_platform_images_sha256": "<verified-installed-canonical-map-hash>",
  "platform_images_sha256": "<candidate-canonical-map-hash>",
  "bootstrap_sha256": "<candidate-bootstrap-bundle-hash>",
  "issued_at": 0,
  "expires_at": 0,
  "acknowledgements": {
    "drain_verified": false,
    "callbacks_reconciled": false,
    "no_automatic_schema_rollback": false,
    "destination_writes_acknowledged": false,
    "isolated_rehearsal_verified": false
  },
  "evidence": {
    "drain": "<production-drain-evidence-hash>",
    "recovery": "<production-recovery-evidence-hash>",
    "rehearsal": "<isolated-staging-evidence-hash>"
  }
}
```

Set the acknowledgements only after verifying their actual private evidence.
Acceptance expires at most one hour after issue and must still be valid before
installing files after the offline backup. The predecessor cannot be null: this
helper updates an existing installation. Staging also needs its existing
unexpired, at most 24-hour host delivery configuration.

## Human SSO transport and execution

Use the approved personal SSO profile on **THINKBOOK** for the AWS CLI. Browser
sign-in, when needed, happens on **MACBOOK**. All commands on THINKBOOK run through
`/home/ayushparihar/.local/bin/agent-run`. This is an audited human SSM/root
maintenance action; the application CI roles cannot invoke arbitrary shell.

1. Review the helper bytes and record their SHA256, approval SHA256 and candidate
   manifest SHA256 privately. Store the helper and approval outside Git; upload
   them with the human role to a unique private prefix such as
   `approvals/platform-maintenance/<nonce>/` in the existing artifact bucket using
   `aws s3 cp --sse AES256`. Do not publish their bodies or resource identifiers.
2. Put the audited `AWS-RunShellScript` parameters in a mode-0600 JSON file outside
   Git. Its commands must create a mode-0700 directory under
   `/root/agentclash-maintenance/<nonce>`, use `umask 077`, download the two exact
   objects, verify **both** reviewed SHA256 values with `sha256sum --check`, then
   execute the host command below. Set a bounded `executionTimeout` appropriate
   to the three-database backup. No shell tracing or child output forwarding.
3. Submit that private parameter file through the existing approved operator role:

   ```sh
   # THINKBOOK; placeholders refer to the already reviewed private destination.
   /home/ayushparihar/.local/bin/agent-run aws --profile <human-sso-profile> \
     --region <region> ssm send-command --document-name AWS-RunShellScript \
     --instance-ids <verified-instance> --parameters file://<private-parameters.json>
   ```

   The final command inside the audited SSM operation runs on the **AWS HOST**:

   ```sh
   python3.12 /root/agentclash-maintenance/<nonce>/platform_update.py \
     --approval /root/agentclash-maintenance/<nonce>/platform-update.json \
     --release <exact-private-candidate-manifest-sha256>
   ```

The host command checks root ownership, AWS account and IMDSv2 instance identity,
uses only canonical installed modules and holds the existing deployment lock.
There is no root override or alternate installation path. Retain the SSM command
ID and reviewed helper hash in the private operation record.

## Result and recovery boundary

The command fences and drains, verifies clean application/Temporal/Valkey stops,
takes an offline backup through the old runtime, and cleanly stops Caddy. It
durably snapshots all eight changed source files, source-hash metadata, host settings,
Compose environment, generation reference, runtime fingerprint and current/deployed
release records. It copies **no decrypted secret payloads**; the old generation
stays in tmpfs. It changes only the approved source files/metadata and host
`platform_images_sha256`, then calls existing `host.prepare` with the same pinned
secret references. It explicitly starts Valkey, Temporal and Caddy behind the
closed fence. Applications remain stopped; no database/namespace/schema
initialization, Temporal schema upgrade or application migration runs here.

Success writes private `platform-update-completed.json`, leaves the deployed
predecessor byte-for-byte unchanged, and removes only the helper-owned recovery
marker and unresolved transaction. The record retains the prior offline backup
digest and the fresh maintenance backup digest, even after application deployment
overwrites `last-offline-backup.json`. The subsequent ordinary deployment records
its own additional backup digest in `candidate-release.json`/promotion history.

Next use the existing approved **closed deploy**, private in-container real API
smoke, separately reviewed promotion, and production browser verification. Real
browser acceptance of the exact candidate runs in isolated staging beforehand.
Do not substitute synthetic authentication/browser fixtures for production smoke.

On failure, inspect `platform-update-unresolved.json`, the owned
`recovery-unverified` marker, `snapshot_directory`, both backup records, actual
container states, installed hashes and runtime control files. A failed first
journal write may leave its `.platform-update-pending` file and owned marker;
the snapshot directory is named `platform-update-<approval-sha256>`. The failure
path attempts to fence, and stops Caddy if reload fails. If host/Docker control
is unavailable, apply the separately approved operator network fence.

Do not rerun the installer, delete markers, or blindly retry. Before application
deployment/migration starts, the reviewed operator recovery can restore the
saved eight source files/metadata, old host settings and control files, and remove
exactly the four `added_source_paths` recorded in the durable transaction. Verify
those names against this helper's fixed `ADDED` allowlist, refuse links or other
unexpected paths, and fsync their parent directories after removal. Validate the
restored old platform hash and manifest, then verify the retained old tmpfs
generation and data identities and start only the old platform
behind the fence, and reconcile the helper's exact transaction before removing
its own marker. If tmpfs was lost, reprepare the verified old manifest with its
original pinned secret references. After any application migration or destination
writes, use reviewed fix-forward or rehearsed restore/reconciliation; this helper
performs no automatic database rollback.
