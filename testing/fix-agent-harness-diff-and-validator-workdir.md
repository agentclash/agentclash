# Fix Agent Harness Diff And Validator Workdir — Test Contract

## Functional Behavior

- Agent Harness artifact diff capture includes newly created untracked files after Codex runs.
- Existing modified files continue to appear in the captured binary diff.
- Command validators with no `working_directory` still run in the execution default workdir.
- Command validators with a relative `working_directory` run relative to the cloned repository workdir.
- Command validators with an absolute `working_directory` continue to run exactly there.
- Command validators run through bash so coding template shell initialization and common developer commands work.

## Unit Tests

- `TestExecuteAgentHarnessExecutionRunsCodexAndRecordsTrace` — verifies the harness runs Codex, records artifacts, and validators still pass.
- New test coverage verifies `git add --intent-to-add --all` happens before `git diff --binary`.
- New test coverage verifies relative validator working directories resolve under the default repository workdir.
- New test coverage verifies absolute validator working directories are preserved.
- Existing workflow tests verify command validators execute via bash.

## Integration / Functional Tests

- Backend workflow tests should pass for the Agent Harness execution activity.
- No database migration is required.

## Smoke Tests

- Run the focused backend workflow tests from `backend/`.
- Optionally start a new Agent Harness execution with validators pointed at module directories and confirm it reaches scoring without the root-module failure.

## E2E Tests

N/A — this change is backend workflow plumbing and covered by activity-level tests.

## Manual / cURL Tests

N/A — no HTTP API surface changes.
