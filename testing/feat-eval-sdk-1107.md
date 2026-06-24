# feat/eval-sdk-1107 — Test Contract

## Functional Behavior

- Python package at `sdk/python/agentclash_eval/` with typed public API.
- `assert_agent(...)` raises AssertionError with concise failure summary.
- `evaluate(...)` returns EvalReport matching schema from #1106.
- Accepts strings, dicts, and AgentEvalResult.
- Zero telemetry; no pytest plugin on import.
- README shows complete local eval in under 30 lines.

## Unit Tests

- `test_assert_agent_passes_with_contains`
- `test_assert_agent_raises_on_failure`
- `test_evaluate_accepts_dict_result`
- `test_evaluate_accepts_agent_eval_result`
- `test_output_schema_rejects_invalid_json`
- `test_no_telemetry_attributes`
- `test_report_matches_schema_when_jsonschema_installed`

## Integration / Functional Tests

- Report dict validates against eval-report JSON schema when jsonschema installed.

## Smoke Tests

- `cd sdk/python/agentclash_eval && pip install -e ".[dev]" && pytest`

## E2E Tests

- N/A — CLI runner comes in #1110.

## Manual Tests

- Run README quick-start example with pytest.
