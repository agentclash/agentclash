# feat/tools-library — Test Contract

## Functional Behavior

- Every library tool accepts every input shape allowed by its published JSON Schema.
  - Omitting the optional prefix from `list-files` lists the workspace root.
  - Omitting the optional command from `run-tests` and `build-project` uses primitive auto-detection.
  - `query-sql` requires `database_path`, matching the runtime primitive.
- Live HTTP variants encode interpolated values for their destination context.
  - JSON string values containing quotes, backslashes, or newlines remain valid JSON.
  - Form values and query/path components cannot inject additional parameters or path segments.
- Library live variants never place `${secrets.*}` values in request URLs, where the HTTP primitive can return them to the agent in the response URL.
- The calculator accepts bounded arithmetic over numeric literals and rejects executable syntax, booleans, oversized expressions, excessive ASTs, non-finite results, and resource-amplifying powers.
- UUID generation uses a runtime available in the shipped sandbox image.
- Bulk library creation rejects empty and oversized batches and enforces a bounded HTTP request body.
- `conflict=suffix` remains deterministic for existing workspace collisions and handles concurrent slug conflicts without silently violating suffix behavior.
- Re-adding a library tool after deletion restores the archived slug instead of reporting a hidden tool as already present.
- The frontend validator and dry-run preview understand destination-encoded placeholders used by live library definitions.

## Unit Tests

- `TestLibraryEntriesValidate` — every default/live definition validates and catalog metadata remains coherent.
- `TestLibraryDelegateDefinitionsAllowSchemaValidInputs` — optional inputs do not leave unresolved delegate placeholders; required runtime inputs are required in the schema.
- `TestLibraryLiveDefinitionsKeepSecretsOutOfURLs` — live request URLs contain no secret placeholders.
- `TestSafeCalcScript` — normal arithmetic succeeds; code execution, booleans, oversized input, and power amplification fail.
- `TestCreateToolsFromLibraryInputValidate` — empty and oversized entry arrays fail validation.
- `TestCreateToolsFromLibraryConflicts` — skip and suffix behavior remain correct.
- Repository create-tool coverage — a matching archived workspace tool is restored with the new definition and becomes visible again.
- Template resolution tests — JSON, query, and path encoders escape special characters without changing ordinary values.
- Frontend definition tests — encoded placeholders validate and simulate using their underlying declared parameter.
- Frontend gallery tests — added-state uses workspace slugs, bulk add sends catalog slugs, and failure/empty states remain actionable.

## Integration / Functional Tests

- `go test -short -race ./...` passes from `backend/`.
- Frontend type checking, linting, unit tests, and production build pass from `web/`.
- OpenAPI lint resolves the library paths and schemas; `entries.maxItems` matches backend validation.

## Smoke Tests

- `GET /v1/tool-library` returns a non-empty catalog with categories and valid definitions.
- `POST /v1/workspaces/{id}/tools/from-library` creates a known default entry and reports unknown/conflicting entries in `skipped`.
- Adding one gallery entry refreshes the workspace tool list and navigates to the created tool.

## E2E Tests

N/A — authenticated browser E2E needs a deployed session. The component/API behavior is covered by unit and handler tests; the existing Vercel preview remains the manual click-through target.

## Manual / cURL Tests

- `curl -s "$API/v1/tool-library" -H "Authorization: Bearer $TOKEN" | jq '.items | length'` returns at least 50.
- POST a known slug to `/v1/workspaces/$WS/tools/from-library`; expect `201`, one created item, and an empty `skipped` array.
- POST more than the documented maximum number of entries; expect `400 validation_error` and no tools created.
- Add the same slug again with default conflict handling; expect it in `skipped`.
- Add the same slug with `conflict=suffix`; expect a newly suffixed tool slug.
