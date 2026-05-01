# codex/fix-agent-harness-e2b-runtime - Test Contract

## Functional Behavior
- Agent Harness execution must run Codex in E2B with the workspace OpenAI secret available to every sandbox command, not only sandbox creation.
- Codex must run from the cloned repository working directory using the Codex-supported `-C <repo>` form.
- Execution failures must leave enough event detail to diagnose missing sandbox config, auth problems, clone failures, Codex failures, and validator failures.
- The scoring phase must execute command validators from `evaluation_config.validators` and persist pass/fail events.
- A failed required validator must fail the harness execution instead of marking it completed.
- The UI/CLI should be able to show a ranking-ready score summary from events without requiring challenge packs.

## Unit Tests
- `TestExecuteAgentHarnessExecutionRunsCodexAndRecordsTrace` verifies env forwarding, Codex command shape, output events, and validator events.
- Add workflow tests for command validator failure marking execution failed.
- Add sandbox tests proving E2B session default env is merged into per-command env.
- API tests continue to verify execution lists include events.

## Integration / Functional Tests
- Backend Agent Harness API tests pass.
- Backend workflow Agent Harness tests pass.
- Backend sandbox tests pass.
- Web Agent Harness tests pass.

## Smoke Tests
- From `backend/`: `go test ./internal/api -run AgentHarness`
- From `backend/`: `go test ./internal/workflow -run AgentHarness`
- From `backend/`: `go test ./internal/sandbox/...`
- From `web/`: `npm test -- --run agent-harnesses`
- From `web/`: `npx tsc --noEmit`

## E2E Tests
- N/A - no live E2B credentials are available in this environment. Manual smoke below covers the deployed path.

## Manual / cURL Tests
- Configure worker with `SANDBOX_PROVIDER=e2b`, `E2B_API_KEY`, `E2B_TEMPLATE_ID=codex`, and a stable `AGENTCLASH_SECRETS_MASTER_KEY`.
- Add workspace secret `OPENAI_API_KEY`.
- Create an Agent Harness for a public GitHub repository and run it.
- Confirm events include `sandbox.created`, `repository.clone.*`, `codex.exec.output`, `validator.command.*`, and `scoring.completed`.
- Confirm failed validators mark the execution failed with a useful `status_reason`.
