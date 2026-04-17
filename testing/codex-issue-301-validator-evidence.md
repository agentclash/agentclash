# Issue 301: expose validator evidence on scorecards

## Functional expectations

1. Add an optional `evidence` field to scorecard `ValidatorDetail`.
2. The backend must populate `ValidatorDetail.evidence` from already-persisted validator `RawOutput`; no database migration is required.
3. `ValidatorDetail.evidence` must be type-discriminated with a `kind` field and only contain JSON-safe values.
4. The backend must expose string-valued expected/actual evidence for string-style validators that already persist `actual_value` and `expected_value`.
5. The backend must expose structured regex evidence containing at least the pattern and actual value.
6. The backend must expose structured JSON schema evidence containing the schema reference/draft, actual value, and validation error list when present.
7. The backend must expose structured JSON path evidence containing path, comparator, actual, expected, and existence state when present.
8. Validators without a tailored evidence mapping must still expose a fallback `custom` evidence shape rather than dropping raw evidence entirely.
9. The scorecard OpenAPI schema must describe the new `ValidatorDetail.evidence` field and the validator evidence union under `components/schemas`.
10. Frontend API types must model `ValidatorDetail.evidence`.
11. The scorecard inspector must render evidence instead of the placeholder hint when evidence is present.
12. For string-style evidence, the inspector must show expected and actual values side by side.
13. For regex evidence, the inspector must show the pattern, actual value, and whether a match was found; matched text should be visually distinguishable when a match can be computed.
    Regex highlighting is best-effort for JavaScript `RegExp` patterns and may fall back to plain text when patterns are not directly highlightable in the client.
14. For JSON schema evidence, the inspector must show the schema reference/draft, actual value, and validation errors as a structured list when present.
15. For fallback/custom evidence, the inspector must show a readable raw JSON payload and keep the prose reason visible.
16. Existing scorecard behavior for validators without evidence must remain non-breaking.

## Tests to add or run

- Backend integration coverage in `backend/internal/repository/repository_integration_test.go`
  - scorecard document includes `validator_details[].evidence` for exact/string validators
  - scorecard document includes structured evidence for JSON validators
  - fallback/custom evidence path is preserved for unmapped validator shapes
- Frontend tests in `web/`
  - validator evidence helpers or inspector rendering logic for string, regex, JSON schema, and custom evidence
- Validation runs
  - `go test ./internal/repository`
  - `go test ./internal/scoring`
  - `npm test -- --run ...` or `vitest run ...` for the added web tests

## Manual verification

1. Load a scorecard with a failing validator and confirm the inspector shows evidence instead of the placeholder text.
2. Confirm a string-style validator shows expected and actual panels.
3. Confirm a regex validator shows pattern and highlighted/identified match information when applicable.
4. Confirm a JSON schema validator shows schema info and validation errors when applicable.
