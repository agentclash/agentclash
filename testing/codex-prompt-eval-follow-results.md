# Codex Prompt Eval Follow Results - Test Contract

Issue: #591
Branch: `codex/prompt-eval-follow-results`

## Functional Behavior

- `agentclash prompt-eval run [path] --follow` launches experiments and waits for them to finish.
- `agentclash prompt-eval results <experiment-id>` fetches persisted playground experiment results and prints the same result envelope/table shape used by `run --follow`.
- `--poll-interval` and `--timeout` control follow polling.
- Follow treats terminal experiment status plus stable results as complete.
- Aggregation reports completed cases, execution errors, assertion passes/fails, assertion pass rate, dimension scores, threshold verdict, and telemetry fields.
- Exit codes are `0` for pass, `3` for threshold/assertion gate failure, `4` for post-launch provider/execution/timeout/non-auth failures, `5` for config/auth/workspace/validation errors including auth failures discovered while polling, and `1` for unexpected internal errors.
- Structured output remains parseable for non-zero exits.

## Unit Tests

- `TestPromptEvalRunFollowPassesGate` - completed experiment with passing validator exits 0 and prints result rows.
- `TestPromptEvalRunFollowFailsAssertionGate` - failed assertion exits 3 with parseable JSON.
- `TestPromptEvalRunFollowReportsExecutionError` - failed provider/test case after launch exits 4.
- `TestPromptEvalRunFollowTimesOutWithPartialResults` - timeout exits 4 and includes partial results.
- `TestPromptEvalRunFollowAuthFailureDuringPoll` - 401 during poll exits 5 before silently continuing.
- `TestPromptEvalResultsCommandPrintsStableEnvelope` - direct results command prints schemaVersion, summary, rows, thresholds, and telemetry fields.

## Integration / Functional Tests

- Run focused Go tests in `cli/cmd`.
- Run `go test -short -race -count=1 ./...` from `cli/`.

## Smoke Tests

- Fake API tests cover follow/result behavior. Hosted smoke is deferred until a stable workspace fixture exists.

## E2E Tests

N/A - GitHub Action integration follows in #592.

## Manual / cURL Tests

N/A - fake API tests pin the polling and result API contract.
