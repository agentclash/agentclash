# feat/dataset-trace-ingest-890 — Test Contract

## Functional Behavior

- A workspace can ingest production traces into a dataset as **reviewable candidates** before promotion.
- Supported ingest sources:
  - `otel` — OTLP JSON with `gen_ai.*` semantic convention attributes
  - `braintrust`, `langsmith`, `phoenix` — vendor span/export JSONL using the same field shapes as dataset import adapters
  - `agentclash` — existing run agent replay/events (`run_id` + optional `run_agent_id`)
- Raw trace payloads are stored as workspace artifacts when provided inline; import batches reference the artifact id.
- A configurable redaction hook drops or hashes sensitive metadata keys before candidate persistence.
- Promotion creates a provenance-rich `dataset_example` with `source=trace|promotion`, preserves `source_run_id` / `source_trace_id` / `source_platform`, allows edited `expected`, and snapshots a new dataset version.
- Re-promoting the same candidate is idempotent and returns the existing example.
- API endpoints are workspace-scoped:
  - `POST /v1/workspaces/{workspaceId}/datasets/{datasetId}/traces/import`
  - `GET /v1/workspaces/{workspaceId}/datasets/{datasetId}/trace-candidates`
  - `POST /v1/workspaces/{workspaceId}/datasets/{datasetId}/trace-candidates/{candidateId}/promote`
- CLI exposes:
  - `agentclash dataset import-traces`
  - `agentclash dataset trace-candidates list`
  - `agentclash dataset promote`

## Unit Tests

- OTLP parser extracts input/output/model metadata from `gen_ai.input.messages`, `gen_ai.output.messages`, and usage attributes.
- Vendor span export parsing reuses adapter normalization for Braintrust/LangSmith/Phoenix rows.
- AgentClash replay parsing builds candidates from transcript turns or model-call output.
- Redaction removes configured metadata keys and hashes configured fields.
- Promotion repository tests verify provenance fields, version snapshot, and idempotent re-promote.

## Integration / Functional Tests

- Manager tests cover ingest → list candidates → promote → example + version creation.
- Manager tests verify schema enforcement blocks promotion when dataset input schema is enforced and candidate input is invalid.
- OpenAPI documents the new endpoints, request bodies, response bodies, pagination, and errors.

## Smoke Tests

- `cd backend && go test -short -race -count=1 ./internal/datasets/traces ./internal/api ./internal/repository`
- `cd backend && go test -short -race -count=1 ./...`
- `cd cli && go test -short -race -count=1 ./...`
- `npx @redocly/cli lint docs/api-server/openapi.yaml`

## E2E Tests

- N/A — trace candidate review UI can land in a follow-up; this slice ships backend + CLI ingest/promote plumbing.

## Manual / cURL Tests

1. Import OTLP candidates:
   ```bash
   curl -sS -X POST "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/datasets/$DATASET_ID/traces/import" \
     -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"source_platform":"otel","payload":{...}}'
   ```
   Expected: import batch id and pending candidates.
2. List candidates:
   ```bash
   curl -sS "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/datasets/$DATASET_ID/trace-candidates" \
     -H "Authorization: Bearer $AGENTCLASH_TOKEN"
   ```
3. Promote one candidate with edited expected output:
   ```bash
   curl -sS -X POST "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/datasets/$DATASET_ID/trace-candidates/$CANDIDATE_ID/promote" \
     -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"expected":{"answer":"corrected"}}'
   ```
   Expected: promoted example + new dataset version with provenance preserved.
