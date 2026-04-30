# codex/agent-harnesses - Test Contract

## Functional Behavior
- Agent Harnesses are a workspace-scoped abstraction distinct from challenge packs.
- Users can create and list harness definitions that describe a long-running coding-agent task without uploading a challenge-pack bundle.
- A Codex/E2B harness records:
  - `name`
  - `description`
  - `task_prompt`
  - `codex_template` defaulting to `codex`
  - `codex_model`
  - `auth_mode`, one of `chatgpt_device`, `api_key_secret`, or `bring_your_own_env`
  - optional `openai_api_key_secret_name`
  - optional `e2b_api_key_secret_name`
  - optional `repository_url`
  - optional `base_branch`
  - `evaluation_config` JSON for future reuse of validators, LLM judges, and scoring dimensions
- API creation rejects empty names, empty task prompts, unsupported auth modes, and API-key auth without an OpenAI secret name.
- API list/get routes enforce workspace authorization.
- CLI exposes `agent-harness list`, `agent-harness create`, and `agent-harness get`.
- Workspace navigation exposes "Agent Harnesses" as its own page, not under challenge-pack authoring.
- The Agent Harnesses UI can list existing harnesses and submit a create form using the new API.

## Unit Tests
- `TestAgentHarnessManager_CreateValidatesRequiredFields` - rejects invalid input and auth-mode/secret mismatches.
- `TestAgentHarnessManager_CreatePersistsHarness` - stores defaults and config fields through the repository contract.
- `TestAgentHarnessRoutes_CreateAndList` - verifies API request/response shape and workspace authorization behavior.
- CLI command tests cover request body construction for create and table/json rendering for list/get.
- Web tests cover the create dialog payload, auth-mode controls, and list rendering.

## Integration / Functional Tests
- Repository SQL compiles through `sqlc` generated code and Go build.
- Backend route registration compiles and remains compatible with existing route middleware.
- Web TypeScript compiles with the new API types and page route.
- Existing challenge-pack routes and run creation are unchanged by the new abstraction.

## Smoke Tests
- From `backend/`: `go test ./internal/api ./internal/domain ./internal/repository`.
- From `cli/`: `go test ./cmd`.
- From `web/`: `npm test -- --run agent-harnesses` or the closest focused Vitest target available.
- From repo root where feasible: `go test ./...` in changed Go modules.

## E2E Tests
- N/A for this PR: the execution worker that provisions an E2B sandbox and runs `codex exec` is intentionally left behind a persisted config boundary. The harness definition stores enough Codex/E2B config for that worker to be added next without challenge-pack authoring.

## Manual / cURL Tests
```bash
curl -X POST http://localhost:8080/v1/workspaces/$WORKSPACE_ID/agent-harnesses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Codex long-running repo task",
    "description": "Checks whether Codex can autonomously complete a multi-step change.",
    "task_prompt": "Clone the repository, implement the requested feature, run tests, and summarize the diff.",
    "auth_mode": "api_key_secret",
    "openai_api_key_secret_name": "OPENAI_API_KEY",
    "e2b_api_key_secret_name": "E2B_API_KEY",
    "repository_url": "https://github.com/example/repo",
    "base_branch": "main",
    "evaluation_config": {
      "validators": [{"type": "command", "command": "go test ./..."}],
      "llm_judges": [{"key": "autonomy", "rubric": "Did the agent complete the task without manual intervention?"}]
    }
  }'
# Expected: 201 with an agent harness object, auth_mode api_key_secret, codex_template codex.

curl http://localhost:8080/v1/workspaces/$WORKSPACE_ID/agent-harnesses \
  -H "Authorization: Bearer $TOKEN"
# Expected: 200 with {"items":[...]} including the created harness.
```
