# Codex Prompt Eval Compile Run - Test Contract

Issue: #590
Branch: `codex/prompt-eval-compile-run`

## Functional Behavior

- `agentclash prompt-eval run [path]` validates local config and remote resources before writing anything.
- Run requires a workspace and uses the same model/provider resolution as `validate --remote`.
- Test cases are grouped by assertion signature; heterogeneous assertion signatures compile into separate playgrounds.
- Missing playgrounds are created with `name`, `prompt_template`, and an `evaluation_spec`.
- Existing matching playgrounds are updated with the current prompt template and evaluation spec.
- Missing test cases are created; changed test cases are updated; orphaned test cases are left untouched.
- `tests[].assert[]` compiles into `expectations.prompt_eval_assertions` and playground-level validators using signature-level validator keys.
- One fresh experiment is created for each resolved model in each compiled playground.
- Structured output returns schemaVersion, workspace ID, config hash, model/test/case counts, playground IDs, experiment IDs, and UI links.

## Unit Tests

- `TestPromptEvalRunCreatesPlaygroundTestCasesAndExperiments` - fake API captures exact create payloads and experiment payloads.
- `TestPromptEvalRunUpdatesExistingResourcesWithoutDeletingOrphans` - fake API captures playground/test-case update payloads and proves no DELETE is called.
- `TestPromptEvalRunGroupsByAssertionSignature` - heterogeneous assertions create separate playgrounds with signature suffixes.
- `TestPromptEvalRunValidatorMappingCoversSupportedAssertions` - exact/equals/contains/regex/json_schema/json_path_match/boolean_assert map to expected validator declarations.
- `TestPromptEvalRunJSONEnvelope` - run output includes config hash, workspace, playgrounds, experiments, UI links, and counts.

## Integration / Functional Tests

- Run focused Go tests in `cli/cmd`.
- Run `go test -short -race -count=1 ./...` from `cli/`.

## Smoke Tests

- `go run . prompt-eval run /tmp/agentclash-prompt-eval.yaml --workspace ws-1 --api-url <fake-or-local-api>` is covered by fake API tests. Hosted smoke is deferred until a workspace fixture exists.

## E2E Tests

N/A - this PR launches experiments but does not follow or aggregate results; E2E follows in #591.

## Manual / cURL Tests

N/A - fake API tests pin the request payloads for this write slice.
