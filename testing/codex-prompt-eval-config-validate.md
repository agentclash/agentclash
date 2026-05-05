# Codex Prompt Eval Config Validate - Test Contract

Issue: #588
Branch: `codex/prompt-eval-config-validate`

## Functional Behavior

- `agentclash prompt-eval init [path]` writes a V1 `.agentclash/prompt-eval.yaml` scaffold.
- `prompt-eval init` refuses to overwrite an existing file unless `--force` is passed.
- `agentclash prompt-eval validate [path]` parses YAML and validates local semantics without contacting the API.
- Valid configs require `schemaVersion: 1`, non-empty `name`, non-empty `prompt.template`, at least one model, and at least one test.
- A model must specify `alias` or `model_alias_id`; `provider_account: default` is allowed locally but returns a warning.
- Validation catches duplicate test keys, missing template variables, unsupported template control syntax, unknown assertions, invalid RE2 regex, empty `json_schema`, and case counts over `--max-cases`.
- Single-test configs validate successfully but return a warning about coarse pass-rate gates.
- Validation computes deterministic assertion signatures so later compiler work can group heterogeneous tests safely.
- `challenge-pack init --template prompt_eval` prints a deprecation pointer to `prompt-eval init` while still producing the existing challenge-pack scaffold.

## Unit Tests

- `TestPromptEvalInitWritesScaffold` - creates the expected YAML scaffold and validates it.
- `TestPromptEvalInitRefusesExistingFile` - fails unless `--force` is passed.
- `TestPromptEvalValidateAcceptsValidConfigWithWarnings` - returns valid with warnings for `provider_account: default` and one test.
- `TestPromptEvalValidateRejectsLocalSemanticErrors` - covers duplicate test keys, missing variables, unsupported template syntax, unknown assertions, invalid regex, empty JSON schema, no tests, missing model selector, and `--max-cases`.
- `TestPromptEvalValidateJSONEnvelope` - `--json` returns `schemaVersion: 1`, valid/errors/warnings, case count, model count, assertion signatures, and exit code.
- `TestChallengePackPromptEvalTemplateMentionsPromptEvalInit` - existing template still works and tells users about the new prompt-eval scaffold.

## Integration / Functional Tests

- Run the focused Go tests in `cli/cmd` for the new command and challenge-pack scaffold behavior.
- Run `go test -short -count=1 ./cmd`.

## Smoke Tests

- `go run . prompt-eval init /tmp/agentclash-prompt-eval.yaml --force`
- `go run . prompt-eval validate /tmp/agentclash-prompt-eval.yaml`
- `go run . prompt-eval validate /tmp/agentclash-prompt-eval.yaml --json`

## E2E Tests

N/A - this PR does not create remote playgrounds or launch experiments.

## Manual / cURL Tests

N/A - this PR is local CLI validation only and does not call backend APIs.
