# Step 9 CI/CD — Test Contract

Locked before implementation. This step prepares and tests delivery source;
publishing, provisioning and live GitHub/AWS activation require their later steps.
Private configuration, approval identities and operational evidence stay outside Git.

## Functional Behavior

- An always-running PR/main aggregate reports required checks, with conditional
  jobs covering backend/runtime/CLI consumers, frontend, terminal/core, migrations,
  deployment code, image contexts and infrastructure. Missing/cancelled required
  jobs cannot pass. Existing CLI/npm and self-host distribution stay separate.
- Trusted main builds the four Linux application images once from allowlisted
  contexts. Secret/vulnerability gates precede publishing immutable ECR tags and
  content-addressed manifests, SBOM/provenance and a verified bootstrap bundle.
  Private identifiers/reports are never emitted to public CI logs or artifacts.
- Workflows are inert without explicit activation/configuration. Short-lived OIDC
  roles use exact subjects and destinations, with separate build, staging and
  production permissions. Deploy roles cannot read runtime secrets, modify IAM,
  publish arbitrary commands or select unrelated hosts. Infrastructure application
  remains a separately authorized operator path.
- Environment protection is checked before cloud credentials: reviewer/branch/
  bypass restrictions must exist, not merely a YAML environment label. Provide a
  human SSO fallback using the same guarded manifest/SSM path.
- A host transaction validates immutable artifacts, platform compatibility and
  expiring private operator approval, serializes all mutation, verifies drain,
  stops writers cleanly, backs up state and runs the one-off app migrator. It never
  upgrades Temporal schemas as part of an application release.
- Start exactly the candidate images, verify dependencies, API/terminal readiness
  and fresh pollers on all worker queues. Deployment stays fenced for final smoke
  verification. A separately approved promotion opens intake; failure recloses it.
- Failures preserve blocking transaction markers and private recovery evidence.
  No automatic database downgrade, cache reset or unsafe image rollback. The first
  release explicitly has no earlier compatible AWS production image. Later
  releases record a previous manifest without treating it as proof of compatibility.
- Staging/rehearsal uses the same image digests and isolated private configuration.
  Production requires matching rehearsal/backup/recovery evidence. Public edge,
  browser and provider acceptance and temporary capacity remain later live gates.

## Unit Tests

- Manifest, approval, expiry, platform hash, environment and destination guards
  reject malformed, mismatched or mutable inputs before side effects.
- Release state-machine tests cover ordering, the first release, stale promotion,
  failed stop/backup/migration/readiness, crash markers, closed intake and no
  overlapping terminal replacement. Nested host operations do not reacquire locks.
- Build/publication tests verify deterministic hashes, bundle allowlists, safe
  extraction inputs, scanner failures and quiet error handling.
- CI tests verify path-to-consumer coverage, aggregate failure semantics, exact
  OIDC/role restrictions, protected environments and disabled activation defaults.

## Integration / Functional Tests

- Exercise delivery and promotion with isolated fake external command endpoints,
  real temporary state files and immutable fixture objects; inject failures and
  verify preserved markers, unchanged state and closed fences.
- Build/rehearse a bootstrap bundle outside Git, verify binary/source hashes and
  its exact archive inventory, and run relevant existing platform guard tests.
- Run the affected Go, terminal and infrastructure checks locally; record any
  unrelated pre-existing failures honestly. Validate workflow and IAM templates.

## Smoke Tests

- Local CLI help and dry validation require no AWS credentials or mutations.
- Missing activation, approval, private configuration or environment protection
  fails before token exchange/cloud mutation, with sanitized output.
- Verify no secret or private fixture enters commits, build contexts or artifacts;
  run staged and cumulative secret scans on reviewed local commits.

## E2E Tests

Live GitHub protection/OIDC denial tests, ECR publication, SSM target enforcement,
temporary staging, real AWS restore/deploy/promote, external DNS/TLS/browser and
provider checks are deferred to Steps 10–14. Local doubles are not live proof.

## Manual / Operator Tests

Document exact private configuration and approval formats, promotion/recovery
commands, GitHub protection verification and SSO fallback. Prepare a concrete
resource/permission/cost handoff for Step 10 without creating paid resources.
Complete the contract walkthrough and stop for the next numbered approval.
