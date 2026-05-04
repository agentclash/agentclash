# codex/issue-82-regression-provenance - Test Contract

## Functional Behavior
- Regression case API responses expose failure-derived provenance from metadata as stable top-level optional fields: source failure fingerprint, source failure cluster key, and source challenge key.
- Existing first-class source fields such as `source_case_key` remain unchanged.
- Existing `metadata` remains unchanged and still contains the raw promotion metadata.
- The regression suite case list shows failure provenance without requiring users to open raw JSON.
- The regression case detail page shows the same provenance near existing source/evidence fields and keeps the raw metadata JSON available.
- Cases without failure-derived metadata render exactly as before except for absent provenance chips/fields.

## Unit Tests
- Backend API response test verifies metadata-derived provenance fields are populated.
- Web suite/detail rendering test or focused helper test verifies provenance is shown when present and absent when missing.

## Integration / Functional Tests
- Backend regression API package tests pass.
- Web typecheck, lint, and focused regression suite/case tests pass.

## Smoke Tests
- `go test ./internal/api` from `backend/`.
- `npx tsc --noEmit`, `npm run lint`, and focused Vitest from `web/`.

## E2E Tests
- N/A - this is response shaping and existing page rendering.

## Manual / cURL Tests
```bash
curl "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/regression-suites/$SUITE_ID/cases"
# Expected: promoted failure-derived cases include source_failure_fingerprint and source_failure_cluster_key at top level when metadata contains them.
```
