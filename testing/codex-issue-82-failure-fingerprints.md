# codex/issue-82-failure-fingerprints - Test Contract

## Functional Behavior
- Failure review items expose a deterministic `failure_fingerprint` for an exact run-agent failure instance.
- Failure review items expose a deterministic `failure_cluster_key` for grouping the same challenge failure shape across different runs.
- `failure_fingerprint` changes when run-scoped identity changes.
- `failure_cluster_key` stays stable when only run-scoped identity changes and the challenge/check/class shape is unchanged.
- Identity generation is based on canonical, sorted fields so ordering differences in failed checks or refs do not create noisy IDs.
- The API wire contract, generated-facing TypeScript types, and CI regression promotion metadata include both identity fields.

## Unit Tests
- `TestBuildRunAgentItemsComputesStableFailureIdentity` verifies non-empty prefixed IDs, fingerprint uniqueness across runs, and cluster stability across runs.
- Existing failure review read-model tests continue to pass and assert the new JSON fields are present.
- CLI regression tests, if present for promotion metadata, continue to pass with the new metadata keys.

## Integration / Functional Tests
- Backend failure-review API tests continue to pass because the read model serializes the new fields without changing filters, pagination, or promotion behavior.
- OpenAPI and TypeScript contract files include the new fields as required strings.
- CI regression promotion metadata preserves existing fields while adding source fingerprint and source cluster key.

## Smoke Tests
- `go test ./backend/internal/failurereview ./backend/internal/api`
- `go test ./cli/cmd`

## E2E Tests
- N/A - this slice adds read-model identity fields and CI metadata only. Full curation/trend workflows are intentionally reserved for follow-up issue #82 slices.

## Manual / cURL Tests
```bash
curl "$AGENTCLASH_API_URL/api/workspaces/$WORKSPACE_ID/runs/$RUN_ID/failures" \
  -H "Authorization: Bearer $AGENTCLASH_TOKEN"
# Expected: each item contains non-empty failure_fingerprint and failure_cluster_key strings.
```
