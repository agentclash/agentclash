# feat/dataset-generation-892 — Test Contract

## Functional Behavior

- A workspace can start an in-house synthetic generation job from existing dataset examples (seeds).
- v1 ships **Self-Instruct** only; other strategies return a clear unsupported error until follow-ups land.
- Generation runs asynchronously via `SyntheticDatasetGenerationWorkflow` on the worker task queue.
- Each job stores config, progress counts, token usage, cost, rejections, and optional output dataset version id.
- Accepted rows become `dataset_examples` with `source=synthetic` and provenance metadata (`generator`, `model`, `job_id`).
- Filters in v1:
  - JSON schema validation when the dataset enforces `input_schema`
  - Input dedup via canonical JSON hash against existing examples and the in-batch accept set
  - Parse/structure validation for generator output
- API endpoints:
  - `POST /v1/workspaces/{workspaceId}/datasets/{datasetId}/generate`
  - `GET /v1/workspaces/{workspaceId}/datasets/{datasetId}/generations/{jobId}`
- CLI exposes `agentclash dataset generate <datasetId> --strategy self-instruct --count N --provider-account ... --model-alias ... [--seeds-tag ...] [--follow]`.

## Unit Tests

- Self-Instruct prompt builder includes seed examples and requests JSON `{input, expected}` output.
- Output parser rejects malformed model responses.
- Dedup filter rejects duplicate canonical inputs.
- Schema filter rejects invalid inputs when schema enforcement is enabled.
- Repository maps generation job rows and records rejections.
- Manager tests verify `ActionManageDatasets` authorization and workflow start.

## Integration / Functional Tests

- Workflow activity tests use `provider.FakeClient` to accept one synthetic row end-to-end (mocked DB/repo where needed).
- OpenAPI documents generate + get job endpoints, enums, and error responses.

## Smoke Tests

- `cd backend && go test -short -race -count=1 ./internal/datasets/generation ./internal/api ./internal/repository ./internal/workflow -run 'DatasetGeneration|Synthetic|SelfInstruct'`
- `cd backend && go test -short -race -count=1 ./...`
- `cd cli && go test -short -race -count=1 ./...`
- `npx @redocly/cli lint docs/api-server/openapi.yaml`

## E2E Tests

- N/A for v1 — web generation wizard deferred.

## Manual / cURL Tests

1. Start generation:
   ```bash
   curl -sS -X POST "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/datasets/$DATASET_ID/generate" \
     -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"strategy":"self_instruct","target_count":5,"provider_account_id":"...","model_alias_id":"...","seeds_tag":"seed"}'
   ```
2. Poll job:
   ```bash
   curl -sS "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/datasets/$DATASET_ID/generations/$JOB_ID" \
     -H "Authorization: Bearer $AGENTCLASH_TOKEN"
   ```
   Expected: status transitions to `completed`, accepted/rejected counts populated, cost fields set, optional `version_id` when enabled.
