# codex/add-openclaw-agent-harness — Test Contract

## Functional Behavior
- Creating an Agent Harness may specify `harness_kind: "openclaw_e2b"`.
- OpenClaw harnesses default to an OpenClaw-ready E2B template when `codex_template` is omitted.
- Hosted execution loads the configured workspace API key secret, exposes provider-compatible OpenClaw environment variables, performs a headless local OpenClaw setup against the sandbox workspace, runs one embedded `openclaw agent` turn inside the cloned repo, streams runner output events, captures git artifacts, and runs the existing validators/LLM judge plumbing exactly like the Codex harness path.
- Existing Codex harness behavior remains backward compatible.

## Unit Tests
- API validation accepts `openclaw_e2b`, rejects unknown harness kinds, and persists the default OpenClaw template.
- Repository create params persist `harness_kind`.
- Workflow tests verify the OpenClaw command shape, provider secret mapping, template selection, and live output event type.
- CLI tests verify `--harness-kind openclaw_e2b` and generic API key secret payloads.
- UI tests verify the dialog creates an OpenClaw harness payload from an inferred OpenAI/Anthropic/OpenRouter-compatible secret.

## Integration / Functional Tests
- `go test ./internal/api ./internal/repository ./internal/workflow` from `backend/`.
- `go test ./cmd` from `cli/`.
- Relevant web unit test for the harness creation dialog.

## Smoke Tests
- `go test ./internal/workflow -run TestExecuteAgentHarnessExecutionRunsOpenClaw`.
- `go test ./cmd -run TestAgentHarnessCreateBuildsOpenClaw`.

## E2E Tests
- N/A — hosted E2B execution depends on external sandbox/template credentials and provider API keys. The branch should still make the runner command deterministic and unit-covered.

## Manual / cURL Tests
```bash
curl -X POST "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/agent-harnesses" \
  -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "OpenClaw repo harness",
    "harness_kind": "openclaw_e2b",
    "task_prompt": "Make a small docs PR and run tests.",
    "auth_mode": "api_key_secret",
    "openai_api_key_secret_name": "OPENAI_API_KEY",
    "repository_url": "https://github.com/acme/repo",
    "base_branch": "main"
  }'
# Expected: 201, response includes "harness_kind":"openclaw_e2b" and an OpenClaw template id.
```
