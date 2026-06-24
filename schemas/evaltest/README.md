# Local Eval Test Schemas

Language-neutral JSON Schema contracts for the pre-deploy eval SDK (`agentclash evaltest`).

Parent epic: [#1104](https://github.com/agentclash/agentclash/issues/1104)  
ADR: [docs/adr/predeploy-eval-sdk-repo-strategy.md](../docs/adr/predeploy-eval-sdk-repo-strategy.md)

## Schemas

| File | Purpose |
|------|---------|
| `agent-result.schema.json` | Agent trace/result shape adapters must emit |
| `eval-report.schema.json` | Full eval run report (SDK + CLI output) |

## Exit codes

The `agentclash evaltest run` command and SDK runners use these stable exit codes:

| Code | Name | Meaning |
|------|------|---------|
| 0 | `success` | All eval cases passed |
| 1 | `assertion_failed` | One or more metric assertions failed |
| 2 | `config_error` | Invalid eval config, unknown schema version, or test authoring error |
| 3 | `provider_error` | Judge/model provider or runtime error during eval execution |
| 4 | `internal_error` | Internal SDK/runner bug or unexpected failure |

Exit code semantics are mirrored in the `exit_code` field of every `eval-report.schema.json` document.

## Golden fixtures

Fixtures in `fixtures/` validate against `eval-report.schema.json` in CI via `cli/cmd/evaltest_schema_test.go`:

- `all-pass.json` — every case passed
- `metric-failure.json` — deterministic metric failure with evidence
- `provider-error.json` — provider/runtime error (exit 3)
- `config-error.json` — malformed/authoring error (exit 2)
- `multi-turn.json` — multi-turn messages and tool calls

## Contract sync

Python (`agentclash-evals`) and TypeScript (`@agentclash/evals`) SDKs must emit reports that validate against these schemas. Breaking changes require bumping `schema_version` and updating all fixtures.
