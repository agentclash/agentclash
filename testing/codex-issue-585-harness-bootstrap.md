# Test Contract: Issue #585 Harness Bootstrap

## Intent

Make repository setup a first-class Agent Harness stage so arbitrary repo stacks can prepare dependencies before the coding agent runs.

## Expectations

- `execution_config.setup_commands` runs after repository checkout and before the agent command.
- Setup command events are distinct from agent, validator, and infrastructure events.
- Setup command failures stop the harness with a setup-specific failure reason.
- Harness executions record runtime/template metadata and repository setup hints.
- Agent failures remain distinct from setup failures.

## Verification

- `go test ./internal/workflow`
