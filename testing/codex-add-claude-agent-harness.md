# codex/add-claude-agent-harness — Test Contract

## Functional Behavior
- Creating an Agent Harness may specify `harness_kind: "claude_e2b"`.
- Claude harnesses default to a Claude-ready E2B template when `codex_template` is omitted.
- Hosted execution loads the configured workspace API key secret, exposes it as `ANTHROPIC_API_KEY`, runs Claude Code in non-interactive print mode inside the cloned repo, streams runner output events, captures git artifacts, and runs the existing validators/LLM judge plumbing exactly like the Codex harness path.
- Existing Codex harness behavior remains backward compatible.

## Unit Tests
- API validation accepts `claude_e2b`, rejects unknown harness kinds, and persists the default Claude template.
- Repository create params persist `harness_kind`.
- Workflow tests verify the Claude command shape, Anthropic secret mapping, template selection, and live output event type.
- CLI tests verify `--harness-kind claude_e2b` and generic API key secret payloads.
- UI tests verify the dialog creates a Claude harness payload from inferred `ANTHROPIC_API_KEY`.

## Integration / Functional Tests
- `go test ./internal/api ./internal/repository ./internal/workflow` from `backend/`.
- `go test ./cmd` from `cli/`.
- Relevant web unit test for the harness creation dialog.

## Smoke Tests
- `go test ./internal/workflow -run TestExecuteAgentHarnessExecutionRunsClaude`.
- `go test ./cmd -run TestAgentHarnessCreateBuildsClaude`.

## E2E Tests
- N/A — hosted E2B execution depends on external sandbox/template credentials and provider API keys. The branch should still make the runner command deterministic and unit-covered.

## Manual / cURL Tests
```bash
curl -X POST "$AGENTCLASH_API_URL/v1/workspaces/$WORKSPACE_ID/agent-harnesses" \
  -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Claude repo harness",
    "harness_kind": "claude_e2b",
    "task_prompt": "Make a small docs PR and run tests.",
    "auth_mode": "api_key_secret",
    "openai_api_key_secret_name": "ANTHROPIC_API_KEY",
    "repository_url": "https://github.com/acme/repo",
    "base_branch": "main"
  }'
# Expected: 201, response includes "harness_kind":"claude_e2b" and a Claude template id.
```
