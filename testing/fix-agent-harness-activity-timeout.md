# Fix Agent Harness Activity Timeout — Test Contract

## Functional Behavior

- Agent Harness executions must not use the shared 5 second bookkeeping activity timeout for the long-running sandbox/Codex/evaluation activity.
- The execution activity timeout must come from `execution_config.timeout_seconds` when configured.
- When no harness timeout is configured, the execution activity must use the existing Agent Harness default of 30 minutes.
- Short status-transition activities should keep the existing default activity options.
- The API-observed failure mode, `workflow.execute_agent_harness_execution` failing with `activity StartToClose timeout` a few seconds after `codex.exec.started`, should not happen for ordinary long-running Codex work.

## Unit Tests

- Add workflow-level coverage proving `workflow.execute_agent_harness_execution` receives a StartToClose timeout derived from `execution_config.timeout_seconds`.
- Keep `TestAgentHarnessTimeoutDefaults` passing for default and explicit timeout parsing.
- Existing Agent Harness activity tests must continue to pass.

## Integration / Functional Tests

- `go test ./internal/workflow -run AgentHarness`
- `go test ./internal/api -run AgentHarness`

## Smoke Tests

- `go test ./internal/workflow`

## E2E Tests

- N/A — this change adjusts Temporal activity options and is covered by workflow/unit tests.

## Manual / cURL Tests

- Confirm via hosted API that the affected workspace has `PROVIDER_OPENAI_API_KEY` configured:

```bash
curl -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
  https://api.agentclash.dev/v1/workspaces/511e2d3e-9076-4db3-b9f2-5ef54ab591d5/secrets
# Expected: metadata includes PROVIDER_OPENAI_API_KEY
```

- After deployment, start the harness again and expect it to remain running beyond 5 seconds instead of failing with `activity StartToClose timeout`.
