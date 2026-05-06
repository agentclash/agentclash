# codex/prompt-eval-results-polish — Test Contract

## Functional Behavior
- `agentclash prompt-eval results <experiment-id>` in table mode prints a readable verdict summary before assertion rows.
- Passing assertions render green, failing assertions render red, evaluator/runtime errors render red or yellow as appropriate through the existing output color system.
- Results output includes gate verdict, pass rate, assertion pass/fail counts, execution errors, threshold, and dimension scores when present.
- Assertion rows include result, case key, assertion type/key, score, expected value, actual value, and error/reason.
- `--json` and `--output yaml` remain unchanged structured envelopes.
- `--no-color` / test color disabling still removes ANSI colors through the existing output package.

## Unit Tests
- `TestPromptEvalResultsCommandPrintsReadableTable` verifies table output includes verdict summary, pass/fail labels, expected/actual context, and dimension rows.
- Existing prompt-eval JSON tests continue to pass unchanged.
- Existing output color tests continue to pass.

## Integration / Functional Tests
- Run `go test ./cmd -run 'TestPromptEvalResultsCommand|TestPromptEvalRunFollow'`.

## Smoke Tests
- Run `go test ./cmd ./internal/output`.
- Run `go build ./...`.

## E2E Tests
N/A — this is CLI rendering only; live backend behavior is unchanged.

## Manual / cURL Tests
```bash
agentclash prompt-eval results 2e5de156-21de-4aba-aef3-4bc7491f3bac --threshold 1
```

Expected: human table output shows a failing gate, green PASS rows, a red FAIL row, and summary/dimension context.
