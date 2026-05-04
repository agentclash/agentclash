# codex/issue-456-regression-flywheel-skill — Test Contract

## Functional Behavior
- Replace the regression flywheel stub with source-aligned guidance for failure inspection, regression suite creation/update, failure promotion, case editing, duplicate/quality checks, and suite-only verification.
- Use hosted production examples by default with `AGENTCLASH_API_URL="https://api.agentclash.dev"`.
- Document the exact CLI commands and argument shapes:
  - `agentclash run failures <RUN_ID>`
  - `agentclash regression-suite list|get|create|update|cases`
  - `agentclash regression-suite case update <CASE_ID>`
  - `agentclash run promote-failure <RUN_ID> <CHALLENGE_IDENTITY_ID>`
  - `agentclash eval start ... --scope suite_only --suite <SUITE_ID_OR_EXACT_NAME>`
  - `agentclash run create ... --scope suite_only --suite <SUITE_ID>`
- Do not document nonexistent commands such as `regression-suite case create`, `regression-suite case get`, `regression-suite case update` fields beyond title/description/status/severity, or `run promote-failure <RUN_ID> <FAILURE_FINGERPRINT>`.
- Document exact payload fields:
  - suite create: `source_challenge_pack_id`, `name`, optional `description`, optional `default_gate_severity`
  - suite patch: optional `name`, `description`, `status`, `default_gate_severity`
  - promote failure: `suite_id`, `promotion_mode`, `title`, optional `run_agent_id`, `failure_summary`, `status`, `severity`, `validator_overrides`, `metadata`
  - case patch: optional `title`, `description`, `status`, `severity`
- Explain promotion requirements: active suite, matching source challenge pack, promotable failure item, available promotion mode, challenge identity may be ambiguous across agents unless `--run-agent` is passed.
- Explain duplicate/quality checks: promotion is idempotent by suite, run agent, and challenge identity; cross-run or cross-suite cluster duplicates still require manual review using failure fingerprints/clusters and existing suite cases.

## Unit Tests
- `web/src/lib/docs.test.ts` should assert the regression flywheel skill page exists and includes source-backed commands, payload fields, status enums, promotion modes, duplicate caveats, suite-only verification, and related skills.

## Integration / Functional Tests
- `npm test -- src/lib/docs.test.ts` from `web/` must pass.
- `go test ./cmd -run 'TestRegression|TestRunFailures|TestRunPromoteFailure|TestContractAlignment'` from `cli/` must pass.
- `go test ./internal/api -run 'Test.*Regression|Test.*Failure'` from `backend/` must pass when feasible.

## Smoke Tests
- `git diff --check` must pass.

## E2E Tests
- N/A — documentation-only skill update; no hosted API mutation is required.

## Manual / cURL Tests
- Read the final `SKILL.md` and verify every command, flag, field, state, duplicate behavior, failure mode, and status enum claim is traceable to:
  - `cli/cmd/regression_suite.go`
  - `cli/cmd/run.go`
  - `cli/cmd/run_create_helpers.go`
  - `cli/cmd/eval.go`
  - `backend/internal/api/regression_suites.go`
  - `backend/internal/api/failure_reviews.go`
  - `backend/internal/domain/regression.go`
  - `backend/internal/repository/regression.go`
  - `backend/internal/api/run_reads.go`
  - `web/src/lib/api/types.ts`
