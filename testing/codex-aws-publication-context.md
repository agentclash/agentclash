# Private publication connection — test contract

## Functional behavior

- Publication uses the same selected Docker daemon and image bytes as the completed build, including a named runner context or an explicitly configured host.
- Registry authentication remains in a new private temporary Docker configuration. Existing registry credentials and credential helpers are not copied or modified. Only the selected context is transferred privately when needed.
- A context transfer failure stops before registry authentication or any image push. AWS and GitHub credentials, endpoints and context archive bytes never enter public output.
- Source scans, image identity checks, immutable private publication, protected environment review and deployment approval gates remain required.

## Unit tests

- Named context export/import preserves selection across credential isolation and copies no prior registry auth.
- Default context and explicit host keep their connection settings without unnecessary context export.
- Failed context export or import prevents registry login and publication.
- Existing scanned-image identity and private manifest tests pass with the new connection handoff.

## Integration and smoke tests

- Reproduce a named context becoming unavailable with an empty Docker configuration; importing only that context restores daemon access with no registry login or changes to the original configuration.
- Required platform checks and configured source/history secret scans pass before publication.
- Normal protected CI builds, scans and publishes a candidate. Privately verify its exact source, image digests, manifest, reports and bootstrap hashes; audit public logs and artifacts.

## E2E and manual checks

- Verify the private candidate without deploying it. Railway data remains retained, AWS app delivery stays closed, and Vercel configuration is unchanged.
- Real-data restore, new fleet provisioning and application cutover remain outside this change.
