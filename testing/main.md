# main — Test Contract

## Functional Behavior
- Opening the Runs page and switching to the `Eval Sessions` tab must not crash the page when the backend has no eval-session evidence warnings.
- `GET /v1/eval-sessions` and `GET /v1/eval-sessions/{id}` must serialize `evidence_warnings` as an empty JSON array when there are no warnings, not `null`.
- The frontend eval-session list and eval-session detail views must tolerate `evidence_warnings: null` from older or inconsistent payloads without throwing.
- Empty warning arrays should render as "No evidence warnings" in the list view and should hide the warning section in the detail view.

## Unit Tests
- `TestBuildEvalSessionResponsesAlwaysEmitWarningArrays` — list/detail API response builders return non-nil `evidence_warnings`.
- `go test ./src/lib/__tests__/eval-sessions.test.ts` equivalent frontend unit coverage should continue passing after the null-guard helper change.

## Integration / Functional Tests
- `go test ./internal/api -run 'TestBuildGetEvalSessionResponse|TestListEvalSessionsHandler'` verifies the backend response envelope.
- `pnpm test -- --runInBand eval-sessions create-eval-session-dialog` verifies the frontend eval-session helpers and nearby UI tests still pass.

## Smoke Tests
- Load `/workspaces/:workspaceId/runs`, click `Eval Sessions`, and verify the page stays rendered.
- Load `/workspaces/:workspaceId/eval-sessions/:evalSessionId` for a session without evidence warnings and verify the detail page does not crash.

## E2E Tests
N/A — not applicable for this targeted bug fix.

## Manual / cURL Tests
```bash
curl -s "$API_URL/v1/eval-sessions?workspace_id=$WORKSPACE_ID&limit=20&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq '.items[0].evidence_warnings'
# Expected: [] or a string array, never null

curl -s "$API_URL/v1/eval-sessions/$EVAL_SESSION_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '.evidence_warnings'
# Expected: [] or a string array, never null
```
