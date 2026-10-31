# Build source references — Test Contract

## Functional Behavior

- New authoring operations constrain rule source references and evidence source references to the authoritative source blocks actually supplied for that stage.
- A model cannot substitute a rule identifier, unrelated message, or invented identifier for a source reference. Exact quote and semantic validation remain mandatory.
- Existing persisted operations retain their provider request, response schema, and repair feedback bytes. Freeze the new behavior at admission; omitted flags retain the old contract.
- Bounded handler repair identifies the available source identifiers without adding calls, changing spending allowances, or remapping invalid output into accepted evidence.

## Unit Tests

- Validate actual generated authoring JSON schemas against valid and invented source references for preparation and edits, including evidence references, multiple sources and no new authority.
- Verify newly admitted plans preserve the behavior flag through serialization; retained plans keep their prior schema and prompt/repair contracts.
- Verify invalid source evidence still fails server validation.

## Integration / Functional Tests

- Exercise handler repair with a real isolated database, record the rejected output and a valid repaired output, and verify accepted policy provenance and settled accounting.
- Reproduce the observed invalid-reference shape using synthetic identifiers. Keep private production/staging response records outside Git.

## Smoke Tests

- Build a protected release, deploy the same manifest to isolated staging, and verify additive migration ledger/index, health and fresh pollers.
- Complete real Build and Improve journeys with bounded provider spend, then use the protected production deployment and promotion gates.

## E2E Tests

- Browser Build shows pricing, persists its first-message title and completes a real run; Improve accepts its first input. Reopen the resulting project and its run history.
- Observe API console errors and retain failure evidence. Never clear counters or bypass provenance checks to make a smoke test pass.

## Manual / cURL Tests

- Read config and health through the approved operator transport while the candidate is closed; after promotion verify normal public HTTPS.
- Preserve all databases/cache and maintain an approved rollback predecessor; do not run Down migrations or existing Temporal schema upgrades.
