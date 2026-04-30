# codex/simplify-agent-harness-ui - Test Contract

## Functional Behavior
- Agent Harness creation should be task-first, not a raw backend config form.
- The create dialog should not show OpenAI Secret, Description, E2B Template, Codex Model, or Evaluation Config fields.
- The visible create flow should collect only the essentials currently needed by users:
  - repository URL
  - base branch
  - task prompt
- The harness name should be generated from the repository URL when possible, otherwise from the task prompt.
- The OpenAI API key secret should be automatically selected from existing workspace secrets:
  - prefer `OPENAI_API_KEY`
  - otherwise use the first secret whose key contains both `OPENAI` and `KEY`
  - otherwise block submit and direct the user to add `OPENAI_API_KEY` under Secrets
- Hidden backend defaults should still be sent:
  - `auth_mode: "api_key_secret"`
  - `openai_api_key_secret_name`
  - `codex_template: "codex"`
  - default evaluation config with command validator and LLM judge

## Unit Tests
- `CreateAgentHarnessDialog` fetches workspace secrets and posts a valid harness payload using the inferred OpenAI secret.
- `CreateAgentHarnessDialog` disables creation when no OpenAI secret is available.
- `CreateAgentHarnessDialog` tests should not depend on hidden raw config fields.

## Integration / Functional Tests
- Web TypeScript compiles with the simplified dialog and existing API types.
- Existing Agent Harness list/run UI still compiles.

## Smoke Tests
- From `web/`: `npm test -- --run agent-harnesses`
- From `web/`: `npx tsc --noEmit`

## E2E Tests
- N/A - this is a UI simplification for the existing create flow.

## Manual / cURL Tests
- Open Agent Harnesses in the workspace UI.
- Click New Harness.
- Confirm the form shows repository, branch, and task fields only.
- Confirm creation uses the workspace `OPENAI_API_KEY` secret automatically.
