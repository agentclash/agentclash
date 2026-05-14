# Voice Schema Drift Test - Test Contract

## Functional Behavior
- Add a CI-visible test that keeps CLI embedded voice schemas byte-for-byte aligned with `docs/schemas`.
- Cover all three current voice report schemas.
- Fail with a clear message naming the schema file that drifted.

## Unit Tests
- `TestEmbeddedVoiceSchemasMatchDocsSchemas` passes when embedded CLI schema copies match docs schemas.

## Integration / Functional Tests
- `cd cli && go test ./cmd` passes.

## Smoke Tests
- N/A - test-only change.

## E2E Tests
- N/A.

## Manual / cURL Tests
- N/A.
