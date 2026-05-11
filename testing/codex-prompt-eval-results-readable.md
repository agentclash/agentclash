# codex/prompt-eval-results-readable — Test Contract

## Functional Behavior
- `agentclash prompt-eval results <experiment-id>` table mode renders a compact summary table instead of loose key/value lines.
- Dimension scores render as a compact table.
- Main assertion rows remain narrow and readable: result, case, assertion type, score, and assertion key.
- Expected/actual/error details do not stretch the main table; they render below only for failed/error rows.
- `--json` and `--output yaml` remain unchanged.
- Color behavior continues through the existing output package and respects `--no-color`.

## Unit Tests
- Update `TestPromptEvalResultsCommandPrintsReadableTable` to assert compact summary/assertion tables and failure details.
- Existing JSON results tests continue to pass.

## Integration / Functional Tests
- Run `go test ./cmd -run 'TestPromptEvalResultsCommand|TestPromptEvalRunFollow'`.

## Smoke Tests
- Run `go test ./cmd ./internal/output`.
- Run `go build ./...`.

## E2E Tests
N/A — CLI rendering only.

## Manual / cURL Tests
```bash
agentclash prompt-eval results 2e5de156-21de-4aba-aef3-4bc7491f3bac --threshold 1
```

Expected: output is readable in a normal terminal-width window, with a compact assertion table and a separate failure detail block.
