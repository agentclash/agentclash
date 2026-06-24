# feat/dataset-evals-889 — Test Contract

## Functional Behavior

- A workspace can start a dataset eval from a pinned `dataset_version_id`, a `eval_pack_version_id`, a `challenge_id`, and one or more `agent_deployment_ids`.
- The dataset version is immutable input: materialization reads `dataset_version_examples`, not mutable current examples.
- Materialization creates or reuses a `challenge_input_set` plus `challenge_input_items` whose case IDs are stable for the same dataset version, pack version, challenge, and mapping.
- Each materialized input item can be traced back to its `dataset_example_id`.
- Re-running the same pinned dataset version and binding reuses the existing materialized input set instead of duplicating it.
- Running a dataset eval delegates to the existing run creation path; it must not create a parallel execution engine.
- A results read endpoint returns per-example outcomes for dataset-linked runs when result data exists.
- API endpoints are workspace-scoped and use existing dataset/run authorization checks:
  - `POST /v1/workspaces/{workspaceId}/datasets/{datasetId}/evals`
  - `GET /v1/workspaces/{workspaceId}/datasets/{datasetId}/results`
- CLI exposes `agentclash dataset eval <datasetId> --version ... --pack ... --challenge ... --deployment ... [--follow]`.

## Unit Tests

- Materialization tests verify stable generated case IDs and checksum-stable reuse for the same binding.
- Materialization tests verify dataset examples are mapped into stored case documents with input and expected payloads preserved.
- Manager tests verify cross-dataset or cross-workspace version IDs are rejected.
- Manager tests verify no run is created when materialization fails validation.
- CLI helper tests verify repeated `--deployment` flags and required dataset eval flags are encoded correctly.

## Integration / Functional Tests

- Backend API/manager tests cover bind → materialize → create queued run request using a fake run creator.
- Backend tests verify a second eval for the same binding reuses the existing input set link.
- Backend tests verify result rows include `dataset_example_id`, `dataset_version_id`, run identifiers, and verdict fields when linked data exists.
- OpenAPI documents the new endpoints, request body, response body, and errors.

## Smoke Tests

- `cd backend && go test -short -race -count=1 ./internal/api ./internal/repository ./internal/domain`
- `cd backend && go test -short -race -count=1 ./...`
- `cd cli && go test -short -race -count=1 ./...`
- `npx @redocly/cli lint docs/api-server/openapi.yaml`

## E2E Tests

- N/A — this slice adds backend/CLI dataset eval plumbing. Full browser UI for "Run eval on dataset" can be implemented in a follow-up.

## Manual / cURL Tests

1. Create a dataset and snapshot a version with at least one active example.
2. Start an eval:
   ```bash
   curl -sS -X POST "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/datasets/$DATASET_ID/evals" \
     -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"version_id":"'$VERSION_ID'","eval_pack_version_id":"'$PACK_VERSION_ID'","challenge_id":"'$CHALLENGE_ID'","agent_deployment_ids":["'$DEPLOYMENT_ID'"]}'
   ```
   Expected: response contains a normal run identifier and the materialized input set identifier.
3. Re-run the same request.
   Expected: response references the same materialized input set.
4. Fetch results:
   ```bash
   curl -sS "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/datasets/$DATASET_ID/results?version_id=$VERSION_ID" \
     -H "Authorization: Bearer $AGENTCLASH_TOKEN"
   ```
   Expected: response includes per-example linked outcomes when the run has scored.
