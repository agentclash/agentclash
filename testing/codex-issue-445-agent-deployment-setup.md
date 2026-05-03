# Codex Issue 445 Agent Deployment Setup - Test Contract

## Functional Behavior
- The `agentclash-agent-deployment-setup` skill explains how to create, select, and verify AgentClash deployments from the current CLI/API contract.
- The skill documents exact commands and aliases for `agentclash deployment list`, `agentclash deployment create`, `agentclash deploy ...`, and `--from-file`.
- The skill documents source-backed deployment request fields: `name`, `agent_build_id`, `build_version_id`, `runtime_profile_id`, optional `provider_account_id`, optional `model_alias_id`, optional `model`, and optional `deployment_config`.
- The skill states that the build version must be `ready` before deployment creation.
- The skill states that the backend requires `provider_account_id` and either `model_alias_id` or `model`; CLI flag creation has no `--model` flag, so raw model auto-alias use must go through `--from-file`.
- The skill documents run compatibility checks: deployments must be active, must have snapshots, and run creation uses `agent_deployment_ids` via `agentclash run create --deployments`.
- The skill uses `https://api.agentclash.dev` for hosted production examples.
- The skill links related skills in dependency order: CLI setup, runtime resources setup, agent build author, eval runner.

## Unit Tests
- `web/src/lib/docs.test.ts` should assert that the generated deployment setup skill page contains key source-backed commands, fields, readiness rules, and run compatibility language.

## Integration / Functional Tests
- The existing docs generator should continue to expose the nested agent-build skill page, `/docs-md/...` paths, `/llms.txt`, and `/llms-full.txt`.
- No docs navigation or category changes should be required because the folder already exists in the agent-build-skills category.

## Smoke Tests
- `cd web && npm test -- docs.test.ts`
- `cd web && npm run lint`

## E2E Tests
N/A - this is a documentation/skill content change with coverage through the docs generator tests.

## Manual / cURL Tests
- Review `web/content/agent-skills/agent-build-skills/agentclash-agent-deployment-setup/SKILL.md` and confirm each command/field maps to `cli/cmd/deployment.go`, `backend/internal/api/agent_builds.go`, `backend/internal/api/run_service.go`, or `web/src/lib/api/types.ts`.
