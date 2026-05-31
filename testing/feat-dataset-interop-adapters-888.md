# feat/dataset-interop-adapters-888 — Test Contract

## Functional Behavior

- Import adapters normalize external dataset rows into the canonical dataset example shape: `input`, `expected`, `metadata`, `tags`, and optional `external_id`.
- OpenAI Evals JSONL supports rows with `input` plus `ideal`, and rows with an `item` object. `ideal` maps to `expected`; `item` rows preserve the item payload as canonical input unless a mapping selects a label/output field.
- Braintrust rows map `input`, `expected`, `metadata`, and `tags` one-to-one.
- LangSmith rows map `inputs` to `input`, `outputs` to `expected`, and carry available metadata without losing stable IDs.
- Phoenix rows map `input`, `output`, and `metadata`; a configured `example_id_key` or generic `id_key` becomes `external_id`.
- Generic JSONL and CSV support field mapping for input, output, metadata, tags, and stable ID fields.
- Dry-run imports return a preview of normalized rows plus row-indexed validation errors without mutating dataset examples or versions.
- Additive imports create or update matching `external_id` examples, keep examples that are absent from the import file, and create a new dataset version after a successful non-dry-run import.
- Replace imports make the incoming external-ID set the active dataset contents: matching IDs update, new IDs insert, and active examples with missing external IDs are archived or otherwise removed from the active set before snapshotting.
- Enforced dataset input schemas reject invalid rows and report row indices; non-enforced schemas report validation errors without blocking valid rows.
- Exports stream dataset examples for the requested format and preserve canonical fields and `external_id` where the target format supports it.
- API endpoints are workspace-scoped and require the existing dataset authorization checks:
  - `POST /v1/workspaces/{workspaceId}/datasets/{datasetId}/import`
  - `GET /v1/workspaces/{workspaceId}/datasets/{datasetId}/export`
- CLI commands call those endpoints:
  - `agentclash dataset import <datasetId> <file> --format ... --mode ... [--dry-run] [--map ...]`
  - `agentclash dataset export <datasetId> --format ... [--version ...]`

## Unit Tests

- Adapter tests in `backend/internal/datasets/adapters` cover OpenAI, Braintrust, LangSmith, Phoenix, generic JSONL, and CSV normalization.
- Export adapter tests verify canonical examples serialize back into OpenAI, Braintrust, LangSmith, Phoenix, JSONL, and CSV shapes.
- Mapping tests verify `input_keys`, `output_keys`, `metadata_keys`, `tags_key`, `id_key`, and Phoenix-style `example_id_key`.
- Validation tests verify enforced schemas produce row-indexed failures and do not silently mutate invalid rows.
- Import mode tests verify additive upsert behavior and replace active-set behavior.
- CLI command tests or focused helper tests verify import/export request paths, query parameters, and payload/mapping parsing.

## Integration / Functional Tests

- Backend API tests import equivalent logical examples from OpenAI, Braintrust, LangSmith, and Phoenix fixtures into a dataset and verify equivalent canonical examples.
- Backend API tests verify dry-run import returns preview rows and no new dataset version.
- Backend API tests verify non-dry-run import creates a dataset version snapshot.
- Backend API tests verify export followed by re-import yields equivalent canonical examples, ignoring server-generated IDs and timestamps.
- OpenAPI includes the import/export endpoints, request parameters, and response schemas.

## Smoke Tests

- `cd backend && go test -short -race -count=1 ./internal/datasets/... ./internal/api -run 'TestDataset'`
- `cd backend && go test -short -race -count=1 ./...`
- `cd cli && go test -short -race -count=1 ./...`
- `npx @redocly/cli lint docs/api-server/openapi.yaml`

## E2E Tests

- N/A — this change adds backend/CLI import-export plumbing. Browser upload UI can be validated in a follow-up if web polish is added beyond the API/CLI slice.

## Manual / cURL Tests

1. Create or select a workspace dataset.
2. Dry-run import a JSONL fixture:
   ```bash
   curl -sS -X POST "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/datasets/$DATASET_ID/import?format=openai&mode=add&dry_run=true" \
     -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
     -F "file=@examples/datasets/openai-evals.jsonl"
   ```
   Expected: response contains normalized preview rows and no new dataset version.
3. Import the same fixture without `dry_run`.
   Expected: examples are created with `source=import`, a dataset version is created, and row errors are empty.
4. Export the dataset:
   ```bash
   curl -sS "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/datasets/$DATASET_ID/export?format=braintrust" \
     -H "Authorization: Bearer $AGENTCLASH_TOKEN"
   ```
   Expected: JSONL output contains canonical input/expected/metadata/tags fields in Braintrust shape.
