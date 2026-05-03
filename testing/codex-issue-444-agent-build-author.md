# Codex Issue 444 Agent Build Author - Test Contract

## Functional Behavior
- The `agentclash-agent-build-author` skill explains how to create and edit AgentClash agent builds and build versions from the current CLI/API contract.
- The skill documents exact command names and flags for `agentclash build list|get|create`, `agentclash build version create|get|update|validate|ready`, and `--spec-file`.
- The skill documents source-backed agent build fields: `name` and optional `description`.
- The skill documents source-backed build version spec fields: `agent_kind`, `interface_spec`, `policy_spec`, `reasoning_spec`, `memory_spec`, `workflow_spec`, `guardrail_spec`, `model_spec`, `output_schema`, `trace_contract`, `publication_spec`, `tools`, and `knowledge_sources`.
- The skill states that `policy_spec.instructions` is required for validation/readiness and that ready versions are immutable/deployable.
- The skill uses `https://api.agentclash.dev` for hosted production examples.
- The skill links related skills in dependency order, including CLI setup, runtime resources setup, and deployment setup.

## Unit Tests
- `web/src/lib/docs.test.ts` should assert that the generated agent build author skill page contains key source-backed commands and fields.

## Integration / Functional Tests
- The existing docs generator should continue to expose the nested agent-build skill page, `/docs-md/...` paths, `/llms.txt`, and `/llms-full.txt`.
- No docs navigation or category changes should be required because the folder already exists in the agent-build-skills category.

## Smoke Tests
- `cd web && npm test -- docs.test.ts`
- `cd web && npm run lint`

## E2E Tests
N/A - this is a documentation/skill content change with coverage through the docs generator tests.

## Manual / cURL Tests
- Review `web/content/agent-skills/agent-build-skills/agentclash-agent-build-author/SKILL.md` and confirm each command/field maps to `cli/cmd/build.go`, `backend/internal/api/agent_builds.go`, or `web/src/lib/api/types.ts`.
