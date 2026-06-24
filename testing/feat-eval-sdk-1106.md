# feat/eval-sdk-1106 — Test Contract

## Functional Behavior

- Versioned JSON schemas exist at `schemas/evaltest/agent-result.schema.json` and `schemas/evaltest/eval-report.schema.json`.
- Report schema includes evidence for debugging failures without hosted AgentClash.
- Report schema includes CI fields: pass/fail/skipped/errored counts, metric failures, duration, cost.
- Report schema supports single-turn and multi-turn cases, tool calls, and retrieval context.
- Report schema includes `schema_version`.
- Exit codes 0–4 documented in `schemas/evaltest/README.md` and `cli/cmd/exit_codes.go`.
- Golden fixtures exist for: all pass, metric failure, provider error, malformed config, multi-turn.

## Unit Tests

- `TestEvalReportSchemaAcceptsFixtures` — all golden fixtures validate against eval-report schema.
- `TestEvalReportSchemaRejectsUnknownVersion` — unknown schema_version rejected.
- `TestAgentResultSchemaAcceptsMultiTurnFixture` — agent-result schema accepts multi-turn shape.
- `TestEvaltestExitCodesDocumented` — exit codes registered in documentedExitCodes for evaltest run.

## Integration / Functional Tests

- N/A — schema-only issue; CLI runner integration comes in #1110.

## Smoke Tests

- `cd cli && go test -short -race -count=1 ./cmd -run TestEvalReport`
- `cd cli && go test -short -race -count=1 ./cmd -run TestEvaltestExitCodes`

## E2E Tests

- N/A — no runner yet.

## Manual / cURL Tests

- Reviewer validates fixtures with: `cd cli && go test -v ./cmd -run TestEvalReportSchemaAcceptsFixtures`
