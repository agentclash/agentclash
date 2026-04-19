# Main Input Set Selection — Test Contract

## Functional Behavior
- Users can list challenge input sets for a selected challenge-pack version through the API.
- The run-creation UI automatically loads input sets after a runnable challenge-pack version is selected.
- If a version has multiple input sets, the UI shows a dropdown by input-set name instead of requiring manual ID entry.
- If a version has exactly one input set, the UI auto-selects it.
- If a version has no input sets, the UI leaves input-set selection unset and does not block run creation.
- The create-run request still sends `challenge_input_set_id` only when one is selected.

## Unit Tests
- Backend handler returns input-set summaries for a challenge-pack version.
- Backend handler maps not-found and auth failures correctly.
- Frontend create-run dialog loads input sets when version selection changes.
- Frontend create-run dialog auto-selects the only input set when one exists.
- Frontend create-run dialog requires an explicit dropdown choice when multiple input sets exist.

## Integration / Functional Tests
- GET input sets endpoint returns IDs plus stable labels (`input_key`, `name`) for a runnable challenge-pack version.
- Create-run dialog submits the selected input-set ID in the request body.
- Create-run dialog clears stale input-set selection when the chosen version changes.

## Smoke Tests
- Challenge-pack pages still load normally.
- Run dialog still opens and can create runs for packs with no input sets.

## E2E Tests
- N/A — not adding browser E2E coverage in this change.

## Manual / cURL Tests
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/v1/workspaces/<workspace-id>/challenge-pack-versions/<version-id>/input-sets"
# Expected: 200 with items[] containing id, challenge_pack_version_id, input_key, name

curl -X POST "http://localhost:8080/v1/runs" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "workspace_id": "<workspace-id>",
    "challenge_pack_version_id": "<version-id>",
    "challenge_input_set_id": "<input-set-id>",
    "agent_deployment_ids": ["<deployment-id>"]
  }'
# Expected: 201 with the selected challenge_input_set_id echoed on the run
```
