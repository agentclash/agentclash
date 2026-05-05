# Codex Prompt Eval Remote Preflight - Test Contract

Issue: #589
Branch: `codex/prompt-eval-remote-preflight`

## Functional Behavior

- `agentclash prompt-eval validate [path] --remote` runs local validation first, then performs read-only API checks.
- Remote validation requires a workspace from normal CLI precedence and exits with the prompt-eval validation code when missing.
- Remote validation resolves each model by `model_alias_id` or by unique `alias` from `/v1/workspaces/{workspaceID}/model-aliases`.
- Remote validation resolves provider accounts from `/v1/workspaces/{workspaceID}/provider-accounts`.
- `provider_account: default` remains a local warning, but `--ci` rejects it because CI configs must be pinned.
- Remote validation name-matches playgrounds as `Prompt Eval: {name}` or `Prompt Eval: {name} [{signatureHash}]`.
- Duplicate matching playground names are errors; zero matches are reported as dry-run creates.
- Existing playground test cases are compared by `case_key` to report dry-run create/update/no-op/orphan counts without creating or updating anything.
- `validate --remote --json` includes remote validation details in the existing validation envelope.
- The PR adds a copy-paste GitHub Actions concurrency snippet for prompt-eval CI docs.

## Unit Tests

- `TestPromptEvalValidateRemoteRequiresWorkspace` - `--remote` without workspace returns a structured validation error.
- `TestPromptEvalValidateRemoteResolvesAliasesAndProviderAccounts` - fake API returns one model alias/provider account and validation succeeds.
- `TestPromptEvalValidateRemoteRejectsUnknownOrAmbiguousModelAlias` - zero and multiple alias matches fail.
- `TestPromptEvalValidateRemoteRejectsProviderAmbiguity` - zero or multiple provider account matches fail.
- `TestPromptEvalValidateRemoteRejectsDefaultProviderInCI` - `--remote --ci` rejects `provider_account: default`.
- `TestPromptEvalValidateRemoteDetectsDuplicatePlaygrounds` - multiple matching playgrounds fail.
- `TestPromptEvalValidateRemoteReportsDryRunCounts` - create/update/no-op/orphan counts are computed from fake test cases.
- `TestPromptEvalValidateRemoteIsReadOnly` - fake API fails the test if POST/PATCH/DELETE is called.

## Integration / Functional Tests

- Run focused Go tests in `cli/cmd`.
- Run `go test -short -race -count=1 ./...` from `cli/`.

## Smoke Tests

- `go run . prompt-eval validate /tmp/agentclash-prompt-eval.yaml --remote --workspace ws-1 --api-url <fake-or-local-api>` is covered by fake API tests; no hosted smoke is required for this PR because it is read-only and depends on workspace-specific fixtures.

## E2E Tests

N/A - this PR does not create playgrounds, test cases, or experiments.

## Manual / cURL Tests

N/A - fake API tests pin the API contract for this read-only preflight slice.
