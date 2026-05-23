# feat/case-templating-840 — Test Contract

## Functional Behavior

- Challenge pack authors can use `{{key}}` and nested `{{parent.child}}` placeholders in `code_execution` validator `test_command` strings.
- Placeholders resolve from the per-case `payload` map and from structured `inputs[]` keys (input values override payload keys on conflict).
- At bundle validation time, every `test_command` containing placeholders must resolve for **every** case in every input set (strict mode).
- At run time, `code_execution` checks use the active run agent's first input-set case to render `test_command` before sandbox exec.
- `prompt_eval` challenge `instructions` use the same renderer (payload + inputs), not inputs-only.

## Unit Tests

- `TestBuildCaseTemplateContext_MergesPayloadAndInputs` — input overrides payload key
- `TestRenderCaseTemplate_ResolvesTopLevelAndNestedPaths` — `{{order_id}}`, `{{customer.id}}`
- `TestRenderCaseTemplate_StrictMissingKey` — returns error for unresolved placeholder
- `TestRenderCaseTemplateLenient_LeavesUnresolved` — non-strict leaves literal `{{missing}}`
- `TestExtractCaseTemplatePlaceholders` — dedupe / syntax
- `TestValidateBundleCaseTemplates_*` — pass/fail bundle validation cases
- `TestExecuteCodeExecutionCheck_RendersTestCommand` — engine substitutes before exec

## Integration / Functional Tests

- N/A — bundle validation + engine unit tests cover integration surface for this change.

## Smoke Tests

- `cd backend && go test -short -race -count=1 ./internal/challengepack/... ./internal/engine/... -run 'CaseTemplate|CodeExecutionCheck_Renders'`

## E2E Tests

- N/A — no full stack run required for templating-only change.

## Manual / cURL Tests

- N/A — validation is exercised via `challengepack.ValidateBundle` in tests and existing pack upload paths.
