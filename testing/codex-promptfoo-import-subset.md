# Codex Promptfoo Import Subset - Test Contract

Issue: #593
Branch: `codex/promptfoo-import-subset`

## Functional Behavior

- `agentclash prompt-eval import-promptfoo <file>` reads a YAML promptfoo config and writes an AgentClash prompt-eval YAML config.
- The importer prints a compatibility matrix so users can see what is imported, refused, and lossy.
- The default output path is stdout; `--out <path>` writes a file and refuses to overwrite unless `--force` is passed.
- The importer converts only a lossless V1 subset: exactly one inline prompt string, inline provider strings, inline tests with `vars`, and supported deterministic assertions.
- Supported assertion mappings are `equals`/`is-equal` -> `exact_match`, `contains` -> `contains`, `regex` -> `regex`, and JSON-schema-bearing `is-json` -> `json_schema`.
- Unsupported or lossy Promptfoo features fail by default with exit code `5`, including `llm-rubric`, schemaless `is-json`, Nunjucks control flow, JavaScript-incompatible or RE2-incompatible regex, and `icontains`.
- `--lossy` only enables documented safe conversions. In this PR, `icontains` is converted to `contains` only when the expected value has no ASCII letters; otherwise it remains refused.
- Imported configs pass existing `prompt-eval validate`.

## Unit Tests

- `TestPromptEvalImportPromptfooBasicConversion` - converts prompt, provider, vars, contains/equality assertions, and produces valid prompt-eval YAML.
- `TestPromptEvalImportPromptfooWritesOutputWithForce` - writes to `--out`, refuses overwrite without `--force`, and overwrites with force.
- `TestPromptEvalImportPromptfooRejectsUnsupportedAssertions` - rejects `llm-rubric`.
- `TestPromptEvalImportPromptfooRejectsSchemalessIsJSON` - rejects `is-json` without a schema.
- `TestPromptEvalImportPromptfooRejectsNunjucksControlFlow` - rejects `{% ... %}` / `{# ... #}`.
- `TestPromptEvalImportPromptfooRejectsIncompatibleRegex` - rejects JavaScript regex features that RE2 cannot compile.
- `TestPromptEvalImportPromptfooIContainsLossyRules` - refuses `icontains` by default and under `--lossy` unless the value is safely case-neutral.

## Integration / Functional Tests

- Run focused Go tests for prompt-eval import.
- Run existing prompt-eval validation tests to confirm imported configs use the same schema.

## Smoke Tests

- Run `go test ./cmd -run 'TestPromptEvalImportPromptfoo|TestPromptEvalValidate' -count=1` from `cli/`.

## E2E Tests

N/A - this is a local file conversion command with no backend dependency.

## Manual / cURL Tests

```bash
cd cli
go test ./cmd -run 'TestPromptEvalImportPromptfoo|TestPromptEvalValidate' -count=1
go test -short -race -count=1 ./...
```
